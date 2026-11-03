package adminapi

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/Busnes-app/kydns-server/internal/registry"
	"github.com/Busnes-app/kydns-server/internal/settings"
	"github.com/Busnes-app/kydns-server/internal/store"
)

// Moving a box: an export from a node whose private domain is lan carries
// nas.lan. records, and a fresh install still has home.arpa. Its settings name
// the domain, so they must land before the records are checked against it.
func TestImportBringsItsOwnPrivateDomain(t *testing.T) {
	for _, mode := range []string{"replace", "merge"} {
		t.Run(mode, func(t *testing.T) {
			s, err := store.Open(filepath.Join(t.TempDir(), "kydns.db"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { s.Close() })
			if err := s.PutSettings(validStoredSettings()); err != nil {
				t.Fatal(err)
			}
			reg := registry.New(s, "home.arpa.", func() error { return nil })
			tok, _ := reg.CreateToken("test")
			h := settings.NewHolder(func() (store.Settings, error) { v, _, err := s.Settings(); return v, err })
			if err := h.Rebuild(); err != nil {
				t.Fatal(err)
			}
			// As serve.go wires it: applying settings moves the registry's zone.
			svc := settings.NewService(s, h, func(snap *settings.Snapshot) { reg.SetZone(snap.Raw.PrivateDomain) }, notAReplica)
			srv := &testSrv{h: NewAPI(reg, nil, nil).WithSettings(svc).Handler(), tok: tok}

			dto := toSettingsDTO(validStoredSettings())
			dto.PrivateDomain = "lan"
			doc, _ := json.Marshal(map[string]any{
				"records":  []map[string]string{{"name": "nas.lan.", "type": "A", "value": "192.168.1.5"}},
				"settings": dto,
			})
			if rec := srv.do(t, "POST", "/api/v1/import?mode="+mode, string(doc)); rec.Code != http.StatusOK {
				t.Fatalf("import: %d %s", rec.Code, rec.Body)
			}
			if reg.Zone() != "lan." {
				t.Errorf("zone = %q, want lan.", reg.Zone())
			}
			recs, _ := reg.Records()
			if len(recs) != 1 || recs[0].Name != "nas.lan." {
				t.Errorf("records = %+v", recs)
			}
		})
	}
}

// A document whose records do not fit its own domain is refused before its
// settings change anything.
func TestImportReplaceChecksTheRegistryBeforeTheDomainMoves(t *testing.T) {
	srv, svc := testAPIWithSettings(t)
	dto := toSettingsDTO(validStoredSettings())
	dto.PrivateDomain = "lan"
	doc, _ := json.Marshal(map[string]any{
		"records":  []map[string]string{{"name": "nas.elsewhere.", "type": "A", "value": "192.168.1.5"}},
		"settings": dto,
	})
	if rec := srv.do(t, "POST", "/api/v1/import?mode=replace", string(doc)); rec.Code < 400 {
		t.Fatalf("import: %d, want refused", rec.Code)
	}
	if cur, _ := svc.Get(); cur.PrivateDomain != "home.arpa" {
		t.Errorf("private domain = %q after a refused import", cur.PrivateDomain)
	}
}

// Sol's case: a constraint only the store enforces (two services with one
// name) must not leave the domain moved behind a refused replace.
func TestImportReplaceRefusedByTheStoreLeavesTheDomain(t *testing.T) {
	srv, svc := testAPIWithSettings(t)
	dto := toSettingsDTO(validStoredSettings())
	dto.PrivateDomain = "lan"
	doc, _ := json.Marshal(map[string]any{
		"services": []map[string]any{
			{"name": "nas", "addresses": []map[string]string{{"address": "192.168.1.5"}}},
			{"name": "nas", "addresses": []map[string]string{{"address": "192.168.1.6"}}},
		},
		"settings": dto,
	})
	if rec := srv.do(t, "POST", "/api/v1/import?mode=replace", string(doc)); rec.Code < 400 {
		t.Fatalf("import: %d, want refused", rec.Code)
	}
	if cur, _ := svc.Get(); cur.PrivateDomain != "home.arpa" {
		t.Errorf("private domain = %q after a refused import", cur.PrivateDomain)
	}
}
