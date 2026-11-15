package rdnss

import (
	"errors"
	"fmt"
	"net"

	"github.com/Busnes-app/kydns-server/internal/dhcpd"
)

// ErrBridged is the refusal inside a bridge-mode container, where the
// interface is a veth and no advertisement would reach the LAN.
var ErrBridged = errors.New("router advertisements have to reach the LAN, which this deployment cannot: " +
	"run KyDNS as a native package install, or in Docker with network_mode: host")

// Qualifies reports whether advertisements may be sent on the interface.
func Qualifies(name string) error {
	ifi, err := net.InterfaceByName(name)
	if err != nil {
		return fmt.Errorf("interface %q: %w", name, err)
	}
	if ifi.Flags&net.FlagUp == 0 {
		return fmt.Errorf("interface %q is down", name)
	}
	if ifi.Flags&net.FlagLoopback != 0 {
		return fmt.Errorf("interface %q is the loopback", name)
	}
	if ifi.Flags&net.FlagMulticast == 0 {
		return fmt.Errorf("interface %q does not support multicast", name)
	}
	if dhcpd.IsVeth(name) {
		return ErrBridged
	}
	return nil
}
