package dnsserver

import (
	"sync"
	"time"
)

// histSlots is the hour ring in one-minute buckets; daySlots is the 24-hour
// ring in daySpan-minute buckets. Both are fixed, so memory stays constant.
const (
	histSlots = 60
	daySlots  = 144
	daySpan   = 10
)

// Bucket is one slot of traffic: a minute in the hour ring, daySpan minutes in
// the day ring. Like RefusalStats it is counts only, so a bucket can be served
// to the dashboard whatever the query-logging opt-in says.
type Bucket struct {
	Minute        int64  `json:"minute"` // unix minute the slot starts, i.e. unix seconds / 60
	Authoritative uint64 `json:"authoritative"`
	Forwarded     uint64 `json:"forwarded"`
	Blocked       uint64 `json:"blocked"`
	CacheHits     uint64 `json:"cache_hits"`
	CacheMisses   uint64 `json:"cache_misses"`
	LatencySum    uint64 `json:"latency_sum_ms"`
	LatencyCount  uint64 `json:"latency_count"`
}

// ring is a fixed run of span-minute slots. There is no ticker: a slot carries
// the minute it starts, so a slot whose stamp does not match is stale and reads
// as empty. That means a server left idle overnight cannot wake up graphing
// yesterday's traffic.
type ring struct {
	span  int64
	slots []Bucket
}

// current returns the live slot holding minute, resetting a stale one first.
func (r *ring) current(minute int64) *Bucket {
	start := minute - minute%r.span
	b := &r.slots[(start/r.span)%int64(len(r.slots))]
	if b.Minute != start {
		*b = Bucket{Minute: start}
	}
	return b
}

// snapshot returns every slot ending at the one holding nowMin, oldest first.
// Quiet slots come back as zero samples rather than being omitted, so the x
// axis is evenly spaced and a quiet period draws as a floor.
func (r *ring) snapshot(nowMin int64) []Bucket {
	n := int64(len(r.slots))
	last := nowMin - nowMin%r.span
	out := make([]Bucket, 0, n)
	for i := n - 1; i >= 0; i-- {
		m := last - i*r.span
		if b := r.slots[(m/r.span)%n]; b.Minute == m {
			out = append(out, b)
			continue
		}
		out = append(out, Bucket{Minute: m})
	}
	return out
}

// history keeps the last hour by the minute and the last day by daySpan.
type history struct {
	mu        sync.Mutex
	hour, day ring
	now       func() time.Time
}

func newHistory(now func() time.Time) *history {
	if now == nil {
		now = time.Now
	}
	return &history{
		hour: ring{span: 1, slots: make([]Bucket, histSlots)},
		day:  ring{span: daySpan, slots: make([]Bucket, daySlots)},
		now:  now,
	}
}

// each runs fn on the live slot of both rings. The caller must not hold mu.
func (h *history) each(fn func(*Bucket)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	minute := h.now().Unix() / 60
	fn(h.hour.current(minute))
	fn(h.day.current(minute))
}

func (h *history) add(o outcome, d time.Duration) {
	h.each(func(b *Bucket) {
		switch o {
		case outcomeAuthoritative:
			b.Authoritative++
		case outcomeForwarded:
			b.Forwarded++
		case outcomeBlocked:
			b.Blocked++
		}
		// Refusals and errors are counted in the totals but not graphed: stacked
		// into a rare-event sliver they would be unreadable.
		b.LatencySum += uint64(d.Milliseconds())
		b.LatencyCount++
	})
}

func (h *history) addCache(hit bool) {
	h.each(func(b *Bucket) {
		if hit {
			b.CacheHits++
		} else {
			b.CacheMisses++
		}
	})
}

// snapshot returns the last hour: histSlots one-minute buckets ending at the
// current minute.
func (h *history) snapshot() []Bucket {
	nowMin := h.now().Unix() / 60
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.hour.snapshot(nowMin)
}

// snapshotDay returns the last 24 hours: daySlots buckets of daySpan minutes,
// the newest still filling.
func (h *history) snapshotDay() []Bucket {
	nowMin := h.now().Unix() / 60
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.day.snapshot(nowMin)
}
