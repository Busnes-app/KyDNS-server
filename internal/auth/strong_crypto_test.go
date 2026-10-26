package auth

import (
	"context"
	"crypto/fips140"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestStrongOIDCRequiresHTTPSAndRefusesRedirects(t *testing.T) {
	for _, field := range []string{"issuer", "authorization_endpoint", "token_endpoint", "jwks_uri", "userinfo_endpoint", "discovery redirect"} {
		t.Run(field, func(t *testing.T) {
			var issuer string
			var requests atomic.Int32
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if field == "discovery redirect" && r.URL.Path != "/discovery" {
					http.Redirect(w, r, "/discovery", 302)
					return
				}
				metadata := map[string]any{"issuer": issuer, "authorization_endpoint": issuer + "/authorize", "token_endpoint": issuer + "/token", "jwks_uri": issuer + "/keys", "userinfo_endpoint": issuer + "/userinfo"}
				if field != "issuer" && field != "discovery redirect" {
					metadata[field] = "http://id.example.invalid/endpoint"
				}
				_ = json.NewEncoder(w).Encode(metadata)
			})
			var idp *httptest.Server
			if field == "issuer" {
				idp = httptest.NewServer(handler)
			} else {
				idp = httptest.NewTLSServer(handler)
			}
			defer idp.Close()
			issuer = idp.URL
			client := NewSSOClient(issuer, "client", "")
			client.HTTPClient = idp.Client()
			_, err := client.AuthURL("https://dns.example/callback", "state", "nonce", "challenge")
			if (err != nil) != fips140.Enabled() {
				t.Fatalf("mode=%v error=%v", fips140.Enabled(), err)
			}
			if field == "issuer" && fips140.Enabled() && requests.Load() != 0 {
				t.Fatal("contacted plaintext issuer")
			}
			if fips140.Enabled() {
				before := requests.Load()
				claims, err := client.ExchangeCode(context.Background(), "https://dns.example/callback", "code", "verifier", "nonce")
				if err == nil || claims != nil {
					t.Fatal("exchange accepted unsafe metadata")
				}
				if field == "issuer" && requests.Load() != before {
					t.Fatal("exchange contacted plaintext issuer")
				}
			}
		})
	}
}
