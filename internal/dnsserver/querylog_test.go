package dnsserver

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Busnes-app/kydns-server/internal/store"
	"github.com/Busnes-app/kydns-server/internal/zone"
)

func entries(l *QueryLog, f QueryFilter) []QueryEntry {
	var out []QueryEntry
	l.Each(f, func(e QueryEntry) bool { out = append(out, e); return true })
	return out
}

// The in-memory log holds only what the operator opted into: nothing with
// query logging off, no client without log_client_ip, and nothing at all once
// either is turned off again.
func TestQueryLogFollowsTheLoggingOptIns(t *testing.T) {
	h := zone.NewHolder(func() (zone.Input, error) {
		return zone.Input{Zone: "home.arpa.", Services: []store.Service{{ID: 1, Name: "nas", Addresses: []store.Address{{Address: "192.168.1.30"}}}}}, nil
	}, nil)
	if err := h.Rebuild(); err != nil {
		t.Fatal(err)
	}
	s := New(Options{Holder: h, ACL: NewACL(prefixes(t, "0.0.0.0/0")), Auth: NewAuthoritative("home.arpa.", 60, nil),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})

	s.query(t, "nas.home.arpa.", "192.168.1.5")
	if n := len(entries(s.QueryLog(), QueryFilter{})); n != 0 {
		t.Fatalf("%d entries with query logging off", n)
	}
	s.SetLogging(true, false)
	s.query(t, "nas.home.arpa.", "192.168.1.5")
	got := entries(s.QueryLog(), QueryFilter{})
	if len(got) != 1 || got[0].Name != "nas.home.arpa" || got[0].Client != "" || got[0].Policy != "local" {
		t.Fatalf("with query logging only: %+v", got)
	}
	s.SetLogging(true, true)
	s.query(t, "nas.home.arpa.", "192.168.1.5")
	if got := entries(s.QueryLog(), QueryFilter{Client: "192.168.1.5"}); len(got) != 1 {
		t.Fatalf("with client IP logging: %+v", got)
	}
	s.SetLogging(true, false)
	if n := len(entries(s.QueryLog(), QueryFilter{})); n != 0 {
		t.Errorf("%d entries survived turning client IP logging off", n)
	}
	s.query(t, "nas.home.arpa.", "192.168.1.5")
	s.SetLogging(false, false)
	if n := len(entries(s.QueryLog(), QueryFilter{})); n != 0 {
		t.Errorf("%d entries survived turning query logging off", n)
	}
}

// The ring keeps the newest entries and filters by suffix at label boundaries.
func TestQueryLogRingAndFilters(t *testing.T) {
	l := NewQueryLog(3)
	at := time.Unix(1000, 0)
	for i, name := range []string{"a.example", "b.example", "badexample", "c.example"} {
		l.Add(QueryEntry{Time: at.Add(time.Duration(i) * time.Second), Name: name})
	}
	var names []string
	oldest := l.Each(QueryFilter{}, func(e QueryEntry) bool { names = append(names, e.Name); return true })
	if len(names) != 3 || names[0] != "c.example" || names[2] != "b.example" || !oldest.Equal(at.Add(time.Second)) {
		t.Fatalf("ring = %v oldest %v", names, oldest)
	}
	if got := entries(l, QueryFilter{Domain: "Example."}); len(got) != 2 {
		t.Errorf("suffix filter matched %+v; badexample must not match example", got)
	}
}
