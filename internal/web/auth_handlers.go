package web

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"time"

	"github.com/Busnes-app/kydns-server/internal/auth"
	"github.com/Busnes-app/kydns-server/internal/store"
)

// registrar is the sliver of *http.ServeMux that registration uses. ServeMux
// cannot be enumerated, so this is what lets a test derive the route table
// from the router rather than hand-listing it.
type registrar interface {
	Handle(pattern string, h http.Handler)
	HandleFunc(pattern string, h func(http.ResponseWriter, *http.Request))
}

// Routes registers the web transport on a mux it shares with the API. The mux
// is not where writes are refused: wrap the whole server handler in WriteGate.
func (s *Server) Routes(mux *http.ServeMux) { s.routes(mux) }

func (s *Server) routes(mux registrar) {
	mux.HandleFunc("GET /api/v1/installer/network", s.installerNetwork)
	mux.HandleFunc("POST "+PathInstallerDirectory, s.installerDirectory)
	mux.HandleFunc("GET /setup", s.getSetup)
	mux.HandleFunc("POST "+PathSetup, s.postSetup)
	mux.HandleFunc("GET /login", s.requireSetup(s.getLogin))
	mux.HandleFunc("POST "+PathLogin, s.requireSetup(s.postLogin))
	mux.HandleFunc("POST "+PathLogout, s.requireCSRF(s.postLogout))
	mux.HandleFunc("GET /auth/sso/login", s.getSSOLogin)
	mux.HandleFunc("GET /auth/sso/callback", s.getSSOCallback)
	mux.HandleFunc("POST /auth/sso/backchannel-logout", s.postBackchannelLogout)
	s.pageRoutes(mux)
}

func (s *Server) getSetup(w http.ResponseWriter, r *http.Request) {
	if s.hasAdmin() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	s.renderBare(w, "setup.html", map[string]any{"Title": "Set up KyDNS"})
}

// postSetup consumes the one-time token and creates the admin account. It is
// unauthenticated by necessity, so the token is the whole gate.
func (s *Server) postSetup(w http.ResponseWriter, r *http.Request) {
	if s.hasAdmin() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "malformed form", http.StatusBadRequest)
		return
	}
	fail := func(msg string) {
		w.WriteHeader(http.StatusBadRequest)
		s.renderBare(w, "setup.html", map[string]any{"Title": "Set up KyDNS", "Error": msg})
	}

	// No token was minted when an admin existed at startup, and "" must not
	// match "". Constant-time so the token cannot be recovered by timing.
	given := r.PostFormValue("token")
	if s.o.SetupToken == "" || subtle.ConstantTimeCompare([]byte(given), []byte(s.o.SetupToken)) != 1 {
		s.o.Logger.Warn("setup attempted with an invalid token", "source", s.sourceKey(r))
		fail("That setup token is not correct. Read it from the setup-token file in the KyDNS data directory.")
		return
	}
	password := r.PostFormValue("password")
	if len(password) < minPasswordLen {
		fail("Choose a password of at least 12 characters.")
		return
	}
	if password != r.PostFormValue("confirm") {
		fail("The two passwords do not match.")
		return
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		fail(err.Error())
		return
	}
	if err := s.o.Store.SetAdminPassword(hash); err != nil {
		fail(err.Error())
		return
	}
	s.o.Logger.Info("admin account created")
	// Spent: it must not sit in the data directory, or in its backups.
	if s.o.SetupTokenFile != "" {
		if err := os.Remove(s.o.SetupTokenFile); err != nil && !errors.Is(err, fs.ErrNotExist) {
			s.o.Logger.Warn("could not delete the spent setup token", "path", s.o.SetupTokenFile, "error", err)
		}
	}
	s.setSessionCookie(w, r, s.o.Sessions.Create())
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) getLogin(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.session(r); ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.renderLogin(w, r.URL.Query().Get("error"))
}

// renderLogin shows the sign-in page, offering SSO only when it is switched on.
func (s *Server) renderLogin(w http.ResponseWriter, errMsg string) {
	sso, _ := s.o.Store.SSOSettings()
	s.renderBare(w, "login.html", map[string]any{
		"Title":      "Sign in",
		"SSOEnabled": sso.Enabled,
		"Error":      errMsg,
	})
}

// verifyPassword is a seam so a test can stand in for a saturated checker.
var verifyPassword = auth.VerifyPassword

func (s *Server) postLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "malformed form", http.StatusBadRequest)
		return
	}
	key := s.sourceKey(r)
	// Sleep before answering, so guessing gets slower without ever locking out.
	// The attempt counts before the sleep, so parallel guesses cannot share one
	// delay; a correct password resets it and a busy checker refunds it.
	if d := s.o.Backoff.Attempt(key); d > 0 {
		select {
		case <-time.After(d):
		case <-r.Context().Done():
			return
		}
	}
	generation := s.o.Sessions.RecoveryGeneration()
	hash, err := s.o.Store.AdminHash()
	ok := false
	if err == nil {
		ok, err = verifyPassword(hash, r.PostFormValue("password"))
	}
	if errors.Is(err, auth.ErrBusy) {
		s.o.Backoff.Refund(key)
		w.WriteHeader(http.StatusServiceUnavailable)
		s.renderLogin(w, "KyDNS is busy checking other sign-ins. Try again in a moment.")
		return
	}
	if !ok {
		s.o.Logger.Warn("failed login", "source", key)
		w.WriteHeader(http.StatusUnauthorized)
		s.renderLogin(w, "That password is not correct.")
		return
	}
	s.o.Backoff.Reset(key)
	s.setSessionCookie(w, r, s.o.Sessions.CreateAuthenticated("", "", 0, generation))
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) postLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(auth.CookieName); err == nil {
		s.o.Sessions.Destroy(c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     auth.CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.requestSecure(r),
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

const (
	cookieSSOState    = "kydns_sso_state"
	cookieSSOVerifier = "kydns_sso_verifier"
	cookieSSONonce    = "kydns_sso_nonce"
	// cookieSSOLink carries what the login is for: "1" links, "stepup" confirms.
	// The page a step-up returns to is kept server-side on the session that
	// started it (SetStepUpReturn), never read from a cookie.
	cookieSSOLink = "kydns_sso_link"
)

// beforeSSOSession lets a test land an unlink between a callback's link check
// and its session; nil in production.
var beforeSSOSession func()

// stepUpPages are the tabs a step-up can return to, by the form's back value.
var stepUpPages = map[string]string{
	"": "/settings/tokens", "sso": "/settings/sso", "backups": "/settings/backups",
}

// stepUpWindow is how long a fresh single sign-on login counts as a step-up.
const stepUpWindow = 5 * time.Minute

// stepUp proves the admin is at the browser before a credential that outlives
// the session is minted (KyPost-Server Q2 A): the current password, or a
// single sign-on login made with max_age=0 in the last five minutes and dated
// from its auth_time (KyCalendar Q6 A). Password checks share the sign-in
// backoff, so a stolen session cannot guess through this form faster.
func (s *Server) stepUp(r *http.Request, sess *auth.Session) error {
	if pw := r.PostFormValue("current_password"); pw != "" {
		_, err := s.checkPassword(r, pw)
		return err
	}
	if age := time.Since(sess.SSO.AuthTime); !sess.SSO.AuthTime.IsZero() && age < stepUpWindow && age > -time.Minute {
		return nil
	}
	return errors.New("Enter your current password, or confirm with single sign-on, to continue.")
}

// checkPassword verifies the admin's current password for a signed-in action,
// and returns the hash it verified against.
func (s *Server) checkPassword(r *http.Request, pw string) (string, error) {
	key := s.sourceKey(r)
	if d := s.o.Backoff.Attempt(key); d > 0 {
		select {
		case <-time.After(d):
		case <-r.Context().Done():
			return "", r.Context().Err()
		}
	}
	hash, err := s.o.Store.AdminHash()
	ok := false
	if err == nil {
		ok, err = verifyPassword(hash, pw)
	}
	if errors.Is(err, auth.ErrBusy) {
		s.o.Backoff.Refund(key)
		return "", errors.New("KyDNS is busy checking other sign-ins. Try again in a moment.")
	}
	if !ok {
		s.o.Logger.Warn("failed password check", "source", key)
		return "", errors.New("That password is not correct.")
	}
	s.o.Backoff.Reset(key)
	return hash, nil
}

// postAccountPassword changes the admin's password after checking the current
// one, then signs this browser in again: the change ended every session.
func (s *Server) postAccountPassword(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.session(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	next := r.PostFormValue("new_password")
	if len(next) < minPasswordLen {
		s.settingsError(w, r, errors.New("Choose a password of at least 12 characters."))
		return
	}
	if next != r.PostFormValue("confirm") {
		s.settingsError(w, r, errors.New("The two passwords do not match."))
		return
	}
	verified, err := s.checkPassword(r, r.PostFormValue("current_password"))
	if err != nil {
		s.settingsError(w, r, err)
		return
	}
	hash, err := auth.HashPassword(next)
	if err != nil {
		s.settingsError(w, r, err)
		return
	}
	// Conditional on the hash just verified and this session's generation, so
	// a recovery reset landing in between is never undone by this request.
	generation, err := s.o.Store.ChangeAdminPassword(verified, hash, sess.RecoveryGeneration, time.Now())
	if err != nil {
		s.settingsError(w, r, err)
		return
	}
	s.o.Logger.Info("admin password changed; every session and unbound token was revoked", "source", s.sourceKey(r))
	s.setSessionCookie(w, r, s.o.Sessions.CreateAuthenticated("", "", 0, generation))
	backToSettings(w, r)
}

func (s *Server) getSSOLogin(w http.ResponseWriter, r *http.Request) {
	sso, err := s.o.Store.SSOClient()
	if err != nil || !sso.Enabled {
		http.Redirect(w, r, "/login?error=SSO+is+not+enabled", http.StatusSeeOther)
		return
	}
	s.startSSO(w, r, sso, "")
}

// postSSOLink starts linking an identity to the signed-in admin. It is a
// CSRF-checked POST so no other site can link an account on the admin's behalf.
func (s *Server) postSSOLink(w http.ResponseWriter, r *http.Request) {
	// A linked identity signs in without the password, so it outlives this
	// session: linking one needs the step-up a token does (KyPost-Server Q2 A).
	sess, ok := s.session(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if _, err := s.checkPassword(r, r.PostFormValue("current_password")); err != nil {
		s.settingsError(w, r, err)
		return
	}
	sso, err := s.o.Store.SSOClient()
	if err != nil {
		s.settingsError(w, r, err)
		return
	}
	// The callback links only for a session that passed this check: its
	// purpose cookie alone is the browser's to forge.
	s.o.Sessions.AllowLink(sess.ID, time.Now().Add(stepUpWindow))
	s.startSSO(w, r, sso, "1")
}

// postTokenStepUp signs in again through single sign-on with max_age=0, so
// the token form (or the SSO or backups tab, by back=) accepts the fresh login
// as a step-up.
func (s *Server) postTokenStepUp(w http.ResponseWriter, r *http.Request) {
	sso, err := s.o.Store.SSOClient()
	if err != nil || !sso.Enabled {
		s.settingsError(w, r, errors.New("single sign-on is not enabled"))
		return
	}
	if sess, ok := s.session(r); ok {
		page, ok := stepUpPages[r.PostFormValue("back")]
		if !ok {
			page = stepUpPages[""]
		}
		s.o.Sessions.SetStepUpReturn(sess.ID, page)
	}
	s.startSSO(w, r, sso, "stepup")
}

// startSSO begins a login; purpose is "" to sign in, "1" to link, "stepup" to confirm.
func (s *Server) startSSO(w http.ResponseWriter, r *http.Request, sso store.SSOSettings, purpose string) {
	verifier, challenge, err := auth.GeneratePKCE()
	if err != nil {
		http.Error(w, "failed to generate PKCE challenge", http.StatusInternalServerError)
		return
	}

	state, err := auth.GenerateState()
	if err != nil {
		http.Error(w, "failed to generate state", http.StatusInternalServerError)
		return
	}
	nonce, err := auth.GenerateState()
	if err != nil {
		http.Error(w, "failed to generate nonce", http.StatusInternalServerError)
		return
	}

	s.setSSOCookie(w, r, cookieSSOState, state, 300)
	s.setSSOCookie(w, r, cookieSSOVerifier, verifier, 300)
	s.setSSOCookie(w, r, cookieSSONonce, nonce, 300)
	if purpose != "" {
		s.setSSOCookie(w, r, cookieSSOLink, purpose, 300)
	} else {
		s.clearSSOCookie(w, r, cookieSSOLink)
	}

	client := auth.NewSSOClient(sso.IssuerURL, sso.ClientID, sso.ClientSecret)
	client.FreshLogin = purpose == "stepup"
	redirectURI := s.ssoRedirectURI(r)
	authURL, err := client.AuthURL(redirectURI, state, nonce, challenge)
	if err != nil {
		s.clearSSOCookies(w, r)
		http.Error(w, "Identity provider unavailable or incompatible with the configured security policy", http.StatusBadGateway)
		return
	}
	http.Redirect(w, r, authURL, http.StatusSeeOther)
}

func (s *Server) getSSOCallback(w http.ResponseWriter, r *http.Request) {
	generation := s.o.Sessions.RecoveryGeneration()
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	stateCookie, err := r.Cookie(cookieSSOState)
	if err != nil || stateCookie.Value == "" || stateCookie.Value != state {
		s.o.Logger.Warn("sso state mismatch or missing", "source", s.sourceKey(r))
		s.clearSSOCookies(w, r)
		w.WriteHeader(http.StatusBadRequest)
		sso, _ := s.o.Store.SSOSettings()
		s.renderBare(w, "login.html", map[string]any{
			"Title":      "Sign in",
			"SSOEnabled": sso.Enabled,
			"Error":      "Authentication state expired or invalid. Please try again.",
		})
		return
	}

	verifierCookie, err := r.Cookie(cookieSSOVerifier)
	if err != nil || verifierCookie.Value == "" {
		s.clearSSOCookies(w, r)
		w.WriteHeader(http.StatusBadRequest)
		sso, _ := s.o.Store.SSOSettings()
		s.renderBare(w, "login.html", map[string]any{
			"Title":      "Sign in",
			"SSOEnabled": sso.Enabled,
			"Error":      "Authentication session expired. Please try again.",
		})
		return
	}
	verifier := verifierCookie.Value
	nonceCookie, err := r.Cookie(cookieSSONonce)
	if err != nil || nonceCookie.Value == "" {
		s.clearSSOCookies(w, r)
		w.WriteHeader(http.StatusBadRequest)
		sso, _ := s.o.Store.SSOSettings()
		s.renderBare(w, "login.html", map[string]any{
			"Title": "Sign in", "SSOEnabled": sso.Enabled,
			"Error": "Authentication session expired. Please try again.",
		})
		return
	}
	nonce := nonceCookie.Value

	var isLink, isStepUp bool
	if linkCookie, err := r.Cookie(cookieSSOLink); err == nil {
		isLink, isStepUp = linkCookie.Value == "1", linkCookie.Value == "stepup"
	}
	stepUpPage := stepUpPages[""]
	if old, ok := s.session(r); ok && isStepUp {
		if page := s.o.Sessions.TakeStepUpReturn(old.ID); page != "" {
			stepUpPage = page
		}
	}
	s.clearSSOCookies(w, r)

	sso, err := s.o.Store.SSOClient()
	if err != nil {
		http.Error(w, "failed to load SSO settings", http.StatusInternalServerError)
		return
	}

	client := auth.NewSSOClient(sso.IssuerURL, sso.ClientID, sso.ClientSecret)
	redirectURI := s.ssoRedirectURI(r)

	claims, err := client.ExchangeCode(r.Context(), redirectURI, code, verifier, nonce)
	if err != nil {
		s.o.Logger.Warn("sso code exchange failed", "error", err, "source", s.sourceKey(r))
		w.WriteHeader(http.StatusUnauthorized)
		// The detail is in the log; provider and network errors are not for strangers.
		s.renderLogin(w, "Single sign-on failed. Try again, or sign in with your password.")
		return
	}

	signIn := func(issuer, subject string, revision uint64) {
		login := auth.SSOLogin{Issuer: claims.Issuer, Subject: claims.Sub, SID: claims.SID, IssuedAt: claims.IssuedAt, AuthTime: claims.AuthTime}
		// A step-up proves presence only with a fresh auth_time; without one the
		// provider ignored max_age=0, and the old session stays as it was.
		if isStepUp && (claims.AuthTime.IsZero() || time.Since(claims.AuthTime) > stepUpWindow) {
			s.o.Logger.Warn("sso step-up refused: no fresh auth_time", "sub", claims.Sub)
			http.Redirect(w, r, stepUpPage+"?stepup=stale", http.StatusSeeOther)
			return
		}
		if beforeSSOSession != nil {
			beforeSSOSession()
		}
		sess, ok := s.o.Sessions.CreateSSOLogin(login, issuer, subject, revision, generation)
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			s.renderLogin(w, "This sign-in was ended at the identity provider. Sign in again.")
			return
		}
		// The link was checked before this session existed. Unlinking writes
		// first and ends sessions after, so re-reading now catches an unlink
		// that landed in between, whichever way round the two ran.
		if issuer == "" {
			if cur, err := s.o.Store.AdminIdentity(); err != nil || cur.SSOSub != claims.Sub {
				s.o.Sessions.Destroy(sess.ID)
				w.WriteHeader(http.StatusForbidden)
				s.renderLogin(w, "This account is not linked to the KyDNS admin. Sign in with your password and link it under Settings.")
				return
			}
		}
		s.setSessionCookie(w, r, sess)
		dest := "/"
		if isStepUp {
			dest = stepUpPage
		}
		http.Redirect(w, r, dest, http.StatusSeeOther)
	}
	managed, authority, e := s.o.Store.InstallerAuthority(claims.Issuer, claims.Sub)
	if e != nil {
		http.Error(w, "administrator binding unavailable", 500)
		return
	}
	if managed {
		if isLink || !authority.Allowed {
			http.Error(w, "use the explicitly assigned administrator identity", 403)
			return
		}
		signIn(claims.Issuer, claims.Sub, authority.Revision)
		return
	}
	ident, err := s.o.Store.AdminIdentity()
	if err != nil {
		s.o.Logger.Error("failed to load admin identity", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Linking needs the admin's own session: the link cookie alone is client-held.
	if isLink {
		sess, ok := s.session(r)
		if !ok {
			http.Redirect(w, r, "/login?error=Session+expired+during+linking", http.StatusSeeOther)
			return
		}
		if !s.o.Sessions.TakeLink(sess.ID) {
			s.o.Logger.Warn("sso link refused: not started with the password", "source", s.sourceKey(r))
			http.Redirect(w, r, "/settings/account?error=Confirm+your+password+to+link+an+identity", http.StatusSeeOther)
			return
		}
		if err := s.o.Store.LinkAdminSSOAtGeneration(claims.Sub, claims.Username, claims.Email, sso.IssuerURL, sso.ClientID, sess.RecoveryGeneration); err != nil {
			s.o.Logger.Error("failed to link sso identity", "error", err)
			http.Redirect(w, r, "/settings/account?error=Failed+to+link+identity", http.StatusSeeOther)
			return
		}
		s.o.Logger.Info("sso identity linked to admin", "sub", claims.Sub, "username", claims.Username)
		http.Redirect(w, r, "/settings/account", http.StatusSeeOther)
		return
	}

	// Only the identity the admin linked from a signed-in session may log in.
	// Nothing links on first login: an IdP-side admin role is not ownership.
	if !sso.Enabled || ident.SSOSub == "" || ident.SSOSub != claims.Sub {
		s.o.Logger.Warn("sso login refused: identity not linked", "sub", claims.Sub, "username", claims.Username, "source", s.sourceKey(r))
		w.WriteHeader(http.StatusForbidden)
		s.renderLogin(w, "This account is not linked to the KyDNS admin. Sign in with your password and link it under Settings.")
		return
	}

	s.o.Logger.Info("sso login successful", "sub", claims.Sub, "username", claims.Username)
	signIn("", "", 0)
}

// ssoLogoutFence outlives the five-minute login cookies, so a logout that lands
// while the browser is still finishing that login also ends it.
const ssoLogoutFence = 10 * time.Minute

// postBackchannelLogout is the OpenID Connect Back-Channel Logout receiver: the
// provider POSTs a signed logout_token when a login ends there. It is anonymous,
// since the signature is its authority, and deliberately not rate limited: a
// KyIdentity restore sends a burst, and every refusal spends one of its retries.
// s.logout reuses one discovered verifier, so a POST costs the provider nothing.
func (s *Server) postBackchannelLogout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	sso, err := s.o.Store.SSOSettings()
	if err != nil {
		http.Error(w, "failed to load SSO settings", http.StatusInternalServerError)
		return
	}
	claims, err := s.logout.Verify(r.Context(), auth.NewSSOClient(sso.IssuerURL, sso.ClientID, ""), r.PostFormValue("logout_token"))
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_request"}`))
		return
	}
	fenceUntil := time.Now().Add(ssoLogoutFence)
	if claims.SID != "" {
		s.o.Sessions.EndSSOLogin(claims.Issuer, claims.SID, fenceUntil)
	} else {
		// iat has whole-second precision: a login from that same second ends too.
		s.o.Sessions.EndSSOSubject(claims.Issuer, claims.Subject, claims.IssuedAt.Add(time.Second), fenceUntil)
	}
	_ = s.o.Store.RecordAudit(store.AuditEvent{Actor: "sso", Action: "auth.sso_backchannel_logout",
		Resource: "session", IP: s.sourceKey(r), Outcome: "success"})
	w.WriteHeader(http.StatusOK)
}

func (s *Server) ssoRedirectURI(r *http.Request) string {
	if b, managed, err := s.o.Store.InstallerConfiguration(); err == nil && managed {
		return b.SSO.RedirectURI
	}

	scheme := "http"
	if s.requestSecure(r) {
		scheme = "https"
	}
	host := r.Host

	return fmt.Sprintf("%s://%s/auth/sso/callback", scheme, host)
}

func (s *Server) setSSOCookie(w http.ResponseWriter, r *http.Request, name, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/auth/sso/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.requestSecure(r),
	})
}

func (s *Server) clearSSOCookie(w http.ResponseWriter, r *http.Request, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/auth/sso/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.requestSecure(r),
	})
}

func (s *Server) clearSSOCookies(w http.ResponseWriter, r *http.Request) {
	s.clearSSOCookie(w, r, cookieSSOState)
	s.clearSSOCookie(w, r, cookieSSOVerifier)
	s.clearSSOCookie(w, r, cookieSSONonce)
	s.clearSSOCookie(w, r, cookieSSOLink)
}
