package web

import (
	"database/sql"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Busnes-app/kydns-server/internal/store"
)

// upgradedStore is a store whose one-time DNSSEC notice is pending, as the
// upgrade migration leaves it.
func upgradedStore(t *testing.T) func(*Options) {
	return func(o *Options) {
		path := filepath.Join(t.TempDir(), "upgraded.db")
		st, err := store.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { st.Close() })
		if err := st.PutSettings(store.Settings{PrivateDomain: "home.arpa"}); err != nil {
			t.Fatal(err)
		}
		db, err := sql.Open("sqlite", path)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		if _, err := db.Exec(`UPDATE settings SET dnssec_notice = 1`); err != nil {
			t.Fatal(err)
		}
		o.Store = st
	}
}

func notice(t *testing.T, h http.Handler, c *http.Cookie, action, csrf string) *http.Response {
	t.Helper()
	rec := postForm(t, h, PathDNSSECNotice, url.Values{"action": {action}, "csrf_token": {csrf}}, c)
	return rec.Result()
}

func TestDNSSECNoticeEnable(t *testing.T) {
	h, srv, c, csrf := loggedIn(t, upgradedStore(t))
	if !strings.Contains(page(t, h, "/", c), "KyDNS can now validate DNSSEC itself") {
		t.Fatal("the upgrade notice is not on the dashboard")
	}
	if r := notice(t, h, c, "enable", csrf); r.StatusCode != http.StatusSeeOther {
		t.Fatalf("status %d", r.StatusCode)
	}
	if cur, _ := srv.o.Settings.Get(); !cur.DNSSECValidate {
		t.Fatal("Turn it on did not turn it on")
	}
	if on, _ := srv.o.Store.DNSSECNotice(); on {
		t.Fatal("the notice is still pending")
	}
	if strings.Contains(page(t, h, "/", c), "KyDNS can now validate DNSSEC itself") {
		t.Fatal("the notice is still shown")
	}
}

func TestDNSSECNoticeDismissLeavesTheSetting(t *testing.T) {
	h, srv, c, csrf := loggedIn(t, upgradedStore(t))
	if r := notice(t, h, c, "dismiss", csrf); r.StatusCode != http.StatusSeeOther {
		t.Fatalf("status %d", r.StatusCode)
	}
	if cur, _ := srv.o.Settings.Get(); cur.DNSSECValidate {
		t.Fatal("Not now turned validation on")
	}
	if strings.Contains(page(t, h, "/", c), "KyDNS can now validate DNSSEC itself") {
		t.Fatal("a dismissed notice came back")
	}
}

func TestDNSSECNoticeRefusesBadRequests(t *testing.T) {
	h, srv, c, csrf := loggedIn(t, upgradedStore(t))
	if r := notice(t, h, c, "enable", "wrong"); r.StatusCode != http.StatusForbidden {
		t.Fatalf("wrong CSRF: status %d", r.StatusCode)
	}
	if r := notice(t, h, c, "everything", csrf); r.StatusCode != http.StatusBadRequest {
		t.Fatalf("unknown action: status %d", r.StatusCode)
	}
	if r := notice(t, h, nil, "enable", csrf); r.StatusCode != http.StatusSeeOther || r.Header.Get("Location") != "/login" {
		t.Fatalf("anonymous: status %d to %q", r.StatusCode, r.Header.Get("Location"))
	}
	if cur, _ := srv.o.Settings.Get(); cur.DNSSECValidate {
		t.Fatal("a refused request changed the setting")
	}
	if on, _ := srv.o.Store.DNSSECNotice(); !on {
		t.Fatal("a refused request retired the notice")
	}
}

func TestServerSettingsDNSSECCheckbox(t *testing.T) {
	h, srv, c, csrf := loggedIn(t)
	form := validForm(csrf)
	form.Set("dnssec_validate", "on")
	if rec := postForm(t, h, "/settings/server", form, c); rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if cur, _ := srv.o.Settings.Get(); !cur.DNSSECValidate {
		t.Fatal("the box did not save")
	}
	if !strings.Contains(page(t, h, "/settings/server", c), `name="dnssec_validate" checked`) {
		t.Fatal("the saved box is not shown checked")
	}
	if rec := postForm(t, h, "/settings/server", validForm(csrf), c); rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d", rec.Code)
	}
	if cur, _ := srv.o.Settings.Get(); cur.DNSSECValidate {
		t.Fatal("a cleared box stayed on")
	}
}

func TestDNSSECClockBanner(t *testing.T) {
	skip := true
	h, srv, c, csrf := loggedIn(t, func(o *Options) {
		o.DNSSECClock = func() (bool, string) { return skip, "the kernel reports the clock is not synchronised (NTP)" }
	})
	if strings.Contains(page(t, h, "/", c), "signature dates are not being checked") {
		t.Fatal("banner shown while validation is off")
	}
	form := validForm(csrf)
	form.Set("dnssec_validate", "on")
	postForm(t, h, "/settings/server", form, c)
	if cur, _ := srv.o.Settings.Get(); !cur.DNSSECValidate {
		t.Fatal("setup: validation not on")
	}
	if !strings.Contains(page(t, h, "/", c), "signature dates are not being checked") {
		t.Fatal("no banner while time checks are skipped")
	}
	skip = false
	if strings.Contains(page(t, h, "/", c), "signature dates are not being checked") {
		t.Fatal("banner outlived the condition")
	}
}
