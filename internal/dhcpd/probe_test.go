package dhcpd

import (
	"net/netip"
	"testing"
)

func TestNopProberNeverBlocks(t *testing.T) {
	if (nopProber{}).InUse(netip.MustParseAddr("192.168.1.10"), "aa:aa:aa:aa:aa:aa") {
		t.Fatal("nopProber reported an address in use")
	}
}
