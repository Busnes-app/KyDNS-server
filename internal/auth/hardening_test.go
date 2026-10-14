package auth

import (
	"fmt"
	"testing"
	"time"
)

func TestSessionsCreatedBeforeTheCutoffAreVoid(t *testing.T) {
	s := NewSessions(time.Hour, 12*time.Hour)
	old := s.Create()
	cutoff := old.Created.Add(time.Nanosecond)
	s.SetNotBefore(func() time.Time { return cutoff })
	if _, ok := s.Get(old.ID); ok {
		t.Fatal("a session from before the cutoff survived")
	}
	s.now = func() time.Time { return cutoff.Add(time.Second) }
	fresh := s.Create()
	if _, ok := s.Get(fresh.ID); !ok {
		t.Fatal("a session from after the cutoff was voided")
	}
}

// Rotating addresses must neither grow the map without bound nor get a
// fresh bucket per address inside one IPv6 /64.
func TestBackoffIsBoundedAndGroupsIPv6By64(t *testing.T) {
	b := NewBackoff()
	for i := range backoffMaxKeys * 2 {
		b.Fail(fmt.Sprintf("10.%d.%d.%d", i>>16&255, i>>8&255, i&255))
	}
	if n := len(b.failures); n > backoffMaxKeys {
		t.Fatalf("backoff holds %d keys, cap is %d", n, backoffMaxKeys)
	}

	b = NewBackoff()
	b.Fail("2001:db8:1:2::1")
	b.Fail("2001:db8:1:2:ffff::9")
	if d := b.Delay("2001:db8:1:2::abcd"); d != backoffBase<<1 {
		t.Fatalf("delay inside the /64 = %v, want %v after two failures", d, backoffBase<<1)
	}
	if d := b.Delay("2001:db8:1:3::1"); d != 0 {
		t.Fatalf("a different /64 inherited a delay: %v", d)
	}
	if d := b.Delay("192.0.2.1"); d != 0 {
		t.Fatalf("IPv4 caller inherited a delay: %v", d)
	}
}
