package dnsserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/Busnes-app/kydns-server/internal/upstream"
	"github.com/miekg/dns"
	"golang.org/x/sync/singleflight"
)

// UpstreamStatus is what the last query to one upstream did. Without it, a
// firewall that blocks port 853 presents to the operator as "DNS is broken".
type UpstreamStatus struct {
	Spec      string
	Secure    bool
	LastError string
	LastErrAt time.Time
	LastOKAt  time.Time
}

// fwdState pairs the upstream list with its status slice. They are only ever
// swapped together, so an in-flight query cannot record a result against an
// upstream that is no longer at that index.
//
// wg counts queries currently using ups. A reader increments it before it can
// be swapped out (see Forwarder.acquireState) and decrements it when done, so
// Replace can wait for exactly those queries to finish before closing.
type fwdState struct {
	ups []upstream.Upstream
	wg  sync.WaitGroup

	mu     sync.Mutex
	status []UpstreamStatus
}

// close releases every retired upstream's connections, skipping any instance
// also present in keep: Replace may carry an unchanged Upstream over into the
// new list, and closing it would tear down a pool that is still live.
func (st *fwdState) close(keep []upstream.Upstream) {
	for _, u := range st.ups {
		if carriedOver(keep, u) {
			continue
		}
		if err := u.Close(); err != nil {
			slog.Default().Warn("failed to close retired upstream", "upstream", u.String(), "error", err)
		}
	}
}

func carriedOver(keep []upstream.Upstream, u upstream.Upstream) bool {
	for _, k := range keep {
		if k == u {
			return true
		}
	}
	return false
}

// verdict keeps only the rcode and the OPT carrying the EDE. Any records a
// bogus SERVFAIL brings along are the data that failed validation, and must
// not reach a client or be spliced into a CNAME chain.
func verdict(m *dns.Msg) *dns.Msg {
	out := new(dns.Msg)
	out.MsgHdr = m.MsgHdr
	out.AuthenticatedData = false
	out.Question = m.Question
	out.Extra = []dns.RR{m.IsEdns0()}
	return out
}

// Forwarder resolves non-authoritative queries through the cache and, on a
// miss, the upstream list in order.
type Forwarder struct {
	mu      sync.RWMutex
	state   *fwdState
	timeout time.Duration
	cache   *Cache
	group   singleflight.Group
}

func NewForwarder(ups []upstream.Upstream, timeout time.Duration, c *Cache) *Forwarder {
	f := &Forwarder{timeout: timeout, cache: c}
	f.Replace(ups)
	return f
}

// Replace swaps the upstream list. Status starts clean: the new upstreams
// have never been tried, and a stale error would send the operator after a
// problem that no longer exists.
//
// Replace takes ownership of every upstream it retires: once it returns, the
// caller must not use or close them itself. An upstream instance that is
// still live when its retirement actually runs (present in whichever list is
// current at that moment, not necessarily the list this call installed) is
// never closed — a rapid Replace(a)->Replace(b)->Replace(a) must not let the
// first retirement close the second Replace's a out from under it.
//
// A query already in flight holds the retiring state and may still be
// mid-exchange on one of its connections. acquireState/wg tracks exactly
// those queries, so the retired upstreams are closed only after every one of
// them has finished — not on a timer guessing how long that could take.
func (f *Forwarder) Replace(ups []upstream.Upstream) {
	st := &fwdState{ups: ups, status: make([]UpstreamStatus, len(ups))}
	for i, u := range ups {
		st.status[i] = UpstreamStatus{Spec: u.String(), Secure: u.Secure()}
	}
	f.mu.Lock()
	old := f.state
	f.state = st
	f.mu.Unlock()
	if old != nil {
		go func() {
			old.wg.Wait()
			// Re-read whichever list is live now, not the one captured when
			// this retirement was scheduled: a later Replace may have
			// brought one of old's upstreams back before this goroutine woke.
			old.close(f.currentState().ups)
		}()
	}
}

// currentState is for readers that only need a consistent snapshot, not a
// guarantee that the upstreams inside it stay open — Status never touches a
// connection.
func (f *Forwarder) currentState() *fwdState {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.state
}

// acquireState loads the state a query will use and marks it in-use so
// Replace cannot close its upstreams until the caller calls the returned
// done func. RLock ties the increment to the swap: any reader that observes
// the old state here provably incremented wg before Replace's write lock
// could have swapped it out from under it.
func (f *Forwarder) acquireState() (st *fwdState, done func()) {
	f.mu.RLock()
	st = f.state
	st.wg.Add(1)
	f.mu.RUnlock()
	return st, st.wg.Done
}

// Status is a snapshot for the UI, copied so callers cannot race the recorder.
func (f *Forwarder) Status() []UpstreamStatus {
	st := f.currentState()
	st.mu.Lock()
	defer st.mu.Unlock()
	return append([]UpstreamStatus(nil), st.status...)
}

// record writes against the state the query started with. A result arriving
// after a swap lands on the retired object and is discarded with it, which is
// correct: it describes upstreams that are no longer configured.
func (st *fwdState) record(i int, err error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if err != nil {
		st.status[i].LastError = err.Error()
		st.status[i].LastErrAt = time.Now()
		return
	}
	st.status[i].LastError = ""
	st.status[i].LastErrAt = time.Time{}
	st.status[i].LastOKAt = time.Now()
}

// Resolve answers from cache, or collapses concurrent identical misses into a
// single upstream query. That collapse is what survives the boot-time
// stampede when every device on the LAN wakes at once.
func (f *Forwarder) Resolve(ctx context.Context, q dns.Question, do bool) (*dns.Msg, error) {
	if m, ok := f.cache.Get(q, do); ok {
		return m, nil
	}
	key := fmt.Sprintf("%s|%d|%t", strings.ToLower(dns.Fqdn(q.Name)), q.Qtype, do)
	v, err, _ := f.group.Do(key, func() (any, error) {
		if m, ok := f.cache.Get(q, do); ok {
			return m, nil
		}
		m, err := f.exchange(ctx, q, do)
		if err != nil {
			return nil, err
		}
		f.cache.Put(q, do, m)
		return m, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*dns.Msg).Copy(), nil
}

func (f *Forwarder) exchange(ctx context.Context, q dns.Question, do bool) (*dns.Msg, error) {
	req := new(dns.Msg)
	req.SetQuestion(dns.Fqdn(q.Name), q.Qtype)
	req.SetEdns0(1232, do)
	// The upstream is the validator, so it is never told to skip checking, and
	// AD asks for its verdict without the RRSIG payload (RFC 6840 5.7).
	req.CheckingDisabled = false
	req.AuthenticatedData = true

	st, done := f.acquireState()
	defer done()
	if len(st.ups) == 0 {
		return nil, errors.New("no upstreams configured")
	}
	var lastErr error
	for i, u := range st.ups {
		attempt, cancel := context.WithTimeout(ctx, f.timeout)
		resp, err := u.Exchange(attempt, req)
		cancel()
		if err == nil && resp != nil && u.Secure() && bogus(resp) {
			// The upstream validated the answer and rejected it. That verdict
			// is final: asking the next upstream would let one that does not
			// validate hand back the very answer that failed. It is not an
			// outage either, so it is not recorded against u. Like AD, it
			// counts only from an authenticated channel: on plaintext anyone
			// on the path could forge one to stop the failover.
			return verdict(resp), nil
		}
		if err == nil && resp != nil && resp.Rcode != dns.RcodeServerFailure {
			if !u.Secure() {
				// Cleared here rather than at the response boundary, so a
				// plaintext answer cannot carry AD into the cache either.
				resp.AuthenticatedData = false
			}
			st.record(i, nil)
			return resp, nil
		}
		var reason string // bare: Status.Spec already names the upstream
		switch {
		case err != nil:
			reason = err.Error()
			lastErr = err
		case resp == nil:
			reason = "returned no message"
			lastErr = fmt.Errorf("upstream %s %s", u, reason)
		default:
			reason = "returned " + dns.RcodeToString[resp.Rcode]
			lastErr = fmt.Errorf("upstream %s %s", u, reason)
		}
		// record keeps the bare reason so the UI doesn't print the upstream
		// twice in adjacent columns; lastErr keeps the spec prefix, since the
		// aggregate error below has no such column to lean on.
		st.record(i, errors.New(reason))
	}
	return nil, fmt.Errorf("all upstreams failed: %w", lastErr)
}

// dnssecEDE are the RFC 8914 Extended DNS Errors that say a SERVFAIL is a
// DNSSEC validation failure rather than a resolver that could not get an
// answer. Cloudflare, Quad9 and Google all attach one (9 or 6) to a bogus
// SERVFAIL.
var dnssecEDE = map[uint16]bool{
	dns.ExtendedErrorCodeUnsupportedDNSKEYAlgorithm: true,
	dns.ExtendedErrorCodeUnsupportedDSDigestType:    true,
	dns.ExtendedErrorCodeDNSBogus:                   true,
	dns.ExtendedErrorCodeSignatureExpired:           true,
	dns.ExtendedErrorCodeSignatureNotYetValid:       true,
	dns.ExtendedErrorCodeDNSKEYMissing:              true,
	dns.ExtendedErrorCodeRRSIGsMissing:              true,
	dns.ExtendedErrorCodeNoZoneKeyBitSet:            true,
	dns.ExtendedErrorCodeNSECMissing:                true,
}

// bogus reports a SERVFAIL that carries a DNSSEC Extended DNS Error.
//
// ponytail: a validator that sends no EDE is indistinguishable from an outage,
// so its bogus SERVFAIL still falls through to the next upstream. Closing that
// needs local validation (trust anchor, chain walking), which KyDNS does not do.
func bogus(m *dns.Msg) bool {
	opt := m.IsEdns0()
	if m.Rcode != dns.RcodeServerFailure || opt == nil {
		return false
	}
	for _, o := range opt.Option {
		if ede, ok := o.(*dns.EDNS0_EDE); ok && dnssecEDE[ede.InfoCode] {
			return true
		}
	}
	return false
}
