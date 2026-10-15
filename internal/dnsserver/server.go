package dnsserver

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Busnes-app/kydns-server/internal/zone"
	"github.com/miekg/dns"
)

// PolicyDecider is the blacklist policy's slice of the DNS pipeline. Keeping
// it an interface here means dnsserver never imports the policy package, and a
// test can block a name with six lines.
//
// decision is the query log's policy field: "allow", "deny", a list name, or
// "forwarded".
type PolicyDecider interface {
	Decide(name string) (blocked bool, decision string, ttl uint32)
}

type Options struct {
	Holder    *zone.Holder
	ACL       *ACL
	Auth      *Authoritative
	Forwarder *Forwarder
	// Policy is consulted only for names the authoritative lookup declines, so
	// a public list can never blackhole a local service. Nil means no filtering.
	Policy      PolicyDecider
	LogQueries  bool
	LogClientIP bool
	Logger      *slog.Logger
}

type Server struct {
	o           Options
	metrics     *Metrics
	logQueries  atomic.Bool
	logClientIP atomic.Bool
	mu          sync.Mutex
	srvs        []*dns.Server
}

func New(o Options) *Server {
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	s := &Server{o: o, metrics: NewMetrics()}
	s.SetLogging(o.LogQueries, o.LogClientIP)
	return s
}

// Metrics is the read side of the query counters, mirroring ACL.Stats.
func (s *Server) Metrics() *Metrics { return s.metrics }

// SetLogging changes both logging opt-ins. The client IP stays a separate
// choice from query logging: turning on one must never turn on the other.
func (s *Server) SetLogging(queries, clientIP bool) {
	s.logQueries.Store(queries)
	s.logClientIP.Store(clientIP)
}

// LogQueries and LogClientIP report the current logging opt-ins, mirroring
// Cache.Len and ACL.Stats as the read side of a live-tunable value.
func (s *Server) LogQueries() bool  { return s.logQueries.Load() }
func (s *Server) LogClientIP() bool { return s.logClientIP.Load() }

// ServeDNS is the whole pipeline: opcode and class checks, ACL, view
// resolution, authoritative lookup, then forwarding.
func (s *Server) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	start := time.Now()
	reply := func(m *dns.Msg, source, view, policy string) {
		// Every reply passes through here, so no path can forget the datagram
		// ceiling. Over-large answers are common now that DO=1 is forwarded.
		if _, udp := w.RemoteAddr().(*net.UDPAddr); udp {
			m.Truncate(clientUDPSize(r))
		}
		if err := w.WriteMsg(m); err != nil {
			s.o.Logger.Warn("write reply", "error", writeErrText(err, s.logClientIP.Load()))
		}
		d := time.Since(start)
		s.metrics.Record(source, m.Rcode, d)
		s.logQuery(r, m, w, source, view, policy, d)
	}
	fail := func(rcode int, source string) {
		m := new(dns.Msg)
		m.SetRcode(r, rcode)
		reply(m, source, "", "")
	}

	if r.Opcode != dns.OpcodeQuery {
		fail(dns.RcodeNotImplemented, "opcode")
		return
	}
	if len(r.Question) != 1 {
		fail(dns.RcodeFormatError, "question")
		return
	}
	q := r.Question[0]
	if q.Qclass != dns.ClassINET {
		fail(dns.RcodeRefused, "class")
		return
	}

	src := sourceAddr(w)
	if !s.o.ACL.Allow(src) {
		fail(dns.RcodeRefused, "acl")
		return
	}

	snap := s.o.Holder.Current()
	view := ""
	if snap != nil {
		view = snap.Matcher.Match(src)
	}

	block := func(ttl uint32, decision string) {
		m := new(dns.Msg)
		m.SetRcode(r, dns.RcodeNameError)
		m.Authoritative = false // the block is local policy, not zone data
		m.RecursionAvailable = true
		m.Ns = []dns.RR{blockSOA(q.Name, ttl)}
		reply(m, "blocked", view, decision)
	}
	edns := r.IsEdns0()
	do := edns != nil && edns.Do()

	if m := s.o.Auth.Answer(snap, view, q); m != nil {
		rcode := m.Rcode
		m.SetRcode(r, rcode)
		m.Authoritative = true
		if blocked, decision, ttl := s.finishChain(m, q, do); blocked {
			block(ttl, decision)
			return
		}
		reply(m, "authoritative", view, "local")
		return
	}

	// The name is not ours, so it would be forwarded. Filtering decides here,
	// before anything leaves the machine, and on CNAME targets in the answer.
	blocked, policy, ttl := s.decide(q.Name)
	if blocked {
		block(ttl, policy)
		return
	}

	if s.o.Forwarder == nil {
		fail(dns.RcodeServerFailure, "forward")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), forwardTimeout)
	defer cancel()
	resp, err := s.o.Forwarder.Resolve(ctx, q, do)
	if err != nil {
		// LOGGING.md: query names are logged only with query logging on.
		if s.logQueries.Load() {
			s.o.Logger.Warn("forward failed", "qname", q.Name, "error", err)
		} else {
			s.o.Logger.Warn("forward failed", "error", err)
		}
		fail(dns.RcodeServerFailure, "forward")
		return
	}
	if blocked, decision, ttl := s.cloaked(resp.Answer); blocked {
		block(ttl, decision)
		return
	}
	out := resp.Copy()
	rcode := resp.Rcode
	out.SetRcode(r, rcode)
	out.Authoritative = false
	out.RecursionAvailable = true
	if edns == nil {
		stripOPT(out)
	}
	reply(out, "forward", view, policy)
}

// forwardTimeout bounds one upstream resolution.
const forwardTimeout = 10 * time.Second

// decide asks the policy about name. A nil policy blocks nothing.
func (s *Server) decide(name string) (bool, string, uint32) {
	if s.o.Policy == nil {
		return false, "", 0
	}
	return s.o.Policy.Decide(name)
}

// cloaked reports a block on any CNAME target in rrs, so a tracker hidden
// behind a first-party alias is blocked like the tracker itself.
func (s *Server) cloaked(rrs []dns.RR) (bool, string, uint32) {
	for _, rr := range rrs {
		if c, ok := rr.(*dns.CNAME); ok {
			if blocked, decision, ttl := s.decide(c.Target); blocked {
				return true, decision, ttl
			}
		}
	}
	return false, "", 0
}

// finishChain completes an authoritative CNAME chain that leaves our zones,
// since a LAN stub resolver cannot. Only records on the chain are appended, so
// an upstream cannot plant data for other names. It reports a policy block on
// any outside name along the chain. On a forward failure the partial chain is
// still a correct answer.
func (s *Server) finishChain(m *dns.Msg, q dns.Question, do bool) (bool, string, uint32) {
	if q.Qtype == dns.TypeCNAME || len(m.Answer) == 0 || len(m.Answer) > cnameChaseDepth || s.o.Forwarder == nil {
		return false, "", 0
	}
	last, ok := m.Answer[len(m.Answer)-1].(*dns.CNAME)
	if !ok || s.o.Auth.Owns(last.Target) {
		return false, "", 0
	}
	if blocked, decision, ttl := s.decide(last.Target); blocked {
		return true, decision, ttl
	}
	ctx, cancel := context.WithTimeout(context.Background(), forwardTimeout)
	defer cancel()
	resp, err := s.o.Forwarder.Resolve(ctx, dns.Question{Name: last.Target, Qtype: q.Qtype, Qclass: q.Qclass}, do)
	if err != nil {
		return false, "", 0
	}
	chain := onChain(last.Target, resp.Answer)
	if blocked, decision, ttl := s.cloaked(chain); blocked {
		return true, decision, ttl
	}
	m.Answer = append(m.Answer, chain...)
	if resp.Rcode == dns.RcodeNameError {
		m.Rcode = dns.RcodeNameError
	}
	return false, "", 0
}

// onChain keeps the records owned by name or by a CNAME target reachable
// from it.
func onChain(name string, rrs []dns.RR) []dns.RR {
	names := map[string]bool{dns.CanonicalName(name): true}
	for grew := true; grew; { // each pass adds a hop or ends; bounded by len(rrs)
		grew = false
		for _, rr := range rrs {
			if c, ok := rr.(*dns.CNAME); ok && names[dns.CanonicalName(c.Hdr.Name)] && !names[dns.CanonicalName(c.Target)] {
				names[dns.CanonicalName(c.Target)] = true
				grew = true
			}
		}
	}
	var out []dns.RR
	for _, rr := range rrs {
		if names[dns.CanonicalName(rr.Header().Name)] {
			out = append(out, rr)
		}
	}
	return out
}

// sourceAddr extracts the client address, unmapped so v4-in-v6 peers match
// v4 rules and unzoned because netip.Prefix matches no zoned address at all:
// a link-local peer arrives as fe80::1%eth0, which would otherwise make
// fe80::/10 in allow_query unmatchable and miss every view.
func sourceAddr(w dns.ResponseWriter) netip.Addr {
	host, _, err := net.SplitHostPort(w.RemoteAddr().String())
	if err != nil {
		return netip.Addr{}
	}
	a, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return a.Unmap().WithZone("")
}

// writeErrText renders a WriteMsg error for logging. A *net.OpError's
// Error() embeds the peer address (net.OpError.Addr), which would leak the
// client IP through an ungated path — the same information log_client_ip
// exists to gate. Unless that flag is already on, the op and the underlying
// error are reported without it.
func writeErrText(err error, includeAddr bool) string {
	if includeAddr {
		return err.Error()
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return opErr.Op + ": " + opErr.Err.Error()
	}
	return err.Error()
}

// clientUDPSize is the datagram budget the client advertised. A client that
// sent no OPT record does not speak EDNS0, so 512 is its ceiling. Larger
// offers are capped at 1232, past which UDP fragments (DNS Flag Day 2020).
func clientUDPSize(r *dns.Msg) int {
	if edns := r.IsEdns0(); edns != nil {
		return min(int(edns.UDPSize()), 1232)
	}
	return dns.MinMsgSize
}

// stripOPT removes the EDNS0 record from a response. The forwarder always
// speaks EDNS0 upstream, but a client that did not offer an OPT record must
// not be handed one back.
func stripOPT(m *dns.Msg) {
	extra := m.Extra[:0]
	for _, rr := range m.Extra {
		if rr.Header().Rrtype != dns.TypeOPT {
			extra = append(extra, rr)
		}
	}
	m.Extra = extra
}

// blockSOA synthesizes the authority record that lets a client cache a block.
// Its owner is the queried name, so the negative answer is cached for exactly
// that name and nothing wider.
func blockSOA(qname string, ttl uint32) *dns.SOA {
	n := dns.Fqdn(qname)
	return &dns.SOA{
		Hdr:     dns.RR_Header{Name: n, Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: ttl},
		Ns:      n,
		Mbox:    n,
		Serial:  1,
		Refresh: 3600, Retry: 600, Expire: 604800, Minttl: ttl,
	}
}

// logQuery honors the two-flag policy: query logging is off by default, and
// the client IP needs its own separate flag. The policy field says which
// decision produced the answer; it never says who asked.
func (s *Server) logQuery(r, m *dns.Msg, w dns.ResponseWriter, source, view, policy string, d time.Duration) {
	if !s.logQueries.Load() || len(r.Question) == 0 {
		return
	}
	q := r.Question[0]
	args := []any{
		"qname", q.Name,
		"qtype", dns.TypeToString[q.Qtype],
		"rcode", dns.RcodeToString[m.Rcode],
		"source", source,
		"view", view,
		"policy", policy,
		"duration_ms", d.Milliseconds(),
	}
	if s.logClientIP.Load() {
		args = append(args, "client", w.RemoteAddr().String())
	}
	s.o.Logger.Info("query", args...)
}

// ListenAndServe starts UDP and TCP listeners on addr and blocks until one
// fails.
func (s *Server) ListenAndServe(addr string) error {
	errs := make(chan error, 2)
	s.mu.Lock()
	for _, network := range []string{"udp", "tcp"} {
		srv := &dns.Server{Addr: addr, Net: network, Handler: s}
		s.srvs = append(s.srvs, srv)
		go func() { errs <- srv.ListenAndServe() }()
	}
	s.mu.Unlock()
	return <-errs
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var joined error
	for _, srv := range s.srvs {
		if err := srv.ShutdownContext(ctx); err != nil {
			joined = errors.Join(joined, err)
		}
	}
	s.srvs = nil
	return joined
}
