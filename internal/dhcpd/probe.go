package dhcpd

import (
	"net"
	"net/netip"
	"time"
)

// Prober answers one question: is this address already in use by something
// other than the client mac? A client still on its own expired address is
// not a conflict. A false negative costs a duplicate address, which the
// client then declines; a false positive costs one address for ten minutes.
// The budget is small because it sits in the path of every new OFFER.
type Prober interface {
	InUse(ip netip.Addr, mac string) bool
}

// nopProber never reports a conflict: for hosts without the probe, and for
// tests.
type nopProber struct{}

func (nopProber) InUse(netip.Addr, string) bool { return false }

type prober struct {
	iface  string
	budget time.Duration
}

// probePort is the destination of the throwaway UDP datagram InUse sends.
// Its value is arbitrary: nothing needs to be listening there and any reply
// is ignored. What matters is the side effect — before the kernel can send,
// it must resolve L2 for the destination, which is what leaves an ARP entry
// for InUse to read back.
const probePort = 33434

// NewProber returns a Prober that nudges the kernel into ARP-resolving a
// candidate address and reads the result back from /proc/net/arp. This is
// deliberate: an ICMP echo would need CAP_NET_RAW, which this service does
// not have and will not be granted, so there is one socket mechanism here,
// not two.
func NewProber(iface string, budget time.Duration) Prober {
	if budget <= 0 {
		budget = 100 * time.Millisecond
	}
	return &prober{iface: iface, budget: budget}
}

// InUse is a best-effort check on a fixed budget, not proof. A send failure
// or an unreadable neighbour table returns false rather than an error: a false
// negative here costs one address that the client declines (there is a
// DHCPDECLINE path for exactly that); blocking an OFFER because this probe
// misfired would be worse.
func (p *prober) InUse(ip netip.Addr, mac string) bool {
	deadline := time.Now().Add(p.budget)
	nudgeARP(ip)
	if wait := time.Until(deadline); wait > 0 {
		time.Sleep(wait)
	}
	live, err := reachableNeighbours(p.iface)
	if err != nil {
		return false
	}
	hw, ok := live[ip]
	return ok && hw != mac
}

// nudgeARP sends one throwaway UDP datagram to ip. Its only purpose is the
// side effect: it makes the kernel resolve L2 for ip, which leaves an ARP
// entry behind whether or not anything is listening on the UDP port. Any
// error is silent — the caller has nothing useful to do with it, and the
// probe degrades to whatever the ARP table already held.
func nudgeARP(ip netip.Addr) {
	c, err := net.Dial("udp4", netip.AddrPortFrom(ip, probePort).String())
	if err != nil {
		return
	}
	defer c.Close()
	c.Write([]byte{0})
}
