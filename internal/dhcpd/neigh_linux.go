package dhcpd

import (
	"encoding/binary"
	"net"
	"net/netip"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// reachableNeighbours maps the IPv4 neighbours on iface that have answered
// recently to their MAC, read over netlink: REACHABLE or permanent entries,
// and STALE, DELAY or PROBE ones the kernel confirmed in the last
// recentlyConfirmed. /proc/net/arp is no use here: it marks every one of those
// complete, and a small LAN keeps a departed device's STALE entry
// indefinitely, so every recycled address looked taken. A device that is
// still around but silent longer than that is missed instead, the false
// negative the prober prefers.
func reachableNeighbours(iface string) (map[netip.Addr]string, error) {
	ifi, err := net.InterfaceByName(iface)
	if err != nil {
		return nil, err
	}
	rib, err := syscall.NetlinkRIB(syscall.RTM_GETNEIGH, syscall.AF_INET)
	if err != nil {
		return nil, err
	}
	msgs, err := syscall.ParseNetlinkMessage(rib)
	if err != nil {
		return nil, err
	}
	return parseNeighbours(msgs, ifi.Index), nil
}

// recentlyConfirmed is how long since its last confirmation a STALE, DELAY or
// PROBE neighbour still counts. A 100 ms nudge cannot make one REACHABLE (the
// kernel waits 5 s in DELAY), and the device a reservation was just taken
// from is usually in one of those states.
const recentlyConfirmed = 10 * time.Minute

// userHZ is the unit of nda_cacheinfo ages (clock_t), fixed at 100 on Linux.
const userHZ = 100

// parseNeighbours reads RTM_NEWNEIGH messages: a struct ndmsg (family, pad,
// ifindex, state, flags, type; 12 bytes) followed by route attributes.
func parseNeighbours(msgs []syscall.NetlinkMessage, index int) map[netip.Addr]string {
	const ndmsgLen = 12
	live := map[netip.Addr]string{}
	for _, m := range msgs {
		if m.Header.Type != syscall.RTM_NEWNEIGH || len(m.Data) < ndmsgLen {
			continue
		}
		ifindex := int(int32(binary.NativeEndian.Uint32(m.Data[4:8])))
		state := binary.NativeEndian.Uint16(m.Data[8:10])
		if ifindex != index {
			continue
		}
		var ip netip.Addr
		var mac string
		confirmed := time.Duration(-1) // age of the last confirmation, when known
		for b := m.Data[ndmsgLen:]; len(b) >= 4; {
			n, typ := int(binary.NativeEndian.Uint16(b[0:2])), binary.NativeEndian.Uint16(b[2:4])
			if n < 4 || n > len(b) {
				break
			}
			switch typ {
			case unix.NDA_DST:
				ip, _ = netip.AddrFromSlice(b[4:n])
			case unix.NDA_LLADDR:
				mac = normalizeMAC(net.HardwareAddr(b[4:n]).String())
			case unix.NDA_CACHEINFO:
				if n >= 8 {
					confirmed = time.Duration(binary.NativeEndian.Uint32(b[4:8])) * time.Second / userHZ
				}
			}
			b = b[min((n+3)&^3, len(b)):]
		}
		recent := state&(unix.NUD_STALE|unix.NUD_DELAY|unix.NUD_PROBE) != 0 && confirmed >= 0 && confirmed < recentlyConfirmed
		if state&(unix.NUD_REACHABLE|unix.NUD_PERMANENT) == 0 && !recent {
			continue
		}
		if ip.IsValid() && mac != "" {
			live[ip.Unmap()] = mac
		}
	}
	return live
}
