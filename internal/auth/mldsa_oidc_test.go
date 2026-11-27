package auth

import (
	"context"
	"crypto/mldsa"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

func TestSSOClientVerifiesMLDSA65AndMixedJWKS(t *testing.T) {
	mlKey, err := mldsa.GenerateKey(mldsa.MLDSA65())
	if err != nil {
		t.Fatalf("generate ML-DSA-65 key: %v", err)
	}
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}

	const (
		mlKID  = "mldsa-dns-1"
		rsaKID = "rsa-dns-1"
	)

	var issuer string
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                issuer,
			"authorization_endpoint":                issuer + "/authorize",
			"token_endpoint":                        issuer + "/token",
			"jwks_uri":                              issuer + "/jwks",
			"id_token_signing_alg_values_supported": []string{"ML-DSA-65", "RS256"},
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{
			Keys: []jose.JSONWebKey{
				{Key: mlKey.PublicKey(), KeyID: mlKID, Algorithm: string(jose.MLDSA65), Use: "sig"},
				{Key: &rsaKey.PublicKey, KeyID: rsaKID, Algorithm: string(jose.RS256), Use: "sig"},
			},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	issuer = srv.URL

	client := NewSSOClient(issuer, "kydns-client", "secret")
	client.HTTPClient = srv.Client()

	ctx, _, verifier, err := client.providerVerifier(context.Background())
	if err != nil {
		t.Fatalf("providerVerifier: %v", err)
	}

	now := time.Now().UTC()
	claims, err := json.Marshal(map[string]any{
		"iss":   issuer,
		"sub":   "dns-user-1",
		"aud":   "kydns-client",
		"exp":   now.Add(10 * time.Minute).Unix(),
		"iat":   now.Unix(),
		"nonce": "nonce-dns",
	})
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}

	sign := func(alg jose.SignatureAlgorithm, key any, kid string) string {
		t.Helper()
		signer, err := jose.NewSigner(
			jose.SigningKey{Algorithm: alg, Key: key},
			(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", kid),
		)
		if err != nil {
			t.Fatalf("new signer %s: %v", alg, err)
		}
		jws, err := signer.Sign(claims)
		if err != nil {
			t.Fatalf("sign %s: %v", alg, err)
		}
		tok, err := jws.CompactSerialize()
		if err != nil {
			t.Fatalf("compact serialize %s: %v", alg, err)
		}
		return tok
	}

	mlToken := sign(jose.MLDSA65, mlKey, mlKID)
	idTok, err := verifier.Verify(ctx, mlToken)
	if err != nil {
		t.Fatalf("verify ML-DSA-65 ID token: %v", err)
	}
	if idTok.Subject != "dns-user-1" {
		t.Fatalf("subject = %q, want dns-user-1", idTok.Subject)
	}

	rsaToken := sign(jose.RS256, rsaKey, rsaKID)
	if _, err := verifier.Verify(ctx, rsaToken); err != nil {
		t.Fatalf("verify RS256 ID token: %v", err)
	}

	tampered := mlToken[:len(mlToken)-4] + "AAAA"
	if tampered == mlToken {
		tampered = mlToken[:len(mlToken)-4] + "BBBB"
	}
	if _, err := verifier.Verify(ctx, tampered); err == nil {
		t.Fatal("tampered ML-DSA-65 token unexpectedly verified")
	}

	confused := sign(jose.RS256, rsaKey, mlKID)
	if _, err := verifier.Verify(ctx, confused); err == nil {
		t.Fatal("cross-algorithm kid-confused token unexpectedly verified")
	}
}

func TestStrongJWKSTransportRejectsMalformedMLDSA65(t *testing.T) {
	badPub := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	validLenPub := base64.RawURLEncoding.EncodeToString(make([]byte, 1952))
	cases := []struct {
		name string
		body string
	}{
		{
			name: "unsupported AKP alg",
			body: `{"keys":[{"kty":"AKP","alg":"ML-DSA-44","kid":"k1","pub":"` + validLenPub + `"}]}`,
		},
		{
			name: "non-sig use",
			body: `{"keys":[{"kty":"AKP","alg":"ML-DSA-65","use":"enc","kid":"k1","pub":"` + validLenPub + `"}]}`,
		},
		{
			name: "AKP with RSA params",
			body: `{"keys":[{"kty":"AKP","alg":"ML-DSA-65","kid":"k1","n":"AAAA","pub":"` + validLenPub + `"}]}`,
		},
		{
			name: "wrong pub length",
			body: `{"keys":[{"kty":"AKP","alg":"ML-DSA-65","kid":"k1","pub":"` + badPub + `"}]}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			c := &http.Client{Transport: strongJWKSTransport{base: srv.Client().Transport}}
			if _, err := c.Get(srv.URL); err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
		})
	}
}
