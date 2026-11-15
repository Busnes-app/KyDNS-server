package dnssec

import (
	"context"
	"crypto/ed25519"
	"encoding/xml"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Busnes-app/kydns-server/internal/dnssec/dnssectest"
	"github.com/miekg/dns"
)

func world(t *testing.T) *dnssectest.World {
	t.Helper()
	test := dnssectest.NewZone("test.", dns.ECDSAP256SHA256).Add(
		"www.test. 300 IN A 192.0.2.1",
		"*.wild.test. 300 IN A 192.0.2.2",
		"b.wild.test. 300 IN A 192.0.2.20",   // a closer encloser below the wildcard
		"a.c.wild.test. 300 IN A 192.0.2.22", // c.wild.test: an empty non-terminal below it
		"alias.test. 300 IN CNAME www.test.",
		"out.test. 300 IN CNAME www.unsigned.test.",
		"a.b.ent.test. 300 IN A 192.0.2.3",
		"*.wc.test. 300 IN CNAME www.test.",
		"dn.test. 300 IN DNAME secure.test.",
	)
	n3 := dnssectest.NewZone("n3.test.", dns.ECDSAP256SHA256).Add("www.n3.test. 300 IN A 192.0.2.4")
	n3.NSEC3, n3.OptOut = true, true
	n3s := dnssectest.NewZone("n3s.test.", dns.ECDSAP256SHA256).Add("www.n3s.test. 300 IN A 192.0.2.14", "*.w.n3s.test. 300 IN A 192.0.2.15",
		"dn.n3s.test. 300 IN DNAME secure.test.")
	n3s.NSEC3 = true
	expired := dnssectest.NewZone("expired.test.", dns.ECDSAP256SHA256).Add("www.expired.test. 300 IN A 192.0.2.5")
	expired.Expired = true
	zones := []*dnssectest.Zone{
		dnssectest.NewZone(".", dns.ECDSAP256SHA256), test, n3, n3s, expired,
		dnssectest.NewZone("plain.n3s.test.", 0).Add("www.plain.n3s.test. 300 IN A 192.0.2.16"),
		dnssectest.NewZone("secure.test.", dns.ED25519).Add("www.secure.test. 300 IN A 192.0.2.6"),
		dnssectest.NewZone("ed.test.", dns.ED448).Add("www.ed.test. 300 IN A 192.0.2.7"),
		dnssectest.NewZone("rsa.test.", dns.RSASHA256).Add("www.rsa.test. 300 IN A 192.0.2.8"),
		dnssectest.NewZone("p384.test.", dns.ECDSAP384SHA384).Add("www.p384.test. 300 IN A 192.0.2.9"),
		dnssectest.NewZone("sha1.test.", dns.RSASHA1).Add("www.sha1.test. 300 IN A 192.0.2.10"),
		dnssectest.NewZone("unsigned.test.", 0).Add("www.unsigned.test. 300 IN A 192.0.2.11"),
		dnssectest.NewZone("real.wc.test.", dns.ECDSAP256SHA256).Add("www.real.wc.test. 300 IN A 192.0.2.21"),
		dnssectest.NewZone("deleg.n3.test.", 0).Add("www.deleg.n3.test. 300 IN A 192.0.2.12"),
		dnssectest.NewZone("wrongkey.test.", dns.ECDSAP256SHA256).Add("www.wrongkey.test. 300 IN A 192.0.2.13"),
	}
	w := dnssectest.New(zones...)
	// wrongkey.test's parent publishes the DS of a key the child never uses.
	other := dnssectest.NewZone("wrongkey.test.", dns.ECDSAP256SHA256)
	for i, rr := range test.RRs {
		if ds, ok := rr.(*dns.DS); ok && ds.Hdr.Name == "wrongkey.test." {
			test.RRs[i] = other.DS()
		}
	}
	return w
}

func newV(w *dnssectest.World) *Validator {
	v := New(w.Query, nil)
	v.UseAnchors(w.Anchors())
	v.now = func() time.Time { return dnssectest.Now }
	return v
}

func check(t *testing.T, v *Validator, w *dnssectest.World, name string, qtype uint16) (Result, *dns.Msg) {
	t.Helper()
	m, _ := w.Query(context.Background(), name, qtype)
	res, err := v.Check(context.Background(), dns.Question{Name: name, Qtype: qtype, Qclass: dns.ClassINET}, m)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return res, m
}

func TestVerdicts(t *testing.T) {
	cases := []struct {
		name  string
		qtype uint16
		want  Security
		ede   uint16
	}{
		{"www.test.", dns.TypeA, Secure, 0},
		{"WWW.Test.", dns.TypeA, Secure, 0},
		{"www.secure.test.", dns.TypeA, Secure, 0},
		{"www.ed.test.", dns.TypeA, Secure, 0},
		{"www.rsa.test.", dns.TypeA, Secure, 0},
		{"www.p384.test.", dns.TypeA, Secure, 0},
		{"test.", dns.TypeDNSKEY, Secure, 0},
		{"secure.test.", dns.TypeDS, Secure, 0},
		{"alias.test.", dns.TypeA, Secure, 0},
		{"alias.test.", dns.TypeCNAME, Secure, 0}, // the CNAME itself, not followed
		{"x.wild.test.", dns.TypeA, Secure, 0},
		{"nope.test.", dns.TypeA, Secure, 0},      // NXDOMAIN
		{"www.test.", dns.TypeAAAA, Secure, 0},    // NODATA
		{"b.ent.test.", dns.TypeA, Secure, 0},     // empty non-terminal
		{"unsigned.test.", dns.TypeDS, Secure, 0}, // proven absent DS
		{"www.n3.test.", dns.TypeA, Secure, 0},
		{"www.n3.test.", dns.TypeAAAA, Secure, 0},  // NSEC3 NODATA
		{"nope.n3s.test.", dns.TypeA, Secure, 0},   // NSEC3 NXDOMAIN
		{"www.n3s.test.", dns.TypeTXT, Secure, 0},  // NSEC3 NODATA
		{"x.w.n3s.test.", dns.TypeA, Secure, 0},    // NSEC3 wildcard
		{"plain.n3s.test.", dns.TypeDS, Secure, 0}, // NSEC3 DS denial, no opt-out
		{"www.plain.n3s.test.", dns.TypeA, Insecure, 0},
		{"www.unsigned.test.", dns.TypeA, Insecure, 0},
		{"out.test.", dns.TypeA, Insecure, 0},          // secure CNAME into an unsigned zone
		{"www.sha1.test.", dns.TypeA, Insecure, 0},     // unsupported algorithm (RFC 6840)
		{"www.deleg.n3.test.", dns.TypeA, Insecure, 0}, // NSEC3 opt-out
		{"nope.n3.test.", dns.TypeA, Insecure, 0},      // NXDOMAIN inside an opt-out span
		{"printer.lan.", dns.TypeA, Insecure, 0},       // built-in NTA
		{"www.test.", dns.TypeRRSIG, Insecure, 0},      // not an RRset to validate
		{"www.test.", dns.TypeANY, Insecure, 0},
		{"www.expired.test.", dns.TypeA, Bogus, dns.ExtendedErrorCodeSignatureExpired},
		{"www.wrongkey.test.", dns.TypeA, Bogus, dns.ExtendedErrorCodeDNSKEYMissing},
		{"www.dn.n3s.test.", dns.TypeA, Bogus, dns.ExtendedErrorCodeNSECMissing}, // NSEC3 NXDOMAIN under a DNAME
	}
	w := world(t)
	v := newV(w)
	for _, c := range cases {
		res, _ := check(t, v, w, c.name, c.qtype)
		if res.Security != c.want || res.EDE != c.ede {
			t.Errorf("%s %s = %+v, want %v ede %d", c.name, dns.TypeToString[c.qtype], res, c.want, c.ede)
		}
	}
}

func TestTampering(t *testing.T) {
	strip := func(t uint16) func(string, uint16, *dns.Msg) {
		return func(_ string, _ uint16, m *dns.Msg) {
			var out []dns.RR
			for _, rr := range m.Answer {
				if rr.Header().Rrtype != t {
					out = append(out, rr)
				}
			}
			m.Answer = out
		}
	}
	cases := []struct {
		what   string
		name   string
		qtype  uint16
		tamper func(string, uint16, *dns.Msg)
		ede    uint16
	}{
		{"stripped RRSIG", "www.test.", dns.TypeA, strip(dns.TypeRRSIG), dns.ExtendedErrorCodeRRSIGsMissing},
		{"forged address", "www.test.", dns.TypeA, forge, dns.ExtendedErrorCodeDNSBogus},
		{"forged Ed448 address", "www.ed.test.", dns.TypeA, forgeAny, dns.ExtendedErrorCodeDNSBogus},
		{"forged RSA address", "www.rsa.test.", dns.TypeA, forgeAny, dns.ExtendedErrorCodeDNSBogus},
		{"NSEC3 NXDOMAIN without closest encloser", "nope.n3s.test.", dns.TypeA, func(_ string, q uint16, m *dns.Msg) {
			if q == dns.TypeA {
				m.Ns = append(m.Ns[:2:2], m.Ns[4:]...) // SOA, then the covers only
			}
		}, dns.ExtendedErrorCodeNSECMissing},
		{"NSEC3 wildcard without proof", "x.w.n3s.test.", dns.TypeA, func(_ string, q uint16, m *dns.Msg) {
			if q == dns.TypeA {
				m.Ns = nil
			}
		}, dns.ExtendedErrorCodeNSECMissing},
		{"missing DS", "www.secure.test.", dns.TypeA, func(n string, q uint16, m *dns.Msg) {
			if q == dns.TypeDS && n == "secure.test." {
				m.Answer, m.Ns = nil, nil
			}
		}, dns.ExtendedErrorCodeNSECMissing},
		{"unsigned answer from a signed zone", "www.secure.test.", dns.TypeA, func(n string, q uint16, m *dns.Msg) {
			if q == dns.TypeA {
				strip(dns.TypeRRSIG)(n, q, m)
			}
		}, dns.ExtendedErrorCodeRRSIGsMissing},
		{"wildcard without proof", "x.wild.test.", dns.TypeA, func(_ string, q uint16, m *dns.Msg) {
			if q == dns.TypeA {
				m.Ns = nil
			}
		}, dns.ExtendedErrorCodeNSECMissing},
		{"NXDOMAIN without proof", "nope.test.", dns.TypeA, func(_ string, q uint16, m *dns.Msg) {
			if q == dns.TypeA {
				m.Ns = m.Ns[:2] // SOA and its RRSIG only
			}
		}, dns.ExtendedErrorCodeNSECMissing},
		{"NODATA turned NXDOMAIN", "www.test.", dns.TypeAAAA, func(_ string, q uint16, m *dns.Msg) {
			if q == dns.TypeAAAA {
				m.Rcode = dns.RcodeNameError
			}
		}, dns.ExtendedErrorCodeNSECMissing},
	}
	for _, c := range cases {
		w := world(t)
		v := newV(w)
		w.Tamper = c.tamper
		res, _ := check(t, v, w, c.name, c.qtype)
		if res.Security != Bogus || res.EDE != c.ede {
			t.Errorf("%s: %+v, want bogus ede %d", c.what, res, c.ede)
		}
	}
}

func TestClockSkew(t *testing.T) {
	w := world(t)
	past := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	v := newV(w)
	v.now = func() time.Time { return past }
	if res, _ := check(t, v, w, "www.test.", dns.TypeA); res.Security != Bogus || res.EDE != dns.ExtendedErrorCodeSignatureNotYetValid {
		t.Fatalf("no clock: %+v, want bogus not-yet-valid", res)
	}
	v = newV(w)
	v.now = func() time.Time { return past }
	v.clock = &Clock{now: v.now, floor: dnssectest.Now, unsynced: func() (bool, bool) { return false, false }, log: discard()}
	if res, _ := check(t, v, w, "www.test.", dns.TypeA); res.Security != Secure {
		t.Fatalf("clock before build: %+v, want secure", res)
	}
	// Only time is skipped: a forged answer is still bogus.
	w.Tamper = forge
	v.Flush()
	if res, _ := check(t, v, w, "www.test.", dns.TypeA); res.Security != Bogus {
		t.Fatalf("forged under skipped time checks: %+v", res)
	}
}

func TestClockStates(t *testing.T) {
	now := dnssectest.Now
	unsynced := true
	c := &Clock{now: func() time.Time { return now }, floor: now.Add(-time.Hour),
		unsynced: func() (bool, bool) { return unsynced, true }, log: discard()}
	if skip, why := c.Skip(); !skip || !strings.Contains(why, "NTP") {
		t.Fatalf("unsynced: %v %q", skip, why)
	}
	unsynced = false
	if skip, _ := c.Skip(); !skip {
		t.Fatal("rechecked inside the 10s window")
	}
	now = now.Add(11 * time.Second)
	if skip, _ := c.Skip(); skip {
		t.Fatal("still skipping after sync")
	}
	c.unsynced = func() (bool, bool) { return true, false } // cannot tell
	now = now.Add(11 * time.Second)
	if skip, _ := c.Skip(); skip {
		t.Fatal("unknown sync state must not skip")
	}
}

func TestCacheAvoidsRepeatLookups(t *testing.T) {
	w := world(t)
	v := newV(w)
	for range 3 {
		check(t, v, w, "www.secure.test.", dns.TypeA)
		check(t, v, w, "alias.test.", dns.TypeA)
	}
	for _, q := range []string{"./DNSKEY", "test./DNSKEY", "test./DS", "secure.test./DS", "secure.test./DNSKEY"} {
		if n := w.Queries[q]; n != 1 {
			t.Errorf("%s asked %d times, want 1", q, n)
		}
	}
}

func TestOperatorNTA(t *testing.T) {
	w := world(t)
	v := newV(w)
	v.SetNTAs([]string{"Expired.Test"})
	if res, _ := check(t, v, w, "www.expired.test.", dns.TypeA); res.Security != Insecure {
		t.Fatalf("NTA: %+v", res)
	}
}

func TestTrimKeepsOnlyTheChain(t *testing.T) {
	w := world(t)
	v := newV(w)
	w.Tamper = func(n string, q uint16, m *dns.Msg) {
		if n == "www.test." && q == dns.TypeA {
			planted, _ := dns.NewRR("bank.example. 300 IN A 198.51.100.1")
			m.Answer = append(m.Answer, planted)
			ns, _ := dns.NewRR("test. 300 IN NS evil.example.")
			m.Ns = append(m.Ns, ns)
		}
	}
	res, m := check(t, v, w, "www.test.", dns.TypeA)
	if res.Security != Secure || len(m.Answer) != 2 || len(m.Ns) != 0 {
		t.Fatalf("%+v answer=%v ns=%v", res, m.Answer, m.Ns)
	}
}

func TestSignatureBudget(t *testing.T) {
	w := world(t)
	v := newV(w)
	check(t, v, w, "www.test.", dns.TypeA) // warm the key cache
	w.Tamper = func(n string, q uint16, m *dns.Msg) {
		if n != "www.test." || q != dns.TypeA {
			return
		}
		sig := m.Answer[1].(*dns.RRSIG)
		for range 50 { // many signatures by the right key, all wrong
			bad := *sig
			bad.Signature = sig.Signature[:4] + "AAAA" + sig.Signature[8:]
			m.Answer = append(m.Answer, &bad)
		}
		m.Answer = append(m.Answer[:1], m.Answer[2:]...)
	}
	m, _ := w.Query(context.Background(), "www.test.", dns.TypeA)
	r := &run{v: v, ctx: context.Background(), budget: maxSigsPerCheck}
	sets, sigs := group(m.Answer)
	k := key{"www.test.", dns.TypeA}
	c, _ := r.cut(nil, ".")
	_ = c
	tz := &zone{name: "test.", keys: []*dns.DNSKEY{w.Zones["test."].Key}}
	if res, _ := r.verifySet(sets[k], sigs[k], tz); res.Security != Bogus {
		t.Fatalf("%+v", res)
	}
	if used := maxSigsPerCheck - r.budget; used != maxSigsPerSet {
		t.Fatalf("tried %d signatures, want %d", used, maxSigsPerSet)
	}
}

// TestSignedDataMatchesMiekg proves the Ed448 path signs the same bytes
// miekg/dns verifies: an Ed25519 signature over signedData verifies there.
func TestSignedDataMatchesMiekg(t *testing.T) {
	z := dnssectest.NewZone("example.", dns.ED25519)
	pub, priv, _ := ed25519.GenerateKey(nil)
	z.Key.PublicKey = encode(pub)
	set := []dns.RR{mustRR("*.example. 300 IN A 192.0.2.1"), mustRR("*.example. 300 IN A 192.0.2.2")}
	for _, rr := range set {
		rr.Header().Name = "Host.Example." // expanded wildcard, mixed case
	}
	sig := &dns.RRSIG{Hdr: dns.RR_Header{Name: "host.example.", Rrtype: dns.TypeRRSIG, Class: dns.ClassINET, Ttl: 300},
		TypeCovered: dns.TypeA, Algorithm: dns.ED25519, Labels: 1, OrigTtl: 300,
		Inception: 1, Expiration: 4000000000, KeyTag: z.Key.KeyTag(), SignerName: "example."}
	data, err := signedData(sig, set)
	if err != nil {
		t.Fatal(err)
	}
	data[2] = dns.ED25519
	sig.Signature = encode(ed25519.Sign(priv, data))
	if err := sig.Verify(z.Key, set); err != nil {
		t.Fatalf("miekg rejects the rebuilt signed data: %v", err)
	}
}

func TestAnchorsMatchIANA(t *testing.T) {
	b, err := os.ReadFile("testdata/root-anchors.xml")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Keys []struct {
			ValidUntil string `xml:"validUntil,attr"`
			KeyTag     uint16
			Algorithm  uint8
			DigestType uint8
			Digest     string
		} `xml:"KeyDigest"`
	}
	if err := xml.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, k := range doc.Keys {
		if k.ValidUntil == "" {
			want = append(want, (&dns.DS{Hdr: dns.RR_Header{Name: ".", Rrtype: dns.TypeDS, Class: dns.ClassINET},
				KeyTag: k.KeyTag, Algorithm: k.Algorithm, DigestType: k.DigestType, Digest: k.Digest}).String())
		}
	}
	if len(want) != len(rootAnchors) {
		t.Fatalf("%d current anchors in the XML, %d embedded", len(want), len(rootAnchors))
	}
	for i, a := range rootAnchors {
		if a.String() != want[i] {
			t.Errorf("anchor %d = %s, want %s", i, a, want[i])
		}
	}
}

func TestUnsupportedDigestIsInsecure(t *testing.T) {
	ds := []dns.RR{mustRR(". 0 IN DS 1 13 99 AABB"), mustRR(". 0 IN DS 1 5 2 AABB")}
	if got := usableDS(ds); len(got) != 0 {
		t.Fatalf("%v", got)
	}
	both := []dns.RR{mustRR(". 0 IN DS 1 13 1 AABB"), mustRR(". 0 IN DS 1 13 2 AABB")}
	if got := usableDS(both); len(got) != 1 || got[0].DigestType != dns.SHA256 {
		t.Fatalf("SHA-1 kept beside SHA-256: %v", got)
	}
}

// forge changes www.test's address after signing.
func forge(n string, q uint16, m *dns.Msg) {
	if n == "www.test." && q == dns.TypeA {
		m.Answer[0].(*dns.A).A[3] = 66
	}
}

// forgeAny changes the first A record of any A answer after signing.
func forgeAny(_ string, q uint16, m *dns.Msg) {
	if q == dns.TypeA && len(m.Answer) > 0 {
		if a, ok := m.Answer[0].(*dns.A); ok {
			a.A[3] = 66
		}
	}
}

// An existing closer encloser blocks the wildcard: a forged expansion of
// *.wild.test for a.b.wild.test must not pass on an NSEC that covers only the
// query name.
func TestWildcardBlockedByCloserEncloser(t *testing.T) {
	w := world(t)
	v := newV(w)
	w.Tamper = func(n string, q uint16, m *dns.Msg) {
		if n != "a.b.wild.test." || q != dns.TypeA {
			return
		}
		w.Tamper = nil
		syn, _ := w.Query(context.Background(), "x.wild.test.", dns.TypeA)
		for _, rr := range syn.Answer {
			rr.Header().Name = n
		}
		var proof []dns.RR // the NSEC covering a.b.wild.test, from b.wild.test
		for _, rr := range m.Ns {
			if rr.Header().Rrtype != dns.TypeSOA && !(rr.Header().Rrtype == dns.TypeRRSIG && rr.(*dns.RRSIG).TypeCovered == dns.TypeSOA) {
				proof = append(proof, rr)
			}
		}
		m.Rcode, m.Answer, m.Ns = dns.RcodeSuccess, syn.Answer, proof
	}
	res, _ := check(t, v, w, "a.b.wild.test.", dns.TypeA)
	if res.Security != Bogus {
		t.Fatalf("forged wildcard behind a closer encloser: %+v", res)
	}
}

// A verdict, and the delegations under it, expire with their signatures.
func TestVerdictExpiresWithSignatures(t *testing.T) {
	w := world(t)
	soon := dnssectest.Now.Add(90 * time.Second)
	w.Zones["test."].Until = soon
	v := newV(w)
	res, _ := check(t, v, w, "www.test.", dns.TypeA)
	if res.Security != Secure || res.Expires.IsZero() || res.Expires.After(soon) {
		t.Fatalf("%+v, want secure expiring by %v", res, soon)
	}
	v.mu.Lock()
	c := v.cuts["secure.test."]
	v.mu.Unlock()
	check(t, v, w, "www.secure.test.", dns.TypeA)
	v.mu.Lock()
	c = v.cuts["secure.test."]
	v.mu.Unlock()
	if c.expires.After(soon) {
		t.Fatalf("delegation cached until %v, past its DS signature at %v", c.expires, soon)
	}
}

// A signed wildcard proof with NSEC3 iterations above the ceiling is insecure,
// decided before any of its hashes are computed.
func TestWildcardNSEC3IterationCeiling(t *testing.T) {
	w := world(t)
	v := newV(w)
	z := w.Zones["n3s.test."]
	w.Tamper = func(n string, q uint16, m *dns.Msg) {
		if n != "x.w.n3s.test." || q != dns.TypeA {
			return
		}
		var forged []dns.RR
		for _, rr := range m.Ns {
			if n3, ok := rr.(*dns.NSEC3); ok {
				n3.Iterations = 5000
				forged = append(forged, z.Signed([]dns.RR{n3})...)
			}
		}
		m.Ns = forged
	}
	res, _ := check(t, v, w, "x.w.n3s.test.", dns.TypeA)
	if res.Security != Insecure {
		t.Fatalf("%+v, want insecure", res)
	}
}

// Running out of one answer's signature budget must not cache the delegation
// it was checking as bogus for every later answer.
func TestExhaustedBudgetIsNotCached(t *testing.T) {
	w := world(t)
	v := newV(w)
	r := &run{v: v, ctx: context.Background(), budget: 0}
	if c, err := r.cut(nil, "."); err != nil || c.kind != cutBogus {
		t.Fatalf("starved root check = %+v, %v", c, err)
	}
	v.mu.Lock()
	_, cached := v.cuts["."]
	v.mu.Unlock()
	if cached {
		t.Fatal("a budget-starved verdict was cached")
	}
	if res, _ := check(t, v, w, "www.test.", dns.TypeA); res.Security != Secure {
		t.Fatalf("a fresh check after a starved one: %+v", res)
	}
}

// An empty non-terminal exists: a signed NODATA for it turned into NXDOMAIN
// is bogus, the genuine NODATA secure.
func TestNXDomainForEmptyNonTerminalIsBogus(t *testing.T) {
	w := world(t)
	v := newV(w)
	if res, _ := check(t, v, w, "b.ent.test.", dns.TypeA); res.Security != Secure {
		t.Fatalf("genuine NODATA: %+v", res)
	}
	w.Tamper = func(n string, q uint16, m *dns.Msg) {
		if n == "b.ent.test." && q == dns.TypeA {
			m.Rcode = dns.RcodeNameError
		}
	}
	if res, _ := check(t, v, w, "b.ent.test.", dns.TypeA); res.Security != Bogus {
		t.Fatalf("NXDOMAIN for an empty non-terminal: %+v", res)
	}
}

// A wildcard CNAME expanded at a real delegation must not be cached as
// "no delegation here", or the child's genuine answers fail for an hour.
func TestWildcardCNAMEDoesNotHideADelegation(t *testing.T) {
	w := world(t)
	v := newV(w)
	now := dnssectest.Now
	v.now = func() time.Time { return now }
	w.Tamper = func(n string, q uint16, m *dns.Msg) {
		if n != "real.wc.test." || q != dns.TypeDS {
			return
		}
		saved := w.Tamper
		w.Tamper = nil
		syn, _ := w.Query(context.Background(), "zzz.wc.test.", dns.TypeCNAME)
		w.Tamper = saved
		for _, rr := range syn.Answer {
			rr.Header().Name = n
		}
		m.Rcode, m.Answer, m.Ns = dns.RcodeSuccess, syn.Answer, nil
	}
	if res, _ := check(t, v, w, "www.real.wc.test.", dns.TypeA); res.Security == Secure {
		t.Fatalf("forged: %+v", res)
	}
	v.mu.Lock()
	c := v.cuts["real.wc.test."]
	v.mu.Unlock()
	if c.kind == cutNone {
		t.Fatal("a wildcard expansion was cached as proof of no delegation")
	}
	w.Tamper = nil
	now = now.Add(bogusCutTTL + time.Second)
	if res, _ := check(t, v, w, "www.real.wc.test.", dns.TypeA); res.Security != Secure {
		t.Fatalf("genuine answer after the failure expired: %+v", res)
	}
}

// The NSEC owned by *.wc.test, replayed under the delegation's own name, would
// read as "real.wc.test has no NS and no DS". Its signature still covers the
// wildcard owner, so it must not count as exact-owner evidence.
func TestWildcardOwnedNSECReplayedIsBogus(t *testing.T) {
	w := world(t)
	v := newV(w)
	w.Tamper = func(n string, q uint16, m *dns.Msg) {
		if n != "real.wc.test." || q != dns.TypeDS {
			return
		}
		// The zone's own NSEC at its wildcard, as the zone signs it.
		nsec := &dns.NSEC{Hdr: dns.RR_Header{Name: "*.wc.test.", Rrtype: dns.TypeNSEC, Class: dns.ClassINET, Ttl: 300},
			NextDomain: "www.test.", TypeBitMap: []uint16{dns.TypeCNAME, dns.TypeRRSIG, dns.TypeNSEC}}
		proof := w.Zones["test."].Signed([]dns.RR{nsec})
		for _, rr := range proof {
			rr.Header().Name = n
		}
		m.Rcode, m.Answer, m.Ns = dns.RcodeSuccess, nil, proof
	}
	if res, _ := check(t, v, w, "www.real.wc.test.", dns.TypeA); res.Security == Secure {
		t.Fatalf("forged: %+v", res)
	}
	v.mu.Lock()
	c := v.cuts["real.wc.test."]
	v.mu.Unlock()
	if c.kind == cutNone {
		t.Fatal("a replayed wildcard NSEC was cached as proof of no delegation")
	}
	// The same NSEC under its own literal wildcard owner is still accepted.
	r := &run{v: v, ctx: context.Background(), budget: maxSigsPerCheck}
	tz := &zone{name: "test.", keys: []*dns.DNSKEY{w.Zones["test."].Key}}
	lit := &dns.NSEC{Hdr: dns.RR_Header{Name: "*.wc.test.", Rrtype: dns.TypeNSEC, Class: dns.ClassINET, Ttl: 300},
		NextDomain: "www.test.", TypeBitMap: []uint16{dns.TypeCNAME, dns.TypeRRSIG, dns.TypeNSEC}}
	if got, _, res := r.proofs(tz, w.Zones["test."].Signed([]dns.RR{lit})); res.Security != Secure || len(got) != 1 {
		t.Fatalf("literal wildcard NSEC refused: %+v", res)
	}
}

// An empty non-terminal blocks the wildcard too: c.wild.test has a descendant,
// so *.wild.test cannot answer for x.c.wild.test, though an NSEC interval
// "covers" c.wild.test.
func TestWildcardBlockedByEmptyNonTerminal(t *testing.T) {
	w := world(t)
	v := newV(w)
	w.Tamper = func(n string, q uint16, m *dns.Msg) {
		if n != "x.c.wild.test." || q != dns.TypeA {
			return
		}
		saved := w.Tamper
		w.Tamper = nil
		syn, _ := w.Query(context.Background(), "x.wild.test.", dns.TypeA)
		ent, _ := w.Query(context.Background(), "c.wild.test.", dns.TypeA) // NODATA: the interval over c.wild.test
		w.Tamper = saved
		for _, rr := range syn.Answer {
			rr.Header().Name = n
		}
		var proof []dns.RR
		for _, rr := range ent.Ns {
			t := rr.Header().Rrtype
			if s, ok := rr.(*dns.RRSIG); ok {
				t = s.TypeCovered
			}
			if t == dns.TypeNSEC {
				proof = append(proof, rr)
			}
		}
		m.Rcode, m.Answer, m.Ns = dns.RcodeSuccess, syn.Answer, proof
	}
	if res, _ := check(t, v, w, "x.c.wild.test.", dns.TypeA); res.Security != Bogus {
		t.Fatalf("wildcard behind an empty non-terminal: %+v", res)
	}
}

// Canonical order is over label octets, not their escaped spelling.
func TestCanonicalOrderUsesOctets(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int // sign
	}{
		{`\255.test.`, "a.test.", 1}, // 0xff after 'a'; the text "\" sorts before it
		{`\090.test.`, "z.test.", 0}, // \090 is 'Z', folded
		{"Z.test.", "a.test.", 1},
		{`\000.test.`, "test.", 1},
		{"a.b.test.", "z.test.", -1}, // rightmost label first
		// Non-ASCII octets are never folded: "É" (0xC3 0x89) stays below "é"
		// (0xC3 0xA9); Unicode folding would make them equal.
		{`\195\137.test.`, `\195\169.test.`, -1},
		{`\255\254.test.`, `\255\254.test.`, 0}, // invalid UTF-8 compares by octet
		{`\200.test.`, "z.test.", 1},
	} {
		got := compare(c.a, c.b)
		if (got > 0) != (c.want > 0) || (got < 0) != (c.want < 0) {
			t.Errorf("compare(%s, %s) = %d, want sign %d", c.a, c.b, got, c.want)
		}
	}
	// "[" (0x5b) < "\255" (0xff) but 0xff > "a": the escaped text lies inside
	// [.test -> a.test, the octets do not.
	n := &dns.NSEC{Hdr: dns.RR_Header{Name: "[.test."}, NextDomain: "a.test."}
	if covers(n, `\255.test.`) {
		t.Fatal(`[.test -> a.test covers \255.test by its spelling`)
	}
	// Unicode folding would move "É" (\195\137) to "é" (\195\169), inside this
	// span; by octets it lies outside it.
	u := &dns.NSEC{Hdr: dns.RR_Header{Name: `\195\160.test.`}, NextDomain: `\195\176.test.`}
	if covers(u, `\195\137.test.`) {
		t.Fatal("a non-ASCII label was folded into the span")
	}
}

// Below a DNAME the zone's NSEC chain proves nothing: names there are
// redirected. A forged NXDOMAIN for www.dn.test is bogus; the genuine
// redirection (signed DNAME, synthesised CNAME, signed target) is secure.
func TestDNAME(t *testing.T) {
	w := world(t)
	v := newV(w)
	// The world has no DNAME synthesis, so www.dn.test comes back NXDOMAIN,
	// proven by dn.test's own NSEC: exactly the forged denial.
	if res, _ := check(t, v, w, "www.dn.test.", dns.TypeA); res.Security != Bogus {
		t.Fatalf("NXDOMAIN under a DNAME: %+v", res)
	}
	d, _ := w.Query(context.Background(), "dn.test.", dns.TypeDNAME)
	target, _ := w.Query(context.Background(), "www.secure.test.", dns.TypeA)
	cname, _ := dns.NewRR("www.dn.test. 300 IN CNAME www.secure.test.")
	m := new(dns.Msg)
	m.SetQuestion("www.dn.test.", dns.TypeA)
	m.Answer = append(append(append([]dns.RR{}, d.Answer...), cname), target.Answer...)
	res, err := v.Check(context.Background(), m.Question[0], m)
	if err != nil || res.Security != Secure {
		t.Fatalf("genuine DNAME answer: %+v %v", res, err)
	}
	// A synthesised CNAME that does not follow the DNAME is not kept.
	bad, _ := dns.NewRR("www.dn.test. 300 IN CNAME evil.example.")
	m.Answer = append(append([]dns.RR{}, d.Answer...), bad)
	if res, _ := v.Check(context.Background(), m.Question[0], m); res.Security == Secure {
		t.Fatalf("mismatched CNAME after a DNAME: %+v", res)
	}
}
