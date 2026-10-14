package web

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
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
	if d := s.o.Backoff.Delay(key); d > 0 {
		time.Sleep(d)
	}
	hash, err := s.o.Store.AdminHash()
	ok := false
	if err == nil {
		ok, err = verifyPassword(hash, r.PostFormValue("password"))
	}
	if errors.Is(err, auth.ErrBusy) {
		w.WriteHeader(http.StatusServiceUnavailable)
		s.renderLogin(w, "KyDNS is busy checking other sign-ins. Try again in a moment.")
		return
	}
	if !ok {
		s.o.Backoff.Fail(key)
		s.o.Logger.Warn("failed login", "source", key)
		w.WriteHeader(http.StatusUnauthorized)
		s.renderLogin(w, "That password is not correct.")
		return
	}
	s.o.Backoff.Reset(key)
	s.setSessionCookie(w, r, s.o.Sessions.Create())
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
	cookieSSOLink     = "kydns_sso_link"
)

func (s *Server) getSSOLogin(w http.ResponseWriter, r *http.Request) {
	sso, err := s.o.Store.SSOClient()
	if err != nil || !sso.Enabled {
		http.Redirect(w, r, "/login?error=SSO+is+not+enabled", http.StatusSeeOther)
		return
	}
	s.startSSO(w, r, sso, false)
}

// postSSOLink starts linking an identity to the signed-in admin. It is a
// CSRF-checked POST so no other site can link an account on the admin's behalf.
func (s *Server) postSSOLink(w http.ResponseWriter, r *http.Request) {
	sso, err := s.o.Store.SSOClient()
	if err != nil {
		s.settingsError(w, r, err)
		return
	}
	s.startSSO(w, r, sso, true)
}

func (s *Server) startSSO(w http.ResponseWriter, r *http.Request, sso store.SSOSettings, isLink bool) {
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
	if isLink {
		s.setSSOCookie(w, r, cookieSSOLink, "1", 300)
	} else {
		s.clearSSOCookie(w, r, cookieSSOLink)
	}

	client := auth.NewSSOClient(sso.IssuerURL, sso.ClientID, sso.ClientSecret)
	redirectURI := s.ssoRedirectURI(r)
	authURL := client.AuthURL(redirectURI, state, nonce, challenge)
	http.Redirect(w, r, authURL, http.StatusSeeOther)
}

func (s *Server) getSSOCallback(w http.ResponseWriter, r *http.Request) {
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

	var isLink bool
	if linkCookie, err := r.Cookie(cookieSSOLink); err == nil && linkCookie.Value == "1" {
		isLink = true
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
		s.setSessionCookie(w, r, s.o.Sessions.CreateSSO(claims.Issuer, claims.Sub, authority.Revision))
		http.Redirect(w, r, "/", http.StatusSeeOther)
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
		if _, ok := s.session(r); !ok {
			http.Redirect(w, r, "/login?error=Session+expired+during+linking", http.StatusSeeOther)
			return
		}
		if err := s.o.Store.LinkAdminSSO(claims.Sub, claims.Username, claims.Email); err != nil {
			s.o.Logger.Error("failed to link sso identity", "error", err)
			http.Redirect(w, r, "/settings?error=Failed+to+link+identity", http.StatusSeeOther)
			return
		}
		sso.Enabled = true
		if err := s.o.Store.SetSSOSettings(sso); err != nil {
			s.o.Logger.Error("failed to enable sso after linking", "error", err)
		}
		s.o.Logger.Info("sso identity linked to admin", "sub", claims.Sub, "username", claims.Username)
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
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
	s.setSessionCookie(w, r, s.o.Sessions.Create())
	http.Redirect(w, r, "/", http.StatusSeeOther)
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
