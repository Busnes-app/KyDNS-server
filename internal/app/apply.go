package app

import (
	"fmt"
	"log/slog"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/Busnes-app/kydns-server/internal/discovery"
	"github.com/Busnes-app/kydns-server/internal/discovery/dhcp"
	"github.com/Busnes-app/kydns-server/internal/dnssec"
	"github.com/Busnes-app/kydns-server/internal/dnsserver"
	"github.com/Busnes-app/kydns-server/internal/health"
	"github.com/Busnes-app/kydns-server/internal/registry"
	"github.com/Busnes-app/kydns-server/internal/settings"
	"github.com/Busnes-app/kydns-server/internal/zone"
)

// liveComponents is every running piece a settings change fans out to. It
// exists as a named type, rather than a closure inside Serve, so a test can
// build one against real components and call Apply directly — proving the
// fan-out actually reaches them, not just that Service.Set returned nil.
type liveComponents struct {
	acl           *dnsserver.ACL
	forwarder     *dnsserver.Forwarder
	cache         *dnsserver.Cache
	dnsSrv        *dnsserver.Server
	authoritative *dnsserver.Authoritative
	checker       *health.Checker
	poller        *discovery.Poller // nil only in tests that do not build one
	zoneHolder    *zone.Holder
	registry      *registry.Registry
	logger        *slog.Logger
	dhcp          *dhcpRunner       // nil in tests that do not build one
	rdnss         *rdnssRunner      // nil in tests that do not build one
	validator     *dnssec.Validator // nil in tests that do not build one

	// prevUpstreams tracks what the forwarder was last told, so a change
	// against the previous upstream list flushes stale cached answers.
	prevUpstreams []string

	// mu serializes the fan-out. A settings save and a replication pull both
	// reach Apply, from different goroutines, and prevUpstreams has one writer.
	mu sync.Mutex

	// current is the settings holder's latest snapshot, read under mu. Every
	// caller publishes before it applies, so applying the latest rather than the
	// argument means two racing callers cannot leave the older one live. Nil in
	// tests that pass the snapshot they mean.
	current func() *settings.Snapshot
}

// Apply fans a validated, already-built settings snapshot out to every live
// component. Every value here is already validated and built, so no swap
// below can fail.
func (l *liveComponents) Apply(s *settings.Snapshot) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.current != nil {
		s = l.current()
	}
	// The private zone first: the snapshot rebuild at the end of this function
	// builds names under it, and the records were already moved into it by the
	// same write that produced this snapshot.
	if zoneChanged(l.authoritative.Zone(), s.Raw.PrivateDomain) {
		old := l.authoritative.Zone()
		l.authoritative.SetZone(s.Raw.PrivateDomain)
		l.registry.SetZone(s.Raw.PrivateDomain)
		// Cached answers are keyed by name, and every name under the old zone is
		// now wrong. Nothing else evicts them, so they would be served until
		// their TTL ran out.
		l.cache.Flush()
		l.logger.Info("private zone renamed", "from", old, "to", l.authoritative.Zone())
	}
	l.acl.Replace(s.AllowQuery)
	l.applyDNSSEC(s)
	l.forwarder.Replace(s.Upstreams)
	flushOnUpstreamChange(l.cache, l.prevUpstreams, s.Raw.Upstreams, l.logger)
	l.prevUpstreams = s.Raw.Upstreams
	l.cache.Retune(s.Raw.CacheEntries, s.Raw.CacheMinTTL, s.Raw.CacheMaxTTL, s.Raw.NegativeMaxTTL)
	l.dnsSrv.SetLogging(s.Raw.LogQueries, s.Raw.LogClientIP)
	l.authoritative.SetTTL(uint32(s.Raw.TTL))
	l.authoritative.SetReverseZones(s.ReverseZones)
	l.checker.Reconfigure(
		time.Duration(s.Raw.HealthInterval)*time.Second,
		time.Duration(s.Raw.HealthTimeout)*time.Second,
		s.Raw.HealthWorkers)
	// The lease source follows the settings: built-in, lease file, or neither.
	// Reconcile is idempotent, so an unchanged configuration is a no-op rather
	// than a restart of a working listener.
	if l.dhcp != nil {
		l.dhcp.Reconcile(s.Raw)
	}
	if l.rdnss != nil {
		l.rdnss.Reconcile(s.Raw)
	}
	if l.poller != nil {
		// Gated on what is actually running, not on what was asked for: a
		// build that refused leaves nothing owning the source, and the old
		// lease file would otherwise keep feeding the zone forever.
		running := false
		if l.dhcp != nil {
			running, _ = l.dhcp.Status()
		}
		if !running {
			if s.Raw.DHCPLeaseFile != "" {
				l.poller.SetSource(&dhcp.DnsmasqSource{Path: s.Raw.DHCPLeaseFile})
			} else {
				l.poller.SetSource(nil)
			}
		}
		l.poller.SetInterval(time.Duration(s.Raw.DiscoveryInterval) * time.Second)
	}
	// Reverse zones are an input to the zone snapshot, so it has to be rebuilt.
	if err := l.zoneHolder.Rebuild(); err != nil {
		l.logger.Error("snapshot rebuild after a settings change failed, still serving the previous snapshot", "error", err)
	}
	l.logger.Info("settings applied")
}

// applyDNSSEC switches local validation and refreshes its automatic negative
// trust anchors: names KyDNS answers itself never reach the validator, but a
// forwarded CNAME tail under them can.
func (l *liveComponents) applyDNSSEC(s *settings.Snapshot) {
	if l.validator == nil {
		return
	}
	l.validator.SetNTAs(dnssecNTAs(s))
	if s.Raw.DNSSECValidate {
		l.forwarder.SetValidator(l.validator)
	} else {
		l.forwarder.SetValidator(nil)
	}
}

func dnssecNTAs(s *settings.Snapshot) []string {
	ntas := []string{s.Raw.PrivateDomain}
	for _, p := range s.ReverseZones {
		ntas = append(ntas, reverseZones(p)...)
	}
	return ntas
}

// reverseZones are the arpa zones covering exactly prefix p: a prefix off
// the label boundary (octet for IPv4, nibble for IPv6) splits into one zone per
// sub-prefix at the next boundary, so the exception never widens past p.
func reverseZones(p netip.Prefix) []string {
	p = p.Masked()
	step := 4
	if p.Addr().Is4() {
		step = 8
	}
	bits := (p.Bits() + step - 1) / step * step
	var out []string
	for i := 0; i < 1<<(bits-p.Bits()); i++ {
		b := p.Addr().AsSlice()
		if k := (bits - 1) / 8; i > 0 { // the split bits all sit in the boundary's byte
			b[k] |= byte(i << (8*(k+1) - bits))
		}
		a, _ := netip.AddrFromSlice(b)
		out = append(out, reverseZone(netip.PrefixFrom(a, bits)))
	}
	return out
}

// reverseZone is the arpa zone of a prefix on the label boundary.
func reverseZone(p netip.Prefix) string {
	a := p.Addr()
	var labels []string
	if a.Is4() {
		b := a.As4()
		for i := 0; i < p.Bits()/8; i++ {
			labels = append([]string{fmt.Sprint(b[i])}, labels...)
		}
		return strings.Join(append(labels, "in-addr.arpa."), ".")
	}
	b := a.As16()
	for i := 0; i < p.Bits()/4; i++ {
		n := b[i/2] >> 4
		if i%2 == 1 {
			n = b[i/2] & 0xf
		}
		labels = append([]string{fmt.Sprintf("%x", n)}, labels...)
	}
	return strings.Join(append(labels, "ip6.arpa."), ".")
}
