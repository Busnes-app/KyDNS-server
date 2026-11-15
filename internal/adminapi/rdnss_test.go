package adminapi

import (
	"encoding/json"
	"net/http"
	"net/netip"
	"strings"
	"testing"
)

func TestRDNSSEndpointsNeedAToken(t *testing.T) {
	h, tok := newAPIWithDHCP(t, nil)
	for _, p := range []string{"/api/v1/rdnss/status", "/api/v1/rdnss/suggest?interface=lo"} {
		if rec := do(t, h, "GET", p, "", ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("anonymous %s = %d", p, rec.Code)
		}
	}
	rec := do(t, h, "GET", "/api/v1/rdnss/status", tok, "")
	var got RDNSSStatus
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &got) != nil || got.Running {
		t.Fatalf("status = %d %s", rec.Code, rec.Body)
	}
	rec = do(t, h, "GET", "/api/v1/rdnss/suggest?interface=lo", tok, "")
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "loopback") {
		t.Fatalf("suggest on lo = %d %s", rec.Code, rec.Body)
	}
	rec = do(t, h, "GET", "/api/v1/rdnss/suggest", tok, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("suggest without an interface = %d", rec.Code)
	}
}

func TestACLWarning(t *testing.T) {
	def := []string{"127.0.0.0/8", "::1/128", "fe80::/10", "fc00::/7"}
	if w := aclWarning(def, netip.MustParseAddr("fd00::53")); w != "" {
		t.Errorf("ULA warned: %s", w)
	}
	if w := aclWarning(def, netip.MustParseAddr("2001:db8:1:2::53")); !strings.Contains(w, "2001:db8:1:2::/64") {
		t.Errorf("global not warned: %q", w)
	}
	if w := aclWarning(append(def, "2001:db8::/32"), netip.MustParseAddr("2001:db8:1:2::53")); w != "" {
		t.Errorf("covered global warned: %s", w)
	}
	// A /128 for the server alone does not cover its clients.
	if w := aclWarning(append(def, "2001:db8:1:2::53/128"), netip.MustParseAddr("2001:db8:1:2::53")); w == "" {
		t.Error("a host route counted as covering the /64")
	}
}

func TestPatchSettingsAppliesRDNSSFields(t *testing.T) {
	srv, svc := testAPIWithSettings(t)
	rec := srv.do(t, "PATCH", "/api/v1/settings", `{"rdnss_enabled":true,"rdnss_interface":"eth0","rdnss_address":"fd00::53"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	v, _ := svc.Get()
	if !v.RDNSSEnabled || v.RDNSSInterface != "eth0" || v.RDNSSAddress != "fd00::53" {
		t.Fatalf("%+v", v)
	}
	rec = srv.do(t, "PATCH", "/api/v1/settings", `{"rdnss_address":"ff02::1"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("multicast accepted: %d %s", rec.Code, rec.Body)
	}
}
