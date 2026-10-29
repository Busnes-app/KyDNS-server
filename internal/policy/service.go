package policy

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/Busnes-app/kydns-server/internal/registry"
	"github.com/Busnes-app/kydns-server/internal/store"
)

// Refresh interval bounds. The floor keeps a misconfigured list from
// hammering a public source every tick.
const (
	minInterval     = 300
	defaultInterval = 86400
	maxBlockTTL     = 3600
	// MaxPause bounds one pause of blocking; it then resumes on its own.
	MaxPause = time.Hour
)

// Service is the blacklist application service both transports call.
// Validation lives here so the JSON API, the CLI and the web UI cannot drift.
type Service struct {
	st     *store.Store
	h      *Holder
	r      *Refresher
	logger *slog.Logger
}

func NewService(st *store.Store, h *Holder, r *Refresher, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{st: st, h: h, r: r, logger: logger}
}

func (s *Service) Settings() (store.BlacklistSettings, error) { return s.st.BlacklistSettings() }

// SetSettings applies the global toggle and block TTL, then rebuilds so the
// change takes effect without a restart.
func (s *Service) SetSettings(enabled bool, blockTTL int) error {
	if blockTTL < 1 || blockTTL > maxBlockTTL {
		return registry.Invalid("block_ttl", "block_ttl_range",
			"the block TTL must be between 1 and %d seconds", maxBlockTTL)
	}
	if err := s.st.SetBlacklistSettings(store.BlacklistSettings{Enabled: enabled, BlockTTL: blockTTL}); err != nil {
		return err
	}
	s.logger.Info("blacklist filtering setting changed", "enabled", enabled, "block_ttl", blockTTL)
	return s.h.Rebuild()
}

// Lists returns list metadata without the downloaded bodies.
func (s *Service) Lists() ([]store.BlacklistList, error) { return s.st.BlacklistListMetas() }

// validateList normalizes and validates a list definition. It never touches
// the store, so ReplacePolicy can validate a whole document before writing
// any of it.
func (s *Service) validateList(l store.BlacklistList) (store.BlacklistList, error) {
	l.Name = strings.ToLower(strings.TrimSpace(l.Name))
	l.URL = strings.TrimSpace(l.URL)
	l.Format = strings.ToLower(strings.TrimSpace(l.Format))
	l.Description = strings.TrimSpace(l.Description)

	if l.Name == "" {
		return l, registry.Invalid("name", "name_required", "a list needs a name")
	}
	if l.Name == PolicyAllow || l.Name == PolicyDeny || l.Name == PolicyForwarded {
		return l, registry.Invalid("name", "name_reserved",
			"%q is a reserved policy name and cannot be used for a list", l.Name)
	}
	if err := validListURL(l.URL); err != nil {
		return l, err
	}
	if l.Format == "" {
		l.Format = FormatDomains
	}
	if !ValidFormat(l.Format) {
		return l, registry.Invalid("format", "format_unsupported",
			"format must be %s, %s or %s", FormatDomains, FormatHosts, FormatAdblock)
	}
	if l.IntervalSeconds == 0 {
		l.IntervalSeconds = defaultInterval
	}
	if l.IntervalSeconds < minInterval {
		return l, registry.Invalid("interval_seconds", "interval_too_short",
			"the refresh interval must be at least %d seconds", minInterval)
	}
	return l, nil
}

// PutList validates and writes a list definition, then rebuilds. A built-in
// may be enabled, disabled and re-tuned, but never renamed away from its
// manifest entry or re-pointed at a different URL.
func (s *Service) PutList(l store.BlacklistList) (int64, error) {
	l, err := s.validateList(l)
	if err != nil {
		return 0, err
	}

	if l.ID != 0 {
		cur, err := s.st.BlacklistListByID(l.ID)
		if err != nil {
			return 0, err
		}
		l.Builtin = cur.Builtin
		if cur.Builtin && (l.URL != cur.URL || l.Name != cur.Name) {
			return 0, registry.Invalid("url", "builtin_immutable",
				"a built-in list keeps its shipped name and URL; disable it instead")
		}
	} else {
		l.Builtin = false // only the manifest seeds built-ins
	}

	id, err := s.st.PutBlacklistList(l)
	if err != nil {
		return 0, err
	}
	s.logger.Info("blacklist list saved", "list", oneLine(l.Name), "enabled", l.Enabled)
	return id, s.h.Rebuild()
}

// oneLine strips CR and LF so a user-supplied value cannot forge log lines.
func oneLine(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\n", ""), "\r", "")
}

func validListURL(raw string) error {
	if raw == "" {
		return registry.Invalid("url", "url_required", "a list needs a source URL")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return registry.Invalid("url", "url_invalid", "%q is not a URL", raw)
	}
	if u.Scheme != "https" {
		return registry.Invalid("url", "url_not_https",
			"a list URL must be https so the download can be verified")
	}
	if u.Host == "" {
		return registry.Invalid("url", "url_invalid", "%q has no host", raw)
	}
	if u.User != nil {
		return registry.Invalid("url", "url_has_credentials",
			"a list URL may not embed credentials; they would be written to backups")
	}
	return nil
}

func (s *Service) DeleteList(id int64) error {
	cur, err := s.st.BlacklistListByID(id)
	if err != nil {
		return err
	}
	if cur.Builtin {
		return registry.Invalid("id", "builtin_immutable",
			"a built-in list cannot be deleted; disable it instead")
	}
	if err := s.st.DeleteBlacklistList(id); err != nil {
		return err
	}
	s.logger.Info("blacklist list removed", "list", cur.Name)
	return s.h.Rebuild()
}

func (s *Service) Rules() ([]store.BlacklistRule, error) { return s.st.BlacklistRules() }

// AddRule normalizes the domain and writes the rule. A duplicate or a
// conflicting rule of the other kind is refused by the store's unique index.
func (s *Service) AddRule(kind, domain string) (int64, error) {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind != PolicyAllow && kind != PolicyDeny {
		return 0, registry.Invalid("kind", "kind_invalid", "a rule is either allow or deny")
	}
	n, err := Normalize(domain)
	if err != nil {
		return 0, registry.Invalid("domain", "domain_invalid", "%q is not a domain name", domain)
	}
	id, err := s.st.PutBlacklistRule(store.BlacklistRule{Kind: kind, Domain: n})
	if err != nil {
		return 0, err
	}
	s.logger.Info("blacklist rule added", "kind", oneLine(kind), "domain", n)
	return id, s.h.Rebuild()
}

func (s *Service) DeleteRule(id int64) error {
	if err := s.st.DeleteBlacklistRule(id); err != nil {
		return err
	}
	s.logger.Info("blacklist rule removed", "id", id)
	return s.h.Rebuild()
}

// Refresh downloads one list now, or every enabled list when id is 0.
func (s *Service) Refresh(ctx context.Context, id int64) error {
	if id == 0 {
		return s.r.RefreshAll(ctx)
	}
	return s.r.RefreshList(ctx, id)
}

// Test answers what the live policy would do with a name, without querying it.
func (s *Service) Test(name string) (Decision, error) {
	n, err := Normalize(name)
	if err != nil {
		return Decision{}, registry.Invalid("name", "name_invalid", "%q is not a domain name", name)
	}
	return s.h.Current().Decide(n), nil
}

// Explanation is what the live policy does with one name, and why.
type Explanation struct {
	Name        string    // normalized
	Blocked     bool      // what a query for it gets right now
	Policy      string    // the query log's policy field for it
	Enabled     bool      // the global filtering switch
	PausedUntil time.Time // zero unless blocking is paused
	Matches     []Match   // every rule and list that matches, in precedence order
}

// Explain answers "why is this blocked": the decision and every source behind
// it. Like Test it queries nothing and counts nothing.
func (s *Service) Explain(name string) (Explanation, error) {
	n, err := Normalize(name)
	if err != nil {
		return Explanation{}, registry.Invalid("name", "name_invalid", "%q is not a domain name", name)
	}
	snap := s.h.Current()
	d := snap.Decide(n)
	e := Explanation{Name: n, Blocked: d.Blocked, Policy: d.Policy, Enabled: snap.Enabled(),
		PausedUntil: s.h.PausedUntil(), Matches: snap.Explain(n)}
	if !e.PausedUntil.IsZero() {
		e.Blocked, e.Policy = false, PolicyPaused
	}
	return e, nil
}

// Pause turns blocking off for d, at most MaxPause, and returns when it
// resumes on its own. Local names and rules are kept; nothing is written.
func (s *Service) Pause(d time.Duration) (time.Time, error) {
	if d < time.Minute || d > MaxPause {
		return time.Time{}, registry.Invalid("minutes", "pause_range",
			"a pause lasts from 1 to %d minutes", int(MaxPause/time.Minute))
	}
	until := s.h.Now().Add(d).Truncate(time.Second)
	s.h.Pause(until)
	s.logger.Info("blacklist blocking paused", "until", until.UTC().Format(time.RFC3339))
	return until, nil
}

// Resume ends a pause now.
func (s *Service) Resume() {
	s.h.Resume()
	s.logger.Info("blacklist blocking resumed")
}

// PausedUntil is when a pause ends, or the zero time when there is none.
func (s *Service) PausedUntil() time.Time { return s.h.PausedUntil() }

// Decide implements dnsserver.PolicyDecider, so the DNS pipeline consults the
// same façade the screens do rather than reaching past it to the holder.
func (s *Service) Decide(name string) (bool, string, uint32) { return s.h.Decide(name) }

// Counters reports blocked totals and counts by list. Never by client.
func (s *Service) Counters() (uint64, map[string]uint64) { return s.h.Counters() }

// TopBlocked reports the most-blocked names. Never by client.
func (s *Service) TopBlocked(n int) []NameCount { return s.h.TopBlocked(n) }

// ReplacePolicy validates every list and rule before writing any of them, then
// writes them all in the one transaction store.ReplaceBlacklist opens and
// rebuilds once. Import --replace goes through here so a bad document can
// neither bypass the rules PutList enforces one at a time, nor leave the
// policy half-wiped.
func (s *Service) ReplacePolicy(set store.BlacklistSettings, lists []store.BlacklistList, rules []store.BlacklistRule) error {
	ls, rs, err := s.preparePolicy(set, lists, rules)
	if err != nil {
		return err
	}
	if err := s.st.ReplaceBlacklist(set, ls, rs); err != nil {
		return err
	}
	// A backup made before built-ins existed must not leave filtering with no
	// shipped list: reseed anything the import didn't restore.
	if err := SeedBuiltins(s.st); err != nil {
		return err
	}
	return s.h.Rebuild()
}

// CheckPolicy reports whether ReplacePolicy would accept the document,
// writing nothing, so an import can refuse it before replacing anything else.
func (s *Service) CheckPolicy(set store.BlacklistSettings, lists []store.BlacklistList, rules []store.BlacklistRule) error {
	_, _, err := s.preparePolicy(set, lists, rules)
	return err
}

// preparePolicy validates and normalizes a whole policy, including the
// uniqueness the store would otherwise enforce only once writing has begun.
func (s *Service) preparePolicy(set store.BlacklistSettings, lists []store.BlacklistList, rules []store.BlacklistRule) ([]store.BlacklistList, []store.BlacklistRule, error) {
	if set.BlockTTL < 1 || set.BlockTTL > maxBlockTTL {
		return nil, nil, registry.Invalid("block_ttl", "block_ttl_range",
			"the block TTL must be between 1 and %d seconds", maxBlockTTL)
	}
	manifest, err := BuiltinManifest()
	if err != nil {
		return nil, nil, err
	}
	builtinNames := make(map[string]bool, len(manifest.Lists))
	for _, b := range manifest.Lists {
		builtinNames[b.Name] = true
	}
	ls := make([]store.BlacklistList, 0, len(lists))
	seen := map[string]bool{}
	for _, l := range lists {
		v, err := s.validateList(l)
		if err != nil {
			return nil, nil, err
		}
		if seen[v.Name] {
			return nil, nil, fmt.Errorf("%w: list %s", store.ErrDuplicateName, v.Name)
		}
		seen[v.Name] = true
		// An imported document's own builtin flag is untrusted: only the
		// shipped manifest may mint an undeletable, immutable list.
		v.Builtin = builtinNames[v.Name]
		ls = append(ls, v)
	}
	rs := make([]store.BlacklistRule, 0, len(rules))
	seen = map[string]bool{}
	for _, r := range rules {
		kind := strings.ToLower(strings.TrimSpace(r.Kind))
		if kind != PolicyAllow && kind != PolicyDeny {
			return nil, nil, registry.Invalid("kind", "kind_invalid", "a rule is either allow or deny")
		}
		n, err := Normalize(r.Domain)
		if err != nil {
			return nil, nil, registry.Invalid("domain", "domain_invalid", "%q is not a domain name", r.Domain)
		}
		if seen[n] {
			return nil, nil, fmt.Errorf("%w: a rule for %s already exists", store.ErrDuplicateName, n)
		}
		seen[n] = true
		rs = append(rs, store.BlacklistRule{Kind: kind, Domain: n})
	}
	return ls, rs, nil
}
