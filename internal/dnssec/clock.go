package dnssec

import (
	"log/slog"
	"runtime/debug"
	"sync"
	"time"
)

// buildFloorConst is the earliest the clock may read when the binary carries
// no VCS time (a build outside git). Moving it forward is optional.
var buildFloorConst = time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)

// buildFloor is the later of the embedded commit time and buildFloorConst.
func buildFloor() time.Time {
	floor := buildFloorConst
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			if s.Key == "vcs.time" {
				if t, err := time.Parse(time.RFC3339, s.Value); err == nil && t.After(floor) {
					floor = t
				}
			}
		}
	}
	return floor
}

// Clock decides whether signature validity periods can be trusted. While the
// clock reads earlier than the build, or the kernel says NTP has not synced,
// time checks are skipped (dnsmasq's dnssec-no-timecheck): otherwise a Pi with
// no RTC rejects every signature and NTP can never resolve its servers.
type Clock struct {
	now      func() time.Time
	floor    time.Time
	unsynced func() (unsynced, known bool)
	log      *slog.Logger

	mu      sync.Mutex
	checked time.Time
	skip    bool
	reason  string
}

// NewClock reads the system clock and, on Linux, adjtimex(2).
func NewClock(log *slog.Logger) *Clock {
	if log == nil {
		log = slog.Default()
	}
	return &Clock{now: time.Now, floor: buildFloor(), unsynced: kernelUnsynced, log: log}
}

// clockRecheck bounds how often the state is re-read.
const clockRecheck = 10 * time.Second

// Skip reports whether time checks are off and why. It logs once per change.
func (c *Clock) Skip() (bool, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if !c.checked.IsZero() && now.Sub(c.checked) < clockRecheck && !now.Before(c.checked) {
		return c.skip, c.reason
	}
	c.checked = now
	reason := ""
	if now.Before(c.floor) {
		reason = "the system clock is earlier than this build"
	} else if u, known := c.unsynced(); known && u {
		reason = "the kernel reports the clock is not synchronised (NTP)"
	}
	skip := reason != ""
	if skip != c.skip {
		if skip {
			c.log.Warn("dnssec time checks skipped", "reason", reason,
				"fix", "they resume by themselves once NTP has set the clock")
		} else {
			c.log.Info("dnssec time checks resumed")
		}
	}
	c.skip, c.reason = skip, reason
	return skip, reason
}
