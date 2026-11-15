package rdnss

import (
	"bytes"
	"encoding/hex"
	"net"
	"net/netip"
	"strings"
	"testing"
)

// unhex reads hex words, ignoring each line from its first non-hex word on.
func unhex(t *testing.T, s string) []byte {
	t.Helper()
	var words []string
	for _, line := range strings.Split(s, "\n") {
		for _, w := range strings.Fields(line) {
			if strings.Trim(w, "0123456789abcdef") != "" {
				break
			}
			words = append(words, w)
		}
	}
	b, err := hex.DecodeString(strings.Join(words, ""))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestAdvertBytes pins every octet against RFC 4861 §4.2 and RFC 8106 §5.
func TestAdvertBytes(t *testing.T) {
	a := Advert{
		MAC:      net.HardwareAddr{0x02, 0, 0, 0, 0, 0x01},
		DNS:      netip.MustParseAddr("fd00::53"),
		Domain:   "home.arpa",
		Lifetime: 1800,
	}
	got, err := a.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	want := unhex(t, `
		86 00 0000            type 134, code 0, checksum (kernel fills it)
		00 00 0000            cur hop limit 0, M=O=0, router lifetime 0
		00000000 00000000     reachable time, retrans timer
		01 01 020000000001    source link-layer address
		19 03 0000 00000708   RDNSS: type 25, length 3, reserved, lifetime 1800
		fd000000000000000000000000000053
		1f 03 0000 00000708   DNSSL: type 31, length 3, reserved, lifetime 1800
		04686f6d65 0461727061 00   home.arpa.
		0000000000            padding to 8 octets`)
	if !bytes.Equal(got, want) {
		t.Fatalf("advert\n got %x\nwant %x", got, want)
	}
	if got[6] != 0 || got[7] != 0 {
		t.Fatal("router lifetime must be 0")
	}
}

func TestAdvertWithdrawAndOptional(t *testing.T) {
	a := Advert{DNS: netip.MustParseAddr("2001:db8::1")}
	got, err := a.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	// No MAC, no domain: header then RDNSS only, lifetime 0.
	want := unhex(t, `86000000 00000000 00000000 00000000
		19030000 00000000 20010db8000000000000000000000001`)
	if !bytes.Equal(got, want) {
		t.Fatalf("got %x\nwant %x", got, want)
	}
	for _, typ := range []byte{3, 5} { // prefix information, MTU: never sent
		for i := 16; i < len(got); i += int(got[i+1]) * 8 {
			if got[i] == typ {
				t.Fatalf("option %d present", typ)
			}
		}
	}
}

func TestAdvertRefuses(t *testing.T) {
	for _, a := range []Advert{
		{DNS: netip.MustParseAddr("192.0.2.1")},
		{DNS: netip.MustParseAddr("::ffff:192.0.2.1")},
		{},
		{DNS: netip.MustParseAddr("fd00::1"), Domain: strings.Repeat("a", 64) + ".arpa"},
		{DNS: netip.MustParseAddr("fd00::1"), Domain: "."},
	} {
		if _, err := a.Marshal(); err == nil {
			t.Errorf("%+v: no error", a)
		}
	}
	long := strings.Repeat(strings.Repeat("a", 63)+".", 4) // 256 octets on the wire
	if ValidDomain(long) {
		t.Error("an over-long domain fits a DNSSL option")
	}
	if !ValidDomain("home.arpa") {
		t.Error("home.arpa refused")
	}
}

func TestIsSolicitation(t *testing.T) {
	for _, c := range []struct {
		b    []byte
		want bool
	}{
		{unhex(t, "85000000 00000000"), true},
		{unhex(t, "85000000 00000000 0101020000000001"), true},
		{unhex(t, "85010000 00000000"), false}, // code must be 0
		{unhex(t, "86000000 00000000"), false}, // an RA, not an RS
		{unhex(t, "850000"), false},            // short
	} {
		if got := isSolicitation(c.b); got != c.want {
			t.Errorf("%x: %v, want %v", c.b, got, c.want)
		}
	}
}
