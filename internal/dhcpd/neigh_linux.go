package dhcpd

import (
	"encoding/binary"
	"net"
	"net/netip"
	"syscall"

	"golang.org/x/sys/unix"
)

// reachableNeighbours maps the IPv4 neighbours on iface that the kernel holds
// as REACHABLE or permanent to their MAC, read over netlink. /proc/net/arp is
// no use here: it marks STALE, DELAY and PROBE entries complete too, and a
// small LAN keeps a departed device's STALE entry indefinitely, so every
// recycled address would look taken. A live device behind a STALE entry is
// missed instead, which is the false negative the prober prefers.
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
		if ifindex != index || state&(unix.NUD_REACHABLE|unix.NUD_PERMANENT) == 0 {
			continue
		}
		var ip netip.Addr
		var mac string
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
			}
			b = b[min((n+3)&^3, len(b)):]
		}
		if ip.IsValid() && mac != "" {
			live[ip.Unmap()] = mac
		}
	}
	return live
}
