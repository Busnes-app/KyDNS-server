// Package dnssectest builds a signed DNS hierarchy in memory and answers
// questions about it like a CD=1 recursive resolver would. Tests only.
package dnssectest

import (
	"context"
	"crypto"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/cloudflare/circl/sign/ed448"
	"github.com/miekg/dns"
)

// Now is the instant every signature is valid around.
var Now = time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)

// Zone is one zone. Unsigned zones carry no keys.
type Zone struct {
	Name    string
	Alg     uint8     // 0: unsigned
	NSEC3   bool      // NSEC3 instead of NSEC
	OptOut  bool      // NSEC3 opt-out: insecure delegations stay out of the chain
	Expired bool      // signatures expired a day before Now
	Until   time.Time // if set, every signature expires then
	Key     *dns.DNSKEY
	priv    crypto.Signer
	ed448   ed448.PrivateKey
	RRs     []dns.RR
}

// World is a set of zones under one root.
type World struct {
	Zones map[string]*Zone
	// Tamper, if set, edits every answer before it is returned.
	Tamper func(name string, qtype uint16, m *dns.Msg)
	// Queries counts upstream questions by "name/TYPE".
	Queries map[string]int
}

// NewZone makes a zone with a fresh key for alg (0 for unsigned).
func NewZone(name string, alg uint8) *Zone {
	z := &Zone{Name: dns.CanonicalName(name), Alg: alg}
	if alg == 0 {
		return z
	}
	z.Key = &dns.DNSKEY{Hdr: dns.RR_Header{Name: z.Name, Rrtype: dns.TypeDNSKEY, Class: dns.ClassINET, Ttl: 3600},
		Flags: 257, Protocol: 3, Algorithm: alg}
	if alg == dns.ED448 {
		pub, priv, err := ed448.GenerateKey(rand.Reader)
		if err != nil {
			panic(err)
		}
		z.Key.PublicKey = base64.StdEncoding.EncodeToString(pub)
		z.ed448 = priv
		return z
	}
	bits := map[uint8]int{dns.RSASHA256: 2048, dns.RSASHA1: 2048, dns.ECDSAP256SHA256: 256,
		dns.ECDSAP384SHA384: 384, dns.ED25519: 256}[alg]
	p, err := z.Key.Generate(bits)
	if err != nil {
		panic(err)
	}
	z.priv = p.(crypto.Signer)
	return z
}

// Add parses records into the zone.
func (z *Zone) Add(rrs ...string) *Zone {
	for _, s := range rrs {
		rr, err := dns.NewRR(s)
		if err != nil {
			panic(err)
		}
		z.RRs = append(z.RRs, rr)
	}
	return z
}

// DS is the zone's DS record (SHA-256).
func (z *Zone) DS() *dns.DS { return z.Key.ToDS(dns.SHA256) }

// New builds a world from zones. Parents get NS (and, for signed children, DS)
// records for each child, and every zone gets an SOA.
func New(zones ...*Zone) *World {
	w := &World{Zones: map[string]*Zone{}, Queries: map[string]int{}}
	for _, z := range zones {
		w.Zones[z.Name] = z
		z.Add(fmt.Sprintf("%s 300 IN SOA ns.%s host.%s 1 7200 3600 1209600 300", z.Name, strings.TrimPrefix(z.Name, "."), strings.TrimPrefix(z.Name, ".")))
		if z.Key != nil {
			z.RRs = append(z.RRs, z.Key)
		}
	}
	for _, z := range zones {
		if z.Name == "." {
			continue
		}
		p := w.zoneOf(parentName(z.Name), false)
		p.Add(z.Name + " 3600 IN NS ns." + z.Name)
		if z.Key != nil {
			p.RRs = append(p.RRs, z.DS())
		}
	}
	return w
}

// Anchors is the root's DS, for the validator under test.
func (w *World) Anchors() []dns.RR { return []dns.RR{w.Zones["."].DS()} }

func parentName(n string) string {
	l := dns.SplitDomainName(n)
	if len(l) <= 1 {
		return "."
	}
	return dns.CanonicalName(strings.Join(l[1:], "."))
}

// zoneOf is the deepest zone holding name; a DS lives in the parent.
func (w *World) zoneOf(name string, ds bool) *Zone {
	n := dns.CanonicalName(name)
	if ds && n != "." {
		n = parentName(n)
	}
	for {
		if z, ok := w.Zones[n]; ok {
			return z
		}
		n = parentName(n)
	}
}

// Query answers like a resolver with CD=1.
func (w *World) Query(_ context.Context, name string, qtype uint16) (*dns.Msg, error) {
	w.Queries[strings.ToLower(name)+"/"+dns.TypeToString[qtype]]++
	m := new(dns.Msg)
	m.SetQuestion(dns.CanonicalName(name), qtype)
	m.Response = true
	w.answer(m, dns.CanonicalName(name), qtype, 0)
	m = m.Copy() // a tamper must not edit the zone itself
	if w.Tamper != nil {
		w.Tamper(dns.CanonicalName(name), qtype, m)
	}
	return m, nil
}

func (w *World) answer(m *dns.Msg, name string, qtype uint16, depth int) {
	z := w.zoneOf(name, qtype == dns.TypeDS)
	if set := z.set(name, qtype); set != nil {
		m.Answer = append(m.Answer, z.signed(set)...)
		return
	}
	if c := z.set(name, dns.TypeCNAME); c != nil && qtype != dns.TypeCNAME {
		m.Answer = append(m.Answer, z.signed(c)...)
		if depth < 8 {
			w.answer(m, dns.CanonicalName(c[0].(*dns.CNAME).Target), qtype, depth+1)
		}
		return
	}
	m.Ns = append(m.Ns, z.signed(z.set(z.Name, dns.TypeSOA))...)
	if z.exists(name) || z.ent(name) {
		m.Ns = append(m.Ns, z.noData(name)...)
		return
	}
	ce := z.encloser(name)
	if wc := z.set("*."+ce, qtype); wc != nil {
		m.Ns = nil
		var set []dns.RR
		for _, rr := range wc {
			c := dns.Copy(rr)
			c.Header().Name = name
			set = append(set, c)
		}
		m.Answer = append(m.Answer, set...)
		s := z.sig(wc)
		s.Hdr.Name = name
		m.Answer = append(m.Answer, s)
		m.Ns = append(m.Ns, z.proveAbsent(name, ce)...)
		return
	}
	m.Rcode = dns.RcodeNameError
	m.Ns = append(m.Ns, z.proveAbsent(name, ce)...)
	if z.NSEC3 {
		m.Ns = append(m.Ns, z.n3cover(wildcardOf(ce))...)
	} else {
		m.Ns = append(m.Ns, z.nsecCover(wildcardOf(ce))...)
	}
}

func wildcardOf(ce string) string {
	if ce == "." {
		return "*."
	}
	return "*." + ce
}

func (z *Zone) set(name string, t uint16) []dns.RR {
	var out []dns.RR
	for _, rr := range z.RRs {
		h := rr.Header()
		if dns.CanonicalName(h.Name) == name && h.Rrtype == t {
			out = append(out, rr)
		}
	}
	// NS at a delegation is not the parent's authoritative data, but a
	// resolver returns it; keep it simple and serve it.
	return out
}

func (z *Zone) exists(name string) bool {
	for _, rr := range z.RRs {
		if dns.CanonicalName(rr.Header().Name) == name {
			return true
		}
	}
	return false
}

func (z *Zone) ent(name string) bool {
	for _, rr := range z.RRs {
		n := dns.CanonicalName(rr.Header().Name)
		if n != name && dns.IsSubDomain(name, n) {
			return true
		}
	}
	return false
}

// encloser is the closest existing ancestor of name in z.
func (z *Zone) encloser(name string) string {
	for n := parentName(name); ; n = parentName(n) {
		if n == z.Name || z.exists(n) || z.ent(n) {
			return n
		}
	}
}

// owners are the names with data, canonically sorted.
func (z *Zone) owners() []string {
	seen := map[string]bool{}
	var out []string
	for _, rr := range z.RRs {
		n := dns.CanonicalName(rr.Header().Name)
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool { return less(out[i], out[j]) })
	return out
}

func less(a, b string) bool {
	la, lb := dns.SplitDomainName(a), dns.SplitDomainName(b)
	for i, j := len(la)-1, len(lb)-1; i >= 0 && j >= 0; i, j = i-1, j-1 {
		if la[i] != lb[j] {
			return la[i] < lb[j]
		}
	}
	return len(la) < len(lb)
}

func (z *Zone) types(name string) []uint16 {
	seen := map[uint16]bool{}
	var out []uint16
	for _, rr := range z.RRs {
		if dns.CanonicalName(rr.Header().Name) == name && !seen[rr.Header().Rrtype] {
			seen[rr.Header().Rrtype] = true
			out = append(out, rr.Header().Rrtype)
		}
	}
	if z.Alg != 0 {
		out = append(out, dns.TypeRRSIG)
		if z.NSEC3 {
			// the NSEC3 bitmap omits NSEC3 itself
		} else {
			out = append(out, dns.TypeNSEC)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func (z *Zone) nsecFor(i int, owners []string) *dns.NSEC {
	next := owners[(i+1)%len(owners)]
	return &dns.NSEC{Hdr: dns.RR_Header{Name: owners[i], Rrtype: dns.TypeNSEC, Class: dns.ClassINET, Ttl: 300},
		NextDomain: next, TypeBitMap: z.types(owners[i])}
}

func (z *Zone) nsecCover(name string) []dns.RR {
	if z.Alg == 0 {
		return nil
	}
	owners := z.owners()
	for i := range owners {
		n := z.nsecFor(i, owners)
		if owners[i] == name || (less(owners[i], name) && (less(name, n.NextDomain) || !less(owners[i], n.NextDomain))) {
			return z.signed([]dns.RR{n})
		}
	}
	return nil
}

func (z *Zone) noData(name string) []dns.RR {
	if z.NSEC3 {
		if rr := z.n3match(name); rr != nil {
			return rr
		}
		return z.proveAbsent(name, z.encloser(name)) // opt-out span
	}
	return z.nsecCover(name)
}

func (z *Zone) proveAbsent(name, ce string) []dns.RR {
	if !z.NSEC3 {
		return z.nsecCover(name)
	}
	nc := lastLabels(name, dns.CountLabel(ce)+1)
	return append(z.n3match(ce), z.n3cover(nc)...)
}

func lastLabels(name string, n int) string {
	l := dns.SplitDomainName(name)
	if n >= len(l) {
		return name
	}
	return dns.CanonicalName(strings.Join(l[len(l)-n:], "."))
}

// n3chain is the zone's NSEC3 chain.
func (z *Zone) n3chain() []*dns.NSEC3 {
	names := map[string]bool{}
	for _, n := range z.owners() {
		if z.OptOut && n != z.Name && len(z.set(n, dns.TypeNS)) > 0 && len(z.set(n, dns.TypeDS)) == 0 {
			continue
		}
		for x := n; x != z.Name; x = parentName(x) { // empty non-terminals too
			names[x] = true
		}
		names[z.Name] = true
	}
	var out []*dns.NSEC3
	for n := range names {
		out = append(out, &dns.NSEC3{Hdr: dns.RR_Header{Name: strings.ToLower(dns.HashName(n, dns.SHA1, 1, "AB")) + "." + z.Name,
			Rrtype: dns.TypeNSEC3, Class: dns.ClassINET, Ttl: 300},
			Hash: dns.SHA1, Flags: optOutFlag(z.OptOut), Iterations: 1, SaltLength: 1, Salt: "AB", TypeBitMap: z.types(n)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Hdr.Name < out[j].Hdr.Name })
	for i, n := range out {
		next := out[(i+1)%len(out)]
		n.NextDomain = strings.ToUpper(strings.SplitN(next.Hdr.Name, ".", 2)[0])
		n.HashLength = 20
	}
	return out
}

func (z *Zone) n3match(name string) []dns.RR {
	for _, n := range z.n3chain() {
		if n.Match(name) {
			return z.signed([]dns.RR{n})
		}
	}
	return nil
}

func (z *Zone) n3cover(name string) []dns.RR {
	for _, n := range z.n3chain() {
		if n.Cover(name) && !n.Match(name) {
			return z.signed([]dns.RR{n})
		}
	}
	return nil
}

// Signed is signed for tests that forge records the zone itself signs.
func (z *Zone) Signed(set []dns.RR) []dns.RR { return z.signed(set) }

// signed returns set followed by its RRSIG (none in an unsigned zone).
func (z *Zone) signed(set []dns.RR) []dns.RR {
	if len(set) == 0 {
		return nil
	}
	if z.Alg == 0 {
		return set
	}
	return append(append([]dns.RR(nil), set...), z.sig(set))
}

func (z *Zone) sig(set []dns.RR) *dns.RRSIG {
	inc, exp := Now.Add(-365*24*time.Hour), Now.Add(5*365*24*time.Hour)
	if z.Expired {
		inc, exp = Now.Add(-30*24*time.Hour), Now.Add(-24*time.Hour)
	}
	if !z.Until.IsZero() {
		exp = z.Until
	}
	s := &dns.RRSIG{Hdr: dns.RR_Header{Ttl: set[0].Header().Ttl}, Algorithm: z.Alg, SignerName: z.Name,
		KeyTag: z.Key.KeyTag(), Inception: uint32(inc.Unix()), Expiration: uint32(exp.Unix()), OrigTtl: set[0].Header().Ttl}
	if z.Alg == dns.ED448 {
		c := &capture{}
		alg := s.Algorithm
		s.Algorithm = dns.ED25519
		if err := s.Sign(c, set); err != nil {
			panic(err)
		}
		s.Algorithm = alg
		c.data[2] = dns.ED448
		s.Signature = base64.StdEncoding.EncodeToString(ed448.Sign(z.ed448, c.data, ""))
		return s
	}
	if err := s.Sign(z.priv, set); err != nil {
		panic(err)
	}
	return s
}

type capture struct{ data []byte }

func (c *capture) Public() crypto.PublicKey { return nil }
func (c *capture) Sign(_ io.Reader, d []byte, _ crypto.SignerOpts) ([]byte, error) {
	c.data = append([]byte(nil), d...)
	return make([]byte, 64), nil
}

func optOutFlag(on bool) uint8 {
	if on {
		return 1
	}
	return 0
}
