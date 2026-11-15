package store

import "testing"

// The RDNSS columns are node-local: they round-trip, the narrow replica write
// sets them, they move no config version, and a pull keeps the local ones.
func TestRDNSSSettingsAreNodeLocal(t *testing.T) {
	s := open(t)
	if err := s.PutSettings(baseSettings()); err != nil {
		t.Fatal(err)
	}
	before, err := s.ConfigVersion()
	if err != nil {
		t.Fatal(err)
	}
	v := baseSettings()
	v.RDNSSEnabled, v.RDNSSInterface, v.RDNSSAddress = true, "eth0", "fd00::53"
	if err := s.PutDHCPSettings(v); err != nil {
		t.Fatal(err)
	}
	got, _, err := s.Settings()
	if err != nil || !got.RDNSSEnabled || got.RDNSSInterface != "eth0" || got.RDNSSAddress != "fd00::53" {
		t.Fatalf("narrow write: %+v %v", got, err)
	}
	v.RDNSSAddress = "fd00::54"
	if err := s.PutSettings(v); err != nil {
		t.Fatal(err)
	}
	if after, _ := s.ConfigVersion(); after != before {
		t.Fatalf("config version moved %d -> %d", before, after)
	}

	incoming := baseSettings() // the primary's row: RDNSS off
	if err := s.ApplySnapshot(SnapshotInput{Settings: incoming}, nil); err != nil {
		t.Fatal(err)
	}
	got, _, _ = s.Settings()
	if !got.RDNSSEnabled || got.RDNSSInterface != "eth0" || got.RDNSSAddress != "fd00::54" {
		t.Fatalf("a pull replaced the local RDNSS settings: %+v", got)
	}
}
