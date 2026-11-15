package web

import (
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"testing"

	"github.com/Busnes-app/kydns-server/internal/adminapi"
)

type fakeRDNSS struct {
	running bool
	addr    netip.Addr
	err     error
}

func (f fakeRDNSS) Status() (bool, netip.Addr, error) { return f.running, f.addr, f.err }

func rdnssValues(csrf string) url.Values {
	return url.Values{"csrf_token": {csrf}, "enabled": {"1"}, "interface": {"eth0"}, "address": {"fd00::53"}}
}

func TestRDNSSCardExplainsTheRouterStep(t *testing.T) {
	h, _, c, _ := loggedIn(t)
	body := page(t, h, "/dhcp", c)
	for _, want := range []string{`id="rdnss"`, "Turn off your router", "both resolvers", "Not advertising", `action="/dhcp/rdnss"`} {
		if !strings.Contains(body, want) {
			t.Errorf("the card lacks %q", want)
		}
	}
}

func TestRDNSSSaveApplies(t *testing.T) {
	h, srv, c, csrf := loggedIn(t)
	before, _ := srv.o.Settings.Get()
	rec := postForm(t, h, PathRDNSSSettings, rdnssValues(csrf), c)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("save = %d: %s", rec.Code, rec.Body)
	}
	v, _ := srv.o.Settings.Get()
	if !v.RDNSSEnabled || v.RDNSSInterface != "eth0" || v.RDNSSAddress != "fd00::53" {
		t.Fatalf("saved %+v", v)
	}
	if v.TTL != before.TTL || v.DHCPEnabled != before.DHCPEnabled || v.PrivateDomain != before.PrivateDomain {
		t.Errorf("an RDNSS save moved another setting: %+v", v)
	}
	// Switching off is a save too.
	off := rdnssValues(csrf)
	off.Del("enabled")
	if rec := postForm(t, h, PathRDNSSSettings, off, c); rec.Code != http.StatusSeeOther {
		t.Fatalf("off = %d", rec.Code)
	}
	if v, _ := srv.o.Settings.Get(); v.RDNSSEnabled {
		t.Fatal("still on")
	}
}

func TestRDNSSSaveRefusesBadInputAndKeepsIt(t *testing.T) {
	h, srv, c, csrf := loggedIn(t)
	vals := rdnssValues(csrf)
	vals.Set("address", "fe80::1")
	rec := postForm(t, h, PathRDNSSSettings, vals, c)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "rdnss_address") ||
		!strings.Contains(rec.Body.String(), `value="fe80::1"`) {
		t.Fatalf("= %d:\n%s", rec.Code, rec.Body)
	}
	if v, _ := srv.o.Settings.Get(); v.RDNSSEnabled {
		t.Fatal("a refused save landed")
	}
}

func TestRDNSSRoutesNeedSessionAndCSRF(t *testing.T) {
	h, srv, c, csrf := loggedIn(t)
	for _, path := range []string{PathRDNSSSettings, PathRDNSSSuggest} {
		if rec := postForm(t, h, path, rdnssValues(csrf), nil); rec.Code == http.StatusOK || rec.Code == http.StatusSeeOther && !strings.Contains(rec.Header().Get("Location"), "login") {
			t.Errorf("anonymous POST %s = %d %s", path, rec.Code, rec.Header().Get("Location"))
		}
		bad := rdnssValues("wrong")
		if rec := postForm(t, h, path, bad, c); rec.Code != http.StatusForbidden {
			t.Errorf("POST %s with a wrong CSRF token = %d", path, rec.Code)
		}
	}
	if v, _ := srv.o.Settings.Get(); v.RDNSSEnabled {
		t.Fatal("an unauthenticated save landed")
	}
}

func TestRDNSSSuggestOnTheLoopbackExplainsAndSavesNothing(t *testing.T) {
	h, srv, c, csrf := loggedIn(t)
	rec := postForm(t, h, PathRDNSSSuggest, url.Values{"interface": {"lo"}, "csrf_token": {csrf}}, c)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "loopback") {
		t.Fatalf("= %d:\n%s", rec.Code, rec.Body)
	}
	if v, _ := srv.o.Settings.Get(); v.RDNSSInterface != "" {
		t.Fatal("the wizard saved")
	}
}

func TestRDNSSCardShowsStatus(t *testing.T) {
	h, _, c, _ := loggedIn(t, func(o *Options) {
		o.API = o.API.WithRDNSS(fakeRDNSS{running: true, addr: netip.MustParseAddr("2001:db8::53")})
	})
	body := page(t, h, "/dhcp", c)
	if !strings.Contains(body, "Advertising") || !strings.Contains(body, "2001:db8::53") {
		t.Errorf("no running status:\n%s", body)
	}
	// The default allow_query has no global prefix: the card says so.
	if !strings.Contains(body, "allow_query does not include 2001:db8::/64") {
		t.Errorf("no allow_query warning")
	}
}

func TestReplicaMaySaveTheRDNSSForm(t *testing.T) {
	h, _, srv, _, c, csrf := replicaWeb(t)
	rec := postForm(t, h, PathRDNSSSettings, rdnssValues(csrf), c)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("= %d:\n%s", rec.Code, rec.Body)
	}
	if v, _ := srv.o.Settings.Get(); !v.RDNSSEnabled {
		t.Fatal("not saved")
	}
}

var _ adminapi.RDNSSRunner = fakeRDNSS{}
