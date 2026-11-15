package app

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"sync"

	"github.com/Busnes-app/kydns-server/internal/rdnss"
	"github.com/Busnes-app/kydns-server/internal/store"
)

// rdnssWanted mirrors dhcpWanted: a replica never advertises, so the LAN hears
// one resolver until the replica is promoted.
func rdnssWanted(v store.Settings, role Role) bool {
	return v.RDNSSEnabled && v.RDNSSInterface != "" && role != RoleReplica
}

var errReplicaNoRDNSS = errors.New(
	"this node is a replica, so it does not send IPv6 DNS advertisements; the primary does, " +
		"and a replica does after it is promoted")

// advertiser is the running sender as the runner needs it.
type advertiser interface{ Stop() error }

// rdnssRunner owns the sender's lifecycle, the way dhcpRunner owns DHCP's.
type rdnssRunner struct {
	logger *slog.Logger
	role   func() Role
	// listen is dns.listen: the advertised address must be one DNS answers on.
	listen string
	// Injected for tests; nil means the real one.
	qualifies func(string) error
	pick      func(string, netip.Addr) (netip.Prefix, error)
	start     func(iface string, addr netip.Addr, domain string) (advertiser, error)

	mu        sync.Mutex
	running   advertiser
	current   store.Settings
	addr      netip.Prefix
	lastError error
}

// Reconcile brings the sender in line with v. Unchanged settings leave a
// running sender alone.
func (r *rdnssRunner) Reconcile(v store.Settings) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !rdnssWanted(v, r.role()) {
		r.stopLocked()
		r.current, r.lastError = v, nil
		if v.RDNSSEnabled && v.RDNSSInterface != "" {
			r.lastError = errReplicaNoRDNSS
		}
		return
	}
	if r.running != nil && rdnssConfigEqual(r.current, v) {
		r.lastError = nil
		return
	}
	addr, err := r.build(v)
	if err != nil {
		// A refused rebuild leaves a working sender alone, like DHCP.
		r.lastError = err
		r.logger.Error("ipv6 dns advertisement is enabled but cannot start", "error", err)
		if r.running == nil {
			r.current = v
		}
		return
	}
	r.stopLocked()
	start := r.start
	if start == nil {
		start = func(iface string, a netip.Addr, domain string) (advertiser, error) {
			return rdnss.Start(iface, a, domain, r.logger)
		}
	}
	s, err := start(v.RDNSSInterface, addr.Addr(), v.PrivateDomain)
	if err != nil {
		r.current, r.lastError = v, err
		r.logger.Error("ipv6 dns advertisement failed to start", "error", err)
		return
	}
	r.running, r.current, r.addr, r.lastError = s, v, addr, nil
	r.logger.Info("ipv6 dns advertisement started", "interface", v.RDNSSInterface, "address", addr.Addr().String())
}

// build runs every host-state check that can refuse a start.
//
// ponytail: the address is chosen when the sender starts. If it later leaves
// the interface, the advertisement keeps naming it until the next save or
// restart; a netlink address watch that calls Reconcile would close that.
func (r *rdnssRunner) build(v store.Settings) (netip.Prefix, error) {
	qualifies, pick := r.qualifies, r.pick
	if qualifies == nil {
		qualifies = rdnss.Qualifies
	}
	if pick == nil {
		pick = rdnss.Pick
	}
	if err := qualifies(v.RDNSSInterface); err != nil {
		return netip.Prefix{}, err
	}
	var configured netip.Addr
	if v.RDNSSAddress != "" {
		a, err := netip.ParseAddr(v.RDNSSAddress)
		if err != nil {
			return netip.Prefix{}, fmt.Errorf("rdnss_address: %q is not an IP address", v.RDNSSAddress)
		}
		configured = a
	}
	p, err := pick(v.RDNSSInterface, configured)
	if err != nil {
		return netip.Prefix{}, err
	}
	if err := listensOn(r.listen, p.Addr()); err != nil {
		return netip.Prefix{}, err
	}
	return p, nil
}

// listensOn refuses an advertisement DNS would not answer: dns.listen bound to
// IPv4 only, or to a different IPv6 address.
func listensOn(listen string, a netip.Addr) error {
	host, _, err := net.SplitHostPort(listen)
	if err != nil || host == "" || host == "::" {
		return nil // ":53" and "[::]:53" listen on every address
	}
	h, err := netip.ParseAddr(host)
	if err != nil {
		return nil // a host name; the bind already resolved it
	}
	if h.Is4() {
		return fmt.Errorf("dns.listen is %s, which answers on IPv4 only, so advertising %s over IPv6 would send clients to a closed port; set dns.listen to \":53\"", listen, a)
	}
	if h != a {
		return fmt.Errorf("dns.listen is %s, so KyDNS does not answer on %s; advertise %s or set dns.listen to \":53\"", listen, a, h)
	}
	return nil
}

func (r *rdnssRunner) stopLocked() {
	if r.running == nil {
		return
	}
	if err := r.running.Stop(); err != nil {
		r.logger.Warn("ipv6 dns advertisement did not withdraw cleanly", "error", err)
	}
	r.running, r.addr = nil, netip.Prefix{}
	r.logger.Info("ipv6 dns advertisement stopped")
}

// Shutdown withdraws the advertisement when KyDNS stops.
func (r *rdnssRunner) Shutdown() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopLocked()
}

// Status is what the API and the UI report.
func (r *rdnssRunner) Status() (running bool, addr netip.Addr, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running != nil, r.addr.Addr(), r.lastError
}

func rdnssConfigEqual(a, b store.Settings) bool {
	return a.RDNSSEnabled == b.RDNSSEnabled && a.RDNSSInterface == b.RDNSSInterface &&
		a.RDNSSAddress == b.RDNSSAddress && a.PrivateDomain == b.PrivateDomain
}
