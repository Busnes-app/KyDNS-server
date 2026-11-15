package dnssec

import (
	"bytes"
	"strings"
	"time"

	"github.com/miekg/dns"
)

// notYetValid mirrors RRSIG.ValidityPeriod's RFC 1982 arithmetic for the
// inception alone, to tell EDE 8 from EDE 7.
func notYetValid(sig *dns.RRSIG, now time.Time) bool {
	const year68 = 1 << 31
	utc := now.UTC().Unix()
	modi := (int64(sig.Inception) - utc) / year68
	return int64(sig.Inception)+modi*year68 > utc
}

// compare orders names canonically (RFC 4034 §6.1).
func compare(a, b string) int {
	la, lb := dns.SplitDomainName(strings.ToLower(a)), dns.SplitDomainName(strings.ToLower(b))
	for i, j := len(la)-1, len(lb)-1; i >= 0 && j >= 0; i, j = i-1, j-1 {
		if c := bytes.Compare([]byte(la[i]), []byte(lb[j])); c != 0 {
			return c
		}
	}
	return len(la) - len(lb)
}

// covers reports whether name falls strictly inside n's span; the last NSEC
// of a zone wraps around to the apex.
func covers(n *dns.NSEC, name string) bool {
	owner, next := n.Hdr.Name, n.NextDomain
	if compare(owner, name) >= 0 {
		return false
	}
	return compare(name, next) < 0 || compare(next, owner) <= 0
}

func anyCovers(nsec []*dns.NSEC, name string) bool {
	for _, n := range nsec {
		if covers(n, name) {
			return true
		}
	}
	return false
}

// cover3 is NSEC3.Cover without its quirk of counting a hash match.
func cover3(n *dns.NSEC3, name string) bool { return n.Cover(name) && !n.Match(name) }

func anyCovers3(nsec3 []*dns.NSEC3, name string) bool {
	for _, n := range nsec3 {
		if cover3(n, name) {
			return true
		}
	}
	return false
}

// encloser is the closest encloser RFC 4035 §5.4 derives from a covering NSEC.
func encloser(n *dns.NSEC, name string) string {
	a, b := common(name, n.Hdr.Name), common(name, n.NextDomain)
	if dns.CountLabel(b) > dns.CountLabel(a) {
		return b
	}
	return a
}

func common(a, b string) string {
	n := dns.CompareDomainName(a, b)
	return lastLabels(dns.CanonicalName(a), n)
}

func wildcard(ce string) string {
	if ce == "." {
		return "*."
	}
	return "*." + ce
}

// nsecNXDomain needs the name and the wildcard at its closest encloser
// covered.
func nsecNXDomain(nsec []*dns.NSEC, name string) bool {
	for _, n := range nsec {
		if covers(n, name) {
			return anyCovers(nsec, wildcard(encloser(n, name)))
		}
	}
	return false
}

// nsecNoData accepts a matching NSEC without the type, an empty
// non-terminal, or a matching wildcard without the type.
func nsecNoData(nsec []*dns.NSEC, name string, qtype uint16) bool {
	for _, n := range nsec {
		if dns.CanonicalName(n.Hdr.Name) == name {
			return noType(n.TypeBitMap, qtype)
		}
	}
	for _, n := range nsec {
		if !covers(n, name) {
			continue
		}
		if dns.IsSubDomain(name, dns.CanonicalName(n.NextDomain)) {
			return true // empty non-terminal
		}
		wc := wildcard(encloser(n, name))
		for _, w := range nsec {
			if dns.CanonicalName(w.Hdr.Name) == wc {
				return noType(w.TypeBitMap, qtype)
			}
		}
	}
	return false
}

// noType reads a matching bitmap. The parent's NSEC at a delegation proves
// nothing about the child's data, only the absence of a DS (RFC 6840 §4.1).
func noType(types []uint16, qtype uint16) bool {
	if has(types, qtype) || has(types, dns.TypeCNAME) {
		return false
	}
	if qtype != dns.TypeDS && has(types, dns.TypeNS) && !has(types, dns.TypeSOA) {
		return false
	}
	return true
}

// nsec3Usable: SHA-1 hashing at or below the RFC 9276 iteration ceiling.
func nsec3Usable(nsec3 []*dns.NSEC3) bool {
	for _, n := range nsec3 {
		if n.Hash != dns.SHA1 || n.Iterations > maxNSEC3Iter {
			return false
		}
	}
	return true
}

// closestEncloser runs the RFC 5155 §8.3 proof: an ancestor of name that
// exists, and a covered next-closer name. It returns that ancestor and the
// NSEC3 covering the next closer.
func closestEncloser(nsec3 []*dns.NSEC3, apex, name string) (string, *dns.NSEC3, bool) {
	labels := dns.SplitDomainName(name)
	for i := 1; i <= len(labels); i++ {
		ce := "."
		if i < len(labels) {
			ce = dns.CanonicalName(strings.Join(labels[i:], "."))
		}
		if !dns.IsSubDomain(apex, ce) {
			return "", nil, false
		}
		matched := false
		for _, n := range nsec3 {
			if n.Match(ce) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		nc := dns.CanonicalName(strings.Join(labels[i-1:], "."))
		for _, n := range nsec3 {
			if cover3(n, nc) {
				return ce, n, true
			}
		}
		return "", nil, false
	}
	return "", nil, false
}

func optOut(n *dns.NSEC3) bool { return n.Flags&1 == 1 }

// nsec3Denial proves NXDOMAIN or NODATA with NSEC3 (RFC 5155 §8).
func nsec3Denial(nsec3 []*dns.NSEC3, apex, name string, qtype uint16, nx bool) Result {
	if !nsec3Usable(nsec3) {
		return insecure
	}
	if !nx {
		for _, n := range nsec3 {
			if n.Match(name) {
				return provedBy(noType(n.TypeBitMap, qtype))
			}
		}
	}
	ce, cover, ok := closestEncloser(nsec3, apex, name)
	if !ok {
		return bogus(dns.ExtendedErrorCodeNSECMissing)
	}
	wc := wildcard(ce)
	if nx {
		if !anyCovers3(nsec3, wc) {
			return bogus(dns.ExtendedErrorCodeNSECMissing)
		}
		if optOut(cover) {
			return insecure // an unsigned delegation may hide the name
		}
		return secure
	}
	if qtype == dns.TypeDS && optOut(cover) {
		return insecure
	}
	for _, n := range nsec3 {
		if n.Match(wc) {
			return provedBy(noType(n.TypeBitMap, qtype))
		}
	}
	return bogus(dns.ExtendedErrorCodeNSECMissing)
}
