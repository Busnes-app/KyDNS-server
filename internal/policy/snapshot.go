package policy

import (
	"strings"

	"github.com/Busnes-app/kydns-server/internal/store"
)

// The values the query log's policy field can take, alongside a list name.
const (
	PolicyForwarded = "forwarded"
	PolicyAllow     = "allow"
	PolicyDeny      = "deny"
	// PolicyPaused is the policy field while blocking is paused.
	PolicyPaused = "paused"
)

// defaultBlockTTL is how long a client should cache a block when settings
// carry no value.
const defaultBlockTTL = 60

// Decision is what the policy says about one forwarded name. Policy is the
// query log's policy field: "deny", "allow", a list name, or "forwarded".
type Decision struct {
	Blocked bool
	Policy  string
	TTL     uint32
	// Name is the normalized query name, or its lowercased form when strict
	// normalization failed.
	// It exists so the block counters can key on it without normalizing twice.
	Name string
}

type namedSet struct {
	name string
	set  *Set
}

// Snapshot is an immutable policy. The DNS hot path reads one with no lock;
// every change builds a new one.
type Snapshot struct {
	enabled bool
	ttl     uint32
	deny    *Set
	allow   *Set
	lists   []namedSet
}

// Build assembles a snapshot. Disabled lists are left out entirely rather than
// carried and skipped, so a disabled list costs nothing on the hot path.
func Build(set store.BlacklistSettings, lists []store.BlacklistList, rules []store.BlacklistRule) *Snapshot {
	ttl := set.BlockTTL
	if ttl <= 0 {
		ttl = defaultBlockTTL
	}
	s := &Snapshot{enabled: set.Enabled, ttl: uint32(ttl)}
	var deny, allow []string
	for _, r := range rules {
		switch r.Kind {
		case PolicyDeny:
			deny = append(deny, r.Domain)
		case PolicyAllow:
			allow = append(allow, r.Domain)
		}
	}
	s.deny, s.allow = NewSet(deny), NewSet(allow)
	for _, l := range lists {
		if !l.Enabled || len(l.Snapshot) == 0 {
			continue
		}
		s.lists = append(s.lists, namedSet{name: l.Name, set: NewSet(l.Snapshot)})
	}
	return s
}

// Decide applies the precedence the spec fixes: an explicit deny, then an
// explicit allow, then the enabled lists. A name it cannot normalize (an
// underscore label, say) is still matched leniently by its suffixes, so it is
// no way around the list, and otherwise forwarded rather than failed.
func (s *Snapshot) Decide(name string) Decision {
	if s == nil || !s.enabled {
		return Decision{Policy: PolicyForwarded}
	}
	n, err := Normalize(name)
	if err != nil {
		n = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(name)), ".")
		if n == "" {
			return Decision{Policy: PolicyForwarded}
		}
	}
	if s.deny.Match(n) {
		return Decision{Blocked: true, Policy: PolicyDeny, TTL: s.ttl, Name: n}
	}
	if s.allow.Match(n) {
		return Decision{Policy: PolicyAllow, Name: n}
	}
	for _, l := range s.lists {
		if l.set.Match(n) {
			return Decision{Blocked: true, Policy: l.name, TTL: s.ttl, Name: n}
		}
	}
	return Decision{Policy: PolicyForwarded, Name: n}
}

// Match is one source that matches a name: an allow or deny rule, or a list,
// with the entry that matched.
type Match struct {
	Source string `json:"source"` // "allow", "deny" or a list name
	Entry  string `json:"entry"`  // the rule or list entry: the name or a parent domain
}

// Explain is Decide plus every source that matches name, in precedence order,
// so "why is this blocked" can name the rule and the lists behind it. name must
// already be normalized. It ignores the enabled switch: the caller reports that.
func (s *Snapshot) Explain(name string) []Match {
	if s == nil {
		return nil
	}
	var out []Match
	for _, src := range []namedSet{{PolicyDeny, s.deny}, {PolicyAllow, s.allow}} {
		if e, ok := src.set.MatchedBy(name); ok {
			out = append(out, Match{Source: src.name, Entry: e})
		}
	}
	for _, l := range s.lists {
		if e, ok := l.set.MatchedBy(name); ok {
			out = append(out, Match{Source: l.name, Entry: e})
		}
	}
	return out
}

// Enabled reports the global filtering switch this snapshot was built with.
func (s *Snapshot) Enabled() bool { return s != nil && s.enabled }
