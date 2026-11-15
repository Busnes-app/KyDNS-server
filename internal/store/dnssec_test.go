package store

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestDNSSECFreshInstallHasNoNotice(t *testing.T) {
	s := open(t)
	v := baseSettings()
	v.DNSSECValidate = true
	if err := s.PutSettings(v); err != nil {
		t.Fatal(err)
	}
	got, _, err := s.Settings()
	if err != nil || !got.DNSSECValidate {
		t.Fatalf("DNSSECValidate = %v, %v", got.DNSSECValidate, err)
	}
	if on, err := s.DNSSECNotice(); err != nil || on {
		t.Fatalf("fresh install notice = %v, %v; want none", on, err)
	}
}

// An upgraded database keeps forwarding upstream verdicts and gets the notice.
func TestDNSSECUpgradeKeepsBehaviourAndNotices(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.PutSettings(baseSettings()); err != nil {
		t.Fatal(err)
	}
	// Wind the database back to before the DNSSEC migration.
	for _, stmt := range []string{
		`ALTER TABLE settings DROP COLUMN dnssec_validate`,
		`ALTER TABLE settings DROP COLUMN dnssec_notice`,
		fmt.Sprintf(`PRAGMA user_version = %d`, len(migrations)-1),
	} {
		if _, err := s.db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	s.Close()

	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, _, err := s.Settings()
	if err != nil || got.DNSSECValidate {
		t.Fatalf("upgrade turned validation on: %v, %v", got.DNSSECValidate, err)
	}
	if on, err := s.DNSSECNotice(); err != nil || !on {
		t.Fatalf("upgrade notice = %v, %v; want pending", on, err)
	}
	if err := s.ClearDNSSECNotice(); err != nil {
		t.Fatal(err)
	}
	if on, _ := s.DNSSECNotice(); on {
		t.Fatal("notice survived ClearDNSSECNotice")
	}
	// Saving settings must not bring it back.
	if err := s.PutSettings(got); err != nil {
		t.Fatal(err)
	}
	if on, _ := s.DNSSECNotice(); on {
		t.Fatal("PutSettings re-armed the notice")
	}
}

func TestPutDHCPSettingsWritesDNSSEC(t *testing.T) {
	s := open(t)
	v := baseSettings()
	if err := s.PutSettings(v); err != nil {
		t.Fatal(err)
	}
	v.DNSSECValidate = true
	if err := s.PutDHCPSettings(v); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := s.Settings(); !got.DNSSECValidate {
		t.Fatal("the replica write path dropped dnssec_validate")
	}
}
