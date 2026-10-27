package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

const backchannelLogoutEvent = "http://schemas.openid.net/event/backchannel-logout"

// ErrInvalidLogoutToken is every reason a back-channel logout token is refused.
var ErrInvalidLogoutToken = errors.New("invalid logout token")

// LogoutClaims names the provider login a back-channel logout token ended.
// SID is empty when the provider ended every login of Subject issued by IssuedAt.
type LogoutClaims struct {
	Issuer, Subject, SID string
	IssuedAt             time.Time
}

// logoutDiscoveryRetry is how long a failed discovery is answered from cache.
const logoutDiscoveryRetry = 30 * time.Second

// LogoutVerifier keeps one discovered verifier for the anonymous logout receiver,
// so strangers' POSTs do not become calls to the provider: discovery runs once per
// (issuer, client ID), the key set stays cached, and a failed discovery is retried
// at most every logoutDiscoveryRetry. The zero value is ready to use.
type LogoutVerifier struct {
	mu               sync.Mutex
	issuer, clientID string
	verifier         *oidc.IDTokenVerifier
	at               time.Time
}

func (v *LogoutVerifier) current(c *SSOClient) *oidc.IDTokenVerifier {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.issuer == c.IssuerURL && v.clientID == c.ClientID && (v.verifier != nil || time.Since(v.at) < logoutDiscoveryRetry) {
		return v.verifier
	}
	// Not the request's context: one caller hanging up must not fail discovery for
	// the callers after it. The client's timeout bounds it.
	_, _, verifier, err := c.providerVerifier(context.Background())
	if err != nil {
		verifier = nil
	}
	v.issuer, v.clientID, v.verifier, v.at = c.IssuerURL, c.ClientID, verifier, time.Now()
	return verifier
}

// Verify checks a logout token per OpenID Connect Back-Channel Logout 1.0 §2.6:
// the ID-token checks (signature, issuer, audience, expiry), then the claims that
// keep an ID token from being replayed as one.
func (v *LogoutVerifier) Verify(ctx context.Context, c *SSOClient, raw string) (*LogoutClaims, error) {
	verifier := v.current(c)
	if verifier == nil {
		return nil, ErrInvalidLogoutToken
	}
	token, err := verifier.Verify(ctx, raw)
	if err != nil {
		return nil, ErrInvalidLogoutToken
	}
	if typ, err := jwtType(raw); err != nil || (typ != "" && typ != "logout+jwt" && typ != "application/logout+jwt") {
		return nil, ErrInvalidLogoutToken
	}
	var claims struct {
		Sub    string                     `json:"sub"`
		SID    string                     `json:"sid"`
		IAT    int64                      `json:"iat"`
		Nonce  *json.RawMessage           `json:"nonce"`
		Events map[string]json.RawMessage `json:"events"`
	}
	if err := token.Claims(&claims); err != nil || claims.Nonce != nil || claims.IAT == 0 || (claims.Sub == "" && claims.SID == "") {
		return nil, ErrInvalidLogoutToken
	}
	var event map[string]any
	if err := json.Unmarshal(claims.Events[backchannelLogoutEvent], &event); err != nil || event == nil {
		return nil, ErrInvalidLogoutToken
	}
	return &LogoutClaims{Issuer: token.Issuer, Subject: claims.Sub, SID: claims.SID, IssuedAt: time.Unix(claims.IAT, 0)}, nil
}

// jwtType reads the protected header's typ; call it only after the signature verified.
func jwtType(raw string) (string, error) {
	header, _, _ := strings.Cut(raw, ".")
	decoded, err := base64.RawURLEncoding.DecodeString(header)
	if err != nil {
		return "", err
	}
	var h struct {
		Typ string `json:"typ"`
	}
	err = json.Unmarshal(decoded, &h)
	return strings.ToLower(h.Typ), err
}
