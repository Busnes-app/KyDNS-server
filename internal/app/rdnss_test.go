package app

import (
	"errors"
	"io"
	"log/slog"
	"net/netip"
	"strings"
	"testing"

	"github.com/Busnes-app/kydns-server/internal/store"
)

type fakeAdvertiser struct{ stops *int }

func (f fakeAdvertiser) Stop() error { *f.stops++; return nil }

type rdnssHarness struct {
	r      *rdnssRunner
	role   Role
	starts []string
	stops  int
}

func newRDNSSHarness(listen string) *rdnssHarness {
	h := &rdnssHarness{role: RoleStandalone}
	h.r = &rdnssRunner{
		logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		role:      func() Role { return h.role },
		listen:    listen,
		qualifies: func(string) error { return nil },
		pick: func(_ string, c netip.Addr) (netip.Prefix, error) {
			if c.IsValid() {
				return netip.PrefixFrom(c, 64), nil
			}
			return netip.MustParsePrefix("fd00::53/64"), nil
		},
		start: func(iface string, a netip.Addr, domain string) (advertiser, error) {
			h.starts = append(h.starts, iface+" "+a.String()+" "+domain)
			return fakeAdvertiser{&h.stops}, nil
		},
	}
	return h
}

func rdnssOn() store.Settings {
	return store.Settings{PrivateDomain: "home.arpa", RDNSSEnabled: true, RDNSSInterface: "eth0"}
}

func TestRDNSSRunnerLifecycle(t *testing.T) {
	h := newRDNSSHarness(":53")
	h.r.Reconcile(store.Settings{}) // off
	if len(h.starts) != 0 {
		t.Fatal("started while off")
	}
	h.r.Reconcile(rdnssOn())
	h.r.Reconcile(rdnssOn()) // unchanged: left alone
	if len(h.starts) != 1 || h.starts[0] != "eth0 fd00::53 home.arpa" {
		t.Fatalf("starts %v", h.starts)
	}
	if running, addr, err := h.r.Status(); !running || addr.String() != "fd00::53" || err != nil {
		t.Fatalf("status %v %v %v", running, addr, err)
	}
	v := rdnssOn()
	v.RDNSSAddress = "fd00::99"
	h.r.Reconcile(v) // changed: restarted
	if len(h.starts) != 2 || h.stops != 1 {
		t.Fatalf("starts %v stops %d", h.starts, h.stops)
	}
	h.r.Reconcile(store.Settings{PrivateDomain: "home.arpa"}) // switched off: withdrawn
	if h.stops != 2 {
		t.Fatalf("stops %d", h.stops)
	}
	if running, _, _ := h.r.Status(); running {
		t.Fatal("still running")
	}
}

func TestRDNSSRunnerReplicaNeverSends(t *testing.T) {
	h := newRDNSSHarness(":53")
	h.r.Reconcile(rdnssOn())
	h.role = RoleReplica
	h.r.Reconcile(rdnssOn())
	if h.stops != 1 {
		t.Fatal("a demoted node kept advertising")
	}
	if _, _, err := h.r.Status(); !errors.Is(err, errReplicaNoRDNSS) {
		t.Fatalf("status error %v", err)
	}
}

func TestRDNSSRunnerRefusals(t *testing.T) {
	h := newRDNSSHarness(":53")
	h.r.qualifies = func(string) error { return errors.New("bridged") }
	h.r.Reconcile(rdnssOn())
	if _, _, err := h.r.Status(); err == nil || len(h.starts) != 0 {
		t.Fatalf("a refused interface started: %v", err)
	}

	// A refused rebuild leaves the running sender alone.
	h = newRDNSSHarness(":53")
	h.r.Reconcile(rdnssOn())
	h.r.pick = func(string, netip.Addr) (netip.Prefix, error) { return netip.Prefix{}, errors.New("no stable") }
	v := rdnssOn()
	v.RDNSSAddress = "fd00::1"
	h.r.Reconcile(v)
	if running, _, err := h.r.Status(); !running || err == nil || h.stops != 0 {
		t.Fatalf("running %v err %v stops %d", running, err, h.stops)
	}

	h = newRDNSSHarness(":53")
	h.r.start = func(string, netip.Addr, string) (advertiser, error) { return nil, errors.New("needs CAP_NET_RAW") }
	h.r.Reconcile(rdnssOn())
	if running, _, err := h.r.Status(); running || err == nil {
		t.Fatalf("running %v err %v", running, err)
	}
}

func TestListensOn(t *testing.T) {
	a := netip.MustParseAddr("fd00::53")
	for listen, ok := range map[string]bool{
		":53": true, "[::]:53": true, "[fd00::53]:53": true, "dns.example:53": true,
		"0.0.0.0:53": false, "192.168.1.2:53": false, "[fd00::1]:53": false,
	} {
		err := listensOn(listen, a)
		if (err == nil) != ok {
			t.Errorf("%s: %v", listen, err)
		}
		if err != nil && !strings.Contains(err.Error(), "dns.listen") {
			t.Errorf("%s: error does not name dns.listen: %v", listen, err)
		}
	}
	h := newRDNSSHarness("0.0.0.0:53")
	h.r.Reconcile(rdnssOn())
	if len(h.starts) != 0 {
		t.Fatal("advertised an address DNS does not answer on")
	}
}
