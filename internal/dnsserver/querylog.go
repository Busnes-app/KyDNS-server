package dnsserver

import (
	"strings"
	"sync"
	"time"
)

// QueryLogSize is how many recent queries the in-memory log keeps.
// ponytail: a fixed ring; a busy network sees minutes, not days. Operators who
// need history keep the stdout query log in their own logging platform.
const QueryLogSize = 10000

// maxLoggedName caps a stored query name. A DNS name is at most 253
// characters, but presentation format escapes odd bytes as \DDD.
const maxLoggedName = 1024

// QueryEntry is one answered query. Client is empty unless client-IP logging
// was on when it was answered.
type QueryEntry struct {
	Time   time.Time
	Client string
	Name   string // presentation format, no trailing dot; untrusted client input
	Type   string
	Rcode  string
	Source string // pipeline outcome: authoritative, forward, blocked, acl, ...
	Policy string // the query log's policy field
}

// Blocked reports whether the query got the blackhole answer.
func (e QueryEntry) Blocked() bool { return e.Source == "blocked" }

// QueryLog is a bounded ring of recent queries, in memory only. It fills only
// while query logging is on, and a restart or turning query logging (or the
// client IP) off empties it, so it never holds more than the stdout log the
// operator opted into. The opt-ins live here, under the ring's lock, so a
// query answered while one is being turned off cannot land after the clear.
type QueryLog struct {
	mu       sync.Mutex
	ring     []QueryEntry
	next     int
	full     bool
	on       bool
	clientIP bool
}

func NewQueryLog(size int) *QueryLog { return &QueryLog{ring: make([]QueryEntry, size)} }

// SetOptIns follows the two logging opt-ins. Turning either off empties the
// ring in the same critical section that every Add checks them in.
func (l *QueryLog) SetOptIns(queries, clientIP bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.on && !queries || l.clientIP && !clientIP {
		l.clearLocked()
	}
	l.on, l.clientIP = queries, clientIP
}

// Add records one query, overwriting the oldest once the ring is full. It
// drops the entry while query logging is off and the client while client-IP
// logging is off, whatever the caller saw a moment earlier.
func (l *QueryLog) Add(e QueryEntry) {
	if len(e.Name) > maxLoggedName {
		e.Name = e.Name[:maxLoggedName]
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.on {
		return
	}
	if !l.clientIP {
		e.Client = ""
	}
	l.ring[l.next] = e
	l.next = (l.next + 1) % len(l.ring)
	l.full = l.full || l.next == 0
}

func (l *QueryLog) clearLocked() {
	clear(l.ring)
	l.next, l.full = 0, false
}

// QueryFilter selects entries. Zero fields match everything.
type QueryFilter struct {
	Client      string    // exact client IP
	Domain      string    // the name or any subdomain of it
	Since       time.Time // inclusive
	Until       time.Time // exclusive
	BlockedOnly bool
}

func (f QueryFilter) match(e QueryEntry) bool {
	if f.Client != "" && e.Client != f.Client {
		return false
	}
	if f.Domain != "" && e.Name != f.Domain && !strings.HasSuffix(e.Name, "."+f.Domain) {
		return false
	}
	if !f.Since.IsZero() && e.Time.Before(f.Since) || !f.Until.IsZero() && !e.Time.Before(f.Until) {
		return false
	}
	return !f.BlockedOnly || e.Blocked()
}

// Each calls fn on every entry f matches, newest first, until fn returns
// false. It returns the oldest entry's time, so a caller can say how far back
// the log reaches; zero when it is empty.
func (l *QueryLog) Each(f QueryFilter, fn func(QueryEntry) bool) time.Time {
	f.Domain = strings.TrimSuffix(strings.ToLower(f.Domain), ".")
	l.mu.Lock()
	n := l.next
	if l.full {
		n = len(l.ring)
	}
	entries := make([]QueryEntry, 0, n)
	for i := 1; i <= n; i++ {
		entries = append(entries, l.ring[(l.next-i+len(l.ring))%len(l.ring)])
	}
	l.mu.Unlock()
	var oldest time.Time
	if len(entries) > 0 {
		oldest = entries[len(entries)-1].Time
	}
	for _, e := range entries {
		if f.match(e) && !fn(e) {
			break
		}
	}
	return oldest
}
