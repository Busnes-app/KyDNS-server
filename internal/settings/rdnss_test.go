package settings

import (
	"errors"
	"strings"
	"testing"

	"github.com/Busnes-app/kydns-server/internal/store"
)

func TestValidateRDNSS(t *testing.T) {
	ok := []func(*store.Settings){
		func(v *store.Settings) {},                            // off by default
		func(v *store.Settings) { v.RDNSSInterface = "eth0" }, // chosen, still off
		func(v *store.Settings) { v.RDNSSEnabled, v.RDNSSInterface = true, "enp3s0" },
		func(v *store.Settings) { v.RDNSSEnabled, v.RDNSSInterface, v.RDNSSAddress = true, "eth0", "fd00::53" },
		func(v *store.Settings) { v.RDNSSAddress = "2001:db8::1" },
	}
	for i, m := range ok {
		v := valid()
		m(&v)
		if err := ValidateStored(v); err != nil {
			t.Errorf("case %d: %v", i, err)
		}
	}
	bad := map[string]func(*store.Settings){
		"enabled without interface": func(v *store.Settings) { v.RDNSSEnabled = true },
		"long interface":            func(v *store.Settings) { v.RDNSSInterface = strings.Repeat("e", 16) },
		"path interface":            func(v *store.Settings) { v.RDNSSInterface = "../../etc" },
		"dot interface":             func(v *store.Settings) { v.RDNSSInterface = ".." },
		"space interface":           func(v *store.Settings) { v.RDNSSInterface = " eth0" },
		"ipv4 address":              func(v *store.Settings) { v.RDNSSAddress = "192.168.1.2" },
		"mapped address":            func(v *store.Settings) { v.RDNSSAddress = "::ffff:192.168.1.2" },
		"link-local":                func(v *store.Settings) { v.RDNSSAddress = "fe80::1" },
		"zoned":                     func(v *store.Settings) { v.RDNSSAddress = "fe80::1%eth0" },
		"loopback":                  func(v *store.Settings) { v.RDNSSAddress = "::1" },
		"multicast":                 func(v *store.Settings) { v.RDNSSAddress = "ff02::1" },
		"unspecified":               func(v *store.Settings) { v.RDNSSAddress = "::" },
		"junk":                      func(v *store.Settings) { v.RDNSSAddress = "not an address" },
	}
	for name, m := range bad {
		v := valid()
		m(&v)
		var fe FieldError
		if err := ValidateStored(v); !errors.As(err, &fe) || !strings.HasPrefix(fe.Field, "rdnss_") {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// Node-local like DHCP: a replica configures its own advertisement, through
// the narrow write, and a mixed write is still refused whole.
func TestReplicaMaySaveTheRDNSSSettings(t *testing.T) {
	w, svc, _ := newReplicaService(t)
	v := valid()
	v.RDNSSEnabled, v.RDNSSInterface, v.RDNSSAddress = true, "eth0", "fd00::53"
	if err := svc.Set(v, ""); err != nil {
		t.Fatalf("a replica could not save its RDNSS settings: %v", err)
	}
	if w.narrowWrites != 1 || w.writes != 0 || !w.cur.RDNSSEnabled || w.cur.RDNSSAddress != "fd00::53" {
		t.Fatalf("narrow %d, whole %d, stored %+v", w.narrowWrites, w.writes, w.cur)
	}
	v.TTL = 999
	if err := svc.Set(v, ""); !errors.Is(err, ErrReadOnlyReplica) {
		t.Fatalf("mixed write: %v", err)
	}
}
