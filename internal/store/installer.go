package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/Busnes-app/ky-primitives/applysetup"
)

const installerConfigKey = "installer.config"

// InstallerConfiguration contains protected target references, never credentials.
func (s *Store) InstallerConfiguration() (applysetup.Common, bool, error) {
	return installerConfiguration(s.db)
}

func installerConfiguration(q rowQuerier) (applysetup.Common, bool, error) {
	raw, err := localSetting(q, installerConfigKey)
	if errors.Is(err, ErrNotFound) {
		return applysetup.Common{}, false, nil
	}
	var b applysetup.Common
	if err != nil {
		return b, false, err
	}
	if err = applysetup.DecodeRaw([]byte(raw), &b); err != nil {
		return b, false, err
	}
	if err = validateInstaller(b); err != nil {
		return b, false, err
	}
	return b, true, nil
}

func validateInstaller(b applysetup.Common) error {
	if err := b.Validate("kydns", "/auth/sso/callback"); err != nil {
		return err
	}
	if b.SSO == nil || len(b.Admins) == 0 {
		return errors.New("KyDNS setup requires KyIdentity SSO and explicit administrator subjects")
	}
	if len(b.Everyday) != 0 {
		return errors.New("KyDNS has no ordinary user accounts; everyday identities must not be assigned this administrator-only client")
	}
	if b.Backup != nil {
		return errors.New("KyDNS installer backup configuration is not implemented yet")
	}
	if b.SSO.SCIMTokenFile != "" || b.SSO.DirectorySecretFile == "" || strings.TrimRight(b.SSO.IssuerURL, "/") != b.SSO.IssuerURL {
		return errors.New("KyDNS requires a canonical issuer and protected signed-directory secret")
	}
	for _, id := range b.Admins {
		if id.Issuer != b.SSO.IssuerURL {
			return errors.New("administrator issuer differs from configured KyIdentity")
		}
	}
	return nil
}

// ApplyInstaller pins new explicit subjects independently of the retained local
// recovery administrator; no password, DNS record, service or legacy token changes.
func (s *Store) ApplyInstaller(ctx context.Context, b applysetup.Common, check bool) (applysetup.Report, error) {
	report := applysetup.NewReport(b)
	if err := validateInstaller(b); err != nil {
		return report, err
	}
	for _, path := range []string{b.SSO.ClientSecretFile, b.SSO.DirectorySecretFile} {
		if secret, err := applysetup.Secret(path); err != nil {
			return report, err
		} else if path == b.SSO.DirectorySecretFile && len(secret) < 16 {
			return report, errors.New("directory secret must have at least 16 bytes")
		}
	}
	raw, _ := json.Marshal(b)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return report, err
	}
	defer tx.Rollback()
	var current string
	err = tx.QueryRowContext(ctx, "SELECT value FROM local_settings WHERE key=?", installerConfigKey).Scan(&current)
	switch {
	case err == nil:
		if current != string(raw) {
			return report, errors.New("installer configuration ownership or values conflict")
		}
		report.Present = append(report.Present, applysetup.Object{Kind: "configuration", Name: "sso"})
		for _, id := range b.Admins {
			var authority string
			e := tx.QueryRowContext(ctx, "SELECT value FROM local_settings WHERE key=?", installerDirectoryKey(id.Issuer, id.Subject)).Scan(&authority)
			if e == nil {
				var state InstallerDirectoryState
				if json.Unmarshal([]byte(authority), &state) != nil || !state.Allowed {
					return report, errors.New("administrator directory authority drifted; setup never reactivates it")
				}
			} else if !errors.Is(e, sql.ErrNoRows) {
				return report, e
			}
			report.Present = append(report.Present, applysetup.Object{Kind: "identity", Name: id.Username, ID: id.Subject})
		}
	case errors.Is(err, sql.ErrNoRows):
		if check {
			return report, errors.New("installer configuration is absent")
		}
		var enabled int
		var sub, legacySecret, legacyIssuer, legacyClient string
		if err = tx.QueryRowContext(ctx, "SELECT enabled,issuer_url,client_id,client_secret FROM sso_settings WHERE id=1").Scan(&enabled, &legacyIssuer, &legacyClient, &legacySecret); err != nil {
			return report, err
		}
		err = tx.QueryRowContext(ctx, "SELECT sso_sub FROM admin WHERE id=1").Scan(&sub)
		if errors.Is(err, sql.ErrNoRows) {
			if _, err = tx.ExecContext(ctx, "INSERT INTO admin(id,password_hash) VALUES(1,?)", ""); err != nil {
				return report, err
			}
		} else if err != nil {
			return report, err
		}
		if enabled != 0 || sub != "" || legacySecret != "" || legacyIssuer != "https://auth.urlxl.com" || legacyClient != "kydns" {
			return report, errors.New("installer refuses to adopt existing SSO configuration or a linked recovery account")
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO local_settings(key,value) VALUES(?,?)", installerConfigKey, string(raw)); err != nil {
			return report, err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO audit_events(actor,action,resource,details,ip,outcome) VALUES(?,?,?,?,?,?)", "installer-local", "installer.configuration_created", "kydns", b.SetupID, "", "success"); err != nil {
			return report, err
		}
		report.Created = append(report.Created, applysetup.Object{Kind: "configuration", Name: "sso"})
		for _, id := range b.Admins {
			report.Created = append(report.Created, applysetup.Object{Kind: "identity", Name: id.Username, ID: id.Subject})
		}
	default:
		return report, err
	}
	if check {
		return report, nil
	}
	if err = tx.Commit(); err != nil {
		return report, err
	}
	return report, nil
}

// InstallerAdministrator returns only a configured exact issuer/subject grant.
// The ordinary/local recovery account retains its original independent authority.
func (s *Store) InstallerAdministrator(issuer, subject string) (managed, allowed bool, err error) {
	return installerAdministrator(s.db, issuer, subject)
}

func installerAdministrator(q rowQuerier, issuer, subject string) (managed, allowed bool, err error) {
	managed, state, err := installerAuthority(q, issuer, subject)
	return managed, state.Allowed, err
}

// InstallerAuthority is InstallerAdministrator with the retained directory state, whose
// Revision a session records at login and whose RevokedAt fences older sessions.
func (s *Store) InstallerAuthority(issuer, subject string) (bool, InstallerDirectoryState, error) {
	return installerAuthority(s.db, issuer, subject)
}

func installerAuthority(q rowQuerier, issuer, subject string) (bool, InstallerDirectoryState, error) {
	b, managed, err := installerConfiguration(q)
	if err != nil || !managed || issuer != b.SSO.IssuerURL {
		return managed, InstallerDirectoryState{}, err
	}
	for _, id := range b.Admins {
		if id.Subject == subject {
			raw, err := localSetting(q, installerDirectoryKey(issuer, subject))
			if errors.Is(err, ErrNotFound) {
				return true, InstallerDirectoryState{Allowed: true}, nil
			}
			if err != nil {
				return true, InstallerDirectoryState{}, err
			}
			var state InstallerDirectoryState
			if json.Unmarshal([]byte(raw), &state) != nil {
				return true, InstallerDirectoryState{}, errors.New("retained directory authority invalid")
			}
			return true, state, nil
		}
	}
	return true, InstallerDirectoryState{}, nil
}
