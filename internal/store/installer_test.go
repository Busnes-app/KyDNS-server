package store

import (
	"context"
	"github.com/Busnes-app/ky-primitives/applysetup"
	"os"
	"path/filepath"
	"testing"
)

func installerBundle(t *testing.T) applysetup.Common {
	t.Helper()
	dir := t.TempDir()
	client := filepath.Join(dir, "client")
	directory := filepath.Join(dir, "directory")
	for _, p := range []string{client, directory} {
		if err := os.WriteFile(p, []byte("fixture-secret-at-least-16"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return applysetup.Common{Schema: applysetup.Schema, Service: "kydns", SetupID: "12345678-1234-1234-1234-123456789abc", SSO: &applysetup.SSO{IssuerURL: "https://identity.example", ClientID: "kydns", ClientSecretFile: client, DirectorySecretFile: directory, RedirectURI: "https://dns.example/auth/sso/callback"}, Admins: []applysetup.Identity{{Issuer: "https://identity.example", Subject: "owner", Username: "administrator"}}}
}
func TestInstallerOwnershipCheckAndDirectoryFence(t *testing.T) {
	s := open(t)
	b := installerBundle(t)
	ctx := context.Background()
	count := func() int {
		var n int
		if err := s.db.QueryRow("SELECT COUNT(*) FROM audit_events").Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if _, err := s.ApplyInstaller(ctx, b, true); err == nil {
		t.Fatal("check accepted absent setup")
	}
	if ok, _ := s.HasAdmin(); ok || count() != 0 {
		t.Fatal("check mutated state")
	}
	r, err := s.ApplyInstaller(ctx, b, false)
	if err != nil || len(r.Created) != 2 {
		t.Fatalf("apply %+v %v", r, err)
	}
	a, err := s.AdminIdentity()
	if err != nil || a.PasswordHash != "" || a.SSOSub != "" {
		t.Fatalf("recovery slot %+v %v", a, err)
	}
	for _, check := range []bool{true, false} {
		r, err = s.ApplyInstaller(ctx, b, check)
		if err != nil || len(r.Created) != 0 || len(r.Present) != 2 || count() != 1 {
			t.Fatalf("resume %+v %v", r, err)
		}
	}
	if _, allowed, _ := s.InstallerAdministrator("https://foreign.example", "owner"); allowed {
		t.Fatal("foreign issuer allowed")
	}
	if _, allowed, _ := s.InstallerAdministrator(b.SSO.IssuerURL, "everyday"); allowed {
		t.Fatal("unassigned allowed")
	}
	off := []byte(`{"id":"owner","active":false,"meta":{"version":"W/\"2\""}}`)
	if err = s.ApplyInstallerDirectory(ctx, b.SSO.IssuerURL, "user.deactivated", off); err != nil {
		t.Fatal(err)
	}
	if err = s.ApplyInstallerDirectory(ctx, b.SSO.IssuerURL, "user.deactivated", off); err != nil || count() != 2 {
		t.Fatalf("duplicate %v audit %d", err, count())
	}
	on := []byte(`{"id":"owner","active":true,"roles":[{"value":"kydns.admin"}],"meta":{"version":"W/\"1\""}}`)
	if err = s.ApplyInstallerDirectory(ctx, b.SSO.IssuerURL, "user.updated", on); err == nil {
		t.Fatal("stale activation accepted")
	}
	for _, check := range []bool{true, false} {
		if _, err = s.ApplyInstaller(ctx, b, check); err == nil {
			t.Fatal("setup reactivated revoked owner")
		}
	}
	if _, allowed, _ := s.InstallerAdministrator(b.SSO.IssuerURL, "owner"); allowed {
		t.Fatal("revoked owner allowed")
	}
}
func TestInstallerPreservesRecoveryAndRefusesForeignConfiguration(t *testing.T) {
	for _, foreign := range []bool{false, true} {
		t.Run(map[bool]string{false: "retained", true: "foreign"}[foreign], func(t *testing.T) {
			s := open(t)
			b := installerBundle(t)
			if err := s.SetAdminPassword("existing-password-hash"); err != nil {
				t.Fatal(err)
			}
			if foreign {
				if err := s.SetSSOSettings(SSOSettings{IssuerURL: "https://foreign.example", ClientID: "different"}); err != nil {
					t.Fatal(err)
				}
			}
			_, err := s.ApplyInstaller(context.Background(), b, false)
			if (err != nil) != foreign {
				t.Fatalf("apply %v", err)
			}
			a, _ := s.AdminIdentity()
			if a.PasswordHash != "existing-password-hash" {
				t.Fatal("changed recovery password")
			}
			if _, managed, _ := s.InstallerConfiguration(); managed == foreign {
				t.Fatal("ownership wrong")
			}
		})
	}
}
