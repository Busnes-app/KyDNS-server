package dhcpd

import (
	"encoding/binary"
	"net/netip"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

// neighMsg builds one RTM_NEWNEIGH message as the kernel sends it.
func neighMsg(index int32, state uint16, ip string, mac []byte, confirmedTicks ...uint32) syscall.NetlinkMessage {
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
	for _, c := range confirmedTicks {
		ci := make([]byte, 16) // nda_cacheinfo: confirmed, used, updated, refcnt
		binary.NativeEndian.PutUint32(ci[0:4], c)
		attr(unix.NDA_CACHEINFO, ci)
	}
	return syscall.NetlinkMessage{Header: syscall.NlMsghdr{Type: syscall.RTM_NEWNEIGH}, Data: b}
}

// Only a neighbour that answered recently counts. A STALE entry confirmed
// long ago is a device that may have left days ago: a small LAN keeps it
// indefinitely, and /proc/net/arp marked it complete, so every recycled
// address looked taken. One confirmed in the last ten minutes, such as the
// device a reservation was just taken from, still counts.
func TestParseNeighboursCountsOnlyRecentAnswers(t *testing.T) {
	mac := []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
	live := parseNeighbours([]syscall.NetlinkMessage{
		neighMsg(2, unix.NUD_REACHABLE, "192.168.1.1", mac),
		neighMsg(2, unix.NUD_STALE, "192.168.1.50", mac, 2*24*3600*userHZ),
		neighMsg(2, unix.NUD_STALE, "192.168.1.51", mac),
		neighMsg(2, unix.NUD_DELAY, "192.168.1.53", mac, 90*userHZ),
		neighMsg(2, unix.NUD_STALE, "192.168.1.54", mac, 120*userHZ),
		neighMsg(2, unix.NUD_INCOMPLETE, "192.168.1.52", nil),
		neighMsg(3, unix.NUD_REACHABLE, "192.168.1.60", mac),
	}, 2)
	for _, ip := range []string{"192.168.1.1", "192.168.1.53", "192.168.1.54"} {
		if got := live[netip.MustParseAddr(ip)]; got != "aa:bb:cc:dd:ee:ff" {
			t.Errorf("%s answered by %q, want aa:bb:cc:dd:ee:ff", ip, got)
		}
	}
	if len(live) != 3 {
		t.Fatalf("neighbours = %v, want only the recent ones on this interface", live)
	}
}

// The real table reads without error on any Linux host.
func TestReachableNeighboursReadsTheKernelTable(t *testing.T) {
	if _, err := reachableNeighbours("lo"); err != nil {
		t.Fatal(err)
	}
}
