package rdnss

import (
	"bytes"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv6"
)

// TestNetnsAdvertisement runs a real sender over a veth pair inside a private
// user and network namespace, and reads what arrives at the other end. It
// skips where unprivileged namespaces are not allowed.
func TestNetnsAdvertisement(t *testing.T) {
	if os.Getenv("KYDNS_RDNSS_NETNS") == "1" {
		netnsInner(t)
		return
	}
	if _, err := exec.LookPath("unshare"); err != nil {
		t.Skip("unshare not installed")
	}
	if _, err := exec.LookPath("ip"); err != nil {
		t.Skip("ip not installed")
	}
	if err := exec.Command("unshare", "-rn", "true").Run(); err != nil {
		t.Skipf("unprivileged network namespaces unavailable: %v", err)
	}
	cmd := exec.Command("unshare", "-rn", os.Args[0], "-test.run=^TestNetnsAdvertisement$", "-test.v")
	cmd.Env = append(os.Environ(), "KYDNS_RDNSS_NETNS=1")
	out, err := cmd.CombinedOutput()
	t.Logf("%s", out)
	if err != nil {
		t.Fatalf("inner test: %v", err)
	}
	if !strings.Contains(string(out), "--- PASS: TestNetnsAdvertisement") {
		t.Fatal("inner test did not pass")
	}
}

func sh(t *testing.T, args ...string) {
	t.Helper()
	if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
		t.Fatalf("%v: %v %s", args, err, out)
	}
}

func netnsInner(t *testing.T) {
	sh(t, "ip", "link", "add", "va", "type", "veth", "peer", "name", "vb")
	sh(t, "ip", "link", "set", "va", "up")
	sh(t, "ip", "link", "set", "vb", "up")
	sh(t, "ip", "-6", "addr", "add", "fd00::53/64", "dev", "va", "nodad")
	deadline := time.Now().Add(10 * time.Second)
	for _, n := range []string{"va", "vb"} { // wait out DAD on the link-locals
		for {
			addrs, err := ifInet6(n)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := linkLocal(addrs); ok {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s: no link-local address", n)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	vb, err := net.InterfaceByName("vb")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := icmp.ListenPacket("ip6:ipv6-icmp", "::")
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	rx := raw.IPv6PacketConn()
	var f ipv6.ICMPFilter
	f.SetAll(true)
	f.Accept(ipv6.ICMPTypeRouterAdvertisement)
	if err := rx.SetICMPFilter(&f); err != nil {
		t.Fatal(err)
	}
	if err := rx.SetControlMessage(ipv6.FlagHopLimit|ipv6.FlagInterface|ipv6.FlagDst, true); err != nil {
		t.Fatal(err)
	}

	s, err := Start("va", netip.MustParseAddr("fd00::53"), "home.arpa", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	read := func() ([]byte, net.Addr) {
		t.Helper()
		buf := make([]byte, 1500)
		rx.SetReadDeadline(time.Now().Add(8 * time.Second))
		for {
			n, cm, src, err := rx.ReadFrom(buf)
			if err != nil {
				t.Fatalf("no RA received: %v", err)
			}
			if cm.IfIndex != vb.Index {
				continue
			}
			if cm.HopLimit != 255 || !cm.Dst.Equal(net.ParseIP("ff02::1")) {
				t.Fatalf("hop limit %d, destination %v", cm.HopLimit, cm.Dst)
			}
			return append([]byte(nil), buf[:n]...), src
		}
	}
	checkRA := func(b []byte, src net.Addr, lifetime uint32) {
		t.Helper()
		if b[0] != typeRA || b[6] != 0 || b[7] != 0 {
			t.Fatalf("not an RA with router lifetime 0: %x", b)
		}
		if ip := src.(*net.IPAddr).IP; !ip.IsLinkLocalUnicast() {
			t.Fatalf("source %v is not link-local", ip)
		}
		found := false
		for i := 16; i+8 <= len(b) && b[i+1] > 0; i += int(b[i+1]) * 8 {
			switch b[i] {
			case optRDNSS:
				found = true
				if got := binary.BigEndian.Uint32(b[i+4:]); got != lifetime {
					t.Fatalf("RDNSS lifetime %d, want %d", got, lifetime)
				}
				if !bytes.Equal(b[i+8:i+24], net.ParseIP("fd00::53")) {
					t.Fatalf("RDNSS address %x", b[i+8:i+24])
				}
			case 3:
				t.Fatal("prefix information sent")
			}
		}
		if !found {
			t.Fatalf("no RDNSS option: %x", b)
		}
	}
	b, src := read()
	checkRA(b, src, Lifetime)

	// A solicitation from the peer is answered, no sooner than 3 s after the
	// first advertisement.
	rs := []byte{typeRS, 0, 0, 0, 0, 0, 0, 0}
	cm := &ipv6.ControlMessage{HopLimit: 255, IfIndex: vb.Index}
	start := time.Now()
	if _, err := rx.WriteTo(rs, cm, &net.IPAddr{IP: net.ParseIP("ff02::2"), Zone: "vb"}); err != nil {
		t.Fatal(err)
	}
	b, src = read()
	checkRA(b, src, Lifetime)
	if waited := time.Since(start); waited > 4*time.Second {
		t.Fatalf("solicitation answered after %v", waited)
	}

	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	b, src = read()
	checkRA(b, src, 0) // withdrawn
}
