package web

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/Busnes-app/kydns-server/internal/adminapi"
	"github.com/Busnes-app/kydns-server/internal/auth"
	"github.com/Busnes-app/kydns-server/internal/config"
	"github.com/Busnes-app/kydns-server/internal/dnsserver"
	"github.com/Busnes-app/kydns-server/internal/policy"
	"github.com/Busnes-app/kydns-server/internal/registry"
	"github.com/Busnes-app/kydns-server/internal/settings"
	"github.com/Busnes-app/kydns-server/internal/store"
)

const testPassword = "a-good-password"

// testSettings is a valid, LAN-only starting point for the settings service.
func testSettings() store.Settings {
	return store.Settings{
		PrivateDomain: "home.arpa",
		Upstreams:     []string{"tls://1.1.1.1:853"},
		AllowQuery:    []string{"192.168.0.0/16"},
		TTL:           60, CacheMinTTL: 5, CacheMaxTTL: 3600,
		NegativeMaxTTL: 300, CacheEntries: 10000,
		DiscoveryInterval: 30, HealthInterval: 30, HealthTimeout: 5, HealthWorkers: 8,
	}
}

// testConfig is the file-owned half: the keys that are read once at startup.
func testConfig() *config.Config {
	return &config.Config{
		DataDir: "/var/lib/kydns",
		DNS:     config.DNSConfig{Listen: ":53"},
		Admin:   config.AdminConfig{Listen: "127.0.0.1:8053"},
	}
}

// newSettings wires a settings service over st, seeded with testSettings.
// isReplica is the same answer the write gate gives, so the service refuses
// what the gate would have refused.
func newSettings(t *testing.T, st *store.Store, isReplica func() bool) *settings.Service {
	t.Helper()
	if err := st.PutSettings(testSettings()); err != nil {
		t.Fatal(err)
	}
	h := settings.NewHolder(func() (store.Settings, error) {
		v, _, err := st.Settings()
		return v, err
	})
	if err := h.Rebuild(); err != nil {
		t.Fatal(err)
	}
	return settings.NewService(st, h, nil, isReplica)
}

func newWeb(t *testing.T, tweak ...func(*Options)) (*http.ServeMux, *Server) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "kydns.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	reg := registry.New(st, "home.arpa.", func() error { return nil })
	acl := dnsserver.NewACL([]netip.Prefix{netip.MustParsePrefix("192.168.0.0/16")})
	cache := dnsserver.NewCache(100, 5, 3600, 300)
	ph := policy.NewHolder(func() (store.BlacklistSettings, []store.BlacklistList, []store.BlacklistRule, error) {
		set, err := st.BlacklistSettings()
		if err != nil {
			return set, nil, nil, err
		}
		lists, err := st.BlacklistLists()
		if err != nil {
			return set, nil, nil, err
		}
		rules, err := st.BlacklistRules()
		return set, lists, rules, err
	})
	if err := policy.SeedBuiltins(st); err != nil {
		t.Fatal(err)
	}
	if err := ph.Rebuild(); err != nil {
		t.Fatal(err)
	}
	pol := policy.NewService(st, ph, policy.NewRefresher(st, policy.NewFetcher(2*time.Second), ph, nil), nil)
	// One settings service behind both, as serve.go wires it: the DHCP tab
	// reads the chosen interface through the API. The role is read through o,
	// which a tweak may still be about to set, and per call, so a test that
	// promotes mid-test is followed.
	var o Options
	set := newSettings(t, st, func() bool {
		return o.Replication != nil && o.Replication().Role == roleReplica
	})
	o = Options{
		Store:      st,
		Registry:   reg,
		API:        adminapi.NewAPI(reg, acl, cache).WithPolicy(pol).WithSettings(set),
		Sessions:   auth.NewSessions(time.Hour, 12*time.Hour),
		Backoff:    auth.NewBackoff(),
		ACL:        acl,
		Cache:      cache,
		Policy:     pol,
		Settings:   set,
		SetupToken: "setup-me",
	}
	for _, f := range tweak {
		f(&o)
	}
	srv := New(o)
	mux := http.NewServeMux()
	srv.Routes(mux)
	return mux, srv
}

// setAllowTailscale flips the live setting through the one write path.
func setAllowTailscale(t *testing.T, srv *Server, on bool) {
	t.Helper()
	v, err := srv.o.Settings.Get()
	if err != nil {
		t.Fatal(err)
	}
	v.AllowTailscale = on
	if err := srv.o.Settings.Set(v, ""); err != nil {
		t.Fatal(err)
	}
}

func get(t *testing.T, h http.Handler, path string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func postForm(t *testing.T, h http.Handler, path string, form url.Values, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func sessionCookie(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.CookieName && c.Value != "" {
			return c
		}
	}
	return nil
}

func setupAndLogin(t *testing.T, h http.Handler) {
	t.Helper()
	rec := postForm(t, h, "/setup", url.Values{
		"token": {"setup-me"}, "password": {testPassword}, "confirm": {testPassword},
	}, nil)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("setup = %d: %s", rec.Code, rec.Body)
	}
}

func loginCookie(t *testing.T, h http.Handler) *http.Cookie {
	t.Helper()
	rec := postForm(t, h, "/login", url.Values{"password": {testPassword}}, nil)
	c := sessionCookie(rec)
	if c == nil {
		t.Fatalf("no session cookie after login: %d %s", rec.Code, rec.Body)
	}
	return c
}

func itoa(i int64) string { return strconv.FormatInt(i, 10) }

// Before an admin exists, every route funnels to /setup.
func TestUnsetupRedirectsToSetup(t *testing.T) {
	h, _ := newWeb(t)
	for _, path := range []string{"/", "/services", "/records", "/settings", "/login"} {
		rec := get(t, h, path, nil)
		if rec.Code != http.StatusSeeOther {
			t.Errorf("GET %s = %d, want 303 to /setup", path, rec.Code)
			continue
		}
		if loc := rec.Header().Get("Location"); loc != "/setup" {
			t.Errorf("GET %s redirected to %q, want /setup", path, loc)
		}
	}
}

func TestSetupRequiresTheToken(t *testing.T) {
	h, _ := newWeb(t)
	rec := postForm(t, h, "/setup", url.Values{
		"token": {"wrong"}, "password": {testPassword}, "confirm": {testPassword},
	}, nil)
	if rec.Code == http.StatusSeeOther {
		t.Fatal("setup succeeded with the wrong token")
	}
	if !strings.Contains(strings.ToLower(rec.Body.String()), "token") {
		t.Errorf("body does not mention the token problem:\n%s", rec.Body)
	}
}

func TestSetupRejectsMismatchedConfirmation(t *testing.T) {
	h, _ := newWeb(t)
	rec := postForm(t, h, "/setup", url.Values{
		"token": {"setup-me"}, "password": {"one-good-password"}, "confirm": {"another-password"},
	}, nil)
	if rec.Code == http.StatusSeeOther {
		t.Error("setup accepted mismatched passwords")
	}
}

func TestSetupRejectsShortPassword(t *testing.T) {
	h, _ := newWeb(t)
	rec := postForm(t, h, "/setup", url.Values{
		"token": {"setup-me"}, "password": {"short"}, "confirm": {"short"},
	}, nil)
	if rec.Code == http.StatusSeeOther {
		t.Error("setup accepted a password below the minimum length")
	}
}

func TestSetupThenLogin(t *testing.T) {
	h, _ := newWeb(t)
	rec := postForm(t, h, "/setup", url.Values{
		"token": {"setup-me"}, "password": {testPassword}, "confirm": {testPassword},
	}, nil)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("setup = %d: %s", rec.Code, rec.Body)
	}
	if sessionCookie(rec) == nil {
		t.Error("setup did not log the operator in")
	}
	// Setup is single-use.
	rec = get(t, h, "/setup", nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
		t.Errorf("GET /setup after setup = %d %q, want a redirect to /login",
			rec.Code, rec.Header().Get("Location"))
	}

	rec = postForm(t, h, "/login", url.Values{"password": {testPassword}}, nil)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("login = %d: %s", rec.Code, rec.Body)
	}
	c := sessionCookie(rec)
	if c == nil {
		t.Fatal("login set no session cookie")
	}
	if !c.HttpOnly {
		t.Error("session cookie is not HttpOnly")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Error("session cookie is not SameSite=Lax")
	}
}

// A second setup attempt must not be able to reset the password.
func TestSetupIsSingleUse(t *testing.T) {
	h, srv := newWeb(t)
	setupAndLogin(t, h)
	rec := postForm(t, h, "/setup", url.Values{
		"token": {"setup-me"}, "password": {"attacker-password"}, "confirm": {"attacker-password"},
	}, nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
		t.Errorf("second setup = %d %q, want a redirect to /login", rec.Code, rec.Header().Get("Location"))
	}
	hash, err := srv.o.Store.AdminHash()
	if err != nil {
		t.Fatal(err)
	}
	if verified(hash, "attacker-password") {
		t.Error("a second setup POST replaced the admin password")
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	h, _ := newWeb(t)
	setupAndLogin(t, h)
	rec := postForm(t, h, "/login", url.Values{"password": {"wrong"}}, nil)
	if sessionCookie(rec) != nil {
		t.Error("a failed login issued a session cookie")
	}
	if rec.Code == http.StatusSeeOther {
		t.Error("a failed login redirected as though it succeeded")
	}
}

func TestProtectedRouteNeedsSession(t *testing.T) {
	h, _ := newWeb(t)
	setupAndLogin(t, h)
	rec := get(t, h, "/services", nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
		t.Errorf("unauthenticated /services = %d %q, want a redirect to /login",
			rec.Code, rec.Header().Get("Location"))
	}
}

func TestLogoutDestroysSession(t *testing.T) {
	h, srv := newWeb(t)
	setupAndLogin(t, h)
	c := loginCookie(t, h)
	if get(t, h, "/services", c).Code != http.StatusOK {
		t.Fatal("session did not grant access")
	}
	sess, _ := srv.o.Sessions.Get(c.Value)
	rec := postForm(t, h, "/logout", url.Values{"csrf_token": {sess.CSRF}}, c)
	cleared := rec.Result().Cookies()[0]
	if cleared.Name != auth.CookieName || cleared.MaxAge != -1 || !cleared.HttpOnly ||
		cleared.SameSite != http.SameSiteLaxMode {
		t.Errorf("logout cookie = %+v, want a protected deletion cookie", cleared)
	}
	if got := get(t, h, "/services", c).Code; got != http.StatusSeeOther {
		t.Errorf("after logout /services = %d, want a redirect", got)
	}
}

// A POST without the session's CSRF token must be rejected.
func TestCSRFRequiredOnPost(t *testing.T) {
	h, srv := newWeb(t)
	setupAndLogin(t, h)
	c := loginCookie(t, h)

	bad := postForm(t, h, "/services/new", url.Values{
		"name": {"kypost"}, "address": {"192.168.1.20"},
	}, c)
	if bad.Code != http.StatusForbidden {
		t.Errorf("POST without a CSRF token = %d, want 403", bad.Code)
	}

	sess, ok := srv.o.Sessions.Get(c.Value)
	if !ok {
		t.Fatal("session vanished")
	}
	good := postForm(t, h, "/services/new", url.Values{
		"name": {"kypost"}, "address": {"192.168.1.20"}, "csrf_token": {sess.CSRF},
	}, c)
	if good.Code == http.StatusForbidden {
		t.Errorf("POST with a valid CSRF token = 403: %s", good.Body)
	}
}

// An anonymous POST redirects to login rather than leaking a 403 that implies
// the route exists behind a valid session.
func TestCSRFMiddlewareRedirectsAnonymous(t *testing.T) {
	h, _ := newWeb(t)
	setupAndLogin(t, h)
	rec := postForm(t, h, "/services/new", url.Values{"name": {"x"}}, nil)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("anonymous POST = %d, want a redirect to /login", rec.Code)
	}
}

// The two POSTs above pin one route. This pins every one the router registers,
// including any added later: a handler wired straight into the mux instead of
// through requireCSRF changes network configuration on an unauthenticated POST.
// Setup and login are the deliberate pre-session pair, named rather than
// skipped by pattern.
func TestEveryPostRouteRequiresSessionAndCSRF(t *testing.T) {
	h, srv := newWeb(t)
	setupAndLogin(t, h)
	c := loginCookie(t, h)
	// Directory events authenticate their signed body, not a browser session.
	preSession := map[string]bool{PathSetup: true, PathLogin: true, "/api/v1/installer/directory": true}
	for _, path := range registeredPostRoutes(t, srv) {
		if preSession[path] {
			continue
		}
		if rec := postForm(t, h, path, url.Values{}, c); rec.Code != http.StatusForbidden {
			t.Errorf("POST %s with a session but no CSRF token = %d, want 403", path, rec.Code)
		}
		if rec := postForm(t, h, path, url.Values{}, nil); rec.Code != http.StatusSeeOther {
			t.Errorf("anonymous POST %s = %d, want a redirect to /login", path, rec.Code)
		}
	}
}

func oidcTestServer(t *testing.T, sub, username, email, role string, tls ...bool) *httptest.Server {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var srv *httptest.Server
	srv = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		claims := map[string]any{"sub": sub, "username": username, "email": email, "role": role}
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer": srv.URL, "authorization_endpoint": srv.URL + "/authorize",
				"token_endpoint": srv.URL + "/token", "userinfo_endpoint": srv.URL + "/userinfo",
				"jwks_uri": srv.URL + "/jwks", "id_token_signing_alg_values_supported": []string{"RS256"},
			})
		case "/jwks":
			_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
				Key: &key.PublicKey, KeyID: "test", Algorithm: string(jose.RS256), Use: "sig",
			}}})
		case "/token":
			signer, _ := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key},
				(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "test"))
			raw, _ := jwt.Signed(signer).Claims(map[string]any{
				"iss": srv.URL, "aud": "kydns", "exp": time.Now().Add(time.Hour).Unix(),
				"iat": time.Now().Unix(), "nonce": "test_nonce", "sub": sub,
				"username": username, "email": email, "role": role,
			}).Serialize()
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok", "id_token": raw, "token_type": "Bearer"})
		case "/userinfo":
			_ = json.NewEncoder(w).Encode(claims)
		default:
			http.NotFound(w, r)
		}
	}))
	if len(tls) > 0 && tls[0] {
		srv.StartTLS()
	} else {
		srv.Start()
	}
	t.Cleanup(srv.Close)
	return srv
}

func TestSSOLoginButtonShownWhenEnabled(t *testing.T) {
	h, srv := newWeb(t)
	setupAndLogin(t, h)

	// Disabled by default: login page doesn't show SSO button
	rec := get(t, h, "/login", nil)
	if strings.Contains(rec.Body.String(), "Sign in with SSO") {
		t.Error("SSO button shown when SSO is disabled")
	}

	// Enable SSO
	_ = srv.o.Store.SetSSOSettings(store.SSOSettings{
		Enabled:   true,
		IssuerURL: "https://auth.example.com",
		ClientID:  "kydns",
	})

	rec = get(t, h, "/login", nil)
	if !strings.Contains(rec.Body.String(), "Sign in with SSO") {
		t.Error("SSO button missing when SSO is enabled")
	}
}

func TestSSOGetLoginSetsPKCEAndRedirects(t *testing.T) {
	h, srv := newWeb(t)
	setupAndLogin(t, h)
	_ = srv.o.Store.SetSSOSettings(store.SSOSettings{
		Enabled:   true,
		IssuerURL: "https://auth.example.com",
		ClientID:  "kydns",
	})

	req := httptest.NewRequest("GET", "/auth/sso/login", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 redirect", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "https://auth.example.com/oauth/authorize") {
		t.Errorf("unexpected redirect: %s", loc)
	}

	// Verify cookies were set
	var stateCookie, verifierCookie, nonceCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == cookieSSOState {
			stateCookie = c
		}
		if c.Name == cookieSSOVerifier {
			verifierCookie = c
		}
		if c.Name == cookieSSONonce {
			nonceCookie = c
		}
	}
	if stateCookie == nil || verifierCookie == nil || nonceCookie == nil {
		t.Fatalf("missing PKCE / state / nonce cookies")
	}
}

// ssoCallback replays a provider redirect carrying the cookies startSSO set.
func ssoCallback(t *testing.T, h http.Handler, link bool, session *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", "/auth/sso/callback?code=mock_code&state=test_state", nil)
	req.AddCookie(&http.Cookie{Name: cookieSSOState, Value: "test_state"})
	req.AddCookie(&http.Cookie{Name: cookieSSOVerifier, Value: "test_verifier"})
	req.AddCookie(&http.Cookie{Name: cookieSSONonce, Value: "test_nonce"})
	if link {
		req.AddCookie(&http.Cookie{Name: cookieSSOLink, Value: "1"})
	}
	if session != nil {
		req.AddCookie(session)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// An identity nobody linked is refused, whatever role its provider gives it.
func TestSSOCallbackRefusesUnlinkedIdentity(t *testing.T) {
	for _, role := range []string{"user", "admin"} {
		t.Run(role, func(t *testing.T) {
			idp := oidcTestServer(t, "sub-1", "bob", "bob@example.com", role)
			h, srv := newWeb(t)
			setupAndLogin(t, h)
			_ = srv.o.Store.SetSSOSettings(store.SSOSettings{Enabled: true, IssuerURL: idp.URL, ClientID: "kydns"})

			rec := ssoCallback(t, h, false, nil)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", rec.Code)
			}
			if sessionCookie(rec) != nil {
				t.Fatal("an unlinked identity got a session")
			}
			if ident, _ := srv.o.Store.AdminIdentity(); ident.SSOSub != "" {
				t.Fatalf("callback linked %q on its own", ident.SSOSub)
			}
		})
	}
}

// Dropping the link cookie must not turn a link attempt into a takeover: the
// unauthenticated attacker from the review, with an IdP admin account.
func TestSSOCallbackWithoutLinkCookieDoesNotAutoLink(t *testing.T) {
	idp := oidcTestServer(t, "attacker", "mallory", "m@example.com", "admin")
	h, srv := newWeb(t)
	setupAndLogin(t, h)
	// SSO never switched on: the stock install.
	_ = srv.o.Store.SetSSOSettings(store.SSOSettings{IssuerURL: idp.URL, ClientID: "kydns"})

	rec := ssoCallback(t, h, false, nil)
	if rec.Code != http.StatusForbidden || sessionCookie(rec) != nil {
		t.Fatalf("status = %d, session = %v; want 403 and none", rec.Code, sessionCookie(rec))
	}
	sso, _ := srv.o.Store.SSOSettings()
	ident, _ := srv.o.Store.AdminIdentity()
	if sso.Enabled || ident.SSOSub != "" {
		t.Fatalf("SSO enabled=%v linked=%q after a refused callback", sso.Enabled, ident.SSOSub)
	}
}

func TestSSOCallbackLogsInTheLinkedIdentity(t *testing.T) {
	idp := oidcTestServer(t, "sub-linked", "yoshi", "y@example.com", "user")
	h, srv := newWeb(t)
	setupAndLogin(t, h)
	_ = srv.o.Store.SetSSOSettings(store.SSOSettings{Enabled: true, IssuerURL: idp.URL, ClientID: "kydns"})
	_ = srv.o.Store.LinkAdminSSO("sub-linked", "yoshi", "y@example.com")

	rec := ssoCallback(t, h, false, nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" || sessionCookie(rec) == nil {
		t.Fatalf("status = %d, location = %q; want 303 to / with a session", rec.Code, rec.Header().Get("Location"))
	}
}

// Switching SSO off stops even the linked identity.
func TestSSOCallbackRefusedWhenDisabled(t *testing.T) {
	idp := oidcTestServer(t, "sub-linked", "yoshi", "y@example.com", "admin")
	h, srv := newWeb(t)
	setupAndLogin(t, h)
	_ = srv.o.Store.SetSSOSettings(store.SSOSettings{Enabled: false, IssuerURL: idp.URL, ClientID: "kydns"})
	_ = srv.o.Store.LinkAdminSSO("sub-linked", "yoshi", "y@example.com")

	if rec := ssoCallback(t, h, false, nil); rec.Code != http.StatusForbidden || sessionCookie(rec) != nil {
		t.Fatalf("status = %d; want 403 and no session", rec.Code)
	}
}

func TestSSOLinkCallbackNeedsASession(t *testing.T) {
	idp := oidcTestServer(t, "attacker", "mallory", "m@example.com", "admin")
	h, srv := newWeb(t)
	setupAndLogin(t, h)
	_ = srv.o.Store.SetSSOSettings(store.SSOSettings{IssuerURL: idp.URL, ClientID: "kydns"})

	rec := ssoCallback(t, h, true, nil)
	if rec.Code != http.StatusSeeOther || !strings.HasPrefix(rec.Header().Get("Location"), "/login") {
		t.Fatalf("status = %d, location = %q; want a redirect to /login", rec.Code, rec.Header().Get("Location"))
	}
	if ident, _ := srv.o.Store.AdminIdentity(); ident.SSOSub != "" {
		t.Fatalf("linked %q without a session", ident.SSOSub)
	}
}

func TestSSOLinkFromSettingsLinksAndEnables(t *testing.T) {
	idp := oidcTestServer(t, "sub-mine", "yoshi", "y@example.com", "user")
	h, srv := newWeb(t)
	setupAndLogin(t, h)
	c := loginCookie(t, h)
	sess, _ := srv.o.Sessions.Get(c.Value)
	_ = srv.o.Store.SetSSOSettings(store.SSOSettings{IssuerURL: idp.URL, ClientID: "kydns"})

	rec := postForm(t, h, "/settings/sso/link", url.Values{"csrf_token": {sess.CSRF}}, c)
	if rec.Code != http.StatusSeeOther || !strings.HasPrefix(rec.Header().Get("Location"), idp.URL) {
		t.Fatalf("link start = %d to %q; want a redirect to the provider", rec.Code, rec.Header().Get("Location"))
	}
	var linkCookie bool
	for _, ck := range rec.Result().Cookies() {
		linkCookie = linkCookie || (ck.Name == cookieSSOLink && ck.Value == "1")
	}
	if !linkCookie {
		t.Fatal("link start did not mark the flow as a link")
	}

	if rec := ssoCallback(t, h, true, c); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/settings" {
		t.Fatalf("link callback = %d to %q", rec.Code, rec.Header().Get("Location"))
	}
	ident, _ := srv.o.Store.AdminIdentity()
	sso, _ := srv.o.Store.SSOSettings()
	if ident.SSOSub != "sub-mine" || !sso.Enabled {
		t.Fatalf("linked=%q enabled=%v; want sub-mine and true", ident.SSOSub, sso.Enabled)
	}
}

// The old GET link entry point is gone: ?link=true neither bypasses the
// "SSO is off" check nor marks a flow as a link.
func TestSSOLoginIgnoresLinkParameter(t *testing.T) {
	h, _ := newWeb(t)
	setupAndLogin(t, h)
	rec := get(t, h, "/auth/sso/login?link=true", nil)
	if rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "SSO+is+not+enabled") {
		t.Fatalf("status = %d, location = %q; want the not-enabled redirect", rec.Code, rec.Header().Get("Location"))
	}
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == cookieSSOState || ck.Name == cookieSSOLink {
			t.Fatalf("set %s while SSO is off", ck.Name)
		}
	}
}

// Exchange failures are logged, not shown to whoever is at the login page.
func TestSSOExchangeErrorIsNotShown(t *testing.T) {
	idp := oidcTestServer(t, "sub-1", "bob", "bob@example.com", "admin")
	h, srv := newWeb(t)
	setupAndLogin(t, h)
	_ = srv.o.Store.SetSSOSettings(store.SSOSettings{Enabled: true, IssuerURL: idp.URL, ClientID: "kydns"})

	req := httptest.NewRequest("GET", "/auth/sso/callback?code=c&state=s", nil)
	req.AddCookie(&http.Cookie{Name: cookieSSOState, Value: "s"})
	req.AddCookie(&http.Cookie{Name: cookieSSOVerifier, Value: "v"})
	req.AddCookie(&http.Cookie{Name: cookieSSONonce, Value: "not-the-issued-nonce"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if body := rec.Body.String(); strings.Contains(body, "nonce") || strings.Contains(body, idp.URL) {
		t.Fatalf("error detail leaked to the page: %s", body)
	}
}

// With no default issuer, enabling SSO needs one; saving it disabled does not.
func TestSSOSettingsEnableNeedsIssuer(t *testing.T) {
	h, srv := newWeb(t)
	setupAndLogin(t, h)
	c := loginCookie(t, h)
	sess, _ := srv.o.Sessions.Get(c.Value)

	rec := postForm(t, h, "/settings/sso", url.Values{"csrf_token": {sess.CSRF}, "enabled": {"1"}, "issuer_url": {" "}}, c)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("enable with no issuer: status = %d, want 400", rec.Code)
	}
	if sso, _ := srv.o.Store.SSOSettings(); sso.Enabled {
		t.Fatalf("SSO enabled with no issuer: %+v", sso)
	}
	rec = postForm(t, h, "/settings/sso", url.Values{"csrf_token": {sess.CSRF}, "issuer_url": {""}}, c)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("disabled save: status = %d, want redirect", rec.Code)
	}
	if sso, _ := srv.o.Store.SSOSettings(); sso.Enabled || sso.IssuerURL != "" || sso.ClientID != "kydns" {
		t.Fatalf("disabled save stored %+v", sso)
	}
}

func TestSSOSettingsSaveAndUnlink(t *testing.T) {
	h, srv := newWeb(t)
	setupAndLogin(t, h)
	c := loginCookie(t, h)
	sess, _ := srv.o.Sessions.Get(c.Value)

	// Save settings
	rec := postForm(t, h, "/settings/sso", url.Values{
		"csrf_token":    {sess.CSRF},
		"enabled":       {"1"},
		"issuer_url":    {"https://auth.example.com"},
		"client_id":     {"kydns-prod"},
		"client_secret": {"sec123"},
	}, c)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want redirect", rec.Code)
	}

	sso, _ := srv.o.Store.SSOSettings()
	if !sso.Enabled || sso.ClientID != "kydns-prod" || sso.ClientSecret != "sec123" {
		t.Errorf("unexpected SSOSettings: %+v", sso)
	}

	// Link admin and then unlink
	_ = srv.o.Store.LinkAdminSSO("sub-123", "user1", "user1@example.com")
	rec = postForm(t, h, "/settings/sso/unlink", url.Values{"csrf_token": {sess.CSRF}}, c)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want redirect", rec.Code)
	}

	ident, _ := srv.o.Store.AdminIdentity()
	if ident.SSOSub != "" || ident.SSOUsername != "" {
		t.Errorf("expected unlinked SSO, got: %+v", ident)
	}
}

func TestSettingsPageRendersWithLinkedSSO(t *testing.T) {
	h, srv := newWeb(t)
	setupAndLogin(t, h)
	c := loginCookie(t, h)

	// 1. Render when unlinked
	rec := get(t, h, "/settings", c)
	if rec.Code != http.StatusOK {
		t.Fatalf("settings page status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Link SSO identity") {
		t.Errorf("expected 'Link SSO identity' in unlinked settings HTML")
	}

	// 2. Link identity and render again
	_ = srv.o.Store.LinkAdminSSO("sso-sub-xyz", "yoshi_admin", "yoshi@example.com")
	rec = get(t, h, "/settings", c)
	if rec.Code != http.StatusOK {
		t.Fatalf("settings page status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "SSO identity linked") || !strings.Contains(rec.Body.String(), "yoshi_admin") {
		t.Errorf("expected 'SSO identity linked' and 'yoshi_admin' in settings HTML, got: %s", rec.Body.String())
	}
}

// verified is VerifyPassword for tests, where the checker is never busy.
func verified(encoded, plaintext string) bool {
	ok, err := auth.VerifyPassword(encoded, plaintext)
	return err == nil && ok
}
