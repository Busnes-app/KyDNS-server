package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Real signatures avoid relying on a JOSE signer's minimum-key policy to test
// the application's verification boundary.
func TestOIDCSigningKeyFloor(t *testing.T) {
	for _, bits := range []int{1024, 2047, 2048} {
		t.Run(fmt.Sprint(bits), func(t *testing.T) {
			key, err := rsa.GenerateKey(rand.Reader, bits)
			if err != nil {
				t.Fatal(err)
			}
			for _, variant := range []struct {
				name, before, after string
			}{
				{name: "canonical"},
				{name: "uppercase type", after: `,"KTY":"EC"`},
				{name: "mixed-case type", after: `,"Kty":"EC"`},
				{name: "uppercase modulus", after: `,"N":"` + base64.RawURLEncoding.EncodeToString(new(big.Int).Lsh(big.NewInt(1), 2047).Bytes()) + `"`},
				{name: "uppercase short modulus", after: `,"N":"AQ"`},
				{name: "aliases before canonical", before: `"KTY":"EC","N":"AQ",`},
				{name: "type and modulus aliases", after: `,"KTY":"EC","N":"AQ"`},
			} {
				t.Run(variant.name, func(t *testing.T) {
					var issuer, token string
					srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						switch r.URL.Path {
						case "/.well-known/openid-configuration":
							_ = json.NewEncoder(w).Encode(map[string]any{"issuer": issuer, "authorization_endpoint": issuer + "/authorize", "token_endpoint": issuer + "/token", "jwks_uri": issuer + "/keys", "id_token_signing_alg_values_supported": []string{"RS256"}})
						case "/keys":
							jwk, err := json.Marshal(map[string]string{"kty": "RSA", "kid": "fixture", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())})
							if err != nil {
								t.Error(err)
								return
							}
							_, _ = fmt.Fprintf(w, `{"keys":[{%s%s%s}]}`, variant.before, jwk[1:len(jwk)-1], variant.after)
						case "/token":
							_ = json.NewEncoder(w).Encode(map[string]string{"id_token": token, "access_token": "fixture", "token_type": "Bearer"})
						default:
							http.NotFound(w, r)
						}
					}))
					defer srv.Close()
					issuer = srv.URL
					header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","kid":"fixture"}`))
					claims, err := json.Marshal(map[string]any{"iss": issuer, "sub": "owner", "aud": "client", "nonce": "nonce", "exp": time.Now().Add(time.Hour).Unix()})
					if err != nil {
						t.Fatal(err)
					}
					input := header + "." + base64.RawURLEncoding.EncodeToString(claims)
					digest := sha256.Sum256([]byte(input))
					sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
					if err != nil {
						t.Fatal(err)
					}
					token = input + "." + base64.RawURLEncoding.EncodeToString(sig)
					client := NewSSOClient(issuer, "client", "")
					client.HTTPClient = srv.Client()
					got, err := client.ExchangeCode(context.Background(), "https://dns.example/auth/sso/callback", "code", "verifier", "nonce")
					if bits < 2048 {
						if err == nil || got != nil {
							t.Fatalf("accepted RSA-%d signed ID token: %+v", bits, got)
						}
						if !strings.Contains(err.Error(), "2048") {
							t.Fatalf("unexpected refusal: %v", err)
						}
					} else if err != nil || got.Sub != "owner" {
						t.Fatalf("strong token: claims=%+v err=%v", got, err)
					}
				})
			}
		})
	}
}
