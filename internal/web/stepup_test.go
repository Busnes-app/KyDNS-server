package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
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

// A stolen session must not choose the SSO provider or link an identity and
// then step up through it: both need the current password.
func TestSSOProviderAndLinkNeedTheCurrentPassword(t *testing.T) {
	idp := oidcTestServer(t, "attacker", "mallory", "m@example.com", "admin")
	h, srv := newWeb(t)
	setupAndLogin(t, h)
	c := loginCookie(t, h)
	sess, _ := srv.o.Sessions.Get(c.Value)
	rec := postForm(t, h, "/settings/sso", url.Values{"csrf_token": {sess.CSRF}, "enabled": {"1"},
		"issuer_url": {idp.URL}, "client_id": {"kydns"}}, c)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("provider change without the password: status = %d", rec.Code)
	}
	if sso, _ := srv.o.Store.SSOSettings(); sso.IssuerURL == idp.URL {
		t.Fatal("the provider changed without the password")
	}
	_ = srv.o.Store.SetSSOSettings(store.SSOSettings{IssuerURL: idp.URL, ClientID: "kydns"})
	rec = postForm(t, h, "/settings/sso/link", url.Values{"csrf_token": {sess.CSRF}}, c)
	if rec.Code != http.StatusBadRequest || strings.HasPrefix(rec.Header().Get("Location"), idp.URL) {
		t.Fatalf("link without the password: status = %d, location %q", rec.Code, rec.Header().Get("Location"))
	}
}

// An unlink landing while a callback is between its link check and its new
// session must still leave that sign-in without a session.
func TestUnlinkDuringAnSSOCallbackLeavesNoSession(t *testing.T) {
	idp := oidcTestServer(t, "sub-linked", "yoshi", "y@example.com", "user")
	h, srv := newWeb(t)
	setupAndLogin(t, h)
	_ = srv.o.Store.SetSSOSettings(store.SSOSettings{Enabled: true, IssuerURL: idp.URL, ClientID: "kydns"})
	_ = srv.o.Store.LinkAdminSSO("sub-linked", "yoshi", "y@example.com")
	beforeSSOSession = func() {
		_ = srv.o.Store.UnlinkAdminSSO()
		srv.o.Sessions.EndLinkedSSO()
	}
	t.Cleanup(func() { beforeSSOSession = nil })
	before := srv.o.Sessions.Len() // setup's own password session
	rec := ssoCallback(t, h, false, nil)
	if rec.Code != http.StatusForbidden || sessionCookie(rec) != nil {
		t.Fatalf("status = %d, session = %v; want 403 and none", rec.Code, sessionCookie(rec))
	}
	if n := srv.o.Sessions.Len(); n != before {
		t.Errorf("%d sessions after the callback, want the %d before it", n, before)
	}
}

// The link callback's purpose cookie is the browser's to set: a stolen session
// that forges it without passing the password check at link start links nothing.
func TestForgedLinkCookieDoesNotLink(t *testing.T) {
	idp := oidcTestServer(t, "attacker", "mallory", "m@example.com", "user")
	h, srv := newWeb(t)
	setupAndLogin(t, h)
	c := loginCookie(t, h)
	_ = srv.o.Store.SetSSOSettings(store.SSOSettings{IssuerURL: idp.URL, ClientID: "kydns"})
	rec := ssoCallback(t, h, true, c)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "error=") {
		t.Fatalf("status = %d, location %q; want the link refused", rec.Code, loc)
	}
	if ident, _ := srv.o.Store.AdminIdentity(); ident.SSOSub != "" {
		t.Fatalf("linked %q through a forged cookie", ident.SSOSub)
	}
}

// The SSO form takes the same step-up as the token form (orchestrator, pass 2):
// a fresh single sign-on login instead of the password, so an administrator
// without a password is not stuck on "That password is not correct".
func TestSSOSettingsAcceptAFreshSSOLogin(t *testing.T) {
	h, srv := newWeb(t)
	setupAndLogin(t, h)
	for name, tc := range map[string]struct {
		authTime time.Time
		want     int
	}{
		"fresh": {time.Now().Add(-time.Minute), http.StatusSeeOther},
		"stale": {time.Now().Add(-time.Hour), http.StatusBadRequest},
	} {
		t.Run(name, func(t *testing.T) {
			sess, _ := srv.o.Sessions.CreateSSOLogin(auth.SSOLogin{Issuer: "https://idp.example", Subject: "owner", AuthTime: tc.authTime},
				"", "", 0, srv.o.Sessions.RecoveryGeneration())
			c := &http.Cookie{Name: auth.CookieName, Value: sess.ID}
			rec := postForm(t, h, "/settings/sso", url.Values{"csrf_token": {sess.CSRF}, "issuer_url": {"https://idp.example"}}, c)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body)
			}
		})
	}
}

// An administrator with no password (installer-managed) sees no password box
// on the SSO tab, gets the single sign-on confirmation instead, and a fresh
// login gets past the step-up to the real answer.
func TestSSOSettingsWithoutAPasswordOffersSSOConfirmation(t *testing.T) {
	h, srv := newWeb(t)
	issuer := "https://identity.example"
	if _, err := srv.o.Store.ApplyInstaller(context.Background(), webInstallerBundle(t, issuer), false); err != nil {
		t.Fatal(err)
	}
	sess, ok := srv.o.Sessions.CreateSSOLogin(auth.SSOLogin{Issuer: issuer, Subject: "owner", AuthTime: time.Now().Add(-time.Minute)},
		issuer, "owner", 0, srv.o.Sessions.RecoveryGeneration())
	if !ok {
		t.Fatal("no session")
	}
	c := &http.Cookie{Name: auth.CookieName, Value: sess.ID}
	body := get(t, h, "/settings/sso", c).Body.String()
	if strings.Contains(body, `id="sso_current"`) || !strings.Contains(body, `name="back" value="sso"`) {
		t.Fatalf("SSO tab for a password-less admin:\n%s", body)
	}
	rec := postForm(t, h, "/settings/sso", url.Values{"csrf_token": {sess.CSRF}, "enabled": {"1"}, "issuer_url": {issuer}}, c)
	if strings.Contains(rec.Body.String(), "That password is not correct") || !strings.Contains(rec.Body.String(), "installer-owned") {
		t.Fatalf("status = %d; the step-up refused a fresh SSO login:\n%s", rec.Code, rec.Body)
	}
}

// Confirming from the SSO tab returns there, also when the provider ignored
// max_age=0. The page is kept on the session server-side: the purpose cookie
// says only "stepup", and editing it cannot choose where the browser lands.
func TestSSOTabStepUpReturnsToTheSSOTab(t *testing.T) {
	idp := oidcTestServer(t, "sub-linked", "yoshi", "y@example.com", "user")
	h, srv := newWeb(t)
	setupAndLogin(t, h)
	_ = srv.o.Store.SetSSOSettings(store.SSOSettings{Enabled: true, IssuerURL: idp.URL, ClientID: "kydns"})
	c := loginCookie(t, h)
	sess, _ := srv.o.Sessions.Get(c.Value)
	rec := postForm(t, h, "/settings/tokens/stepup", url.Values{"csrf_token": {sess.CSRF}, "back": {"sso"}}, c)
	loc, _ := url.Parse(rec.Header().Get("Location"))
	if loc == nil || loc.Query().Get("max_age") != "0" || !strings.Contains(strings.Join(rec.Header().Values("Set-Cookie"), "; "), cookieSSOLink+"=stepup;") {
		t.Fatalf("status = %d, location = %q, cookies %v", rec.Code, rec.Header().Get("Location"), rec.Header().Values("Set-Cookie"))
	}

	old := oidcAuthTime
	oidcAuthTime = func() int64 { return time.Now().Add(-time.Hour).Unix() }
	t.Cleanup(func() { oidcAuthTime = old })
	_ = srv.o.Store.LinkAdminSSO("sub-linked", "yoshi", "y@example.com")
	callback := func(withSession bool) string {
		req := httptest.NewRequest("GET", "/auth/sso/callback?code=mock_code&state=test_state", nil)
		for n, v := range map[string]string{cookieSSOState: "test_state", cookieSSOVerifier: "test_verifier", cookieSSONonce: "test_nonce", cookieSSOLink: "stepup"} {
			req.AddCookie(&http.Cookie{Name: n, Value: v})
		}
		if withSession {
			req.AddCookie(c)
		}
		cb := httptest.NewRecorder()
		h.ServeHTTP(cb, req)
		return cb.Header().Get("Location")
	}
	if got := callback(true); got != "/settings/sso?stepup=stale" {
		t.Fatalf("callback location = %q, want the SSO tab", got)
	}
	// Spent: a second callback, or one without the session, lands on tokens.
	if got := callback(true); got != "/settings/tokens?stepup=stale" {
		t.Fatalf("second callback location = %q, want the token tab", got)
	}
}

// Raising an agent token's access is the same persistence as creating one, so
// it needs the step-up too; lowering it stays one click (owner answer Q3 A).
func TestTokenAccessRaiseNeedsAStepUp(t *testing.T) {
	h, srv, c, csrf := loggedIn(t)
	if _, err := srv.o.Registry.CreateAgentTokenAtGeneration("hermes", store.ScopeMCPRead, false, "", "", srv.o.Sessions.RecoveryGeneration()); err != nil {
		t.Fatal(err)
	}
	toks, _ := srv.o.Registry.Tokens()
	id := strconv.FormatInt(toks[len(toks)-1].ID, 10)
	access := func(scope, queryLog, pw string) int {
		form := url.Values{"id": {id}, "access": {scope}, "query_log": {queryLog}, "csrf_token": {csrf}}
		if pw != "" {
			form.Set("current_password", pw)
		}
		return postForm(t, h, "/settings/tokens/access", form, c).Code
	}
	for _, tc := range []struct{ scope, queryLog string }{{"mcp:write", ""}, {"mcp:read", "on"}} {
		for _, pw := range []string{"", "not-the-password"} {
			if code := access(tc.scope, tc.queryLog, pw); code != http.StatusBadRequest {
				t.Errorf("raise to %s/%q with password %q: status = %d, want 400", tc.scope, tc.queryLog, pw, code)
			}
		}
	}
	if got, _ := srv.o.Registry.Tokens(); got[len(got)-1].Scope != store.ScopeMCPRead || got[len(got)-1].QueryLog {
		t.Fatalf("raised without a step-up: %+v", got[len(got)-1])
	}
	if code := access("mcp:write", "on", testPassword); code != http.StatusSeeOther {
		t.Fatalf("raise with the password: status = %d", code)
	}
	if code := access("mcp:read", "", ""); code != http.StatusSeeOther {
		t.Fatalf("lowering without a password: status = %d, want 303", code)
	}
	if got, _ := srv.o.Registry.Tokens(); got[len(got)-1].Scope != store.ScopeMCPRead || got[len(got)-1].QueryLog {
		t.Fatalf("after lowering: %+v", got[len(got)-1])
	}
}
