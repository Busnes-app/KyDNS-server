package app

import (
	"net/netip"
	"testing"
	"time"

	"github.com/Busnes-app/kydns-server/internal/dnssec"
	"github.com/Busnes-app/kydns-server/internal/dnsserver"
	"github.com/Busnes-app/kydns-server/internal/settings"
	"github.com/Busnes-app/kydns-server/internal/store"
)

func TestReverseZone(t *testing.T) {
	for in, want := range map[string]string{
		"192.168.1.0/24": "1.168.192.in-addr.arpa.",
		"10.0.0.0/8":     "10.in-addr.arpa.",
		"172.16.0.0/12":  "172.in-addr.arpa.", // widened to the octet
		"fd12:3456::/32": "6.5.4.3.2.1.d.f.ip6.arpa.",
		"fd00::/7":       "f.ip6.arpa.",
	} {
		if got := reverseZone(netip.MustParsePrefix(in)); got != want {
			t.Errorf("%s = %s, want %s", in, got, want)
		}
	}
}

func TestApplySwitchesValidation(t *testing.T) {
	fwd := dnsserver.NewForwarder(nil, time.Second, dnsserver.NewCache(10, 1, 60, 60))
	l := &liveComponents{forwarder: fwd, validator: dnssec.New(fwd.Query, nil)}
	snap := &settings.Snapshot{Raw: store.Settings{PrivateDomain: "home.arpa", DNSSECValidate: true}}
	l.applyDNSSEC(snap)
	if !fwd.Validating() {
		t.Fatal("on did not reach the forwarder")
	}
	snap.Raw.DNSSECValidate = false
	l.applyDNSSEC(snap)
	if fwd.Validating() {
		t.Fatal("off did not reach the forwarder")
	}
}
