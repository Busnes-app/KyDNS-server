package web

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Busnes-app/kydns-server/internal/dnsserver"
	"github.com/Busnes-app/kydns-server/internal/registry"
	"github.com/Busnes-app/kydns-server/internal/store"
)

// restartNote covers exactly the keys the config file still owns. Everything
// else is edited above and stored in the database.
const restartNote = "These come from the config file or the environment, and are read at " +
	"startup. Everything else is edited above and stored in the database, where the " +
	"config file no longer has any effect."

type viewRow struct {
	Name        string
	Subnets     string
	Unreachable bool
	References  int
}

// configRow is one line of the read-only config display.
type configRow struct {
	Key   string
	Value string
}

// upstreams is nil when the forwarder is not wired, which the template renders
// as "not enabled" rather than as an empty table.
func (s *Server) upstreams() []dnsserver.UpstreamStatus {
	if s.o.Upstreams == nil {
		return nil
	}
	return s.o.Upstreams()
}

// configRows renders the keys the config file still owns: they are all
// needed before the database is open. Every other key moved into the database,
// and showing the file's copy of one would show a value nothing reads.
func (s *Server) configRows() []configRow {
	c := s.o.Config
	if c == nil {
		return nil
	}
	return []configRow{
		{"data_dir", c.DataDir},
		{"dns.listen", c.DNS.Listen},
		{"admin.listen", c.Admin.Listen},
	}
}

func (s *Server) settingsData(errMsg, newToken string) map[string]any {
	views, err := s.o.Registry.Views()
	if err != nil && errMsg == "" {
		errMsg = err.Error()
	}
	svcs, _ := s.o.Registry.Services()
	recs, _ := s.o.Registry.Records()

	// Banner condition 2, shown per row where the operator is already standing.
	unreachable := map[string]bool{}
	for _, n := range unreachableViews(views, s.allowTailscale()) {
		unreachable[n] = true
	}

	rows := make([]viewRow, 0, len(views))
	for _, v := range views {
		row := viewRow{
			Name:        v.Name,
			Subnets:     strings.Join(v.Subnets, ", "),
			Unreachable: unreachable[v.Name],
		}
		for _, svc := range svcs {
			for _, a := range svc.Addresses {
				if a.View == v.Name {
					row.References++
				}
			}
		}
		for _, r := range recs {
			if r.View == v.Name {
				row.References++
			}
		}
		rows = append(rows, row)
	}

	toks, _ := s.o.Registry.Tokens()
	tokRows := make([]tokenRow, 0, len(toks))
	for _, t := range toks {
		row := tokenRow{Token: t, Agent: store.IsMCPScope(t.Scope)}
		if row.Agent {
			row.RevokeAfter = registry.AgentRevokeAfter(t).UTC().Format("2006-01-02")
		}
		tokRows = append(tokRows, row)
	}
	ident, _ := s.o.Store.AdminIdentity()
	sso, _ := s.o.Store.SSOSettings()

	data := map[string]any{
		"Title": "Settings", "Nav": "settings",
		"Views": rows, "Tokens": tokRows, "NewToken": newToken,
		"Config": s.configRows(), "RestartNote": restartNote, "Error": errMsg,
		"Upstreams":     s.upstreams(),
		"Restart":       s.restartPending(),
		"AdminIdentity": ident,
		"SSOSettings":   sso,
	}
	if s.o.Backup != nil {
		status, err := s.o.Backup.Status()
		if err != nil && errMsg == "" {
			data["Error"] = err.Error()
		}
		data["Backup"] = backupView{Status: status, IntervalMinutes: status.IntervalSec / 60}
	}
	// Absent rather than zero when the service is not wired: the template then
	// renders the read-only view instead of a form full of empty boxes.
	if v, ok := s.liveSettings(); ok {
		data["Server"] = v
	}
	return data
}

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "settings.html", s.settingsData("", ""))
}

func (s *Server) settingsError(w http.ResponseWriter, r *http.Request, err error) {
	w.WriteHeader(http.StatusBadRequest)
	s.render(w, r, "settings.html", s.settingsData(err.Error(), ""))
}

func (s *Server) postViewNew(w http.ResponseWriter, r *http.Request) {
	var subnets []string
	for _, c := range strings.Split(r.PostFormValue("subnets"), ",") {
		if c = strings.TrimSpace(c); c != "" {
			subnets = append(subnets, c)
		}
	}
	if err := s.o.Registry.PutView(store.View{Name: r.PostFormValue("name"), Subnets: subnets}); err != nil {
		s.settingsError(w, r, err)
		return
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (s *Server) postViewDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.o.Registry.DeleteView(r.PostFormValue("name")); err != nil {
		s.settingsError(w, r, err)
		return
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

// tokenRow is a token as the settings page lists it.
type tokenRow struct {
	store.Token
	Agent       bool
	RevokeAfter string // agent tokens: revoked if unused by this date
}

// postTokenNew renders the settings page directly rather than redirecting,
// because the plaintext exists only in this response. An AI agent token also
// gets its MCP setup, which only this response can show filled in.
func (s *Server) postTokenNew(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.session(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	label, scope, queryLog := r.PostFormValue("label"), r.PostFormValue("access"), r.PostFormValue("query_log") == "on"
	var plaintext string
	var err error
	if scope == "" || scope == store.ScopeAdmin {
		plaintext, err = s.o.Registry.CreateIdentityTokenAtGeneration(label, sess.IdentityIssuer, sess.IdentitySubject, sess.RecoveryGeneration)
	} else {
		plaintext, err = s.o.Registry.CreateAgentTokenAtGeneration(label, scope, queryLog, sess.IdentityIssuer, sess.IdentitySubject, sess.RecoveryGeneration)
	}
	if err != nil {
		s.settingsError(w, r, err)
		return
	}
	data := s.settingsData("", plaintext)
	data["NewTokenFor"], data["NewTokenAccess"] = strings.TrimSpace(label), registry.AccessText(scope, queryLog)
	if store.IsMCPScope(scope) {
		_ = s.o.Registry.Audit(store.AuditEvent{Actor: "admin", Action: "agent_token.create", Resource: "label:" + label,
			Details: fmt.Sprintf("scope %s, query_log %t", scope, queryLog), IP: s.sourceKey(r), Outcome: "success"})
		data["AgentURL"] = "https://" + r.Host
	}
	s.render(w, r, "settings.html", data)
}

// postTokenAccess changes an agent token's access; an admin token is refused.
func (s *Server) postTokenAccess(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PostFormValue("id"), 10, 64)
	scope, queryLog := r.PostFormValue("access"), r.PostFormValue("query_log") == "on"
	if err == nil {
		err = s.o.Registry.SetAgentAccess(id, scope, queryLog)
	}
	if err != nil {
		s.settingsError(w, r, err)
		return
	}
	_ = s.o.Registry.Audit(store.AuditEvent{Actor: "admin", Action: "agent_token.access_change", Resource: fmt.Sprintf("token:%d", id),
		Details: fmt.Sprintf("scope %s, query_log %t", scope, queryLog), IP: s.sourceKey(r), Outcome: "success"})
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (s *Server) postTokenDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PostFormValue("id"), 10, 64)
	if err == nil {
		err = s.o.Registry.DeleteToken(id)
	}
	if err != nil {
		s.settingsError(w, r, err)
		return
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (s *Server) postCacheFlush(w http.ResponseWriter, r *http.Request) {
	if s.o.Cache != nil {
		s.o.Cache.Flush()
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

// getExport reuses the API's document builder so the two transports cannot
// diverge on what a backup contains.
func (s *Server) getExport(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "yaml"
	}
	if s.o.API == nil {
		http.Error(w, "export unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Disposition", "attachment; filename=kydns-export."+format)
	if err := s.o.API.WriteExport(w, format); err != nil {
		s.o.Logger.Error("export", "error", err)
	}
}

func (s *Server) postSSOSettings(w http.ResponseWriter, r *http.Request) {
	enabled := r.PostFormValue("enabled") == "1"
	issuerURL := strings.TrimSpace(r.PostFormValue("issuer_url"))
	clientID := strings.TrimSpace(r.PostFormValue("client_id"))
	clientSecret := strings.TrimSpace(r.PostFormValue("client_secret"))
	// The page never renders the secret back, so blank means "keep it".
	if clientSecret == "" && r.PostFormValue("clear_secret") != "1" {
		if cur, err := s.o.Store.SSOSettings(); err == nil {
			clientSecret = cur.ClientSecret
		}
	}

	if enabled && issuerURL == "" {
		s.settingsError(w, r, errors.New("enter an issuer URL to enable single sign-on"))
		return
	}
	if clientID == "" {
		clientID = "kydns"
	}

	err := s.o.Store.SetSSOSettings(store.SSOSettings{
		Enabled:      enabled,
		IssuerURL:    issuerURL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
	})
	if err != nil {
		s.settingsError(w, r, err)
		return
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (s *Server) postSSOUnlink(w http.ResponseWriter, r *http.Request) {
	if err := s.o.Store.UnlinkAdminSSO(); err != nil {
		s.settingsError(w, r, err)
		return
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}
