package rdnss

import (
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/netip"
	"os"
	"sync"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv6"
)

// RFC 4861 §6.2.1 and §10 router constants, and the RFC 8106 §5.1 lifetime.
const (
	MaxInterval     = 600 * time.Second
	MinInterval     = 200 * time.Second
	initialMax      = 16 * time.Second // MAX_INITIAL_RTR_ADVERT_INTERVAL
	initialCount    = 3                // MAX_INITIAL_RTR_ADVERTISEMENTS
	minBetweenRAs   = 3 * time.Second  // MIN_DELAY_BETWEEN_RAS
	maxRADelay      = 500 * time.Millisecond
	Lifetime        = uint32(3 * MaxInterval / time.Second)
	solicitBufBytes = 1500
)

var (
	allNodes   = &net.IPAddr{IP: net.ParseIP("ff02::1")}
	allRouters = &net.IPAddr{IP: net.ParseIP("ff02::2")}
)

// ErrPrivilege is the refusal when the raw socket cannot be opened. It names
// the supported deployments, as DHCP's ErrNotSupported does.
var ErrPrivilege = errors.New("sending router advertisements needs CAP_NET_RAW: " +
	"run KyDNS from the .deb or .rpm package (its systemd unit grants it), " +
	"or in Docker with network_mode: host as root")

// Sender advertises on one interface until Stop.
type Sender struct {
	conn   *ipv6.PacketConn
	raw    *icmp.PacketConn
	ifi    *net.Interface
	src    netip.Addr
	adv    Advert
	logger *slog.Logger

	rs       chan struct{}
	stop     chan struct{}
	done     chan struct{}
	stopOnce sync.Once
}

// Start opens the socket and begins advertising dnsAddr and domain on iface.
func Start(iface string, dnsAddr netip.Addr, domain string, logger *slog.Logger) (*Sender, error) {
	ifi, err := net.InterfaceByName(iface)
	if err != nil {
		return nil, fmt.Errorf("interface %q: %w", iface, err)
	}
	addrs, err := ifInet6(iface)
	if err != nil {
		return nil, err
	}
	src, ok := linkLocal(addrs)
	if !ok {
		return nil, fmt.Errorf("interface %q has no usable link-local address to send from", iface)
	}
	adv := Advert{MAC: ifi.HardwareAddr, DNS: dnsAddr, Domain: domain, Lifetime: Lifetime}
	if !ValidDomain(domain) {
		adv.Domain = ""
	}
	if _, err := adv.Marshal(); err != nil {
		return nil, err
	}
	raw, err := icmp.ListenPacket("ip6:ipv6-icmp", "::")
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return nil, ErrPrivilege
		}
		return nil, fmt.Errorf("open ICMPv6 socket: %w", err)
	}
	s := &Sender{
		raw: raw, conn: raw.IPv6PacketConn(), ifi: ifi, src: src, adv: adv, logger: logger,
		rs: make(chan struct{}, 1), stop: make(chan struct{}), done: make(chan struct{}),
	}
	if err := s.setup(); err != nil {
		raw.Close()
		return nil, err
	}
	go s.read()
	go s.run()
	return s, nil
}

func (s *Sender) setup() error {
	var f ipv6.ICMPFilter
	f.SetAll(true)
	f.Accept(ipv6.ICMPTypeRouterSolicitation)
	for _, err := range []error{
		s.conn.SetICMPFilter(&f),
		s.conn.SetControlMessage(ipv6.FlagHopLimit|ipv6.FlagInterface, true),
		s.conn.SetMulticastInterface(s.ifi),
		s.conn.SetMulticastHopLimit(255),
		s.conn.SetHopLimit(255),
		s.conn.SetMulticastLoopback(false),
		s.conn.JoinGroup(s.ifi, allRouters),
	} {
		if err != nil {
			return fmt.Errorf("configure ICMPv6 socket on %q: %w", s.ifi.Name, err)
		}
	}
	return nil
}

// read wakes the loop for every valid solicitation. The channel holds one, so
// a flood collapses into a single pending reply.
func (s *Sender) read() {
	buf := make([]byte, solicitBufBytes)
	for {
		n, cm, _, err := s.conn.ReadFrom(buf)
		if err != nil {
			return // closed by Stop
		}
		if cm == nil || cm.HopLimit != 255 || cm.IfIndex != s.ifi.Index || !isSolicitation(buf[:n]) {
			continue
		}
		select {
		case s.rs <- struct{}{}:
		default:
		}
	}
}

func (s *Sender) run() {
	defer close(s.done)
	var last time.Time
	sent := 0
	t := time.NewTimer(0)
	defer t.Stop()
	next := time.Now()
	for {
		select {
		case <-s.stop:
			return
		case <-s.rs:
			if n := afterSolicit(time.Now(), last, next, rand.N(maxRADelay)); n.Before(next) {
				next = n
				t.Reset(time.Until(next))
			}
		case <-t.C:
			if err := s.send(s.adv); err != nil {
				s.logger.Warn("router advertisement not sent", "interface", s.ifi.Name, "error", err)
			}
			last = time.Now()
			sent++
			next = last.Add(interval(sent, rand.Float64()))
			t.Reset(time.Until(next))
		}
	}
}

func (s *Sender) send(a Advert) error {
	b, err := a.Marshal()
	if err != nil {
		return err
	}
	cm := &ipv6.ControlMessage{HopLimit: 255, Src: s.src.AsSlice(), IfIndex: s.ifi.Index}
	dst := &net.IPAddr{IP: allNodes.IP, Zone: s.ifi.Name}
	_, err = s.conn.WriteTo(b, cm, dst)
	return err
}

// Stop withdraws the advertisement (lifetime 0, RFC 8106 §5.1) and closes.
func (s *Sender) Stop() error {
	var err error
	s.stopOnce.Do(func() {
		close(s.stop)
		<-s.done
		w := s.adv
		w.Lifetime = 0
		err = s.send(w)
		if cerr := s.raw.Close(); err == nil {
			err = cerr
		}
	})
	return err
}

// interval is the wait after the sent-th advertisement: uniform in
// [MinInterval, MaxInterval], capped at 16 s for the first three (RFC 4861
// §6.2.4). r is uniform in [0, 1).
func interval(sent int, r float64) time.Duration {
	d := MinInterval + time.Duration(r*float64(MaxInterval-MinInterval))
	if sent < initialCount {
		d = min(d, initialMax)
	}
	return d
}

// afterSolicit is when a solicitation's reply goes out: after a random delay,
// never sooner than 3 s after the last RA (RFC 4861 §6.2.6), and never later
// than the one already scheduled.
func afterSolicit(now, last, next time.Time, jitter time.Duration) time.Time {
	at := now.Add(jitter)
	if floor := last.Add(minBetweenRAs); at.Before(floor) {
		at = floor
	}
	if next.Before(at) {
		return next
	}
	return at
}
