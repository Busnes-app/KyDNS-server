package dhcpd

import (
	"encoding/binary"
	"net/netip"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

// neighMsg builds one RTM_NEWNEIGH message as the kernel sends it.
func neighMsg(index int32, state uint16, ip string, mac []byte) syscall.NetlinkMessage {
	b := make([]byte, 12)
	b[0] = syscall.AF_INET
	binary.NativeEndian.PutUint32(b[4:8], uint32(index))
	binary.NativeEndian.PutUint16(b[8:10], state)
	attr := func(typ uint16, v []byte) {
		a := make([]byte, (4+len(v)+3)&^3)
		binary.NativeEndian.PutUint16(a[0:2], uint16(4+len(v)))
		binary.NativeEndian.PutUint16(a[2:4], typ)
		copy(a[4:], v)
		b = append(b, a...)
	}
	attr(unix.NDA_DST, netip.MustParseAddr(ip).AsSlice())
	attr(unix.NDA_LLADDR, mac)
	return syscall.NetlinkMessage{Header: syscall.NlMsghdr{Type: syscall.RTM_NEWNEIGH}, Data: b}
}

// Only a neighbour that answered recently counts. A STALE entry is a device
// that may have left long ago: a small LAN keeps it indefinitely, and
// /proc/net/arp marked it complete, so every recycled address looked taken.
func TestParseNeighboursCountsOnlyReachable(t *testing.T) {
	mac := []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
	live := parseNeighbours([]syscall.NetlinkMessage{
		neighMsg(2, unix.NUD_REACHABLE, "192.168.1.1", mac),
		neighMsg(2, unix.NUD_STALE, "192.168.1.50", mac),
		neighMsg(2, unix.NUD_DELAY, "192.168.1.51", mac),
		neighMsg(2, unix.NUD_INCOMPLETE, "192.168.1.52", nil),
		neighMsg(3, unix.NUD_REACHABLE, "192.168.1.60", mac),
	}, 2)
	if got := live[netip.MustParseAddr("192.168.1.1")]; got != "aa:bb:cc:dd:ee:ff" {
		t.Fatalf("reachable neighbour answered by %q, want aa:bb:cc:dd:ee:ff", got)
	}
	if len(live) != 1 {
		t.Fatalf("neighbours = %v, want only the reachable one on this interface", live)
	}
}

// The real table reads without error on any Linux host.
func TestReachableNeighboursReadsTheKernelTable(t *testing.T) {
	if _, err := reachableNeighbours("lo"); err != nil {
		t.Fatal(err)
	}
}
