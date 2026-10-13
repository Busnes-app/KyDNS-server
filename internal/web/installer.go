package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/Busnes-app/ky-primitives/applysetup"
	"github.com/Busnes-app/ky-primitives/syncauth"
	"github.com/Busnes-app/kydns-server/internal/auth"
)

func (s *Server) ApplyInstaller(ctx context.Context, raw []byte, check bool) (applysetup.Report, error) {
	var b applysetup.Common
	if err := applysetup.DecodeRaw(raw, &b); err != nil {
		return applysetup.Report{}, err
	}
	return s.o.Store.ApplyInstaller(ctx, b, check)
}

func (s *Server) installerNetwork(w http.ResponseWriter, r *http.Request) {
	ip, trusted, scheme := auth.RequestNetwork(r, s.o.Config.Admin.TrustedProxies)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{"clientIp": ip, "proxyHeadersTrusted": trusted, "scheme": scheme})
}

func (s *Server) installerDirectory(w http.ResponseWriter, r *http.Request) {
	b, managed, err := s.o.Store.InstallerConfiguration()
	if err != nil || !managed {
		http.Error(w, "directory is not configured", 503)
		return
	}
	secret, err := applysetup.Secret(b.SSO.DirectorySecretFile)
	if err != nil {
		http.Error(w, "directory credential unavailable", 503)
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, (64<<10)+1))
	if err != nil || len(raw) > 64<<10 {
		http.Error(w, "invalid directory body", 400)
		return
	}
	event, err := syncauth.Verify([]byte(secret), syncauth.FromRequest(r), raw, syncauth.Options{})
	if err != nil {
		http.Error(w, "invalid directory signature", 401)
		return
	}
	if err = s.o.Store.ApplyInstallerDirectory(r.Context(), b.SSO.IssuerURL, event.Type, raw); err != nil {
		http.Error(w, "directory event refused; inspect revision and assigned subject", 422)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
