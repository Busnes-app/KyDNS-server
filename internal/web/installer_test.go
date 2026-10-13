package web

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"github.com/Busnes-app/ky-primitives/applysetup"
	"github.com/Busnes-app/ky-primitives/syncauth"
	"github.com/Busnes-app/kydns-server/internal/auth"
	"github.com/Busnes-app/kydns-server/internal/config"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func webInstallerBundle(t *testing.T, issuer string) applysetup.Common {
	t.Helper()
	dir := t.TempDir()
	client := filepath.Join(dir, "client")
	directory := filepath.Join(dir, "directory")
	for _, p := range []string{client, directory} {
		if err := os.WriteFile(p, []byte("fixture-directory-secret-at-least-16"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return applysetup.Common{Schema: applysetup.Schema, Service: "kydns", SetupID: "12345678-1234-1234-1234-123456789abc", SSO: &applysetup.SSO{IssuerURL: issuer, ClientID: "kydns", ClientSecretFile: client, DirectorySecretFile: directory, RedirectURI: "https://dns.example/auth/sso/callback"}, Admins: []applysetup.Identity{{Issuer: issuer, Subject: "owner", Username: "administrator"}}}
}
func TestInstallerVerifiedOwnerAndTokenDescendantsRevoked(t *testing.T) {
	provider := oidcTestServer(t, "owner", "administrator", "admin@example.com", "user", true)
	roots := x509.NewCertPool()
	roots.AddCert(provider.Certificate())
	old := http.DefaultTransport
	tr := old.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
	http.DefaultTransport = tr
	t.Cleanup(func() { http.DefaultTransport = old; tr.CloseIdleConnections() })
	_, srv := newWeb(t)
	b := webInstallerBundle(t, provider.URL)
	if _, err := srv.o.Store.ApplyInstaller(context.Background(), b, false); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	srv.Routes(mux)
	srv.o.API.Routes(mux)
	h := srv.WriteGate(mux)
	callback := httptest.NewRequest("GET", "/auth/sso/callback?code=mock_code&state=test_state", nil)
	for n, v := range map[string]string{cookieSSOState: "test_state", cookieSSOVerifier: "test_verifier", cookieSSONonce: "test_nonce"} {
		callback.AddCookie(&http.Cookie{Name: n, Value: v})
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, callback)
	c := sessionCookie(rec)
	if rec.Code != 303 || c == nil {
		t.Fatalf("verified owner login: %d %s", rec.Code, rec.Body)
	}
	sess, ok := srv.o.Sessions.Get(c.Value)
	if !ok || sess.IdentityIssuer != provider.URL || sess.IdentitySubject != "owner" {
		t.Fatalf("binding %+v %v", sess, ok)
	}
	if rec = get(t, h, "/", c); rec.Code != 200 {
		t.Fatalf("owner dashboard %d", rec.Code)
	}
	rec = postForm(t, h, "/settings/tokens/new", url.Values{"label": {"owner-api"}, "csrf_token": {sess.CSRF}}, c)
	body := rec.Body.String()
	idx := strings.Index(body, "kydns_")
	if rec.Code != 200 || idx < 0 {
		t.Fatalf("owner token %d %s", rec.Code, body)
	}
	parent := body[idx : idx+70]
	api := func(method, path, token, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if rec = api("GET", "/api/v1/services", parent, ""); rec.Code != 200 {
		t.Fatalf("owner API %d %s", rec.Code, rec.Body)
	}
	rec = api("POST", "/api/v1/tokens", parent, `{"label":"child"}`)
	var child struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &child)
	if rec.Code != 201 || child.Token == "" {
		t.Fatalf("child %d %s", rec.Code, rec.Body)
	}
	tokens, err := srv.o.Registry.Tokens()
	if err != nil || len(tokens) != 2 {
		t.Fatalf("tokens %+v %v", tokens, err)
	}
	for _, token := range tokens {
		if token.Issuer != provider.URL || token.Subject != "owner" {
			t.Fatalf("unbound child %+v", token)
		}
	}
	local, err := srv.o.Registry.CreateToken("retained-local-recovery")
	if err != nil {
		t.Fatal(err)
	}
	event := func(raw, eventType string, signed bool) int {
		r := httptest.NewRequest("POST", PathInstallerDirectory, strings.NewReader(raw))
		if signed {
			headers, err := syncauth.Sign([]byte("fixture-directory-secret-at-least-16"), time.Now(), eventType, "fixture-event", []byte(raw))
			if err != nil {
				t.Fatal(err)
			}
			headers.Apply(r)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	off := `{"id":"owner","active":false,"meta":{"version":"W/\"2\""}}`
	if status := event(off, "user.deactivated", false); status != 401 {
		t.Fatalf("unsigned %d", status)
	}
	if status := event(off, "user.deactivated", true); status != 204 {
		t.Fatalf("signed offboard %d", status)
	}
	if status := event(off, "user.deactivated", true); status != 204 {
		t.Fatalf("duplicate offboard %d", status)
	}
	if rec = get(t, h, "/", c); rec.Code != 303 {
		t.Fatalf("revoked session %d", rec.Code)
	}
	for _, token := range []string{parent, child.Token} {
		if rec = api("GET", "/api/v1/services", token, ""); rec.Code != 401 {
			t.Fatalf("revoked API %d", rec.Code)
		}
	}
	if rec = api("GET", "/api/v1/services", local, ""); rec.Code != 200 {
		t.Fatalf("recovery API changed %d", rec.Code)
	}
	on := `{"id":"owner","active":true,"roles":[{"value":"kydns.admin"}],"meta":{"version":"W/\"1\""}}`
	if status := event(on, "user.updated", true); status != 422 {
		t.Fatalf("stale activation %d", status)
	}
}
func TestInstallerRejectsUnassignedGlobalAdmin(t *testing.T) {
	provider := oidcTestServer(t, "everyday", "ordinary", "user@example.com", "admin", true)
	roots := x509.NewCertPool()
	roots.AddCert(provider.Certificate())
	old := http.DefaultTransport
	tr := old.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
	http.DefaultTransport = tr
	t.Cleanup(func() { http.DefaultTransport = old; tr.CloseIdleConnections() })
	h, srv := newWeb(t)
	b := webInstallerBundle(t, provider.URL)
	if _, err := srv.o.Store.ApplyInstaller(context.Background(), b, false); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/auth/sso/callback?code=mock_code&state=test_state", nil)
	for n, v := range map[string]string{cookieSSOState: "test_state", cookieSSOVerifier: "test_verifier", cookieSSONonce: "test_nonce"} {
		r.AddCookie(&http.Cookie{Name: n, Value: v})
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 || sessionCookie(w) != nil {
		t.Fatalf("unassigned global admin %d %s", w.Code, w.Body)
	}
}
func TestInstallerNetworkUsesTrustedPeer(t *testing.T) {
	for _, trusted := range []bool{false, true} {
		t.Run(map[bool]string{false: "spoof", true: "trusted"}[trusted], func(t *testing.T) {
			cfg := &config.Config{}
			if trusted {
				cfg.Admin.TrustedProxies = []netip.Prefix{netip.MustParsePrefix("192.0.2.10/32")}
			}
			h, _ := newWeb(t, func(o *Options) { o.Config = cfg })
			r := httptest.NewRequest("GET", "/api/v1/installer/network", nil)
			r.RemoteAddr = "192.0.2.10:1234"
			r.Header.Set("X-Forwarded-For", "198.51.100.8")
			r.Header.Set("X-Forwarded-Proto", "https")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			var result struct {
				IP      string `json:"clientIp"`
				Trusted bool   `json:"proxyHeadersTrusted"`
				Scheme  string `json:"scheme"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			wantIP, wantScheme := "192.0.2.10", "http"
			if trusted {
				wantIP, wantScheme = "198.51.100.8", "https"
			}
			if result.IP != wantIP || result.Scheme != wantScheme || result.Trusted != trusted || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("result %+v", result)
			}
		})
	}
}
func TestManagedSettingsNeverRenderInstallerSecrets(t *testing.T) {
	h, srv, local, _ := loggedIn(t)
	issuer := "https://identity.example"
	if _, err := srv.o.Store.ApplyInstaller(context.Background(), webInstallerBundle(t, issuer), false); err != nil {
		t.Fatal(err)
	}
	managed := &http.Cookie{Name: auth.CookieName, Value: srv.o.Sessions.CreateSSO(issuer, "owner", 0).ID}
	populated := regexp.MustCompile(`name="client_secret"[^>]*value="[^"]+"`)
	for name, c := range map[string]*http.Cookie{"managed": managed, "local recovery": local} {
		rec := get(t, h, "/settings", c)
		body := rec.Body.String()
		if rec.Code != 200 || strings.Contains(body, "fixture-directory-secret-at-least-16") || populated.MatchString(body) {
			t.Fatalf("%s settings %d exposes the installer client secret", name, rec.Code)
		}
	}
}

// Reactivation must not revive a session or bound token issued before the revocation,
// even when nothing used them while revoked. A fresh login works.
func TestReactivationLeavesPreRevocationCredentialsDead(t *testing.T) {
	provider := oidcTestServer(t, "owner", "administrator", "admin@example.com", "user", true)
	roots := x509.NewCertPool()
	roots.AddCert(provider.Certificate())
	old := http.DefaultTransport
	tr := old.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
	http.DefaultTransport = tr
	t.Cleanup(func() { http.DefaultTransport = old; tr.CloseIdleConnections() })
	_, srv := newWeb(t)
	if _, err := srv.o.Store.ApplyInstaller(context.Background(), webInstallerBundle(t, provider.URL), false); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	srv.Routes(mux)
	srv.o.API.Routes(mux)
	h := srv.WriteGate(mux)
	login := func() *http.Cookie {
		r := httptest.NewRequest("GET", "/auth/sso/callback?code=mock_code&state=test_state", nil)
		for n, v := range map[string]string{cookieSSOState: "test_state", cookieSSOVerifier: "test_verifier", cookieSSONonce: "test_nonce"} {
			r.AddCookie(&http.Cookie{Name: n, Value: v})
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if c := sessionCookie(w); w.Code == 303 && c != nil {
			return c
		}
		t.Fatalf("login %d %s", w.Code, w.Body)
		return nil
	}
	event := func(raw, eventType string) {
		r := httptest.NewRequest("POST", PathInstallerDirectory, strings.NewReader(raw))
		headers, err := syncauth.Sign([]byte("fixture-directory-secret-at-least-16"), time.Now(), eventType, "fixture-"+eventType, []byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		headers.Apply(r)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 204 {
			t.Fatalf("%s %d %s", eventType, w.Code, w.Body)
		}
	}
	api := func(token string) int {
		r := httptest.NewRequest("GET", "/api/v1/services", nil)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	stale := login()
	token, err := srv.o.Registry.CreateIdentityToken("owner-api", provider.URL, "owner")
	if err != nil {
		t.Fatal(err)
	}
	event(`{"id":"owner","active":false,"meta":{"version":"W/\"2\""}}`, "user.deactivated")
	event(`{"id":"owner","active":true,"roles":[{"value":"kydns.admin"}],"meta":{"version":"W/\"3\""}}`, "user.updated")
	if rec := get(t, h, "/", stale); rec.Code != 303 {
		t.Fatalf("pre-revocation session revived: %d", rec.Code)
	}
	if code := api(token); code != 401 {
		t.Fatalf("pre-revocation token revived: %d", code)
	}
	fresh := login()
	if rec := get(t, h, "/", fresh); rec.Code != 200 {
		t.Fatalf("fresh login after reactivation %d", rec.Code)
	}
	event(`{"id":"owner","active":true,"roles":[{"value":"kydns.admin"}],"meta":{"version":"W/\"4\""}}`, "user.updated")
	if rec := get(t, h, "/", fresh); rec.Code != 200 {
		t.Fatalf("routine directory update ended the session %d", rec.Code)
	}
}
