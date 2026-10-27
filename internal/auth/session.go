package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"sync"
	"time"
)

// CookieName is the session cookie. It is set HttpOnly and SameSite=Lax, and
// Secure whenever the request arrived over TLS.
const CookieName = "kydns_session"

type Session struct {
	IdentityIssuer  string
	IdentitySubject string
	// IdentityRevision is the directory revision at login; see SetIdentityValidator.
	IdentityRevision   uint64
	ID                 string
	CSRF               string
	RecoveryGeneration time.Time
	Created            time.Time
	LastSeen           time.Time
	// SSO is the provider login behind the session; zero for a password login.
	SSO SSOLogin
}

// SSOLogin is what a back-channel logout matches. The issuer scopes the sid and
// subject, so one provider's logout cannot end another provider's logins.
type SSOLogin struct{ Issuer, Subject, SID string }

// Sessions is an in-memory session store. Restart forces re-login, which is
// the deliberate trade: session state never touches the database or backups,
// and a homelab DNS server restarts rarely.
type Sessions struct {
	mu              sync.Mutex
	byID            map[string]*Session
	idle            time.Duration
	absolute        time.Duration
	now             func() time.Time
	identityAllowed func(issuer, subject string, revision uint64) bool
	notBefore       func() time.Time
	// ended fences logged-out sids (issuer, sid) until the time stored.
	ended map[[2]string]time.Time
}

func NewSessions(idle, absolute time.Duration) *Sessions {
	return &Sessions{
		byID: map[string]*Session{}, idle: idle, absolute: absolute, now: time.Now, ended: map[[2]string]time.Time{},
	}
}

func randomHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(buf)
}

func (s *Sessions) Create() *Session { return s.CreateSSO("", "", 0) }

// SetIdentityValidator checks a managed session's current authority on every Get,
// including that no revocation happened after its login revision.
func (s *Sessions) SetIdentityValidator(fn func(issuer, subject string, revision uint64) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.identityAllowed = fn
}

// SetNotBefore supplies the persisted recovery generation checked on every Get.
// It lets a password reset in another process end this one's sessions.
func (s *Sessions) SetNotBefore(fn func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.notBefore = fn
}

// RecoveryGeneration must be captured before reading credentials or exchanging
// an SSO code. A reset during authentication then invalidates its result too.
func (s *Sessions) RecoveryGeneration() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.notBefore != nil {
		return s.notBefore()
	}
	return time.Time{}
}

func (s *Sessions) CreateSSO(issuer, subject string, revision uint64) *Session {
	return s.CreateAuthenticated(issuer, subject, revision, s.RecoveryGeneration())
}

// CreateAuthenticated binds the session to the generation authentication began in.
func (s *Sessions) CreateAuthenticated(issuer, subject string, revision uint64, generation time.Time) *Session {
	sess, _ := s.CreateSSOLogin(SSOLogin{}, issuer, subject, revision, generation)
	return sess
}

// CreateSSOLogin is CreateAuthenticated for a provider login. It refuses a login
// whose sid a back-channel logout already ended; the check and the insert share
// the lock, so a logout racing the callback either is seen here or sees the session.
func (s *Sessions) CreateSSOLogin(login SSOLogin, issuer, subject string, revision uint64, generation time.Time) (*Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if login.SID != "" && now.Before(s.ended[[2]string{login.Issuer, login.SID}]) {
		return nil, false
	}
	sess := &Session{RecoveryGeneration: generation, IdentityIssuer: issuer, IdentitySubject: subject, IdentityRevision: revision, SSO: login, ID: randomHex(32), CSRF: randomHex(32), Created: now, LastSeen: now}
	s.byID[sess.ID] = sess
	return sess, true
}

// EndSSOLogin ends every session of the issuer's sid and fences the sid until
// fenceUntil, so a callback still in flight for it cannot sign in.
func (s *Sessions) EndSSOLogin(issuer, sid string, fenceUntil time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for k, until := range s.ended {
		if !now.Before(until) {
			delete(s.ended, k)
		}
	}
	s.ended[[2]string{issuer, sid}] = fenceUntil
	for id, sess := range s.byID {
		if sess.SSO.Issuer == issuer && sess.SSO.SID == sid {
			delete(s.byID, id)
		}
	}
}

// EndSSOSubject ends the issuer's subject's sessions created before before.
func (s *Sessions) EndSSOSubject(issuer, subject string, before time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, sess := range s.byID {
		if sess.SSO.Issuer == issuer && sess.SSO.Subject == subject && sess.Created.Before(before) {
			delete(s.byID, id)
		}
	}
}

// Get returns a live session and slides its idle window. Both the idle and
// absolute deadlines are enforced here so there is one expiry path, and an
// expired session is dropped rather than merely hidden.
func (s *Sessions) Get(id string) (*Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.byID[id]
	if !ok {
		return nil, false
	}
	now := s.now()
	if now.Sub(sess.LastSeen) > s.idle || now.Sub(sess.Created) > s.absolute ||
		(s.notBefore != nil && !sess.RecoveryGeneration.Equal(s.notBefore())) || (sess.IdentitySubject != "" && (s.identityAllowed == nil || !s.identityAllowed(sess.IdentityIssuer, sess.IdentitySubject, sess.IdentityRevision))) {
		delete(s.byID, id)
		return nil, false
	}
	sess.LastSeen = now
	return sess, true
}

func (s *Sessions) Destroy(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.byID, id)
}

func (s *Sessions) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.byID)
}

// ValidCSRF checks a form token against its own session, in constant time.
func (s *Sessions) ValidCSRF(id, token string) bool {
	sess, ok := s.Get(id)
	if !ok || token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(sess.CSRF), []byte(token)) == 1
}
