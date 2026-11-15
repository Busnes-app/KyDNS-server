package rdnss

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
)

// Kernel address flags (linux/if_addr.h). /proc/net/if_inet6 shows the low
// eight bits, which hold every one that makes an address unfit to advertise.
const (
	ifaTemporary  = 0x01 // RFC 8981 privacy address: rotates, so never advertised
	ifaDADFailed  = 0x08
	ifaDeprecated = 0x20
	ifaTentative  = 0x40
	unstable      = ifaTemporary | ifaDADFailed | ifaDeprecated | ifaTentative
)

// HostAddr is one IPv6 address on an interface.
type HostAddr struct {
	Addr   netip.Prefix
	Stable bool // none of the unstable flags is set
}

// ifInet6 reads the interface's addresses with their kernel flags. net's
// Interface.Addrs drops the flags, and the flags are the whole point.
func ifInet6(name string) ([]HostAddr, error) {
	b, err := os.ReadFile("/proc/net/if_inet6")
	if err != nil {
		return nil, fmt.Errorf("read IPv6 addresses: %w", err)
	}
	return parseIfInet6(string(b), name), nil
}

// parseIfInet6 is split out so it can be tested without a /proc.
func parseIfInet6(table, name string) []HostAddr {
	var out []HostAddr
	for _, line := range strings.Split(table, "\n") {
		f := strings.Fields(line)
		if len(f) != 6 || f[5] != name {
			continue
		}
		raw, err := hex.DecodeString(f[0])
		if err != nil || len(raw) != 16 {
			continue
		}
		bits, err1 := strconv.ParseUint(f[2], 16, 8)
		flags, err2 := strconv.ParseUint(f[4], 16, 8)
		if err1 != nil || err2 != nil || bits > 128 {
			continue
		}
		a := netip.AddrFrom16([16]byte(raw))
		out = append(out, HostAddr{Addr: netip.PrefixFrom(a, int(bits)), Stable: flags&unstable == 0})
	}
	return out
}

// errNoStable says what to do, since the operator reads it verbatim.
var errNoStable = errors.New("no stable IPv6 address to advertise: add a ULA (fd00::/8) " +
	"address to it, or enter the address KyDNS should advertise; temporary, deprecated " +
	"and link-local addresses are never used")

// Choose picks the address to advertise. A configured address must be on the
// interface and stable; otherwise a stable ULA wins over a stable global,
// because a ULA survives the ISP renumbering the global prefix.
func Choose(addrs []HostAddr, configured netip.Addr) (netip.Prefix, error) {
	if configured.IsValid() {
		for _, h := range addrs {
			if h.Addr.Addr() == configured {
				if !h.Stable {
					return netip.Prefix{}, fmt.Errorf("%s is a temporary, deprecated or tentative address, so it cannot be advertised", configured)
				}
				return h.Addr, nil
			}
		}
		return netip.Prefix{}, fmt.Errorf("%s is not an address of this interface", configured)
	}
	var global netip.Prefix
	for _, h := range addrs {
		a := h.Addr.Addr()
		if !h.Stable || !a.IsGlobalUnicast() || a.Is4In6() {
			continue // IsGlobalUnicast is false for link-local, loopback, multicast
		}
		if a.IsPrivate() {
			return h.Addr, nil
		}
		if !global.IsValid() {
			global = h.Addr
		}
	}
	if global.IsValid() {
		return global, nil
	}
	return netip.Prefix{}, errNoStable
}

// linkLocal is the interface's link-local address, the RA source RFC 4861
// §4.2 requires.
func linkLocal(addrs []HostAddr) (netip.Addr, bool) {
	for _, h := range addrs {
		if a := h.Addr.Addr(); a.IsLinkLocalUnicast() && h.Stable {
			return a, true
		}
	}
	return netip.Addr{}, false
}

// Pick reads iface and chooses the address to advertise.
func Pick(iface string, configured netip.Addr) (netip.Prefix, error) {
	addrs, err := ifInet6(iface)
	if err != nil {
		return netip.Prefix{}, err
	}
	p, err := Choose(addrs, configured)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("interface %q: %w", iface, err)
	}
	return p, nil
}
