package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Busnes-app/kydns-server/internal/auth"
	"github.com/Busnes-app/kydns-server/internal/store"
)

// A token outlives the session, so a session alone cannot mint one
// (KyPost-Server Q2 A): the current password must come with it.
func TestTokenCreateNeedsTheCurrentPassword(t *testing.T) {
	h, srv := newWeb(t)
	setupAndLogin(t, h)
	c := loginCookie(t, h)
	sess, _ := srv.o.Sessions.Get(c.Value)
	create := func(pw string) int {
		form := url.Values{"label": {"laptop"}, "csrf_token": {sess.CSRF}}
		if pw != "" {
			form.Set("current_password", pw)
		}
		return postForm(t, h, "/settings/tokens/new", form, c).Code
	}
	for _, pw := range []string{"", "not-the-password"} {
		if code := create(pw); code != http.StatusBadRequest {
			t.Errorf("password %q: status = %d, want 400", pw, code)
		}
	}
	if toks, _ := srv.o.Registry.Tokens(); len(toks) != 0 {
		t.Fatalf("tokens minted without a step-up: %d", len(toks))
	}
	if code := create(testPassword); code != http.StatusOK {
		t.Fatalf("with the password: status = %d", code)
	}
}

// A single sign-on login counts as a step-up only while its auth_time is
// fresh (KyCalendar Q6 A).
func TestTokenCreateAcceptsAFreshSSOLogin(t *testing.T) {
	h, srv := newWeb(t)
	setupAndLogin(t, h)
	for name, tc := range map[string]struct {
		authTime time.Time
		want     int
	}{
		"fresh":   {time.Now().Add(-time.Minute), http.StatusOK},
		"stale":   {time.Now().Add(-time.Hour), http.StatusBadRequest},
		"missing": {time.Time{}, http.StatusBadRequest},
	} {
		t.Run(name, func(t *testing.T) {
			sess, _ := srv.o.Sessions.CreateSSOLogin(auth.SSOLogin{Issuer: "https://idp.example", Subject: "owner", AuthTime: tc.authTime},
				"", "", 0, srv.o.Sessions.RecoveryGeneration())
			c := &http.Cookie{Name: auth.CookieName, Value: sess.ID}
			rec := postForm(t, h, "/settings/tokens/new", url.Values{"label": {name}, "csrf_token": {sess.CSRF}}, c)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

// Confirming with single sign-on asks the provider to authenticate again.
func TestTokenStepUpAsksTheProviderForAFreshLogin(t *testing.T) {
	idp := oidcTestServer(t, "sub-linked", "yoshi", "y@example.com", "user")
	h, srv := newWeb(t)
	setupAndLogin(t, h)
	_ = srv.o.Store.SetSSOSettings(store.SSOSettings{Enabled: true, IssuerURL: idp.URL, ClientID: "kydns"})
	c := loginCookie(t, h)
	sess, _ := srv.o.Sessions.Get(c.Value)
	rec := postForm(t, h, "/settings/tokens/stepup", url.Values{"csrf_token": {sess.CSRF}}, c)
	loc, _ := url.Parse(rec.Header().Get("Location"))
	if rec.Code != http.StatusSeeOther || loc == nil || loc.Query().Get("max_age") != "0" {
		t.Fatalf("status = %d, location = %q; want a redirect with max_age=0", rec.Code, rec.Header().Get("Location"))
	}
	if !strings.Contains(strings.Join(rec.Header().Values("Set-Cookie"), "; "), cookieSSOLink+"=stepup") {
		t.Fatalf("no step-up purpose cookie: %v", rec.Header().Values("Set-Cookie"))
	}
}

// A provider that ignored max_age=0 sends an old auth_time back: that is no
// step-up, and the callback must not hand out a session that counts as one.
func TestTokenStepUpCallbackRefusesAnOldAuthTime(t *testing.T) {
	for name, at := range map[string]int64{"old": time.Now().Add(-time.Hour).Unix(), "missing": 0} {
		t.Run(name, func(t *testing.T) {
			old := oidcAuthTime
			oidcAuthTime = func() int64 { return at }
			t.Cleanup(func() { oidcAuthTime = old })
			idp := oidcTestServer(t, "sub-linked", "yoshi", "y@example.com", "user")
			h, srv := newWeb(t)
			setupAndLogin(t, h)
			_ = srv.o.Store.SetSSOSettings(store.SSOSettings{Enabled: true, IssuerURL: idp.URL, ClientID: "kydns"})
			_ = srv.o.Store.LinkAdminSSO("sub-linked", "yoshi", "y@example.com")

			req := httptest.NewRequest("GET", "/auth/sso/callback?code=mock_code&state=test_state", nil)
			for n, v := range map[string]string{cookieSSOState: "test_state", cookieSSOVerifier: "test_verifier", cookieSSONonce: "test_nonce", cookieSSOLink: "stepup"} {
				req.AddCookie(&http.Cookie{Name: n, Value: v})
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Header().Get("Location") != "/settings/tokens?stepup=stale" || sessionCookie(rec) != nil {
				t.Fatalf("status = %d, location = %q, session = %v", rec.Code, rec.Header().Get("Location"), sessionCookie(rec))
			}
		})
	}
}

// The admin can change a known password from the browser (KyCalendar Q4 A):
// every other session ends, tokens a password could mint are revoked, and
// this browser stays signed in with the new generation.
func TestChangePasswordRevokesSessionsAndUnboundTokens(t *testing.T) {
	h, srv := newWeb(t)
	setupAndLogin(t, h)
	c := loginCookie(t, h)
	other := loginCookie(t, h)
	sess, _ := srv.o.Sessions.Get(c.Value)
	if _, err := srv.o.Registry.CreateToken("cli"); err != nil {
		t.Fatal(err)
	}
	change := func(current, next string) *httptest.ResponseRecorder {
		return postForm(t, h, "/settings/account/password", url.Values{"csrf_token": {sess.CSRF},
			"current_password": {current}, "new_password": {next}, "confirm": {next}}, c)
	}
	if rec := change("wrong-password-here", "a-brand-new-password"); rec.Code != http.StatusBadRequest {
		t.Fatalf("wrong current password: status = %d", rec.Code)
	}
	if rec := change(testPassword, "short"); rec.Code != http.StatusBadRequest {
		t.Fatalf("short password: status = %d", rec.Code)
	}
	rec := change(testPassword, "a-brand-new-password")
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/settings/account" {
		t.Fatalf("status = %d, location %q", rec.Code, rec.Header().Get("Location"))
	}
	fresh := sessionCookie(rec)
	if fresh == nil {
		t.Fatal("this browser was not signed in again")
	}
	if get(t, h, "/settings/account", fresh).Code != http.StatusOK {
		t.Error("the new session does not work")
	}
	for name, ck := range map[string]*http.Cookie{"old": c, "other": other} {
		if _, ok := srv.o.Sessions.Get(ck.Value); ok {
			t.Errorf("%s session survived the change", name)
		}
	}
	if toks, _ := srv.o.Registry.Tokens(); len(toks) != 0 {
		t.Errorf("%d unbound tokens survived the change", len(toks))
	}
	hash, _ := srv.o.Store.AdminHash()
	if !verified(hash, "a-brand-new-password") {
		t.Error("the new password does not verify")
	}
}
