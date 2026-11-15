package dnssec

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/miekg/dns"
	"golang.org/x/sync/singleflight"
)

// Security is a validation verdict.
type Security uint8

const (
	Insecure Security = iota
	Secure
	Bogus
)

// Result is a verdict and, for Bogus, the RFC 8914 Extended DNS Error.
type Result struct {
	Security Security
	EDE      uint16
	// Expires is the earliest signature expiry (or cached delegation expiry)
	// the verdict rests on; a cache must not keep it longer. Zero is none.
	Expires time.Time
}

var (
	secure   = Result{Security: Secure}
	insecure = Result{Security: Insecure}
)

func bogus(ede uint16) Result { return Result{Security: Bogus, EDE: ede} }

// merge keeps the weakest verdict: Bogus, then Insecure, then Secure.
func merge(a, b Result) Result {
	if a.Security == Bogus {
		return a
	}
	if b.Security == Bogus || b.Security == Insecure {
		return b
	}
	return a
}

// Querier asks the upstream one question with DO=1 and CD=1.
type Querier func(ctx context.Context, name string, qtype uint16) (*dns.Msg, error)

// Limits against KeyTrap-style CPU exhaustion (CVE-2023-50387/50868), and the
// RFC 9276 NSEC3 iteration ceiling above which a proof counts as insecure.
const (
	maxSigsPerSet   = 8
	maxSigsPerCheck = 64
	maxNSEC3Iter    = 150
	maxChain        = 12
	maxCuts         = 4096
	minCutTTL       = 60 * time.Second
	maxCutTTL       = time.Hour
	bogusCutTTL     = 60 * time.Second
)

// builtinNTAs are private-use names a LAN router answers itself, unsigned,
// while the root proves they do not exist. RFC 6303 reverse zones too.
var builtinNTAs = func() []string {
	n := []string{"home.arpa.", "internal.", "local.", "lan.", "localdomain.",
		"10.in-addr.arpa.", "168.192.in-addr.arpa.", "254.169.in-addr.arpa.",
		"d.f.ip6.arpa.", "8.e.f.ip6.arpa.", "9.e.f.ip6.arpa.", "a.e.f.ip6.arpa.", "b.e.f.ip6.arpa."}
	for i := 16; i <= 31; i++ {
		n = append(n, strconv.Itoa(i)+".172.in-addr.arpa.")
	}
	return n
}()

// Validator checks forwarded answers from the root anchors down. It is safe
// for concurrent use.
type Validator struct {
	query   Querier
	clock   *Clock
	anchors []dns.RR
	now     func() time.Time
	ntas    atomic.Pointer[[]string]

	group singleflight.Group
	mu    sync.Mutex
	cuts  map[string]cut
}

// New builds a validator. A nil clock never skips time checks.
func New(q Querier, clock *Clock) *Validator {
	v := &Validator{query: q, clock: clock, now: time.Now, cuts: map[string]cut{}}
	for _, a := range rootAnchors {
		v.anchors = append(v.anchors, a)
	}
	v.SetNTAs(nil)
	return v
}

// UseAnchors replaces the root trust anchors. Tests only: a signed test
// hierarchy has its own root.
func (v *Validator) UseAnchors(ds []dns.RR) { v.anchors = ds }

// SetNTAs replaces the operator-derived negative trust anchors (the private
// domain, reverse zones); the built-in private-use names always apply.
func (v *Validator) SetNTAs(names []string) {
	all := append([]string(nil), builtinNTAs...)
	for _, n := range names {
		if n = strings.TrimSpace(n); n != "" && n != "." {
			all = append(all, dns.CanonicalName(n))
		}
	}
	v.ntas.Store(&all)
}

func (v *Validator) isNTA(name string) bool {
	for _, n := range *v.ntas.Load() {
		if dns.IsSubDomain(n, name) {
			return true
		}
	}
	return false
}

// Flush drops every cached key and delegation.
func (v *Validator) Flush() {
	v.mu.Lock()
	v.cuts = map[string]cut{}
	v.mu.Unlock()
}

// zone is a secure zone apex and its zone-signing-capable keys.
type zone struct {
	name string
	keys []*dns.DNSKEY
}

type cutKind uint8

const (
	cutNone     cutKind = iota // no delegation at this name
	cutStop                    // the name does not exist; nothing below can be a cut
	cutSecure                  // signed delegation, keys verified
	cutInsecure                // proven unsigned delegation, or unsupported algorithms
	cutBogus
)

type cut struct {
	kind    cutKind
	zone    *zone
	res     Result
	expires time.Time
}

// run carries one Check's context and signature budget.
type run struct {
	v       *Validator
	ctx     context.Context
	budget  int
	skip    bool      // signature time checks off (Clock)
	expires time.Time // earliest expiry the verdict rests on
	// exhausted: the per-answer budget ran out, so a bogus delegation found
	// meanwhile is not cached for other answers.
	exhausted bool
}

// note lowers the run's expiry to t.
func (r *run) note(t time.Time) {
	if !t.IsZero() && (r.expires.IsZero() || t.Before(r.expires)) {
		r.expires = t
	}
}

// Check validates m, the upstream's answer to q, and trims it to what was
// validated. A non-nil error is a lookup failure, not a verdict.
func (v *Validator) Check(ctx context.Context, q dns.Question, m *dns.Msg) (Result, error) {
	if m.Rcode != dns.RcodeSuccess && m.Rcode != dns.RcodeNameError {
		return insecure, nil
	}
	// RRSIG and ANY are not RRsets to validate; they pass through without AD.
	if q.Qtype == dns.TypeRRSIG || q.Qtype == dns.TypeANY {
		return insecure, nil
	}
	r := &run{v: v, ctx: ctx, budget: maxSigsPerCheck}
	if v.clock != nil {
		r.skip, _ = v.clock.Skip()
	}
	ans, sigs := group(m.Answer)
	name := dns.CanonicalName(q.Name)
	res := secure
	var keep []key
	for hop := 0; ; hop++ {
		if hop == maxChain {
			return bogus(dns.ExtendedErrorCodeDNSBogus), nil
		}
		if d, dk, ok := dnameFor(ans, name); ok {
			vr, err := r.checkSet(dk, d, sigs, m.Ns, false)
			if err != nil {
				return vr, err
			}
			if res = merge(res, vr); res.Security == Bogus {
				return res, nil
			}
			keep = append(keep, dk)
			target, ok := substitute(name, d[0].(*dns.DNAME))
			if !ok {
				return bogus(dns.ExtendedErrorCodeDNSBogus), nil
			}
			ck := key{name, dns.TypeCNAME}
			if c := ans[ck]; len(c) == 1 && dns.CanonicalName(c[0].(*dns.CNAME).Target) == target {
				keep = append(keep, ck) // synthesised; the DNAME is what was signed
			}
			name = target
			continue
		}
		k := key{name, q.Qtype}
		set := ans[k]
		if set == nil && q.Qtype != dns.TypeCNAME {
			k = key{name, dns.TypeCNAME}
			set = ans[k]
		}
		if set == nil {
			break
		}
		vr, err := r.checkSet(k, set, sigs, m.Ns, q.Qtype == dns.TypeDS && k.t == dns.TypeDS)
		if err != nil {
			return vr, err
		}
		if res = merge(res, vr); res.Security == Bogus {
			return res, nil
		}
		keep = append(keep, k)
		if k.t != dns.TypeCNAME || q.Qtype == dns.TypeCNAME {
			trim(m, keep, sigs, false)
			res.Expires = r.expires
			return res, nil
		}
		name = dns.CanonicalName(set[0].(*dns.CNAME).Target)
	}
	// No data at the end of the chain: the denial must be proven.
	vr, err := r.checkDenial(name, q.Qtype, m.Rcode, m.Ns)
	if err != nil {
		return vr, err
	}
	res = merge(res, vr)
	if res.Security != Bogus {
		trim(m, keep, sigs, true)
		res.Expires = r.expires
	}
	return res, nil
}

// checkSet validates one answer RRset. A DS set lives in the parent zone.
func (r *run) checkSet(k key, set []dns.RR, sigs map[key][]*dns.RRSIG, ns []dns.RR, inParent bool) (Result, error) {
	owner := k.name
	if inParent {
		owner = parent(owner)
	}
	z, res, err := r.walk(owner)
	if err != nil || z == nil {
		return res, err
	}
	res, sig := r.verifySet(set, sigs[k], z)
	if res.Security != Secure {
		return res, nil
	}
	// Fewer RRSIG labels than the owner has: a wildcard expansion, valid only
	// with proof that the owner itself does not exist (RFC 4035 §5.3.4).
	if labels := dns.CountLabel(k.name); int(sig.Labels) < labels {
		nsec, nsec3, pr := r.proofs(z, ns)
		if pr.Security != Secure {
			return pr, nil
		}
		// The next closer must not exist, or a closer encloser blocks the
		// wildcard (RFC 4035 §5.3.4, RFC 5155 §8.8).
		nc := lastLabels(k.name, int(sig.Labels)+1)
		if len(nsec3) > 0 && !nsec3Usable(nsec3) {
			return insecure, nil // checked before any hashing (RFC 9276)
		}
		if !anyCovers(nsec, nc) && !anyCovers3(nsec3, nc) {
			return bogus(dns.ExtendedErrorCodeNSECMissing), nil
		}
	}
	return secure, nil
}

// checkDenial proves NXDOMAIN or NODATA for name/qtype from the authority
// section.
func (r *run) checkDenial(name string, qtype uint16, rcode int, ns []dns.RR) (Result, error) {
	owner := name
	if qtype == dns.TypeDS {
		owner = parent(name)
	}
	z, res, err := r.walk(owner)
	if err != nil || z == nil {
		return res, err
	}
	nsec, nsec3, pr := r.proofs(z, ns)
	if pr.Security != Secure {
		return pr, nil
	}
	if len(nsec) > 0 {
		if rcode == dns.RcodeNameError {
			return provedBy(nsecNXDomain(nsec, name)), nil
		}
		return provedBy(nsecNoData(nsec, name, qtype)), nil
	}
	if len(nsec3) > 0 {
		return nsec3Denial(nsec3, z.name, name, qtype, rcode == dns.RcodeNameError), nil
	}
	return bogus(dns.ExtendedErrorCodeNSECMissing), nil
}

func provedBy(ok bool) Result {
	if ok {
		return secure
	}
	return bogus(dns.ExtendedErrorCodeNSECMissing)
}

// proofs verifies every NSEC, NSEC3 and SOA RRset in ns against z and returns
// the denial records. Records outside z are ignored.
func (r *run) proofs(z *zone, ns []dns.RR) ([]*dns.NSEC, []*dns.NSEC3, Result) {
	sets, sigs := group(ns)
	var nsec []*dns.NSEC
	var nsec3 []*dns.NSEC3
	for k, set := range sets {
		if k.t != dns.TypeNSEC && k.t != dns.TypeNSEC3 && k.t != dns.TypeSOA {
			continue
		}
		if !dns.IsSubDomain(z.name, k.name) {
			continue
		}
		if res, _ := r.verifySet(set, sigs[k], z); res.Security != Secure {
			return nil, nil, res
		}
		for _, rr := range set {
			switch x := rr.(type) {
			case *dns.NSEC:
				nsec = append(nsec, x)
			case *dns.NSEC3:
				nsec3 = append(nsec3, x)
			}
		}
	}
	return nsec, nsec3, secure
}

// verifySet returns Secure and the signature that verified, or why none did.
func (r *run) verifySet(set []dns.RR, sigs []*dns.RRSIG, z *zone) (Result, *dns.RRSIG) {
	if len(sigs) == 0 {
		return bogus(dns.ExtendedErrorCodeRRSIGsMissing), nil
	}
	res := bogus(dns.ExtendedErrorCodeDNSBogus)
	tries := 0
	now := r.v.now()
	for _, sig := range sigs {
		if !supportedAlg(sig.Algorithm) || dns.CanonicalName(sig.SignerName) != z.name {
			continue
		}
		if !r.skip && !sig.ValidityPeriod(now) {
			if notYetValid(sig, now) {
				res = bogus(dns.ExtendedErrorCodeSignatureNotYetValid)
			} else {
				res = bogus(dns.ExtendedErrorCodeSignatureExpired)
			}
			continue
		}
		for _, k := range z.keys {
			if k.KeyTag() != sig.KeyTag || k.Algorithm != sig.Algorithm {
				continue
			}
			if r.budget == 0 {
				r.exhausted = true // this answer's limit, not a verdict on the data
				return res, nil
			}
			if tries == maxSigsPerSet {
				return res, nil
			}
			tries++
			r.budget--
			if verifySig(sig, k, set) == nil {
				r.note(sigExpiry(sig, now))
				return secure, sig
			}
		}
	}
	return res, nil
}

// walk finds the zone holding name, from the root down. A nil zone with a
// nil error means insecure or bogus, as res says.
func (r *run) walk(name string) (*zone, Result, error) {
	if r.v.isNTA(name) {
		return nil, insecure, nil
	}
	c, err := r.cut(nil, ".")
	if err != nil {
		return nil, Result{}, err
	}
	if c.kind != cutSecure {
		return nil, c.res, nil
	}
	z := c.zone
	labels := dns.SplitDomainName(name)
	for i := len(labels) - 1; i >= 0; i-- {
		child := dns.CanonicalName(strings.Join(labels[i:], "."))
		c, err := r.cut(z, child)
		if err != nil {
			return nil, Result{}, err
		}
		switch c.kind {
		case cutStop:
			return z, secure, nil
		case cutSecure:
			z = c.zone
		case cutInsecure:
			return nil, insecure, nil
		case cutBogus:
			return nil, c.res, nil
		}
	}
	return z, secure, nil
}

// cut classifies child under the secure zone p (nil for the root), cached
// and collapsed across concurrent queries.
func (r *run) cut(p *zone, child string) (cut, error) {
	v := r.v
	v.mu.Lock()
	c, ok := v.cuts[child]
	v.mu.Unlock()
	if ok && v.now().Before(c.expires) {
		r.note(c.expires)
		return c, nil
	}
	got, err, _ := v.group.Do(child, func() (any, error) {
		outer, spent := r.expires, r.exhausted
		r.expires, r.exhausted = time.Time{}, false
		c, ttl, err := r.classify(p, child)
		sigs, starved := r.expires, r.exhausted
		r.expires, r.exhausted = outer, spent || starved
		if err != nil {
			return cut{}, err
		}
		if starved && c.kind == cutBogus {
			return c, nil // decided by this answer's budget: do not cache
		}
		if c.kind == cutBogus {
			ttl = bogusCutTTL
		}
		// The TTL floor never outlives a signature the cut rests on.
		c.expires = v.now().Add(min(max(ttl, minCutTTL), maxCutTTL))
		if !sigs.IsZero() && sigs.Before(c.expires) {
			c.expires = sigs
		}
		v.mu.Lock()
		if len(v.cuts) >= maxCuts {
			v.cuts = map[string]cut{}
		}
		v.cuts[child] = c
		v.mu.Unlock()
		return c, nil
	})
	if err != nil {
		return cut{}, err
	}
	r.note(got.(cut).expires)
	return got.(cut), nil
}

var errLookup = errors.New("dnssec: upstream lookup failed")

func (r *run) lookup(name string, qtype uint16) (*dns.Msg, error) {
	m, err := r.v.query(r.ctx, name, qtype)
	if err != nil {
		return nil, err
	}
	if m == nil || (m.Rcode != dns.RcodeSuccess && m.Rcode != dns.RcodeNameError) {
		return nil, errLookup
	}
	return m, nil
}

// classify asks for child's DS and decides what kind of cut it is.
func (r *run) classify(p *zone, child string) (cut, time.Duration, error) {
	if p == nil { // the root: the anchors stand in for its DS
		return r.keys(".", r.v.anchors)
	}
	m, err := r.lookup(child, dns.TypeDS)
	if err != nil {
		return cut{}, 0, err
	}
	ttl := minTTL(m)
	sets, sigs := group(m.Answer)
	if ds := sets[key{child, dns.TypeDS}]; ds != nil {
		if res, _ := r.verifySet(ds, sigs[key{child, dns.TypeDS}], p); res.Security != Secure {
			return cut{kind: cutBogus, res: res}, ttl, nil
		}
		c, kttl, err := r.keys(child, ds)
		return c, min(ttl, kttl), err
	}
	if c := sets[key{child, dns.TypeCNAME}]; c != nil {
		if res, _ := r.verifySet(c, sigs[key{child, dns.TypeCNAME}], p); res.Security != Secure {
			return cut{kind: cutBogus, res: res}, ttl, nil
		}
		return cut{kind: cutNone}, ttl, nil
	}
	nsec, nsec3, res := r.proofs(p, m.Ns)
	if res.Security != Secure {
		return cut{kind: cutBogus, res: res}, ttl, nil
	}
	for _, n := range nsec {
		if dns.CanonicalName(n.Hdr.Name) == child {
			return nsecCut(n.TypeBitMap), ttl, nil
		}
	}
	for _, n := range nsec {
		if covers(n, child) {
			if dns.IsSubDomain(child, dns.CanonicalName(n.NextDomain)) {
				return cut{kind: cutNone}, ttl, nil // empty non-terminal
			}
			return cut{kind: cutStop}, ttl, nil
		}
	}
	if len(nsec3) > 0 {
		if !nsec3Usable(nsec3) {
			return cut{kind: cutInsecure}, ttl, nil
		}
		for _, n := range nsec3 {
			if n.Match(child) {
				return nsecCut(n.TypeBitMap), ttl, nil
			}
		}
		if _, cover, ok := closestEncloser(nsec3, p.name, child); ok {
			if optOut(cover) { // opt-out: an unsigned delegation may be here
				return cut{kind: cutInsecure}, ttl, nil
			}
			return cut{kind: cutStop}, ttl, nil
		}
	}
	return cut{kind: cutBogus, res: bogus(dns.ExtendedErrorCodeNSECMissing)}, ttl, nil
}

// nsecCut reads a denial that matched the DS name exactly.
func nsecCut(types []uint16) cut {
	switch {
	case has(types, dns.TypeDS):
		return cut{kind: cutBogus, res: bogus(dns.ExtendedErrorCodeDNSBogus)}
	case has(types, dns.TypeNS) && !has(types, dns.TypeSOA):
		return cut{kind: cutInsecure}
	}
	return cut{kind: cutNone}
}

// keys fetches name's DNSKEY set and checks it against ds.
func (r *run) keys(name string, ds []dns.RR) (cut, time.Duration, error) {
	usable := usableDS(ds)
	if len(usable) == 0 {
		return cut{kind: cutInsecure}, maxCutTTL, nil // RFC 6840 §5.2
	}
	m, err := r.lookup(name, dns.TypeDNSKEY)
	if err != nil {
		return cut{}, 0, err
	}
	ttl := minTTL(m)
	sets, sigs := group(m.Answer)
	k := key{name, dns.TypeDNSKEY}
	set := sets[k]
	var all []*dns.DNSKEY
	for _, rr := range set {
		all = append(all, rr.(*dns.DNSKEY))
	}
	res := bogus(dns.ExtendedErrorCodeDNSKEYMissing)
	for _, d := range usable {
		for _, key := range all {
			if key.KeyTag() != d.KeyTag || key.Algorithm != d.Algorithm || key.Flags&dns.ZONE == 0 {
				continue
			}
			got := key.ToDS(d.DigestType)
			if got == nil || !strings.EqualFold(got.Digest, d.Digest) {
				continue
			}
			vr, _ := r.verifySet(set, sigs[k], &zone{name: name, keys: []*dns.DNSKEY{key}})
			if vr.Security == Secure {
				return cut{kind: cutSecure, zone: &zone{name: name, keys: all}}, ttl, nil
			}
			res = vr
		}
	}
	return cut{kind: cutBogus, res: res}, ttl, nil
}

// supportedAlg is RFC 8624's MUST/RECOMMENDED set minus RSA/SHA-1.
func supportedAlg(a uint8) bool {
	switch a {
	case dns.RSASHA256, dns.RSASHA512, dns.ECDSAP256SHA256, dns.ECDSAP384SHA384, dns.ED25519, dns.ED448:
		return true
	}
	return false
}

// usableDS keeps DS records this validator can use. SHA-1 digests count only
// for a key with no stronger digest (RFC 4509 §3).
func usableDS(rrs []dns.RR) []*dns.DS {
	strong := map[[2]uint16]bool{}
	for _, rr := range rrs {
		d := rr.(*dns.DS)
		if d.DigestType == dns.SHA256 || d.DigestType == dns.SHA384 {
			strong[[2]uint16{d.KeyTag, uint16(d.Algorithm)}] = true
		}
	}
	var out []*dns.DS
	for _, rr := range rrs {
		d := rr.(*dns.DS)
		if !supportedAlg(d.Algorithm) {
			continue
		}
		switch d.DigestType {
		case dns.SHA256, dns.SHA384:
		case dns.SHA1:
			if strong[[2]uint16{d.KeyTag, uint16(d.Algorithm)}] {
				continue
			}
		default:
			continue
		}
		out = append(out, d)
	}
	return out
}

// key names one RRset.
type key struct {
	name string
	t    uint16
}

// group splits a section into RRsets and the signatures over each.
func group(rrs []dns.RR) (map[key][]dns.RR, map[key][]*dns.RRSIG) {
	sets := map[key][]dns.RR{}
	sigs := map[key][]*dns.RRSIG{}
	for _, rr := range rrs {
		h := rr.Header()
		if h.Class != dns.ClassINET {
			continue
		}
		n := dns.CanonicalName(h.Name)
		if s, ok := rr.(*dns.RRSIG); ok {
			sigs[key{n, s.TypeCovered}] = append(sigs[key{n, s.TypeCovered}], s)
			continue
		}
		if h.Rrtype == dns.TypeOPT {
			continue
		}
		sets[key{n, h.Rrtype}] = append(sets[key{n, h.Rrtype}], rr)
	}
	return sets, sigs
}

// trim keeps only the validated answer RRsets, and in the authority section
// only the denial proof (and the SOA on a negative answer).
func trim(m *dns.Msg, keep []key, sigs map[key][]*dns.RRSIG, negative bool) {
	want := map[key]bool{}
	for _, k := range keep {
		want[k] = true
	}
	var ans []dns.RR
	for _, rr := range m.Answer {
		h := rr.Header()
		k := key{dns.CanonicalName(h.Name), h.Rrtype}
		if s, ok := rr.(*dns.RRSIG); ok {
			k.t = s.TypeCovered
		}
		if want[k] {
			ans = append(ans, rr)
		}
	}
	m.Answer = ans
	var ns []dns.RR
	for _, rr := range m.Ns {
		t := rr.Header().Rrtype
		if s, ok := rr.(*dns.RRSIG); ok {
			t = s.TypeCovered
		}
		if t == dns.TypeNSEC || t == dns.TypeNSEC3 || (negative && t == dns.TypeSOA) {
			ns = append(ns, rr)
		}
	}
	m.Ns = ns
}

// dnameFor finds a DNAME set at a proper ancestor of name.
func dnameFor(ans map[key][]dns.RR, name string) ([]dns.RR, key, bool) {
	labels := dns.SplitDomainName(name)
	for i := 1; i < len(labels); i++ {
		k := key{dns.CanonicalName(strings.Join(labels[i:], ".")), dns.TypeDNAME}
		if set := ans[k]; len(set) == 1 {
			return set, k, true
		}
	}
	return nil, key{}, false
}

func substitute(name string, d *dns.DNAME) (string, bool) {
	owner := dns.CanonicalName(d.Hdr.Name)
	prefix := strings.TrimSuffix(name, owner)
	target := dns.CanonicalName(prefix + dns.CanonicalName(d.Target))
	if _, ok := dns.IsDomainName(target); !ok || len(target) > 255 {
		return "", false
	}
	return target, true
}

func parent(name string) string {
	if name == "." {
		return "."
	}
	labels := dns.SplitDomainName(name)
	if len(labels) <= 1 {
		return "."
	}
	return dns.CanonicalName(strings.Join(labels[1:], "."))
}

// lastLabels is the rightmost n labels of name.
func lastLabels(name string, n int) string {
	labels := dns.SplitDomainName(name)
	if n >= len(labels) {
		return name
	}
	return dns.CanonicalName(strings.Join(labels[len(labels)-n:], "."))
}

func minTTL(m *dns.Msg) time.Duration {
	lowest := uint32(maxCutTTL / time.Second)
	for _, sec := range [][]dns.RR{m.Answer, m.Ns} {
		for _, rr := range sec {
			if t := rr.Header().Ttl; t < lowest {
				lowest = t
			}
		}
	}
	return time.Duration(lowest) * time.Second
}

func has(types []uint16, t uint16) bool {
	for _, x := range types {
		if x == t {
			return true
		}
	}
	return false
}
