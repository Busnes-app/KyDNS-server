package web

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Busnes-app/kydns-server/internal/store"
)

// fakeKyIdentity is a TLS provider whose token endpoint returns the next ID token.
type fakeKyIdentity struct {
	key     *rsa.PrivateKey
	issuer  string
	idToken string
	// discoveries and keyFetches count the requests KyDNS made to the provider.
	discoveries, keyFetches atomic.Int32
}

func newFakeKyIdentity(t *testing.T) *fakeKyIdentity {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeKyIdentity{key: key}
	idp := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			f.discoveries.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer": f.issuer, "authorization_endpoint": f.issuer + "/authorize",
				"token_endpoint": f.issuer + "/token", "jwks_uri": f.issuer + "/keys",
				"id_token_signing_alg_values_supported": []string{"RS256"},
			})
		case "/keys":
			f.keyFetches.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]any{
				"kty": "RSA", "alg": "RS256", "kid": "k",
				"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": "AQAB",
			}}})
		case "/token":
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "a", "token_type": "Bearer", "id_token": f.idToken})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(idp.Close)
	f.issuer = idp.URL
	// The server reaches the provider through the default client; trust the test CA.
	previous := http.DefaultTransport
	http.DefaultTransport = idp.Client().Transport
	t.Cleanup(func() { http.DefaultTransport = previous })
	return f
}

func (f *fakeKyIdentity) sign(t *testing.T, typ string, claims map[string]any) string {
	t.Helper()
	claims["iss"], claims["aud"] = f.issuer, "kydns"
	claims["exp"] = time.Now().Add(2 * time.Minute).Unix()
	if _, ok := claims["iat"]; !ok {
		claims["iat"] = time.Now().Unix()
	}
	header := map[string]string{"alg": "RS256", "kid": "k"}
	if typ != "" {
		header["typ"] = typ
	}
	h, _ := json.Marshal(header)
	c, _ := json.Marshal(claims)
	input := base64.RawURLEncoding.EncodeToString(h) + "." + base64.RawURLEncoding.EncodeToString(c)
	digest := sha256.Sum256([]byte(input))
	sig, err := rsa.SignPKCS1v15(rand.Reader, f.key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return input + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func (f *fakeKyIdentity) logoutToken(t *testing.T, sub, sid string) string {
	claims := map[string]any{"sub": sub, "jti": sid + sub, "events": map[string]any{"http://schemas.openid.net/event/backchannel-logout": map[string]any{}}}
	if sid != "" {
		claims["sid"] = sid
	}
	return f.sign(t, "logout+jwt", claims)
}

// login runs the callback for an ID token naming sub and sid.
func (f *fakeKyIdentity) login(t *testing.T, h http.Handler, sub, sid string) *httptest.ResponseRecorder {
	return f.loginAt(t, h, sub, sid, time.Now())
}

// loginAt is login with an ID token the provider issued at iat.
func (f *fakeKyIdentity) loginAt(t *testing.T, h http.Handler, sub, sid string, iat time.Time) *httptest.ResponseRecorder {
	t.Helper()
	f.idToken = f.sign(t, "JWT", map[string]any{"sub": sub, "sid": sid, "nonce": "test_nonce", "iat": iat.Unix()})
	return ssoCallback(t, h, false, nil)
}

func postLogoutToken(t *testing.T, h http.Handler, token string) *httptest.ResponseRecorder {
	t.Helper()
	return postForm(t, h, "/auth/sso/backchannel-logout", url.Values{"logout_token": {token}}, nil)
}

func signedIn(t *testing.T, h http.Handler, c *http.Cookie) bool {
	t.Helper()
	return get(t, h, "/", c).Code == http.StatusOK
}

func TestBackchannelLogoutEndsTheSSOLogin(t *testing.T) {
	f := newFakeKyIdentity(t)
	h, srv := newWeb(t)
	setupAndLogin(t, h)
	password := loginCookie(t, h)
	_ = srv.o.Store.SetSSOSettings(store.SSOSettings{Enabled: true, IssuerURL: f.issuer, ClientID: "kydns"})
	_ = srv.o.Store.LinkAdminSSO("alice", "alice", "a@example.com")

	ended := sessionCookie(f.login(t, h, "alice", "sid-1"))
	keptAt := time.Now()
	kept := sessionCookie(f.loginAt(t, h, "alice", "sid-2", keptAt))
	if ended == nil || kept == nil || !signedIn(t, h, ended) || !signedIn(t, h, kept) {
		t.Fatal("SSO login did not issue working sessions")
	}

	// An ID token is not a logout token, even when the provider signed it.
	if rec := postLogoutToken(t, h, f.idToken); rec.Code != http.StatusBadRequest {
		t.Fatalf("ID token as logout token = %d, want 400", rec.Code)
	}
	// The body is capped at 16 KiB, even for a token the provider signed.
	oversized := f.sign(t, "logout+jwt", map[string]any{"sid": "sid-1", "pad": strings.Repeat("x", 16<<10),
		"events": map[string]any{"http://schemas.openid.net/event/backchannel-logout": map[string]any{}}})
	if rec := postLogoutToken(t, h, oversized); rec.Code != http.StatusBadRequest || !signedIn(t, h, ended) {
		t.Fatalf("oversized logout = %d, want 400 and the session kept", rec.Code)
	}
	rec := postLogoutToken(t, h, f.logoutToken(t, "alice", "sid-1"))
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("logout = %d %q %s", rec.Code, rec.Header().Get("Cache-Control"), rec.Body)
	}
	if signedIn(t, h, ended) {
		t.Fatal("session survived its back-channel logout")
	}
	if !signedIn(t, h, kept) {
		t.Fatal("logout of one login ended another login of the same user")
	}
	// A replay is harmless and still acknowledged.
	if rec := postLogoutToken(t, h, f.logoutToken(t, "alice", "sid-1")); rec.Code != http.StatusOK {
		t.Fatalf("replayed logout = %d", rec.Code)
	}

	// A logout that lands before the callback finishes fences that login.
	if rec := postLogoutToken(t, h, f.logoutToken(t, "alice", "sid-late")); rec.Code != http.StatusOK {
		t.Fatalf("early logout = %d", rec.Code)
	}
	if rec := f.login(t, h, "alice", "sid-late"); rec.Code != http.StatusUnauthorized || sessionCookie(rec) != nil {
		t.Fatalf("callback revived an ended login: %d", rec.Code)
	}

	// A subject-wide token ends that subject's logins and nobody else's.
	if rec := postLogoutToken(t, h, f.logoutToken(t, "bob", "")); rec.Code != http.StatusOK || !signedIn(t, h, kept) {
		t.Fatalf("logout for another subject = %d, or it ended alice's login", rec.Code)
	}
	// Issued the same second as the login: iat is whole seconds, so that login ends too.
	aliceLogout := f.sign(t, "logout+jwt", map[string]any{"sub": "alice", "jti": "alice", "iat": keptAt.Unix(),
		"events": map[string]any{"http://schemas.openid.net/event/backchannel-logout": map[string]any{}}})
	if rec := postLogoutToken(t, h, aliceLogout); rec.Code != http.StatusOK || signedIn(t, h, kept) {
		t.Fatalf("subject-wide logout = %d, or alice's login survived it", rec.Code)
	}
	if !signedIn(t, h, password) {
		t.Fatal("back-channel logout ended the password session")
	}

	// A subject-wide logout fences a callback already holding an ID token issued before
	// it, even for a subject with no session here yet; a later login still works.
	_ = srv.o.Store.LinkAdminSSO("carol", "carol", "c@example.com")
	carolLogout := f.sign(t, "logout+jwt", map[string]any{"sub": "carol", "jti": "carol", "iat": time.Now().Add(-30 * time.Second).Unix(),
		"events": map[string]any{"http://schemas.openid.net/event/backchannel-logout": map[string]any{}}})
	if rec := postLogoutToken(t, h, carolLogout); rec.Code != http.StatusOK {
		t.Fatalf("subject-wide logout of a subject with no session = %d", rec.Code)
	}
	if rec := f.loginAt(t, h, "carol", "sid-c1", time.Now().Add(-time.Minute)); rec.Code != http.StatusUnauthorized || sessionCookie(rec) != nil {
		t.Fatalf("pending callback survived a subject-wide logout: %d", rec.Code)
	}
	if c := sessionCookie(f.login(t, h, "carol", "sid-c2")); c == nil || !signedIn(t, h, c) {
		t.Fatal("login issued after the subject-wide logout was refused")
	}

	snap := filepath.Join(t.TempDir(), "audit.db")
	if err := srv.o.Store.SnapshotTo(snap); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", snap+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_events WHERE action = 'auth.sso_backchannel_logout'`).Scan(&n); err != nil || n != 6 {
		t.Fatalf("audited %d back-channel logouts (%v), want 6", n, err)
	}
}

// An anonymous POST must not make KyDNS call the provider: discovery and the key
// set are fetched once per (issuer, client ID), whatever strangers send.
func TestBackchannelLogoutDiscoversOncePerSettings(t *testing.T) {
	f := newFakeKyIdentity(t)
	h, srv := newWeb(t)
	setupAndLogin(t, h)
	_ = srv.o.Store.SetSSOSettings(store.SSOSettings{Enabled: true, IssuerURL: f.issuer, ClientID: "kydns"})

	for i := 0; i < 5; i++ {
		if rec := postLogoutToken(t, h, f.logoutToken(t, "alice", "sid-1")); rec.Code != http.StatusOK {
			t.Fatalf("valid logout = %d", rec.Code)
		}
		for _, bad := range []string{"garbage", "", "a.b.c"} {
			if rec := postLogoutToken(t, h, bad); rec.Code != http.StatusBadRequest {
				t.Fatalf("logout %q = %d, want 400", bad, rec.Code)
			}
		}
	}
	if d, k := f.discoveries.Load(), f.keyFetches.Load(); d != 1 || k != 1 {
		t.Fatalf("20 logout POSTs made %d discoveries and %d key fetches, want 1 and 1", d, k)
	}

	// A new client ID rebuilds the verifier: the old audience no longer verifies.
	_ = srv.o.Store.SetSSOSettings(store.SSOSettings{Enabled: true, IssuerURL: f.issuer, ClientID: "other"})
	for i := 0; i < 3; i++ {
		if rec := postLogoutToken(t, h, f.logoutToken(t, "alice", "sid-1")); rec.Code != http.StatusBadRequest {
			t.Fatalf("logout for the old client = %d, want 400", rec.Code)
		}
	}
	if d := f.discoveries.Load(); d != 2 {
		t.Fatalf("client change made %d discoveries in all, want 2", d)
	}

	// A new issuer is discovered once, and its tokens verify.
	g := newFakeKyIdentity(t)
	_ = srv.o.Store.SetSSOSettings(store.SSOSettings{Enabled: true, IssuerURL: g.issuer, ClientID: "kydns"})
	for i := 0; i < 3; i++ {
		if rec := postLogoutToken(t, h, g.logoutToken(t, "alice", "sid-1")); rec.Code != http.StatusOK {
			t.Fatalf("logout from the new issuer = %d", rec.Code)
		}
		if rec := postLogoutToken(t, h, "garbage"); rec.Code != http.StatusBadRequest {
			t.Fatalf("garbage logout = %d, want 400", rec.Code)
		}
	}
	if d, k := g.discoveries.Load(), g.keyFetches.Load(); d != 1 || k != 1 || f.discoveries.Load() != 2 {
		t.Fatalf("issuer change made %d discoveries and %d key fetches, want 1 and 1", d, k)
	}
}
