package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func signJWT(t *testing.T, key *rsa.PrivateKey, header map[string]string, claims map[string]any) string {
	t.Helper()
	h, err := json.Marshal(header)
	if err != nil {
		t.Fatal(err)
	}
	c, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	input := base64.RawURLEncoding.EncodeToString(h) + "." + base64.RawURLEncoding.EncodeToString(c)
	digest := sha256.Sum256([]byte(input))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return input + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func TestVerifyLogoutToken(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var issuer string
	idp := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer": issuer, "authorization_endpoint": issuer + "/authorize",
				"token_endpoint": issuer + "/token", "jwks_uri": issuer + "/keys",
				"id_token_signing_alg_values_supported": []string{"RS256"},
			})
		case "/keys":
			_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]any{
				"kty": "RSA", "alg": "RS256", "kid": "k",
				"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": "AQAB",
			}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer idp.Close()
	issuer = idp.URL
	client := NewSSOClient(issuer, "kydns", "")
	client.HTTPClient = idp.Client()
	ctx := context.Background()
	var verifier LogoutVerifier

	valid := func() map[string]any {
		return map[string]any{
			"iss": issuer, "aud": "kydns", "sub": "subject", "sid": "sid-1", "jti": "jti-1",
			"iat": time.Now().Unix(), "exp": time.Now().Add(2 * time.Minute).Unix(),
			"events": map[string]any{backchannelLogoutEvent: map[string]any{}},
		}
	}
	header := func() map[string]string { return map[string]string{"alg": "RS256", "kid": "k", "typ": "logout+jwt"} }

	got, err := verifier.Verify(ctx, client, signJWT(t, key, header(), valid()))
	if err != nil || got.SID != "sid-1" || got.Subject != "subject" || got.Issuer != issuer || got.IssuedAt.IsZero() {
		t.Fatalf("valid token: claims=%+v error=%v", got, err)
	}
	subOnly := valid()
	delete(subOnly, "sid")
	if got, err := verifier.Verify(ctx, client, signJWT(t, key, map[string]string{"alg": "RS256", "kid": "k"}, subOnly)); err != nil || got.SID != "" || got.Subject != "subject" {
		t.Fatalf("untyped subject-wide token: claims=%+v error=%v", got, err)
	}

	type mutation func(h map[string]string, c map[string]any) *rsa.PrivateKey
	cases := map[string]mutation{
		"foreign key":    func(h map[string]string, c map[string]any) *rsa.PrivateKey { return other },
		"wrong audience": func(h map[string]string, c map[string]any) *rsa.PrivateKey { c["aud"] = "other"; return key },
		"wrong issuer": func(h map[string]string, c map[string]any) *rsa.PrivateKey {
			c["iss"] = "https://evil.example"
			return key
		},
		"expired": func(h map[string]string, c map[string]any) *rsa.PrivateKey {
			c["exp"] = time.Now().Add(-time.Minute).Unix()
			return key
		},
		"no exp":           func(h map[string]string, c map[string]any) *rsa.PrivateKey { delete(c, "exp"); return key },
		"no iat":           func(h map[string]string, c map[string]any) *rsa.PrivateKey { delete(c, "iat"); return key },
		"nonce (ID token)": func(h map[string]string, c map[string]any) *rsa.PrivateKey { c["nonce"] = "n"; return key },
		"no events":        func(h map[string]string, c map[string]any) *rsa.PrivateKey { delete(c, "events"); return key },
		"other event": func(h map[string]string, c map[string]any) *rsa.PrivateKey {
			c["events"] = map[string]any{"x": map[string]any{}}
			return key
		},
		"event not object": func(h map[string]string, c map[string]any) *rsa.PrivateKey {
			c["events"] = map[string]any{backchannelLogoutEvent: "x"}
			return key
		},
		"event null": func(h map[string]string, c map[string]any) *rsa.PrivateKey {
			c["events"] = map[string]any{backchannelLogoutEvent: nil}
			return key
		},
		"no sid or sub": func(h map[string]string, c map[string]any) *rsa.PrivateKey {
			delete(c, "sid")
			delete(c, "sub")
			return key
		},
		"JWT typ":  func(h map[string]string, c map[string]any) *rsa.PrivateKey { h["typ"] = "JWT"; return key },
		"alg none": func(h map[string]string, c map[string]any) *rsa.PrivateKey { h["alg"] = "none"; return key },
		"ID token": func(h map[string]string, c map[string]any) *rsa.PrivateKey {
			h["typ"] = "JWT"
			delete(c, "events")
			c["nonce"] = "n"
			return key
		},
	}
	raw := map[string]string{"garbage": "not.a.jwt", "empty": "", "two parts": "a.b"}
	for name, mutate := range cases {
		h, c := header(), valid()
		raw[name] = signJWT(t, mutate(h, c), h, c)
	}
	for name, token := range raw {
		t.Run(name, func(t *testing.T) {
			if got, err := verifier.Verify(ctx, client, token); !errors.Is(err, ErrInvalidLogoutToken) || got != nil {
				t.Fatalf("accepted: claims=%+v error=%v", got, err)
			}
		})
	}
}

// A failed discovery is answered from cache for a while, then retried.
func TestLogoutVerifierRetriesFailedDiscovery(t *testing.T) {
	var discoveries atomic.Int32
	idp := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		discoveries.Add(1)
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	defer idp.Close()
	client := NewSSOClient(idp.URL, "kydns", "")
	client.HTTPClient = idp.Client()
	var verifier LogoutVerifier
	for i := 0; i < 5; i++ {
		if _, err := verifier.Verify(context.Background(), client, "a.b.c"); !errors.Is(err, ErrInvalidLogoutToken) {
			t.Fatalf("verify with the provider down = %v", err)
		}
	}
	if n := discoveries.Load(); n != 1 {
		t.Fatalf("5 POSTs during an outage made %d discoveries, want 1", n)
	}
	verifier.at = verifier.at.Add(-logoutDiscoveryRetry)
	_, _ = verifier.Verify(context.Background(), client, "a.b.c")
	if n := discoveries.Load(); n != 2 {
		t.Fatalf("after the retry interval: %d discoveries, want 2", n)
	}
}

func TestBackchannelLogoutScopesSessions(t *testing.T) {
	s := NewSessions(time.Hour, time.Hour)
	login := func(iss, sub, sid string) *Session {
		sess, ok := s.CreateSSOLogin(SSOLogin{Issuer: iss, Subject: sub, SID: sid, IssuedAt: time.Now()}, "", "", 0, time.Time{})
		if !ok {
			t.Fatalf("login %s/%s/%s refused", iss, sub, sid)
		}
		return sess
	}
	a1, a2 := login("https://a", "alice", "x"), login("https://a", "alice", "y")
	b := login("https://b", "alice", "x")
	password := s.Create()

	s.EndSSOLogin("https://a", "x", time.Now().Add(time.Minute))
	if _, ok := s.Get(a1.ID); ok {
		t.Fatal("session survived its logout")
	}
	if _, ok := s.Get(b.ID); !ok {
		t.Fatal("another issuer's logout with the same sid ended this session")
	}
	if _, ok := s.CreateSSOLogin(SSOLogin{Issuer: "https://a", Subject: "alice", SID: "x"}, "", "", 0, time.Time{}); ok {
		t.Fatal("a fenced sid signed in again")
	}
	login("https://b", "alice", "x") // the fence is per issuer

	fence := time.Now().Add(time.Minute)
	s.EndSSOSubject("https://a", "alice", a2.SSO.IssuedAt, fence)
	if _, ok := s.Get(a2.ID); !ok {
		t.Fatal("subject-wide logout ended a login issued after it")
	}
	s.EndSSOSubject("https://a", "alice", a2.SSO.IssuedAt.Add(time.Second), fence)
	if _, ok := s.Get(a2.ID); ok {
		t.Fatal("subject-wide logout missed a login issued before it")
	}
	// The cutoff is fenced, and an older logout arriving late does not move it back.
	s.EndSSOSubject("https://a", "alice", a2.SSO.IssuedAt.Add(-time.Hour), fence)
	if _, ok := s.CreateSSOLogin(SSOLogin{Issuer: "https://a", Subject: "alice", SID: "z", IssuedAt: a2.SSO.IssuedAt}, "", "", 0, time.Time{}); ok {
		t.Fatal("a login issued before the subject-wide logout signed in")
	}
	if _, ok := s.CreateSSOLogin(SSOLogin{Issuer: "https://a", Subject: "alice", SID: "z", IssuedAt: a2.SSO.IssuedAt.Add(2 * time.Second)}, "", "", 0, time.Time{}); !ok {
		t.Fatal("a login issued after the subject-wide logout was refused")
	}
	login("https://b", "alice", "z") // the fence is per issuer
	if _, ok := s.Get(b.ID); !ok {
		t.Fatal("subject-wide logout reached another issuer")
	}
	if _, ok := s.Get(password.ID); !ok {
		t.Fatal("a back-channel logout ended a password session")
	}
}
