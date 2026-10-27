package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
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

// VerifyLogoutToken checks a logout token per OpenID Connect Back-Channel Logout 1.0
// §2.6: the ID-token checks (signature, issuer, audience, expiry), then the claims
// that keep an ID token from being replayed as one.
func (c *SSOClient) VerifyLogoutToken(ctx context.Context, raw string) (*LogoutClaims, error) {
	ctx, _, verifier, err := c.providerVerifier(ctx)
	if err != nil {
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
