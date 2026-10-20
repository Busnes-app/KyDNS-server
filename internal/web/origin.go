package web

import (
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strings"

	"github.com/Busnes-app/kydns-server/internal/auth"
)

// hostAllowed defeats DNS rebinding: a page on attacker.example whose name is
// re-pointed at this listener still sends Host: attacker.example. IP literals
// and localhost cannot be rebound, so they always pass. Otherwise the name must
// be in KYDNS_ADMIN_HOSTNAMES. With none configured, only a request through a
// trusted HTTPS proxy passes on its Host: the proxy's certificate is what binds
// the name there, since a rebound name fails TLS. A direct loopback request
// must then use localhost or an IP.
func (s *Server) hostAllowed(r *http.Request) bool {
	host := strings.ToLower(r.Host)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.TrimSuffix(strings.Trim(host, "[]"), ".")
	if _, err := netip.ParseAddr(host); err == nil {
		return true
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	names := s.o.Config.Admin.Hostnames
	if len(names) > 0 {
		return slices.Contains(names, host)
	}
	_, trusted, _ := auth.RequestNetwork(r, s.o.Config.Admin.TrustedProxies)
	return trusted
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
