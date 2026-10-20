package web

import (
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/Busnes-app/kydns-server/internal/auth"
)

// hostAllowed is auth.HostAllowed with this server's names and proxies; /mcp
// checks the same function itself.
func (s *Server) hostAllowed(r *http.Request) bool {
	return auth.HostAllowed(r, s.o.Config.Admin.Hostnames, s.o.Config.Admin.TrustedProxies)
}

// crossOrigin reports a browser request made from another site. A state change
// from anywhere but this origin is refused on every route, on top of the
// session's CSRF token: the Joplin Web Clipper token leak (CVE-2026-105783) was
// a cross-origin request nobody checked. A request with no Origin and no
// Sec-Fetch-Site is not from a browser page (the CLI, an agent) and passes.
func (s *Server) crossOrigin(r *http.Request) bool {
	if site := r.Header.Get("Sec-Fetch-Site"); site == "cross-site" || site == "same-site" {
		return true
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return true // "null" from a sandboxed frame or a file
	}
	_, _, scheme := auth.RequestNetwork(r, s.o.Config.Admin.TrustedProxies)
	return !strings.EqualFold(u.Scheme, scheme) || !strings.EqualFold(withPort(u.Host, u.Scheme), withPort(r.Host, scheme))
}

// withPort adds the scheme's default port, so https://kydns.lan and Host
// kydns.lan:443 compare equal.
func withPort(host, scheme string) string {
	if _, _, err := net.SplitHostPort(host); err == nil {
		return host
	}
	if scheme == "https" {
		return net.JoinHostPort(strings.Trim(host, "[]"), "443")
	}
	return net.JoinHostPort(strings.Trim(host, "[]"), "80")
}

func unsafeMethod(m string) bool {
	return m != http.MethodGet && m != http.MethodHead && m != http.MethodOptions
}
