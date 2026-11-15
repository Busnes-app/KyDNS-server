package dnsserver

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Busnes-app/kydns-server/internal/dnssec"
	"github.com/Busnes-app/kydns-server/internal/dnssec/dnssectest"
	"github.com/miekg/dns"
)

// signedWorld is a root, a signed test. and an unsigned zone below it.
func signedWorld() *dnssectest.World {
	return dnssectest.New(
		dnssectest.NewZone(".", dns.ECDSAP256SHA256),
		dnssectest.NewZone("test.", dns.ECDSAP256SHA256).Add(
			"www.test. 300 IN A 192.0.2.1", "edge.test. 300 IN A 192.0.2.9"),
		dnssectest.NewZone("unsigned.test.", 0).Add("www.unsigned.test. 300 IN A 192.0.2.2"),
	)
}

// worldUpstream is a plaintext upstream answering from w: local validation
// must work without trusting the channel.
func worldUpstream(w *dnssectest.World) *fakeUpstream {
	return &fakeUpstream{name: "udp://192.0.2.53:53", reply: func(m *dns.Msg) (*dns.Msg, error) {
		resp, err := w.Query(context.Background(), m.Question[0].Name, m.Question[0].Qtype)
		if resp != nil {
			resp.Id = m.Id
			resp.AuthenticatedData = true // a lying upstream claims everything
		}
		return resp, err
	}}
}

func validatingServer(t *testing.T, w *dnssectest.World) (*Server, *fakeUpstream) {
	t.Helper()
	up := worldUpstream(w)
	s := chaseServer(t, up, nil, nil)
	v := dnssec.New(s.o.Forwarder.Query, nil)
	v.UseAnchors(w.Anchors())
	s.o.Forwarder.SetValidator(v)
	return s, up
}

type flags struct{ do, ad, edns bool }

func askV(s *Server, name string, f flags) *dns.Msg {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), dns.TypeA)
	m.AuthenticatedData = f.ad
	if f.edns || f.do {
		m.SetEdns0(1232, f.do)
	}
	w := &fakeResponseWriter{remote: &net.UDPAddr{IP: net.ParseIP("192.168.1.5"), Port: 12345}}
	s.ServeDNS(w, m)
	return w.msg
}

func hasType(m *dns.Msg, t uint16) bool {
	for _, sec := range [][]dns.RR{m.Answer, m.Ns, m.Extra} {
		for _, rr := range sec {
			if rr.Header().Rrtype == t {
				return true
			}
		}
	}
	return false
}

func TestValidatedADOnlyWhenAsked(t *testing.T) {
	s, _ := validatingServer(t, signedWorld())
	cases := []struct {
		f      flags
		ad     bool
		rrsigs bool
	}{
		{flags{}, false, false},
		{flags{edns: true}, false, false},
		{flags{ad: true}, true, false},
		{flags{do: true}, true, true},
	}
	for _, c := range cases {
		m := askV(s, "www.test.", c.f)
		if m.Rcode != dns.RcodeSuccess || m.AuthenticatedData != c.ad || hasType(m, dns.TypeRRSIG) != c.rrsigs {
			t.Errorf("%+v: rcode %d AD %v RRSIG %v", c.f, m.Rcode, m.AuthenticatedData, hasType(m, dns.TypeRRSIG))
		}
	}
}

func TestValidatedReplyEchoesClientDOAndDropsUpstreamEDE(t *testing.T) {
	w := signedWorld()
	w.Tamper = func(_ string, _ uint16, m *dns.Msg) {
		opt := &dns.OPT{Hdr: dns.RR_Header{Name: ".", Rrtype: dns.TypeOPT}}
		opt.SetDo()
		opt.Option = []dns.EDNS0{&dns.EDNS0_EDE{InfoCode: dns.ExtendedErrorCodeUnsupportedDNSKEYAlgorithm}}
		m.Extra = append(m.Extra, opt)
	}
	s, _ := validatingServer(t, w)
	m := askV(s, "www.test.", flags{edns: true})
	opt := m.IsEdns0()
	if opt == nil || opt.Do() || len(opt.Option) != 0 {
		t.Fatalf("OPT = %v", opt)
	}
}

func TestValidatedInsecureNeverAD(t *testing.T) {
	s, _ := validatingServer(t, signedWorld())
	m := askV(s, "www.unsigned.test.", flags{do: true, ad: true})
	if m.Rcode != dns.RcodeSuccess || m.AuthenticatedData {
		t.Fatalf("rcode %d AD %v, want an answer without AD", m.Rcode, m.AuthenticatedData)
	}
}

func TestValidatedBogusIsServfailWithEDE(t *testing.T) {
	w := signedWorld()
	w.Tamper = func(n string, q uint16, m *dns.Msg) {
		if n == "www.test." && q == dns.TypeA {
			m.Answer[0].(*dns.A).A[3] = 66
		}
	}
	s, _ := validatingServer(t, w)
	m := askV(s, "www.test.", flags{edns: true})
	if m.Rcode != dns.RcodeServerFailure || !hasEDE(m, dns.ExtendedErrorCodeDNSBogus) || len(m.Answer) != 0 {
		t.Fatalf("got %v", m)
	}
	// No EDNS from the client: still SERVFAIL, and no OPT handed back.
	if m := askV(s, "www.test.", flags{}); m.Rcode != dns.RcodeServerFailure || m.IsEdns0() != nil {
		t.Fatalf("plain client: %v", m)
	}
}

func TestValidationIgnoresUpstreamVerdicts(t *testing.T) {
	s, up := validatingServer(t, signedWorld())
	askV(s, "www.test.", flags{})
	q := up.lastSent(t)
	if !q.CheckingDisabled || q.IsEdns0() == nil || !q.IsEdns0().Do() {
		t.Fatalf("upstream query: CD %v DO %v, want CD=1 DO=1", q.CheckingDisabled, q.IsEdns0() != nil && q.IsEdns0().Do())
	}
}

func TestLocalNamesNeverAD(t *testing.T) {
	s, _ := validatingServer(t, signedWorld())
	// cdn.home.arpa is a local CNAME to edge.example.net, forwarded for the tail.
	m := askV(s, "cdn.home.arpa.", flags{do: true, ad: true})
	if m.AuthenticatedData {
		t.Fatalf("local answer carries AD: %v", m)
	}
}

func TestSwitchingValidationFlushesCache(t *testing.T) {
	s, _ := validatingServer(t, signedWorld())
	askV(s, "www.test.", flags{do: true})
	if s.o.Forwarder.cache.Len() == 0 {
		t.Fatal("nothing cached")
	}
	s.o.Forwarder.SetValidator(nil)
	if n := s.o.Forwarder.cache.Len(); n != 0 {
		t.Fatalf("%d entries survived the switch", n)
	}
	if m := askV(s, "www.test.", flags{do: true}); m.AuthenticatedData {
		t.Fatal("validation off: a plaintext upstream's AD must still be cleared")
	}
}

// A lookup started before validation was switched on must not leave its
// unchecked answer in the validating cache.
func TestLookupAcrossTheSwitchIsNotCached(t *testing.T) {
	w := signedWorld()
	release := make(chan struct{})
	var first atomic.Bool
	up := &fakeUpstream{name: "tls://192.0.2.53:853", secure: true, reply: func(m *dns.Msg) (*dns.Msg, error) {
		resp, err := w.Query(context.Background(), m.Question[0].Name, m.Question[0].Qtype)
		resp.Id = m.Id
		if m.Question[0].Name == "www.test." && first.CompareAndSwap(false, true) {
			<-release
			resp.Answer[0].(*dns.A).A[3] = 66 // forged, with a claimed AD
			resp.AuthenticatedData = true
		}
		return resp, err
	}}
	f := newForwarder(up)
	q := dns.Question{Name: "www.test.", Qtype: dns.TypeA, Qclass: dns.ClassINET}
	done := make(chan struct{})
	go func() { f.Resolve(context.Background(), q, true); close(done) }()
	waitFor(t, time.Second, func() bool { return up.calls.Load() == 1 })
	v := dnssec.New(f.Query, nil)
	v.UseAnchors(w.Anchors())
	f.SetValidator(v)
	close(release)
	<-done
	m, err := f.Resolve(context.Background(), q, true)
	if err != nil || m.Rcode != dns.RcodeSuccess || m.AuthenticatedData != true || m.Answer[0].(*dns.A).A[3] == 66 {
		t.Fatalf("validating lookup served the pre-switch answer: %v %v", m, err)
	}
}

// PutUntil never keeps an answer past the deadline, even under a TTL floor.
func TestCachePutUntilBeatsTheTTLFloor(t *testing.T) {
	c := NewCache(10, 600, 3600, 300) // a 10-minute floor
	now := time.Unix(1_000_000, 0)
	c.now = func() time.Time { return now }
	q := dns.Question{Name: "www.test.", Qtype: dns.TypeA, Qclass: dns.ClassINET}
	m := new(dns.Msg)
	m.SetQuestion(q.Name, q.Qtype)
	rr, _ := dns.NewRR("www.test. 300 IN A 192.0.2.1")
	m.Answer = []dns.RR{rr}
	c.PutUntil(q, true, m, now.Add(30*time.Second))
	if _, ok := c.Get(q, true); !ok {
		t.Fatal("not cached at all")
	}
	now = now.Add(31 * time.Second)
	if _, ok := c.Get(q, true); ok {
		t.Fatal("served after the signatures expired")
	}
	c.PutUntil(q, true, m, now.Add(-time.Second))
	if c.Len() != 0 {
		t.Fatal("cached an answer whose signatures had already expired")
	}
}
