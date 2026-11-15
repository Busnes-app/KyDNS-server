// Package rdnss sends IPv6 router advertisements that carry only DNS
// information (RFC 8106 RDNSS and DNSSL) with a router lifetime of 0, so KyDNS
// tells hosts where DNS is without ever becoming their default router.
package rdnss

import (
	"encoding/binary"
	"errors"
	"net"
	"net/netip"

	"github.com/miekg/dns"
)

// ICMPv6 types and RA option types (RFC 4861 §4.1-4.2, RFC 8106 §5).
const (
	typeRS      = 133
	typeRA      = 134
	optSLLA     = 1
	optRDNSS    = 25
	optDNSSL    = 31
	maxOptBytes = 255 * 8 // an option's length field counts 8-octet units
)

// Advert is everything one RA carries. There is deliberately no field for
// router lifetime, flags or prefixes: those are constant zero.
type Advert struct {
	MAC      net.HardwareAddr // source link-layer address; omitted unless 6 bytes
	DNS      netip.Addr       // the RDNSS address
	Domain   string           // the DNSSL search domain; empty omits the option
	Lifetime uint32           // seconds, for both options; 0 withdraws them
}

var errDomain = errors.New("the search domain cannot be encoded in a DNSSL option")

// Marshal builds the ICMPv6 message, checksum zero: the kernel fills it in for
// raw ICMPv6 sockets (RFC 3542 §3.1).
func (a Advert) Marshal() ([]byte, error) {
	if !a.DNS.Is6() || a.DNS.Is4In6() {
		return nil, errors.New("the RDNSS address must be IPv6")
	}
	b := make([]byte, 16, 64)
	b[0] = typeRA
	// b[1] code, b[2:4] checksum, b[4] cur hop limit, b[5] M/O flags,
	// b[6:8] router lifetime, b[8:12] reachable time, b[12:16] retrans timer:
	// all zero.
	if len(a.MAC) == 6 {
		b = append(b, optSLLA, 1)
		b = append(b, a.MAC...)
	}
	// RDNSS: type, length (1 + 2 per address), reserved, lifetime, address.
	b = append(b, optRDNSS, 3, 0, 0)
	b = binary.BigEndian.AppendUint32(b, a.Lifetime)
	ip := a.DNS.As16()
	b = append(b, ip[:]...)
	if a.Domain != "" {
		opt, err := dnssl(a.Domain, a.Lifetime)
		if err != nil {
			return nil, err
		}
		b = append(b, opt...)
	}
	return b, nil
}

// dnssl encodes one domain uncompressed (RFC 8106 §5.2), zero-padded to a
// multiple of 8 octets.
func dnssl(domain string, lifetime uint32) ([]byte, error) {
	name := dns.Fqdn(domain)
	if name == "." {
		return nil, errDomain
	}
	wire := make([]byte, 255)
	n, err := dns.PackDomainName(name, wire, 0, nil, false)
	if err != nil {
		return nil, errDomain
	}
	body := 8 + n
	size := (body + 7) / 8 * 8
	if size > maxOptBytes {
		return nil, errDomain
	}
	opt := make([]byte, size)
	opt[0], opt[1] = optDNSSL, byte(size/8)
	binary.BigEndian.PutUint32(opt[4:8], lifetime)
	copy(opt[8:], wire[:n])
	return opt, nil
}

// ValidDomain reports whether domain fits a DNSSL option.
func ValidDomain(domain string) bool {
	_, err := dnssl(domain, 0)
	return err == nil
}

// isSolicitation checks what RFC 4861 §6.1.1 requires of an RS a router may
// act on. The hop limit is checked by the caller, from the control message.
// Options are not read: the reply is multicast and needs none of them.
func isSolicitation(b []byte) bool {
	return len(b) >= 8 && b[0] == typeRS && b[1] == 0
}
