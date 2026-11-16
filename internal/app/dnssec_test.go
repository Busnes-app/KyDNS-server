package app

import (
	"net/netip"
	"slices"

	"github.com/miekg/dns"
	"testing"
	"time"

	"github.com/Busnes-app/kydns-server/internal/dnssec"
	"github.com/Busnes-app/kydns-server/internal/dnsserver"
	"github.com/Busnes-app/kydns-server/internal/settings"
	"github.com/Busnes-app/kydns-server/internal/store"
)

func TestReverseZones(t *testing.T) {
	for in, want := range map[string][]string{
		"192.168.1.0/24": {"1.168.192.in-addr.arpa."},
		"10.0.0.0/8":     {"10.in-addr.arpa."},
		"192.168.4.0/23": {"4.168.192.in-addr.arpa.", "5.168.192.in-addr.arpa."},
		"fd12:3456::/32": {"6.5.4.3.2.1.d.f.ip6.arpa."},
		"fd00::/7":       {"c.f.ip6.arpa.", "d.f.ip6.arpa."},
		"fd12:3454::/30": {"4.5.4.3.2.1.d.f.ip6.arpa.", "5.5.4.3.2.1.d.f.ip6.arpa.", "6.5.4.3.2.1.d.f.ip6.arpa.", "7.5.4.3.2.1.d.f.ip6.arpa."},
	} {
		if got := reverseZones(netip.MustParsePrefix(in)); !slices.Equal(got, want) {
			t.Errorf("%s = %v, want %v", in, got, want)
		}
	}
	// 172.16.0.0/12 is exactly its sixteen /16 zones; public 172.x stays validated.
	got := reverseZones(netip.MustParsePrefix("172.16.0.0/12"))
	if len(got) != 16 || got[0] != "16.172.in-addr.arpa." || got[15] != "31.172.in-addr.arpa." {
		t.Fatalf("172.16.0.0/12 = %v", got)
	}
	for name, want := range map[string]bool{
		"1.0.20.172.in-addr.arpa.": true,
		"1.0.32.172.in-addr.arpa.": false,
		"1.0.15.172.in-addr.arpa.": false,
		"172.in-addr.arpa.":        false,
	} {
		under := slices.ContainsFunc(got, func(z string) bool { return dns.IsSubDomain(z, name) })
		if under != want {
			t.Errorf("%s under an NTA = %v, want %v", name, !want, want)
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
