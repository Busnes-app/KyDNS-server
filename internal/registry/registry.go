package registry

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/netip"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Busnes-app/kydns-server/internal/store"
)

// Registry is the application service both transports call. Validation lives
// here so the JSON API and the CLI cannot drift apart.
type Registry struct {
	s        *store.Store
	zone     atomic.Pointer[string]
	onChange func() error
}

func New(s *store.Store, zoneFQDN string, onChange func() error) *Registry {
	if onChange == nil {
		onChange = func() error { return nil }
	}
	r := &Registry{s: s, onChange: onChange}
	r.SetZone(zoneFQDN)
	return r
}

// Zone is the private zone every name is validated against.
func (r *Registry) Zone() string {
	if z := r.zone.Load(); z != nil {
		return *z
	}
	return ""
}

// SetZone follows a change to the private domain. Each write validates against
// the zone it reads once, so a rename landing mid-write cannot half-apply.
func (r *Registry) SetZone(zoneFQDN string) {
	z := Normalize(zoneFQDN)
	r.zone.Store(&z)
}

func (r *Registry) knownViews() (map[string]bool, error) {
	views, err := r.s.Views()
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, v := range views {
		out[v.Name] = true
	}
	return out, nil
}

// validateService normalizes and validates a service against a set of known
// view names. It never touches the store, so import --replace can validate a
// whole document before writing any of it.
func validateService(svc store.Service, known map[string]bool, zone string) (store.Service, error) {
	svc.Name = strings.ToLower(strings.TrimSpace(svc.Name))
	if err := ValidateName(svc.Name+"."+zone, zone); err != nil {
		return svc, err
	}
	if len(svc.Addresses) == 0 {
		return svc, invalid("addresses", "addresses_required", "a service needs at least one address")
	}
	for i, a := range svc.Addresses {
		if err := ValidateAddress(a.Address); err != nil {
			return svc, err
		}
		if a.View != "" && !known[a.View] {
			return svc, invalid(fmt.Sprintf("addresses[%d].view", i), "view_unknown", "view %q does not exist", a.View)
		}
	}
	svc.ProxyAddress = strings.TrimSpace(svc.ProxyAddress)
	if svc.ProxyAddress != "" {
		if err := ValidateAddress(svc.ProxyAddress); err != nil {
			return svc, invalid("proxy_address", "proxy_address_invalid", "%s", err)
		}
	}
	if svc.RouteViaProxy && svc.ProxyAddress == "" {
		return svc, invalid("proxy_address", "proxy_address_required",
			"routing through a proxy needs a proxy address")
	}
	for i, al := range svc.Aliases {
		svc.Aliases[i] = strings.ToLower(strings.TrimSpace(al))
		if err := ValidateName(svc.Aliases[i]+"."+zone, zone); err != nil {
			return svc, err
		}
	}
	if err := ValidateMAC(svc.MAC); err != nil {
		return svc, err
	}
	svc.MAC = NormalizeMAC(svc.MAC)
	return svc, nil
}

// macUnique rejects a reservation another service already holds. Callers pass
// the set svc has to be unique within, because validateService may not read
// the store: PutService compares against the stored services, ReplaceAll
// against the document it is about to write. A service being created has ID 0,
// so it must differ from every other; an update may match only itself. An
// empty MAC is no reservation at all and never collides.
func macUnique(svc store.Service, others []store.Service) error {
	if svc.MAC == "" {
		return nil
	}
	for _, o := range others {
		if o.MAC == svc.MAC && (svc.ID == 0 || o.ID != svc.ID) {
			return invalid("mac", "duplicate", "%s is already reserved by the service %q", svc.MAC, o.Name)
		}
	}
	return nil
}

// PutService validates, writes, then rebuilds. A failed validation never
// reaches the store and never triggers a rebuild.
func (r *Registry) PutService(svc store.Service) (int64, error) {
	known, err := r.knownViews()
	if err != nil {
		return 0, err
	}
	svc, err = validateService(svc, known, r.Zone())
	if err != nil {
		return 0, err
	}
	// Read the other services only when there is a reservation to check, so
	// the common service without one still costs a single write.
	if svc.MAC != "" {
		stored, err := r.s.Services()
		if err != nil {
			return 0, err
		}
		if err := macUnique(svc, stored); err != nil {
			return 0, err
		}
	}
	id, err := r.s.PutService(svc)
	if err != nil {
		return 0, err
	}
	return id, r.onChange()
}

func (r *Registry) Services() ([]store.Service, error) { return r.s.Services() }

func (r *Registry) Service(id int64) (store.Service, error) { return r.s.Service(id) }

func (r *Registry) DeleteService(id int64) error {
	if err := r.s.DeleteService(id); err != nil {
		return err
	}
	return r.onChange()
}

// validateRecord normalizes and validates a record against a set of known
// view names. It never touches the store; see validateService.
func validateRecord(rec store.Record, known map[string]bool, zone string) (store.Record, error) {
	rec.Name = Normalize(rec.Name)
	rec.Type = strings.ToUpper(strings.TrimSpace(rec.Type))
	if err := ValidateRecordType(rec.Type); err != nil {
		return rec, err
	}
	switch rec.Type {
	case "A", "AAAA":
		if err := ValidateName(rec.Name, zone); err != nil {
			return rec, err
		}
		if err := ValidateAddress(rec.Value); err != nil {
			return rec, err
		}
	case "CNAME":
		if err := ValidateName(rec.Name, zone); err != nil {
			return rec, err
		}
		rec.Value = Normalize(rec.Value)
	case "PTR":
		// Checking for ".arpa." alone would wrongly accept names in the
		// default private domain, since home.arpa is itself under .arpa.
		if !strings.HasSuffix(rec.Name, ".in-addr.arpa.") && !strings.HasSuffix(rec.Name, ".ip6.arpa.") {
			return rec, invalid("name", "ptr_not_arpa",
				"a PTR name must be under in-addr.arpa. or ip6.arpa.")
		}
		rec.Value = Normalize(rec.Value)
	}
	if rec.View != "" && !known[rec.View] {
		return rec, invalid("view", "view_unknown", "view %q does not exist", rec.View)
	}
	return rec, nil
}

func (r *Registry) PutRecord(rec store.Record) (int64, error) {
	known, err := r.knownViews()
	if err != nil {
		return 0, err
	}
	rec, err = validateRecord(rec, known, r.Zone())
	if err != nil {
		return 0, err
	}
	id, err := r.s.PutRecord(rec)
	if err != nil {
		return 0, err
	}
	return id, r.onChange()
}

func (r *Registry) Records() ([]store.Record, error) { return r.s.Records() }

func (r *Registry) DeleteRecord(id int64) error {
	if err := r.s.DeleteRecord(id); err != nil {
		return err
	}
	return r.onChange()
}

// validateView normalizes and validates a view. It never touches the store;
// see validateService.
func validateView(v store.View) (store.View, error) {
	v.Name = strings.ToLower(strings.TrimSpace(v.Name))
	if err := ValidateLabel(v.Name); err != nil {
		return v, invalid("name", "view_name_invalid", "view name %q must be a single DNS label", v.Name)
	}
	if len(v.Subnets) == 0 {
		return v, invalid("subnets", "subnets_required", "a view needs at least one subnet")
	}
	for i, c := range v.Subnets {
		p, err := netip.ParsePrefix(c)
		if err != nil {
			return v, invalid(fmt.Sprintf("subnets[%d]", i), "cidr_invalid", "%q is not a CIDR", c)
		}
		v.Subnets[i] = p.Masked().String()
	}
	return v, nil
}

func (r *Registry) PutView(v store.View) error {
	v, err := validateView(v)
	if err != nil {
		return err
	}
	if err := r.s.PutView(v); err != nil {
		return err
	}
	return r.onChange()
}

func (r *Registry) Views() ([]store.View, error) { return r.s.Views() }

func (r *Registry) DeleteView(name string) error {
	if err := r.s.DeleteView(name); err != nil {
		return err
	}
	return r.onChange()
}

// ReplaceAll validates every view, service, and record before writing any of
// them, then writes them all in the one transaction store.ReplaceAll opens
// and rebuilds once. Import --replace goes through here so a bad document
// can neither bypass the rules PutService enforces one at a time, nor leave
// the registry half-wiped if a store-level constraint still rejects it.
func (r *Registry) ReplaceAll(views []store.View, services []store.Service, records []store.Record) error {
	// One zone for the whole document: a rename landing mid-import must not
	// validate half of it against each domain.
	vs, ss, rs, err := ValidateDocument(r.Zone(), views, services, records)
	if err != nil {
		return err
	}
	if err := r.s.ReplaceAll(vs, ss, rs); err != nil {
		return err
	}
	return r.onChange()
}

// ValidateDocument normalizes and validates a whole registry against zone
// without touching the store. A replica runs it on a pulled snapshot too.
func ValidateDocument(zone string, views []store.View, services []store.Service, records []store.Record) (
	[]store.View, []store.Service, []store.Record, error) {
	zone = Normalize(zone)
	vs := make([]store.View, 0, len(views))
	known := map[string]bool{}
	for _, v := range views {
		v, err := validateView(v)
		if err != nil {
			return nil, nil, nil, err
		}
		vs = append(vs, v)
		known[v.Name] = true
	}
	ss := make([]store.Service, 0, len(services))
	for _, svc := range services {
		svc, err := validateService(svc, known, zone)
		if err != nil {
			return nil, nil, nil, err
		}
		if err := macUnique(svc, ss); err != nil {
			return nil, nil, nil, err
		}
		ss = append(ss, svc)
	}
	rs := make([]store.Record, 0, len(records))
	for _, rec := range records {
		rec, err := validateRecord(rec, known, zone)
		if err != nil {
			return nil, nil, nil, err
		}
		rs = append(rs, rec)
	}
	return vs, ss, rs, nil
}

const tokenPrefix = "kydns_"

// GenerateToken returns a new plaintext token and its hash. Tokens are
// high-entropy randoms, so SHA-256 is right: a slow KDF on every API call
// would cost real latency and buy nothing against a 256-bit search space.
func GenerateToken() (string, string) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	plaintext := tokenPrefix + hex.EncodeToString(buf)
	return plaintext, HashToken(plaintext)
}

func HashToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

func (r *Registry) CreateToken(label string) (string, error) {
	return r.CreateIdentityToken(label, "", "")
}

// CreateIdentityToken issues a token from local authority. HTTP callers use
// CreateIdentityTokenAtGeneration with their authenticated recovery generation.
func (r *Registry) CreateIdentityToken(label, issuer, subject string) (string, error) {
	generation, err := r.RecoveryGeneration()
	if err != nil {
		return "", err
	}
	return r.CreateIdentityTokenAtGeneration(label, issuer, subject, generation)
}

func (r *Registry) RecoveryGeneration() (time.Time, error) { return r.s.SessionsNotBefore() }

// CreateIdentityTokenAtGeneration uses the generation captured before authentication.
func (r *Registry) CreateIdentityTokenAtGeneration(label, issuer, subject string, generation time.Time) (string, error) {
	plaintext, hash := GenerateToken()
	if _, err := r.s.PutTokenAtGeneration(store.Token{Label: label, Hash: hash, Issuer: issuer, Subject: subject}, generation); err != nil {
		return "", err
	}
	return plaintext, nil
}

func (r *Registry) AuthenticateToken(plaintext string) bool {
	_, _, ok := r.AuthenticateTokenPrincipal(plaintext)
	return ok
}

// AuthenticateTokenPrincipal checks authority on every request, including child creation.
func (r *Registry) AuthenticateTokenPrincipal(plaintext string) (string, string, bool) {
	if !strings.HasPrefix(plaintext, tokenPrefix) {
		return "", "", false
	}
	want := HashToken(plaintext)
	toks, err := r.s.Tokens()
	if err != nil {
		return "", "", false
	}
	for _, t := range toks {
		// An agent's token never reaches the admin API.
		if t.Scope != store.ScopeAdmin || subtle.ConstantTimeCompare([]byte(t.Hash), []byte(want)) != 1 {
			continue
		}
		if t.Issuer != "" || t.Subject != "" {
			managed, allowed, err := r.s.InstallerAdministrator(t.Issuer, t.Subject)
			if err != nil || !managed || !allowed {
				return "", "", false
			}
		}
		_ = r.s.TouchToken(t.ID)
		return t.Issuer, t.Subject, true
	}
	return "", "", false
}

func (r *Registry) Tokens() ([]store.Token, error) { return r.s.Tokens() }

// AgentIdleLimit is how long an MCP token may go unused before it is revoked.
// A constant, not a setting.
const AgentIdleLimit = 90 * 24 * time.Hour

// AgentRevokeAfter is when t is revoked if nobody uses it before then.
func AgentRevokeAfter(t store.Token) time.Time {
	return time.Unix(max(t.LastUsedAt, t.CreatedAt), 0).Add(AgentIdleLimit)
}

// CreateAgentTokenAtGeneration issues an MCP token, fenced against recovery
// like CreateIdentityTokenAtGeneration. The plaintext format is the admin
// token's: the scope lives in the row.
func (r *Registry) CreateAgentTokenAtGeneration(label, scope string, queryLog bool, issuer, subject string, generation time.Time) (string, error) {
	if !store.IsMCPScope(scope) {
		return "", Invalid("scope", "scope_invalid", "an agent token's scope is %s or %s", store.ScopeMCPRead, store.ScopeMCPWrite)
	}
	// Every screen that shows the token names the agent it is for, so it needs a name.
	if label = strings.TrimSpace(label); label == "" {
		return "", Invalid("label", "label_required", "name the agent this token is for")
	}
	plaintext, hash := GenerateToken()
	t := store.Token{Label: label, Hash: hash, Issuer: issuer, Subject: subject, Scope: scope, QueryLog: queryLog}
	if _, err := r.s.PutTokenAtGeneration(t, generation); err != nil {
		return "", err
	}
	return plaintext, nil
}

// SetAgentAccess changes an MCP token between the MCP scopes and its
// query-log flag. An admin token is not found here: it never becomes an agent's.
func (r *Registry) SetAgentAccess(id int64, scope string, queryLog bool) error {
	if !store.IsMCPScope(scope) {
		return Invalid("scope", "scope_invalid", "an agent token's scope is %s or %s", store.ScopeMCPRead, store.ScopeMCPWrite)
	}
	return r.s.SetTokenAccess(id, scope, queryLog)
}

// AccessText says in words what a token can do, for every screen that issues
// or changes one: never a generic "an app wants access".
func AccessText(scope string, queryLog bool) string {
	var s string
	switch scope {
	case store.ScopeMCPRead:
		s = "AI agent, read only (explain blocks, stats, block lists)"
	case store.ScopeMCPWrite:
		s = "AI agent, read and write (also add and remove rules, refresh lists, pause blocking up to 60 minutes)"
	default:
		return "admin: full control of KyDNS through the API and CLI"
	}
	if queryLog {
		return s + ", and can read the query log (browsing history)"
	}
	return s + ", without the query log"
}

// AgentAuth is the outcome of presenting a token at /mcp.
type AgentAuth int

const (
	AgentUnknown AgentAuth = iota // no token has this hash
	AgentRefused                  // a real token, but not an agent's, or its authority is gone
	AgentExpired                  // an agent token idle past AgentIdleLimit, now deleted
	AgentOK
)

// AuthenticateAgent checks an MCP token on every request. An idle token is
// deleted here, so revocation never waits for the daily sweep; the caller
// answers all three refusals alike.
func (r *Registry) AuthenticateAgent(plaintext string, now time.Time) (store.Token, AgentAuth) {
	if !strings.HasPrefix(plaintext, tokenPrefix) {
		return store.Token{}, AgentUnknown
	}
	want := HashToken(plaintext)
	toks, err := r.s.Tokens()
	if err != nil {
		return store.Token{}, AgentUnknown
	}
	for _, t := range toks {
		if subtle.ConstantTimeCompare([]byte(t.Hash), []byte(want)) != 1 {
			continue
		}
		if !store.IsMCPScope(t.Scope) {
			return t, AgentRefused
		}
		if t.Issuer != "" || t.Subject != "" {
			managed, allowed, err := r.s.InstallerAdministrator(t.Issuer, t.Subject)
			if err != nil || !managed || !allowed {
				return t, AgentRefused
			}
		}
		if now.After(AgentRevokeAfter(t)) {
			if gone, _ := r.s.DeleteIdleAgentTokens(now.Add(-AgentIdleLimit), t.ID); len(gone) == 1 {
				r.auditExpired(t)
			}
			return t, AgentExpired
		}
		_ = r.s.TouchToken(t.ID)
		return t, AgentOK
	}
	return store.Token{}, AgentUnknown
}

// ExpireIdleAgentTokens deletes MCP tokens nobody presented for
// AgentIdleLimit, so they leave the token list. AuthenticateAgent refuses an
// idle token on its own; the daemon runs this daily.
func (r *Registry) ExpireIdleAgentTokens(now time.Time) (int, error) {
	gone, err := r.s.DeleteIdleAgentTokens(now.Add(-AgentIdleLimit), 0)
	for _, t := range gone {
		r.auditExpired(t)
	}
	return len(gone), err
}

// Audit records an administrative event in the node-local audit table.
func (r *Registry) Audit(e store.AuditEvent) error { return r.s.RecordAudit(e) }

func (r *Registry) auditExpired(t store.Token) {
	_ = r.s.RecordAudit(store.AuditEvent{Actor: "system", Action: "agent_token.expired",
		Resource: fmt.Sprintf("token:%d", t.ID), Outcome: "success"})
}

func (r *Registry) DeleteToken(id int64) error { return r.s.DeleteToken(id) }
