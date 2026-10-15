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
}

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
}

func NewSessions(idle, absolute time.Duration) *Sessions {
	return &Sessions{
		byID: map[string]*Session{}, idle: idle, absolute: absolute, now: time.Now,
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
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	sess := &Session{RecoveryGeneration: generation, IdentityIssuer: issuer, IdentitySubject: subject, IdentityRevision: revision, ID: randomHex(32), CSRF: randomHex(32), Created: now, LastSeen: now}
	s.byID[sess.ID] = sess
	return sess
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
