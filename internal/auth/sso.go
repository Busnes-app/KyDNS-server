package auth

import (
	"context"
	"crypto/fips140"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// SSOTokenClaims represents the verified claims from KyIdentity, Authentik, Keycloak, or any OIDC provider.
type SSOTokenClaims struct {
	Sub               string         `json:"sub"`
	Username          string         `json:"username"`
	PreferredUsername string         `json:"preferred_username"`
	Email             string         `json:"email"`
	DisplayName       string         `json:"name"`
	Role              string         `json:"role"`
	Roles             []string       `json:"roles"`
	Groups            []string       `json:"groups"`
	Issuer            string         `json:"iss"`
	SID               string         `json:"sid"`
	RawClaims         map[string]any `json:"-"`
}

type OIDCEndpoints struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
}

// SSOClient manages OAuth 2.0 PKCE Authorization Code flow with any OIDC provider.
type SSOClient struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	HTTPClient   *http.Client
}

func NewSSOClient(issuerURL, clientID, clientSecret string) *SSOClient {
	return &SSOClient{
		IssuerURL:    strings.TrimRight(issuerURL, "/"),
		ClientID:     clientID,
		ClientSecret: clientSecret,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// DiscoverEndpoints queries OIDC metadata. Legacy mode retains endpoint
// fallback; the strong profile returns discovery/policy errors to the caller.
func (c *SSOClient) DiscoverEndpoints(ctx context.Context) (OIDCEndpoints, error) {
	defaults := OIDCEndpoints{
		AuthorizationEndpoint: c.IssuerURL + "/oauth/authorize",
		TokenEndpoint:         c.IssuerURL + "/oauth/token",
		UserinfoEndpoint:      c.IssuerURL + "/oauth/userinfo",
	}

	fail := func(err error) (OIDCEndpoints, error) {
		if fips140.Enabled() {
			return OIDCEndpoints{}, err
		}
		return defaults, nil
	}
	if fips140.Enabled() {
		if err := requireOIDCHTTPS(c.IssuerURL); err != nil {
			return fail(err)
		}
	}
	wellKnown := c.IssuerURL + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wellKnown, nil)
	if err != nil {
		return fail(err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.oidcHTTPClient().Do(req)
	if err != nil {
		return fail(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fail(fmt.Errorf("OIDC discovery status %d", resp.StatusCode))
	}

	var disc OIDCEndpoints
	if err := json.NewDecoder(resp.Body).Decode(&disc); err != nil {
		return fail(err)
	}

	if disc.AuthorizationEndpoint == "" {
		disc.AuthorizationEndpoint = defaults.AuthorizationEndpoint
	}
	if disc.TokenEndpoint == "" {
		disc.TokenEndpoint = defaults.TokenEndpoint
	}
	if disc.UserinfoEndpoint == "" {
		disc.UserinfoEndpoint = defaults.UserinfoEndpoint
	}

	if fips140.Enabled() {
		if err := requireOIDCHTTPS(disc.AuthorizationEndpoint, disc.TokenEndpoint, disc.JWKSURI); err != nil {
			return fail(err)
		}
		if disc.UserinfoEndpoint != "" {
			if err := requireOIDCHTTPS(disc.UserinfoEndpoint); err != nil {
				return fail(err)
			}
		}
	}
	return disc, nil
}

// GeneratePKCE creates an RFC 7636 code verifier and S256 code challenge.
func GeneratePKCE() (verifier, challenge string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(buf)
	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])
	return verifier, challenge, nil
}

// GenerateState creates a random state token to protect against CSRF during OAuth flow.
func GenerateState() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// AuthURL constructs the authorization URL using OIDC discovery or default paths.
func (c *SSOClient) AuthURL(redirectURI, state, nonce, codeChallenge string) (string, error) {
	endpoints, err := c.DiscoverEndpoints(context.Background())
	if err != nil {
		return "", err
	}

	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", c.ClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", "openid profile email groups")
	q.Set("state", state)
	q.Set("nonce", nonce)
	q.Set("code_challenge", codeChallenge)
	q.Set("code_challenge_method", "S256")

	sep := "?"
	if strings.Contains(endpoints.AuthorizationEndpoint, "?") {
		sep = "&"
	}

	return fmt.Sprintf("%s%s%s", endpoints.AuthorizationEndpoint, sep, q.Encode()), nil
}

// providerVerifier discovers the provider and builds the ID-token verifier that
// login and back-channel logout share. The returned context carries the client.
func (c *SSOClient) providerVerifier(ctx context.Context) (context.Context, *oidc.Provider, *oidc.IDTokenVerifier, error) {
	if fips140.Enabled() {
		if err := requireOIDCHTTPS(c.IssuerURL); err != nil {
			return nil, nil, nil, err
		}
	}
	client := c.oidcHTTPClient()
	ctx = oidc.ClientContext(ctx, client)
	provider, err := oidc.NewProvider(ctx, c.IssuerURL)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("discover OIDC provider: %w", err)
	}
	var endpoints OIDCEndpoints
	if err := provider.Claims(&endpoints); err != nil {
		return nil, nil, nil, fmt.Errorf("OIDC metadata: %w", err)
	}
	if fips140.Enabled() {
		if err := requireOIDCHTTPS(provider.Endpoint().AuthURL, provider.Endpoint().TokenURL, endpoints.JWKSURI); err != nil {
			return nil, nil, nil, err
		}
		if endpoints.UserinfoEndpoint != "" {
			if err := requireOIDCHTTPS(endpoints.UserinfoEndpoint); err != nil {
				return nil, nil, nil, err
			}
		}
	}
	jwksClient := *client
	base := client.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	jwksClient.Transport = strongJWKSTransport{base: base}
	return ctx, provider, provider.VerifierContext(oidc.ClientContext(ctx, &jwksClient), &oidc.Config{ClientID: c.ClientID}), nil
}

// ExchangeCode exchanges an authorization code with code_verifier for identity claims.
func (c *SSOClient) ExchangeCode(ctx context.Context, redirectURI, code, codeVerifier, nonce string) (*SSOTokenClaims, error) {
	ctx, provider, verifier, err := c.providerVerifier(ctx)
	if err != nil {
		return nil, err
	}
	config := oauth2.Config{
		ClientID: c.ClientID, ClientSecret: c.ClientSecret, Endpoint: provider.Endpoint(),
		RedirectURL: redirectURI, Scopes: []string{oidc.ScopeOpenID, "profile", "email", "groups"},
	}
	token, err := config.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", codeVerifier))
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, fmt.Errorf("token response has no ID token")
	}
	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("verify ID token: %w", err)
	}
	if nonce == "" || subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(nonce)) != 1 {
		return nil, fmt.Errorf("verify ID token: nonce mismatch")
	}
	rawClaims := make(map[string]any)
	if err := idToken.Claims(&rawClaims); err != nil {
		return nil, fmt.Errorf("decode verified ID token claims: %w", err)
	}
	if idToken.Subject == "" {
		return nil, fmt.Errorf("verify ID token: subject is empty")
	}
	// sid comes from the ID token only: back-channel logout names logins by it.
	sid, _ := rawClaims["sid"].(string)
	if userInfo, err := provider.UserInfo(ctx, oauth2.StaticTokenSource(token)); err == nil {
		if userInfo.Subject != idToken.Subject {
			return nil, fmt.Errorf("verify userinfo: subject mismatch")
		}
		var userClaims map[string]any
		if err := userInfo.Claims(&userClaims); err == nil {
			for k, v := range userClaims {
				rawClaims[k] = v
			}
		}
	}
	// Userinfo cannot replace the verified ID-token identity binding.
	rawClaims["iss"] = idToken.Issuer
	rawClaims["sub"] = idToken.Subject
	claims := parseClaimsMap(rawClaims)
	claims.SID = sid
	return claims, nil
}

func parseClaimsMap(m map[string]any) *SSOTokenClaims {
	c := &SSOTokenClaims{
		RawClaims: m,
	}

	if s, ok := m["sub"].(string); ok {
		c.Sub = s
	}
	if u, ok := m["username"].(string); ok {
		c.Username = u
	}
	if pu, ok := m["preferred_username"].(string); ok {
		c.PreferredUsername = pu
	}
	if e, ok := m["email"].(string); ok {
		c.Email = e
	}
	if n, ok := m["name"].(string); ok {
		c.DisplayName = n
	}
	if r, ok := m["role"].(string); ok {
		c.Role = r
	}
	if iss, ok := m["iss"].(string); ok {
		c.Issuer = iss
	}

	// Extract groups (strings or array)
	extractStrings := func(val any) []string {
		var out []string
		switch v := val.(type) {
		case []any:
			for _, item := range v {
				if str, ok := item.(string); ok {
					out = append(out, str)
				}
			}
		case []string:
			out = append(out, v...)
		case string:
			for _, item := range strings.Split(v, ",") {
				if str := strings.TrimSpace(item); str != "" {
					out = append(out, str)
				}
			}
		}
		return out
	}

	if gVal, ok := m["groups"]; ok {
		c.Groups = append(c.Groups, extractStrings(gVal)...)
	}
	if akVal, ok := m["ak_groups"]; ok {
		c.Groups = append(c.Groups, extractStrings(akVal)...)
	}
	if rVal, ok := m["roles"]; ok {
		c.Roles = append(c.Roles, extractStrings(rVal)...)
	}

	// Keycloak realm_access.roles
	if ra, ok := m["realm_access"].(map[string]any); ok {
		if rar, ok := ra["roles"]; ok {
			c.Roles = append(c.Roles, extractStrings(rar)...)
		}
	}

	if c.Username == "" && c.PreferredUsername != "" {
		c.Username = c.PreferredUsername
	}
	if c.Username == "" && c.Email != "" {
		c.Username = c.Email
	}

	return c
}

// Preserve injected roots/timeouts. The transport checks each request, including
// metadata-derived endpoints; the redirect policy prevents HTTPS downgrades.
func (c *SSOClient) oidcHTTPClient() *http.Client {
	if !fips140.Enabled() {
		return c.HTTPClient
	}
	client := *c.HTTPClient
	base := client.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	client.Transport = httpsOIDCTransport{base: base}
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return errors.New("OIDC redirects are refused in strong-crypto mode")
	}
	return &client
}

type httpsOIDCTransport struct{ base http.RoundTripper }

func (t httpsOIDCTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := requireOIDCHTTPS(req.URL.String()); err != nil {
		return nil, err
	}
	return t.base.RoundTrip(req)
}
func requireOIDCHTTPS(endpoints ...string) error {
	for _, raw := range endpoints {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
			return errors.New("strong-crypto mode requires HTTPS OIDC issuer and endpoints without URL credentials")
		}
	}
	return nil
}
