package rdnss

import (
	"testing"
	"time"
)

func TestInterval(t *testing.T) {
	for sent := 0; sent < 6; sent++ {
		for _, r := range []float64{0, 0.5, 0.999} {
			d := interval(sent, r)
			lo, hi := MinInterval, MaxInterval
			if sent < initialCount {
				lo, hi = initialMax, initialMax // 200 s floor, capped at 16 s
			}
			if d < lo || d > hi {
				t.Errorf("sent=%d r=%v: %v outside [%v, %v]", sent, r, d, lo, hi)
			}
		}
	}
	if Lifetime != 1800 {
		t.Errorf("lifetime %d, want 3 x MaxRtrAdvInterval", Lifetime)
	}
}

// TestSolicitFloodRateLimited: however many solicitations arrive, replies are
// at least 3 s apart and never later than the scheduled advertisement.
func TestSolicitFloodRateLimited(t *testing.T) {
	t0 := time.Unix(1_800_000_000, 0)
	last := t0
	next := t0.Add(MaxInterval)
	for i := range 1000 {
		next = afterSolicit(t0.Add(time.Duration(i)*time.Millisecond), last, next, 100*time.Millisecond)
	}
	if got := next.Sub(last); got != minBetweenRAs {
		t.Fatalf("reply %v after the last RA, want %v", got, minBetweenRAs)
	}
	// A quiet link: the reply waits only the jitter.
	long := t0.Add(-time.Hour)
	if got := afterSolicit(t0, long, t0.Add(MaxInterval), 200*time.Millisecond); got != t0.Add(200*time.Millisecond) {
		t.Fatalf("quiet reply at %v", got)
	}
	// Due sooner anyway: unchanged.
	if got := afterSolicit(t0, long, t0.Add(10*time.Millisecond), 200*time.Millisecond); got != t0.Add(10*time.Millisecond) {
		t.Fatalf("scheduled RA moved to %v", got)
	}
}
