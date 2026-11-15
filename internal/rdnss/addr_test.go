package rdnss

import (
	"net/netip"
	"strings"
	"testing"
)

const table = `fe8000000000000000000000000000ab 02 40 20 80 eth0
fd00000000000000000000000000c0de 02 40 00 01 eth0
20010db8000000000000000000000042 02 40 00 00 eth0
fd000000000000000000000000000053 02 40 00 80 eth0
fd000000000000000000000000000054 02 40 00 20 eth0
fd000000000000000000000000000055 02 40 00 40 eth0
fd000000000000000000000000000099 03 40 00 80 wlan0
00000000000000000000000000000001 01 80 10 80 lo
garbage line
`

func TestParseIfInet6(t *testing.T) {
	got := parseIfInet6(table, "eth0")
	if len(got) != 6 {
		t.Fatalf("got %d addresses: %v", len(got), got)
	}
	if got[0].Addr != netip.MustParsePrefix("fe80::ab/64") || !got[0].Stable {
		t.Errorf("link-local: %+v", got[0])
	}
	for i, stable := range []bool{true, false, true, true, false, false} {
		if got[i].Stable != stable {
			t.Errorf("%s stable=%v, want %v", got[i].Addr, got[i].Stable, stable)
		}
	}
}

func TestChoose(t *testing.T) {
	addrs := parseIfInet6(table, "eth0")
	p, err := Choose(addrs, netip.Addr{})
	if err != nil || p.Addr() != netip.MustParseAddr("fd00::53") {
		t.Fatalf("auto: %v %v, want the stable ULA, not the temporary one", p, err)
	}
	if p, err := Choose(addrs, netip.MustParseAddr("2001:db8::42")); err != nil || p.Addr().String() != "2001:db8::42" {
		t.Fatalf("configured: %v %v", p, err)
	}
	for _, bad := range []string{"fd00::c0de", "fd00::54", "fd00::55", "fd00::99"} {
		if _, err := Choose(addrs, netip.MustParseAddr(bad)); err == nil {
			t.Errorf("%s accepted", bad)
		}
	}
	// Only a stable global: it is used.
	g := parseIfInet6(strings.Join(strings.Split(table, "\n")[:3], "\n"), "eth0")
	if p, err := Choose(g, netip.Addr{}); err != nil || p.Addr().String() != "2001:db8::42" {
		t.Fatalf("global fallback: %v %v", p, err)
	}
	// Only link-local and temporary: refused.
	if _, err := Choose(g[:2], netip.Addr{}); err == nil {
		t.Fatal("link-local or temporary chosen")
	}
	if a, ok := linkLocal(addrs); !ok || a.String() != "fe80::ab" {
		t.Fatalf("link-local source: %v %v", a, ok)
	}
}
