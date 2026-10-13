package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

type InstallerDirectoryState struct {
	Revision uint64 `json:"revision"`
	Digest   string `json:"digest"`
	Allowed  bool   `json:"allowed"`
}

func installerDirectoryKey(issuer, subject string) string {
	sum := sha256.Sum256([]byte(issuer + "\x00" + subject))
	return "installer.directory." + hex.EncodeToString(sum[:])
}

// ApplyInstallerDirectory receives verified signed SCIM desired state. A durable
// revision/digest fence refuses reordered authority and conflicting duplicate events.
func (s *Store) ApplyInstallerDirectory(ctx context.Context, issuer, event string, raw []byte) error {
	var u struct {
		ID     string `json:"id"`
		Active *bool  `json:"active"`
		Roles  []struct {
			Value string `json:"value"`
		} `json:"roles"`
		Meta struct {
			Version string `json:"version"`
		} `json:"meta"`
	}
	if len(raw) > 64<<10 || json.Unmarshal(raw, &u) != nil || u.ID == "" || len(u.ID) > 256 {
		return errors.New("invalid directory identity")
	}
	version := strings.TrimSuffix(strings.TrimPrefix(u.Meta.Version, `W/"`), `"`)
	revision, err := strconv.ParseUint(version, 10, 64)
	if err != nil || revision == 0 || u.Meta.Version != `W/"`+strconv.FormatUint(revision, 10)+`"` {
		return errors.New("invalid directory revision")
	}
	allowed := false
	switch event {
	case "user.created", "user.updated":
		if u.Active == nil {
			return errors.New("directory active state is required")
		}
		for _, r := range u.Roles {
			if r.Value == "kydns.admin" {
				allowed = *u.Active
			}
		}
	case "user.deactivated", "user.deleted":
	default:
		return errors.New("unsupported directory event")
	}
	b, managed, err := s.InstallerConfiguration()
	if err != nil || !managed || b.SSO.IssuerURL != issuer {
		return errors.New("directory issuer not configured")
	}
	found := false
	for _, id := range b.Admins {
		if id.Subject == u.ID {
			found = true
		}
	}
	if !found {
		return errors.New("directory cannot grant an unassigned administrator")
	}
	sum := sha256.Sum256(append([]byte(event+"\x00"), raw...))
	digest := hex.EncodeToString(sum[:])
	key := installerDirectoryKey(issuer, u.ID)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var prior string
	err = tx.QueryRowContext(ctx, "SELECT value FROM local_settings WHERE key=?", key).Scan(&prior)
	if err == nil {
		var state InstallerDirectoryState
		if json.Unmarshal([]byte(prior), &state) != nil {
			return errors.New("retained directory state invalid")
		}
		if state.Revision > revision {
			return errors.New("stale directory event")
		}
		if state.Revision == revision {
			if state.Digest != digest {
				return errors.New("conflicting directory revision")
			}
			return nil
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	value, _ := json.Marshal(InstallerDirectoryState{revision, digest, allowed})
	if _, err = tx.ExecContext(ctx, "INSERT INTO local_settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", key, string(value)); err != nil {
		return err
	}
	if !allowed {
		// Revoked credentials stay revoked after a later explicit reactivation.
		if _, err = tx.ExecContext(ctx, "DELETE FROM tokens WHERE issuer=? AND subject=?", issuer, u.ID); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO audit_events(actor,action,resource,details,ip,outcome) VALUES(?,?,?,?,?,?)", "kyidentity", "installer.directory_applied", key, strconv.FormatUint(revision, 10), "", "success"); err != nil {
		return err
	}
	return tx.Commit()
}
