package app

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestTimestampFormatsAndTimezone(t *testing.T) {
	m := newModel(context.Background(), Options{TZ: "Europe/Dublin", preferences: defaultPreferences()})
	instant, _ := time.Parse(time.RFC3339Nano, "2026-09-06T13:35:42.123456789Z")
	for _, tc := range []struct{ date, clock, want string }{
		{"european", "24h", "14:35 · 6 Sep 2026"}, {"us", "24h", "14:35 · Sep 6, 2026"}, {"iso", "24h", "14:35 · 2026-09-06"},
		{"european", "12h", "2:35 PM · 6 Sep 2026"}, {"us", "12h", "2:35 PM · Sep 6, 2026"}, {"iso", "12h", "2:35 PM · 2026-09-06"},
	} {
		m.o.preferences.DateFormat, m.o.preferences.ClockFormat = tc.date, tc.clock
		if got := m.formatTimestamp(instant); got != tc.want {
			t.Fatalf("%+v got %q", tc, got)
		}
	}
	m.o.preferences = defaultPreferences()
	for _, tc := range []struct{ input, want string }{
		{"2026-09-06T23:35:00Z", "00:35 · 7 Sep 2026"},
		{"2026-03-29T00:59:00Z", "00:59 · 29 Mar 2026"},
		{"2026-03-29T01:00:00Z", "02:00 · 29 Mar 2026"},
	} {
		v, _ := time.Parse(time.RFC3339, tc.input)
		if got := m.formatTimestamp(v); got != tc.want {
			t.Fatalf("%s got %s", tc.input, got)
		}
	}
	m.o.preferences.ClockFormat = "12h"
	for _, tc := range []struct{ input, want string }{
		{"2026-01-01T00:00:00Z", "12:00 AM · 1 Jan 2026"}, {"2026-01-01T12:00:00Z", "12:00 PM · 1 Jan 2026"},
	} {
		v, _ := time.Parse(time.RFC3339, tc.input)
		if got := m.formatTimestamp(v); got != tc.want {
			t.Fatal(got)
		}
	}
	if m.formatTimestamp(time.Time{}) != "unavailable" {
		t.Fatal("zero timestamp shown as a date")
	}
}

func sessionRow(t *testing.T, name, stamp string, cost Metric) Row {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{"period": name, "metadata": map[string]any{"firstActivity": stamp, "lastActivity": stamp}})
	r, err := parseRow(raw)
	if err != nil {
		t.Fatal(err)
	}
	r.Usage.Cost = cost
	return r
}
func rowNames(rows []Row) []string {
	out := []string{}
	for _, r := range rows {
		out = append(out, r.Name)
	}
	return out
}
func TestTabSorting(t *testing.T) {
	original := []Row{
		sessionRow(t, "Missing", "invalid", Metric{}),
		sessionRow(t, "Beta", "2026-09-06T10:00:00+02:00", known(4)),
		sessionRow(t, "alpha", "2026-09-06T08:30:00Z", known(4)),
		sessionRow(t, "Zero", "", known(0)),
	}
	for _, tc := range []struct {
		sort string
		want []string
	}{
		{"cost_desc", []string{"Beta", "alpha", "Zero", "Missing"}},
		{"cost_asc", []string{"Zero", "Beta", "alpha", "Missing"}},
		{"name_asc", []string{"alpha", "Beta", "Missing", "Zero"}},
		{"name_desc", []string{"Zero", "Missing", "Beta", "alpha"}},
		{"newest", []string{"alpha", "Beta", "Missing", "Zero"}},
		{"oldest", []string{"Beta", "alpha", "Missing", "Zero"}},
	} {
		rows := append([]Row(nil), original...)
		sortRows(rows, tc.sort)
		if got := rowNames(rows); !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%s: %v", tc.sort, got)
		}
	}
	// Filtering keeps parsed times, including when selecting nested agents.
	parent := original[1]
	parent.Agents = []Row{{Agent: "codex", Usage: Usage{Cost: known(4)}}}
	child := filtered([]Row{parent}, "codex", "")[0]
	if !child.firstActivity.Equal(parent.firstActivity) {
		t.Fatal("filter lost timestamp")
	}
	m := newModel(context.Background(), Options{Group: "daily"})
	m.view = 4
	m.s.Sections = map[string][]Row{"session": original}
	m.o.preferences.SessionsSort = "cost_asc"
	for _, costDisplay := range []bool{true, false} {
		m.cost = costDisplay
		if got := rowNames(m.rows()); !reflect.DeepEqual(got, []string{"Zero", "Beta", "alpha", "Missing"}) {
			t.Fatal(got)
		}
	}
	if original[0].Name != "Missing" {
		t.Fatal("sort mutated snapshot")
	}
	// Equal names across agents have a deterministic order in either direction.
	rows := []Row{{Name: "same", Agent: "z"}, {Name: "same", Agent: "a"}}
	sortRows(rows, "cost_desc")
	if rows[0].Agent != "a" {
		t.Fatal("unstable ties")
	}
	partial := []Row{{Name: "missing"}, {Name: "partial", Usage: Usage{Cost: Metric{Value: 0, Known: true, Partial: true}}}}
	sortRows(partial, "cost_asc")
	if partial[0].Name != "partial" {
		t.Fatal("known partial cost treated as missing")
	}
	// Aggregation starts from a map; equal-cost model order must remain stable.
	m.view = 2
	m.o.preferences.ModelsSort = "cost_asc"
	m.s.Sections["daily"] = []Row{{Models: []Row{{Name: "z", Usage: Usage{Cost: known(1)}}, {Name: "a", Usage: Usage{Cost: known(1)}}, {Name: "unknown"}}}}
	for i := 0; i < 20; i++ {
		if got := rowNames(m.rows()); !reflect.DeepEqual(got, []string{"a", "z", "unknown"}) {
			t.Fatal(got)
		}
	}
}

func TestPresentationPreferencesAndLocalControls(t *testing.T) {
	path := isolatedPreferences(t)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	// Existing config files inherit all four new defaults.
	if err := os.WriteFile(path, []byte("currency = 'EUR'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	p, err := readPreferences(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.DateFormat != "european" || p.ClockFormat != "24h" || p.ModelsSort != "cost_desc" || p.SessionsSort != "cost_desc" {
		t.Fatal(p)
	}
	o, err := options([]string{"--demo"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	m := newModel(context.Background(), o)
	m.request = 12
	m.fxRequest = 9
	m.loading = true
	before, _ := snapshotCachePath(m.o, m.o.Range)
	for _, key := range []string{"3", "s", "5", "s", "s", "s", "s", "D", "H"} {
		next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
		m = next.(model)
		// Tab presses can forward spinner events; presentation controls must do no work.
		if key != "3" && key != "5" && cmd != nil {
			t.Fatalf("%s returned a command", key)
		}
	}
	if m.request != 12 || m.fxRequest != 9 || !m.loading {
		t.Fatal("presentation triggered loading")
	}
	after, _ := snapshotCachePath(m.o, m.o.Range)
	if before != after {
		t.Fatal("presentation invalidated report cache")
	}
	o, err = options(nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	restored := newModel(context.Background(), o)
	p = restored.o.preferences
	if p.ModelsSort != "cost_asc" || p.SessionsSort != "newest" || p.DateFormat != "us" || p.ClockFormat != "12h" || p.Currency != "EUR" {
		t.Fatal(p)
	}
	for _, bad := range []string{"date_format = 'other'", "clock_format = '13h'", "models_sort = 'newest'", "sessions_sort = 'random'"} {
		if err := os.WriteFile(path, []byte(bad), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readPreferences(path); err == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
}

func TestTimestampCacheAndExports(t *testing.T) {
	m := newModel(context.Background(), Options{TZ: "Europe/Dublin", Group: "daily", CacheDir: t.TempDir(), ExportDir: t.TempDir()})
	m.view = 4
	m.o.preferences.DateFormat = "us"
	m.o.preferences.ClockFormat = "12h"
	r := sessionRow(t, "example", "2026-09-06T13:35:42.123456789+02:00", known(0))
	m.s = Snapshot{Loaded: time.Now(), Sections: map[string][]Row{"session": {r}}}
	if err := writeSnapshotCache(m.o, m.o.Range, m.s); err != nil {
		t.Fatal(err)
	}
	cached, err := readSnapshotCache(m.o, m.o.Range)
	if err != nil {
		t.Fatal(err)
	}
	if !cached.Sections["session"][0].firstActivity.Equal(r.firstActivity) {
		t.Fatal("disk cache lost derived time")
	}
	want := "2026-09-06T11:35:42.123456789Z"
	for _, kind := range []string{"json", "csv"} {
		path, err := m.writeExport(kind)
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "11:35 AM") || !strings.Contains(string(data), want) {
			t.Fatalf("%s lost precision: %s", kind, data)
		}
		if kind == "csv" {
			records, err := csv.NewReader(strings.NewReader(string(data))).ReadAll()
			if err != nil {
				t.Fatal(err)
			}
			cols := map[string]int{}
			for i, key := range records[0] {
				cols[key] = i
			}
			if records[1][cols["first_activity"]] != want || records[1][cols["estimated_cost_usd"]] != "0" {
				t.Fatal(records)
			}
		}
	}
	// Raw metadata and parsed times remain independent of formatting choices.
	if got := m.metadataValue(r, "firstActivity"); got != "12:35 PM · Sep 6, 2026" {
		t.Fatal(got)
	}
	if !strings.Contains(string(r.Metadata["firstActivity"]), ".123456789+02:00") {
		t.Fatal("presentation rewrote raw metadata")
	}
}

func TestSortControlsAndTimestampLayouts(t *testing.T) {
	m := newModel(context.Background(), Options{TZ: "Europe/Dublin", Group: "daily"})
	m.s = Snapshot{Loaded: time.Date(2026, 9, 6, 13, 35, 0, 0, time.UTC), Sections: map[string][]Row{"session": {sessionRow(t, "sample", "2026-09-06T13:35:00Z", known(1))}}}
	m.view = 4
	for _, size := range [][2]int{{50, 16}, {80, 24}, {100, 32}, {150, 45}} {
		m.width, m.height = size[0], size[1]
		view := ansi.Strip(m.View())
		if !strings.Contains(view, "[s] Cost") {
			t.Fatalf("sort hidden at %v:\n%s", size, view)
		}
		if !strings.Contains(view, "14:35 · 6 Sep 2026") {
			t.Fatalf("timestamp wrong at %v:\n%s", size, view)
		}
		if lipgloss.Width(view) > m.width || lipgloss.Height(view) > m.height {
			t.Fatalf("overflow at %v", size)
		}
		m.help = true
		help := ansi.Strip(m.View())
		for _, word := range []string{"Sessions also", "D  Date:", "H  Clock:"} {
			if !strings.Contains(help, word) {
				t.Fatalf("missing %s at %v:\n%s", word, size, help)
			}
		}
		m.help = false
	}
}

func BenchmarkSessionSort(b *testing.B) {
	rows := make([]Row, 10000)
	for i := range rows {
		rows[i] = Row{Name: "session", Agent: "codex", firstActivity: time.Unix(int64(i), 0), Usage: Usage{Cost: known(float64(i % 100))}}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		copyRows := append([]Row(nil), rows...)
		sortRows(copyRows, "newest")
	}
}

func TestChronologicalSortPreservesSubMinutePrecision(t *testing.T) {
	m := newModel(context.Background(), Options{TZ: "America/New_York", Group: "daily"})
	m.view = 4
	m.o.preferences.SessionsSort = "newest"
	m.s.Sections = map[string][]Row{"session": {
		sessionRow(t, "a-older", "2026-09-06T13:35:42.000000001Z", known(1)),
		sessionRow(t, "z-newer", "2026-09-06T13:35:42.000000002Z", known(1)),
	}}
	for _, date := range []string{"european", "us", "iso"} {
		for _, clock := range []string{"12h", "24h"} {
			m.o.preferences.DateFormat, m.o.preferences.ClockFormat = date, clock
			rows := m.rows()
			if rows[0].Name != "z-newer" {
				t.Fatal("sort used display precision")
			}
			if m.formatTimestamp(rows[0].firstActivity) != m.formatTimestamp(rows[1].firstActivity) {
				t.Fatal("fixture must display identical minute")
			}
		}
	}
}

func TestHelpScrollsAndKeepsTabSelection(t *testing.T) {
	m := newModel(context.Background(), Options{})
	m.width, m.height, m.view, m.cursor = 50, 16, 4, 3
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = next.(model)
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	m = next.(model)
	if cmd != nil || m.helpOffset == 0 || m.cursor != 3 || m.view != 4 {
		t.Fatal("help navigation changed tab or requested work")
	}
	if !strings.Contains(ansi.Strip(m.View()), "Quit") {
		t.Fatal("last help controls inaccessible")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(model)
	if m.help || m.cursor != 3 {
		t.Fatal("help did not close cleanly")
	}
}
