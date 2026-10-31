package dnsserver

import (
	"testing"
	"time"
)

// atMinute is a clock pinned to the start of minute n of an arbitrary hour.
func atMinute(base time.Time, n int) time.Time {
	return base.Add(time.Duration(n) * time.Minute)
}

func TestHistoryBucketsByMinute(t *testing.T) {
	base := time.Unix(1_700_000_000, 0).Truncate(time.Minute)
	now := base
	h := newHistory(func() time.Time { return now })

	h.add(outcomeAuthoritative, 5*time.Millisecond)
	now = atMinute(base, 1)
	h.add(outcomeForwarded, 5*time.Millisecond)
	h.add(outcomeForwarded, 5*time.Millisecond)

	got := h.snapshot()
	if n := len(got); n != histSlots {
		t.Fatalf("snapshot has %d buckets, want %d", n, histSlots)
	}
	last := got[histSlots-1]
	if last.Forwarded != 2 {
		t.Errorf("current minute Forwarded = %d, want 2", last.Forwarded)
	}
	prev := got[histSlots-2]
	if prev.Authoritative != 1 {
		t.Errorf("previous minute Authoritative = %d, want 1", prev.Authoritative)
	}
}

// The ring is lazily reset, so a slot last written an hour ago must read as an
// empty minute. Reporting hour-old traffic as current is the one failure that
// would make the graph actively lie.
func TestHistoryStaleSlotReadsAsEmpty(t *testing.T) {
	base := time.Unix(1_700_000_000, 0).Truncate(time.Minute)
	now := base
	h := newHistory(func() time.Time { return now })

	h.add(outcomeForwarded, time.Millisecond)
	now = atMinute(base, histSlots) // exactly one full lap later

	got := h.snapshot()
	for i, b := range got {
		if b.Forwarded != 0 {
			t.Fatalf("bucket %d still reports %d forwarded a full lap later", i, b.Forwarded)
		}
	}
	if last := got[histSlots-1]; last.Minute != now.Unix()/60 {
		t.Errorf("last bucket minute = %d, want %d", last.Minute, now.Unix()/60)
	}
}

// A gap in traffic is real information: those minutes must come back as zero
// samples, not be dropped so the chart interpolates straight through them.
func TestHistoryGapIsZeroNotMissing(t *testing.T) {
	base := time.Unix(1_700_000_000, 0).Truncate(time.Minute)
	now := base
	h := newHistory(func() time.Time { return now })

	h.add(outcomeForwarded, time.Millisecond)
	now = atMinute(base, 3)
	h.add(outcomeForwarded, time.Millisecond)

	got := h.snapshot()
	for i := histSlots - 3; i < histSlots-1; i++ {
		if got[i].Forwarded != 0 {
			t.Errorf("bucket %d = %d, want a zero sample", i, got[i].Forwarded)
		}
	}
	if got[histSlots-4].Forwarded != 1 {
		t.Errorf("the older sample was lost: %+v", got[histSlots-4])
	}
	// Every minute is present and strictly increasing, so the x axis is even.
	for i := 1; i < len(got); i++ {
		if got[i].Minute != got[i-1].Minute+1 {
			t.Fatalf("bucket %d minute %d does not follow %d", i, got[i].Minute, got[i-1].Minute)
		}
	}
}

func TestHistoryCountsCacheHitsPerMinute(t *testing.T) {
	base := time.Unix(1_700_000_000, 0).Truncate(time.Minute)
	now := base
	h := newHistory(func() time.Time { return now })

	h.addCache(true)
	h.addCache(true)
	h.addCache(false)

	last := h.snapshot()[histSlots-1]
	if last.CacheHits != 2 || last.CacheMisses != 1 {
		t.Errorf("hits/misses = %d/%d, want 2/1", last.CacheHits, last.CacheMisses)
	}
}

// The day ring files traffic by ten minutes, aligned to the clock, so 10:07 and
// 10:03 share a bar and 10:10 starts the next.
func TestHistoryDayBucketsByTenMinutes(t *testing.T) {
	base := time.Unix(1_700_000_000, 0).Truncate(10 * time.Minute)
	now := atMinute(base, 3)
	h := newHistory(func() time.Time { return now })

	h.add(outcomeBlocked, time.Millisecond)
	now = atMinute(base, 7)
	h.add(outcomeBlocked, time.Millisecond)
	now = atMinute(base, 10)
	h.add(outcomeForwarded, time.Millisecond)

	got := h.snapshotDay()
	if n := len(got); n != daySlots {
		t.Fatalf("day has %d buckets, want %d", n, daySlots)
	}
	if prev := got[daySlots-2]; prev.Blocked != 2 || prev.Minute != base.Unix()/60 {
		t.Errorf("previous bucket = %+v, want 2 blocked starting at minute %d", prev, base.Unix()/60)
	}
	if last := got[daySlots-1]; last.Forwarded != 1 || last.Blocked != 0 {
		t.Errorf("current bucket = %+v, want 1 forwarded only", last)
	}
	// 24 hours, evenly spaced, so the x axis is honest.
	for i := 1; i < len(got); i++ {
		if got[i].Minute != got[i-1].Minute+daySpan {
			t.Fatalf("bucket %d minute %d does not follow %d", i, got[i].Minute, got[i-1].Minute)
		}
	}
	if span := got[daySlots-1].Minute - got[0].Minute + daySpan; span != 24*60 {
		t.Errorf("day spans %d minutes, want %d", span, 24*60)
	}
}

// A day-old slot must not be reported as today's traffic.
func TestHistoryDayStaleSlotReadsAsEmpty(t *testing.T) {
	base := time.Unix(1_700_000_000, 0).Truncate(10 * time.Minute)
	now := base
	h := newHistory(func() time.Time { return now })

	h.add(outcomeForwarded, time.Millisecond)
	now = base.Add(24 * time.Hour) // exactly one lap later

	for i, b := range h.snapshotDay() {
		if b.Forwarded != 0 {
			t.Fatalf("bucket %d still reports %d forwarded a day later", i, b.Forwarded)
		}
	}
}
