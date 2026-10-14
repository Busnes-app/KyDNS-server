package dnsserver

import (
	"bytes"
	"log/slog"
	"net"
	"strings"
	"testing"

	"github.com/Busnes-app/kydns-server/internal/store"
	"github.com/Busnes-app/kydns-server/internal/zone"
	"github.com/miekg/dns"
)

// chainUpstream answers any name with a CNAME to final.example.net. and that
// name's A record, plus one out-of-chain record a hostile upstream might slip
// in to poison a local name.
func chainUpstream() *fakeUpstream {
	return &fakeUpstream{name: "tls://1.1.1.1:853", secure: true, reply: func(m *dns.Msg) (*dns.Msg, error) {
		resp := new(dns.Msg)
		resp.SetReply(m)
		qn := m.Question[0].Name
		rrs := []string{"final.example.net. 300 IN A 203.0.113.7", "kypost.home.arpa. 300 IN A 6.6.6.6"}
		if qn != "final.example.net." {
			rrs = append([]string{qn + " 300 IN CNAME final.example.net."}, rrs...)
		}
		for _, s := range rrs {
			rr, err := dns.NewRR(s)
			if err != nil {
				return nil, err
			}
			resp.Answer = append(resp.Answer, rr)
		}
		return resp, nil
	}}
}

func chaseServer(t *testing.T, up *fakeUpstream, blocked map[string]string, logger *slog.Logger) *Server {
	t.Helper()
	h := zone.NewHolder(func() (zone.Input, error) {
		return zone.Input{
			Zone:    "home.arpa.",
			Records: []store.Record{{Name: "cdn.home.arpa.", Type: "CNAME", Value: "edge.example.net."}},
		}, nil
	}, nil)
	if err := h.Rebuild(); err != nil {
		t.Fatal(err)
	}
	return New(Options{
		Holder:    h,
		ACL:       NewACL(prefixes(t, "0.0.0.0/0")),
		Auth:      NewAuthoritative("home.arpa.", 60, nil),
		Forwarder: newForwarder(up),
		Policy:    &stubPolicy{blocked: blocked},
		Logger:    logger,
	})
}

func serve(s *Server, name string) *dns.Msg {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), dns.TypeA)
	w := &fakeResponseWriter{remote: &net.UDPAddr{IP: net.ParseIP("192.168.1.5"), Port: 12345}}
	s.ServeDNS(w, m)
	return w.msg
}

func answerText(m *dns.Msg) []string {
	var out []string
	for _, rr := range m.Answer {
		out = append(out, strings.Join(strings.Fields(rr.String()), " "))
	}
	return out
}

// A stub resolver cannot finish a chain itself, so an in-zone CNAME to an
// outside name is completed through the forwarder, chain records only.
func TestOutOfZoneCNAMECompletedViaForwarder(t *testing.T) {
	m := serve(chaseServer(t, chainUpstream(), nil, nil), "cdn.home.arpa.")
	want := []string{
		"cdn.home.arpa. 60 IN CNAME edge.example.net.",
		"edge.example.net. 300 IN CNAME final.example.net.",
		"final.example.net. 300 IN A 203.0.113.7",
	}
	if got := answerText(m); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("Answer = %q, want %q", got, want)
	}
	if m.Rcode != dns.RcodeSuccess {
		t.Errorf("rcode = %s, want NOERROR", dns.RcodeToString[m.Rcode])
	}
}

func TestOutOfZoneCNAMETargetBlocked(t *testing.T) {
	for _, name := range []string{"edge.example.net.", "final.example.net."} {
		up := chainUpstream()
		m := serve(chaseServer(t, up, map[string]string{name: "steven-black"}, nil), "cdn.home.arpa.")
		if m.Rcode != dns.RcodeNameError || len(m.Answer) != 0 {
			t.Errorf("%s blocked: rcode %s answer %v, want a bare NXDOMAIN", name, dns.RcodeToString[m.Rcode], m.Answer)
		}
		if len(m.Ns) != 1 || m.Ns[0].Header().Name != "cdn.home.arpa." {
			t.Errorf("%s blocked: Ns = %v, want the block SOA for the query name", name, m.Ns)
		}
		if name == "edge.example.net." && up.calls.Load() != 0 {
			t.Error("a blocked chase target still reached the upstream")
		}
	}
}

// CNAME cloaking: a tracker hidden behind a first-party CNAME is blocked.
func TestForwardedCNAMETargetBlocked(t *testing.T) {
	m := serve(chaseServer(t, chainUpstream(), map[string]string{"final.example.net.": "steven-black"}, nil), "www.example.com.")
	if m.Rcode != dns.RcodeNameError || len(m.Answer) != 0 {
		t.Errorf("rcode %s answer %v, want a bare NXDOMAIN", dns.RcodeToString[m.Rcode], m.Answer)
	}
	if len(m.Ns) != 1 || m.Ns[0].Header().Name != "www.example.com." {
		t.Errorf("Ns = %v, want the block SOA for the query name", m.Ns)
	}
}

// LOGGING.md: query names are never logged by default.
func TestForwardFailureLogsQnameOnlyWithQueryLogging(t *testing.T) {
	var buf bytes.Buffer
	s := chaseServer(t, deadUpstream("tls://1.1.1.1:853", true), nil, slog.New(slog.NewTextHandler(&buf, nil)))
	serve(s, "secret.example.com.")
	if out := buf.String(); !strings.Contains(out, "forward failed") || strings.Contains(out, "secret.example.com") {
		t.Errorf("log = %q, want the failure without the query name", out)
	}
	buf.Reset()
	s.SetLogging(true, false)
	serve(s, "secret2.example.com.")
	if !strings.Contains(buf.String(), `msg="forward failed" qname=secret2.example.com.`) {
		t.Errorf("log = %q, want the query name once query logging is on", buf.String())
	}
}

// A client may advertise up to 65535 bytes; past 1232 fragmentation makes UDP
// unreliable and spoofable, so replies are capped (DNS Flag Day 2020).
func TestClientUDPSizeCapped(t *testing.T) {
	m := new(dns.Msg)
	m.SetQuestion("example.com.", dns.TypeA)
	m.SetEdns0(65535, false)
	if got := clientUDPSize(m); got != 1232 {
		t.Errorf("clientUDPSize = %d, want 1232", got)
	}
	m = new(dns.Msg).SetEdns0(800, false)
	if got := clientUDPSize(m); got != 800 {
		t.Errorf("clientUDPSize = %d, want the smaller advertised 800", got)
	}
}
