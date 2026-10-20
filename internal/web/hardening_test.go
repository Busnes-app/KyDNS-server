package web

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Busnes-app/kydns-server/internal/auth"
	"github.com/Busnes-app/kydns-server/internal/config"
)

// With an admin present at startup no token is minted, and an empty token
// must not match it.
func TestSetupRefusesWhenNoTokenWasMinted(t *testing.T) {
	h, srv := newWeb(t, func(o *Options) { o.SetupToken = "" })
	rec := postForm(t, h, "/setup", url.Values{
		"token": {""}, "password": {testPassword}, "confirm": {testPassword},
	}, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if has, _ := srv.o.Store.HasAdmin(); has {
		t.Fatal("setup created an admin with an empty token")
	}
}

// A database error must not reopen /setup.
func TestSetupFailsClosedOnDatabaseError(t *testing.T) {
	h, srv := newWeb(t)
	srv.o.Store.Close()
	rec := postForm(t, h, "/setup", url.Values{
		"token": {"setup-me"}, "password": {testPassword}, "confirm": {testPassword},
	}, nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
		t.Fatalf("status = %d, location = %q; want a redirect to /login", rec.Code, rec.Header().Get("Location"))
	}
}

// A saturated password checker is "try again", never a strike against the
// caller: otherwise a login flood locks the owner out.
func TestLoginBusyIsNotAFailedAttempt(t *testing.T) {
	h, srv := newWeb(t)
	setupAndLogin(t, h)
	verifyPassword = func(string, string) (bool, error) { return false, auth.ErrBusy }
	t.Cleanup(func() { verifyPassword = auth.VerifyPassword })

	req := httptest.NewRequest("POST", "/login", strings.NewReader(url.Values{"password": {testPassword}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if d := srv.o.Backoff.Delay(srv.sourceKey(req)); d != 0 {
		t.Fatalf("a busy checker cost a strike: delay %v", d)
	}
}

// A password reset in another process ends this process's sessions.
func TestResetAdminEndsLiveSessions(t *testing.T) {
	h, srv := newWeb(t)
	setupAndLogin(t, h)
	c := loginCookie(t, h)
	if rec := get(t, h, "/settings", c); rec.Code != http.StatusOK {
		t.Fatalf("settings before reset = %d", rec.Code)
	}
	hash, err := auth.HashPassword("a-brand-new-password")
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.o.Store.ResetAdmin(hash, time.Now()); err != nil {
		t.Fatal(err)
	}
	if rec := get(t, h, "/settings", c); rec.Code != http.StatusSeeOther {
		t.Fatalf("settings after reset = %d, want a redirect to /login", rec.Code)
	}
}

func TestTransportGate(t *testing.T) {
	proxy := netip.MustParsePrefix("10.0.0.5/32")
	_, srv := newWeb(t, func(o *Options) {
		o.Config = &config.Config{Admin: config.AdminConfig{TrustedProxies: []netip.Prefix{proxy}}}
	})
	gate := srv.TransportGate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	cases := []struct {
		name, remote, path, proto string
		want                      int
	}{
		{"loopback", "127.0.0.1:5000", "/", "", http.StatusOK},
		{"loopback v6", "[::1]:5000", "/settings", "", http.StatusOK},
		{"mapped loopback", "[::ffff:127.0.0.1]:5000", "/", "", http.StatusOK},
		{"LAN client", "192.168.1.20:5000", "/login", "", http.StatusForbidden},
		{"LAN client claiming https", "192.168.1.20:5000", "/login", "https", http.StatusForbidden},
		{"LAN health check", "192.168.1.20:5000", "/healthz", "", http.StatusOK},
		{"LAN health check, API alias", "192.168.1.20:5000", "/api/v1/healthz", "", http.StatusOK},
		{"trusted proxy over https", "10.0.0.5:5000", "/login", "https", http.StatusOK},
		{"trusted proxy over http", "10.0.0.5:5000", "/login", "http", http.StatusForbidden},
		{"trusted proxy, no scheme", "10.0.0.5:5000", "/login", "", http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.path, nil)
			req.RemoteAddr = tc.remote
			// Loopback answers only to loopback names (TestHostHeaderIsChecked).
			req.Host = "127.0.0.1:8053"
			if tc.proto != "" {
				req.Header.Set("X-Forwarded-Proto", tc.proto)
			}
			rec := httptest.NewRecorder()
			gate.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
			if rec.Header().Get("Content-Security-Policy") != "frame-ancestors 'self'" {
				t.Fatal("missing frame-ancestors")
			}
		})
	}
}

// The saved secret never reaches the page; a blank field keeps it and only
// the explicit checkbox removes it.
func TestSSOClientSecretIsNeverRendered(t *testing.T) {
	h, srv := newWeb(t)
	setupAndLogin(t, h)
	c := loginCookie(t, h)
	sess, _ := srv.o.Sessions.Get(c.Value)
	save := func(extra url.Values) {
		t.Helper()
		form := url.Values{"csrf_token": {sess.CSRF}, "issuer_url": {"https://idp.example"}, "client_id": {"kydns"}}
		for k, v := range extra {
			form[k] = v
		}
		if rec := postForm(t, h, "/settings/sso", form, c); rec.Code != http.StatusSeeOther {
			t.Fatalf("save = %d: %s", rec.Code, rec.Body)
		}
	}
	secret := func() string {
		sso, _ := srv.o.Store.SSOSettings()
		return sso.ClientSecret
	}

	save(url.Values{"client_secret": {"s3cret-value"}})
	if body := get(t, h, "/settings", c).Body.String(); strings.Contains(body, "s3cret-value") {
		t.Fatal("client secret rendered into the settings page")
	}
	save(nil)
	if secret() != "s3cret-value" {
		t.Fatalf("a blank field dropped the saved secret: %q", secret())
	}
	save(url.Values{"clear_secret": {"1"}})
	if secret() != "" {
		t.Fatalf("clear_secret left %q", secret())
	}
}

// Reset commits after the old password was read, but before login creates a session.
func TestResetRejectsOverlappingPasswordLogin(t *testing.T) {
	h, srv := newWeb(t)
	setupAndLogin(t, h)
	newHash, err := auth.HashPassword("a-brand-new-password")
	if err != nil {
		t.Fatal(err)
	}
	verifyPassword = func(hash, password string) (bool, error) {
		ok, err := auth.VerifyPassword(hash, password)
		if resetErr := srv.o.Store.ResetAdmin(newHash, time.Now()); resetErr != nil {
			t.Fatal(resetErr)
		}
		return ok, err
	}
	t.Cleanup(func() { verifyPassword = auth.VerifyPassword })
	c := loginCookie(t, h)
	if rec := get(t, h, "/settings", c); rec.Code != http.StatusSeeOther {
		t.Fatalf("overlapping old-password login accessed settings: %d", rec.Code)
	}
	verifyPassword = auth.VerifyPassword
	rec := postForm(t, h, "/login", url.Values{"password": {"a-brand-new-password"}}, nil)
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == auth.CookieName && get(t, h, "/settings", cookie).Code == http.StatusOK {
			return
		}
	}
	t.Fatal("new password could not access settings after recovery")
}
