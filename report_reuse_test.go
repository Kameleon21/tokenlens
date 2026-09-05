package main

import (
	"context"
	tea "github.com/charmbracelet/bubbletea"
	"testing"
	"time"
)

func TestReportFreshness(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		name     string
		age, ttl time.Duration
		want     bool
	}{
		{"recent", time.Minute, 5 * time.Minute, true},
		{"boundary", 5 * time.Minute, 5 * time.Minute, false},
		{"disabled", 0, 0, false},
		{"future", -time.Second, time.Minute, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := snapshotFresh(Snapshot{Loaded: now.Add(-tc.age)}, tc.ttl, now); got != tc.want {
				t.Fatalf("got %v", got)
			}
		})
	}
	if _, err := options([]string{"--cache-ttl=-1s"}, now); err == nil {
		t.Fatal("accepted negative TTL")
	}
}

func TestRecentReportSkipsBackend(t *testing.T) {
	for _, disk := range []bool{false, true} {
		o, err := options([]string{"--ccusage", "/missing/tokenlens-test-backend", "--cache-dir", t.TempDir()}, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		m := newModel(context.Background(), o)
		s := Snapshot{Loaded: time.Now(), Sections: map[string][]Row{"daily": {{Name: "test"}}}}
		if disk {
			if err := writeSnapshotCache(o, o.Range, s); err != nil {
				t.Fatal(err)
			}
		} else {
			m.remember(o.Range, s)
		}
		batch := m.refresh(o.Range)().(tea.BatchMsg)
		start := time.Now()
		msg := batch[len(batch)-1]()
		reused, ok := msg.(reusedMsg)
		if !ok {
			t.Fatalf("expected cached report without backend; got %T", msg)
		}
		next, _ := m.Update(reused)
		n := next.(model)
		if n.loading || !n.cached || n.s.Sections["daily"][0].Name != "test" {
			t.Fatal("report not displayed")
		}
		t.Logf("disk=%v report reuse: %s", disk, time.Since(start))
		// The same report must not complete a forced request from cache.
		batch = n.refresh(o.Range, true)().(tea.BatchMsg)
		if _, ok := batch[len(batch)-1]().(reusedMsg); ok {
			t.Fatal("forced refresh reused report")
		}
		n.cancel()
	}
}

func TestRefreshInFlightAndCurrencyIsolation(t *testing.T) {
	m := fixtureModel()
	m.loading = true
	m.pending = m.o.Range
	m.request = 7
	if cmd := m.refresh(m.pending, true); cmd != nil || m.request != 7 {
		t.Fatal("duplicate refresh restarted load")
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	n := next.(model)
	if n.request != 7 || !n.loading || n.o.Currency == m.o.Currency {
		t.Fatal("currency changed usage request")
	}
	if n.fxCancel != nil {
		n.fxCancel()
	}
	next, _ = n.Update(reusedMsg{Snapshot{}, Range{}, 6})
	if next.(model).request != 7 || !next.(model).loading {
		t.Fatal("stale cache response accepted")
	}
}

func TestReportMemoryBoundAndNoCache(t *testing.T) {
	m := newModel(context.Background(), Options{})
	for i := 0; i < 20; i++ {
		m.remember(Range{Since: time.Unix(int64(i), 0).String()}, Snapshot{Loaded: time.Unix(int64(i+1), 0)})
	}
	if len(m.reports) != 16 {
		t.Fatal("memory cache not bounded")
	}
	m.o.NoCache = true
	m.remember(Range{Since: "disabled"}, Snapshot{})
	if _, ok := m.reports[Range{Since: "disabled"}]; ok {
		t.Fatal("no-cache ignored")
	}
}
