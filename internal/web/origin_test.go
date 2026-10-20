package web

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"

	"github.com/Busnes-app/kydns-server/internal/config"
	"github.com/Busnes-app/kydns-server/internal/store"
)

// Browser-origin hardening, after the Joplin Web Clipper token leak
// (GHSA-9728-v7ww-mxjv): wildcard CORS, no Origin check, and a consent prompt
// that did not say who was asking let any website mint a full-access token.

const ownOrigin = "http://127.0.0.1:8053"

// stack is the admin listener's handler exactly as serve.go builds it, with
// the web UI, the API and /mcp on one mux.
type stack struct {
	h      http.Handler
	srv    *Server
	cookie *http.Cookie
	csrf   string
}

func newStack(t *testing.T, tweak ...func(*Options)) *stack {
	t.Helper()
	mux, srv := newWeb(t, tweak...)
	srv.o.API.Routes(mux)
	s := &stack{h: srv.TransportGate(srv.o.API.WriteGate(srv.WriteGate(mux))), srv: srv}
	if rec := s.send(t, "POST", "/setup", url.Values{"token": {"setup-me"}, "password": {testPassword}, "confirm": {testPassword}}.Encode(), nil); rec.Code != http.StatusSeeOther {
		t.Fatalf("setup = %d: %s", rec.Code, rec.Body)
	}
	s.cookie = sessionCookie(s.send(t, "POST", "/login", url.Values{"password": {testPassword}}.Encode(), nil))
	if s.cookie == nil {
		t.Fatal("no session")
	}
	sess, _ := srv.o.Sessions.Get(s.cookie.Value)
	s.csrf = sess.CSRF
	return s
}

// send is a request from this host's loopback with Host 127.0.0.1:8053 unless
// header overrides it. A "Cookie" header key means "attach the session".
func (s *stack) send(t *testing.T, method, path, body string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:5555"
	req.Host = "127.0.0.1:8053"
	if body != "" && !strings.HasPrefix(body, "{") {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for k, v := range header {
		switch k {
		case "Host":
			req.Host = v
		case "RemoteAddr":
			req.RemoteAddr = v
		case "Cookie":
			req.AddCookie(s.cookie)
		default:
			req.Header.Set(k, v)
		}
	}
	rec := httptest.NewRecorder()
	s.h.ServeHTTP(rec, req)
	return rec
}

func (s *stack) agentToken(t *testing.T) string {
	t.Helper()
	gen, _ := s.srv.o.Registry.RecoveryGeneration()
	tok, err := s.srv.o.Registry.CreateAgentTokenAtGeneration("hermes", store.ScopeMCPWrite, false, "", "", gen)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

const toolsList = `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`

func mcpHeaders(tok string, extra map[string]string) map[string]string {
	h := map[string]string{"Authorization": "Bearer " + tok, "Content-Type": "application/json", "Accept": "application/json, text/event-stream"}
	for k, v := range extra {
		h[k] = v
	}
	return h
}

// Requirement 1: /mcp refuses any request carrying an Origin header, even the
// server's own origin with a valid token. Agents are not browsers.
func TestMCPRefusesAnyOrigin(t *testing.T) {
	s := newStack(t)
	tok := s.agentToken(t)
	if rec := s.send(t, "POST", "/mcp", toolsList, mcpHeaders(tok, nil)); rec.Code != http.StatusOK {
		t.Fatalf("agent without Origin = %d: %s", rec.Code, rec.Body)
	}
	for _, origin := range []string{"https://evil.example", ownOrigin, "null"} {
		if rec := s.send(t, "POST", "/mcp", toolsList, mcpHeaders(tok, map[string]string{"Origin": origin})); rec.Code != http.StatusForbidden {
			t.Errorf("POST /mcp with Origin %s = %d, want 403", origin, rec.Code)
		}
		if rec := s.send(t, "OPTIONS", "/mcp", "", map[string]string{"Origin": origin, "Access-Control-Request-Method": "POST"}); rec.Code != http.StatusForbidden {
			t.Errorf("preflight /mcp from %s = %d, want 403", origin, rec.Code)
		}
	}
}

// Requirement 2: no response ever carries a CORS grant, wildcard or not. The UI
// is same-origin and needs none; /mcp gets none at all.
func TestNoCORSHeadersAnywhere(t *testing.T) {
	s := newStack(t)
	tok := s.agentToken(t)
	for _, path := range []string{"/", "/settings", "/login", "/stats.json", "/api/v1/services", "/api/v1/tokens", "/mcp", "/agent/skills/kydns/SKILL.md"} {
		for _, method := range []string{"GET", "POST", "OPTIONS"} {
			for _, origin := range []string{"", "https://evil.example", ownOrigin} {
				h := mcpHeaders(tok, map[string]string{"Cookie": "", "Access-Control-Request-Method": "POST"})
				if origin != "" {
					h["Origin"] = origin
				}
				rec := s.send(t, method, path, toolsList, h)
				for k := range rec.Header() {
					if strings.HasPrefix(k, "Access-Control-") {
						t.Errorf("%s %s Origin %q: sent %s: %q", method, path, origin, k, rec.Header().Get(k))
					}
				}
			}
		}
	}
}

func (s *stack) tokenCount(t *testing.T) int {
	toks, err := s.srv.o.Registry.Tokens()
	if err != nil {
		t.Fatal(err)
	}
	return len(toks)
}

// Requirement 3: issuing, changing and revoking an agent token needs the
// session and its CSRF token, and a cross-origin request is refused even with
// both. A forged cross-site form post changes nothing.
func TestForgedCrossSiteTokenRequestsAreRefused(t *testing.T) {
	s := newStack(t)
	s.agentToken(t)
	toks, _ := s.srv.o.Registry.Tokens()
	id := itoa(toks[0].ID)
	forms := map[string]url.Values{
		"/settings/tokens/new":    {"label": {"evil"}, "access": {"mcp:write"}, "query_log": {"on"}},
		"/settings/tokens/access": {"id": {id}, "access": {"mcp:read"}, "query_log": {"on"}},
		"/settings/tokens/delete": {"id": {id}},
	}
	attacks := map[string]struct {
		header   map[string]string
		withCSRF bool
	}{
		"cross-site Origin, valid CSRF":           {map[string]string{"Cookie": "", "Origin": "https://evil.example"}, true},
		"Origin null, valid CSRF":                 {map[string]string{"Cookie": "", "Origin": "null"}, true},
		"Sec-Fetch-Site cross-site, no Origin":    {map[string]string{"Cookie": "", "Sec-Fetch-Site": "cross-site"}, true},
		"another port is another origin":          {map[string]string{"Cookie": "", "Origin": "http://127.0.0.1:9999"}, true},
		"same origin without the CSRF token":      {map[string]string{"Cookie": "", "Origin": ownOrigin}, false},
		"no session, valid CSRF":                  {map[string]string{"Origin": ownOrigin}, true},
		"rebound host name, same-looking request": {map[string]string{"Cookie": "", "Host": "evil.example:8053", "Origin": "http://evil.example:8053"}, true},
	}
	for path, form := range forms {
		for name, a := range attacks {
			f := url.Values{}
			for k, v := range form {
				f[k] = v
			}
			if a.withCSRF {
				f.Set("csrf_token", s.csrf)
			}
			before, _ := s.srv.o.Registry.Tokens()
			rec := s.send(t, "POST", path, f.Encode(), a.header)
			after, _ := s.srv.o.Registry.Tokens()
			if rec.Code == http.StatusOK || rec.Code == http.StatusSeeOther && rec.Header().Get("Location") != "/login" {
				t.Errorf("%s, %s: accepted with %d", path, name, rec.Code)
			}
			if len(after) != len(before) || after[0].Scope != before[0].Scope || after[0].QueryLog != before[0].QueryLog {
				t.Errorf("%s, %s: tokens changed %+v -> %+v", path, name, before, after)
			}
		}
	}
	// The real form, from the real page, works.
	legit := url.Values{"label": {"hermes 2"}, "access": {"mcp:read"}, "csrf_token": {s.csrf}}
	if rec := s.send(t, "POST", "/settings/tokens/new", legit.Encode(), map[string]string{"Cookie": "", "Origin": ownOrigin, "Sec-Fetch-Site": "same-origin"}); rec.Code != http.StatusOK || s.tokenCount(t) != 2 {
		t.Fatalf("same-origin create = %d, %d tokens", rec.Code, s.tokenCount(t))
	}
	// The bearer API refuses a cross-origin caller too, whatever it carries.
	admin, _ := s.srv.o.Registry.CreateToken("cli")
	if rec := s.send(t, "POST", "/api/v1/tokens", `{"label":"x","scope":"mcp:write"}`, map[string]string{"Authorization": "Bearer " + admin, "Origin": "https://evil.example"}); rec.Code != http.StatusForbidden {
		t.Errorf("cross-origin POST /api/v1/tokens = %d, want 403", rec.Code)
	}
}

// Requirement 3, DNS rebinding: the admin UI, the API and /mcp answer only to
// their own host names.
func TestHostHeaderIsChecked(t *testing.T) {
	s := newStack(t)
	tok := s.agentToken(t)
	for _, path := range []string{"/mcp", "/api/v1/services", "/settings", "/login", "/agent/skills/kydns/SKILL.md"} {
		for host, want := range map[string]bool{"evil.example": false, "evil.example:8053": false, "kydns.home.arpa": false,
			"127.0.0.1:8053": true, "localhost:8053": true, "[::1]:8053": true} {
			rec := s.send(t, "POST", path, toolsList, mcpHeaders(tok, map[string]string{"Host": host}))
			if refused := rec.Code == http.StatusForbidden && strings.Contains(rec.Body.String(), "KYDNS_ADMIN_HOSTNAMES"); refused == want {
				t.Errorf("%s with Host %s: %d %q", path, host, rec.Code, rec.Body.String())
			}
		}
	}
	if rec := s.send(t, "GET", "/healthz", "", map[string]string{"Host": "evil.example"}); rec.Code != http.StatusOK {
		t.Errorf("monitors keep /healthz on any host: %d", rec.Code)
	}

	proxy := netip.MustParsePrefix("10.0.0.5/32")
	viaProxy := func(s *stack, host string) int {
		return s.send(t, "GET", "/login", "", map[string]string{"Host": host, "RemoteAddr": "10.0.0.5:4000", "X-Forwarded-Proto": "https"}).Code
	}
	named := newStack(t, func(o *Options) {
		o.Config = &config.Config{Admin: config.AdminConfig{TrustedProxies: []netip.Prefix{proxy}, Hostnames: []string{"kydns.lan"}}}
	})
	if got := viaProxy(named, "kydns.lan"); got != http.StatusOK {
		t.Errorf("configured name through the proxy = %d", got)
	}
	if got := viaProxy(named, "evil.example"); got != http.StatusForbidden {
		t.Errorf("other name through the proxy = %d, want 403", got)
	}
	// With no names configured the proxy's TLS certificate binds the name: a
	// rebound name fails the handshake before it reaches KyDNS.
	unnamed := newStack(t, func(o *Options) {
		o.Config = &config.Config{Admin: config.AdminConfig{TrustedProxies: []netip.Prefix{proxy}}}
	})
	if got := viaProxy(unnamed, "kydns.lan"); got != http.StatusOK {
		t.Errorf("unconfigured, through the proxy = %d", got)
	}
}

// Requirement 4: the screen that issues a token says who it is for and what it
// grants; an agent token must be named.
func TestTokenScreenNamesAgentAndAccess(t *testing.T) {
	s := newStack(t)
	for _, c := range []struct {
		form url.Values
		want []string
	}{
		{url.Values{"label": {"hermes on the NAS"}, "access": {"mcp:write"}, "query_log": {"on"}}, []string{"Token for “hermes on the NAS”", "read and write", "pause blocking", "can read the query log (browsing history)"}},
		{url.Values{"label": {"openclaw"}, "access": {"mcp:read"}}, []string{"Token for “openclaw”", "read only", "without the query log"}},
		{url.Values{"label": {"laptop"}}, []string{"Token for “laptop”", "admin: full control"}},
	} {
		c.form.Set("csrf_token", s.csrf)
		rec := s.send(t, "POST", "/settings/tokens/new", c.form.Encode(), map[string]string{"Cookie": ""})
		body := rec.Body.String()
		for _, w := range c.want {
			if !strings.Contains(body, w) {
				t.Errorf("%v: page lacks %q", c.form, w)
			}
		}
	}
	unnamed := url.Values{"label": {"  "}, "access": {"mcp:read"}, "csrf_token": {s.csrf}}
	before := s.tokenCount(t)
	if rec := s.send(t, "POST", "/settings/tokens/new", unnamed.Encode(), map[string]string{"Cookie": ""}); rec.Code != http.StatusBadRequest || s.tokenCount(t) != before {
		t.Errorf("unnamed agent token = %d, want refused", rec.Code)
	}
}
