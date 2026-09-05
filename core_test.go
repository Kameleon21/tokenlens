package main

import (
	"context"
	"encoding/json"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRanges(t *testing.T) {
	loc, _ := time.LoadLocation("America/New_York")
	now := time.Date(2026, 3, 1, 1, 0, 0, 0, time.UTC)
	cases := []struct {
		s, u, g string
		n       int
		want    Range
		bad     bool
	}{
		{"", "", "daily", 0, Range{"2026-02-01", "2026-02-28"}, false},
		{"20240229", "", "daily", 0, Range{"2024-02-29", ""}, false},
		{"", "2024-02-29", "weekly", 0, Range{"", "2024-02-29"}, false},
		{"", "", "daily", 2, Range{"2026-02-27", "2026-02-28"}, false},
		{"", "", "weekly", 1, Range{"2026-02-23", "2026-02-28"}, false},
		{"", "", "monthly", 3, Range{"2025-12-01", "2026-02-28"}, false},
		{"20260230", "", "daily", 0, Range{}, true}, {"2026-2-01", "", "daily", 0, Range{}, true}, {"20260301", "20260201", "daily", 0, Range{}, true}, {"20260301", "", "daily", 1, Range{}, true}, {"", "", "daily", -1, Range{}, true},
	}
	for _, c := range cases {
		r, e := resolveRange(c.s, c.u, c.n, c.g, now, loc)
		if (e != nil) != c.bad || !c.bad && r != c.want {
			t.Errorf("%+v: got %+v, %v", c, r, e)
		}
	}
	dst := time.Date(2026, 3, 9, 12, 0, 0, 0, loc)
	r, e := resolveRange("", "", 2, "daily", dst, loc)
	if e != nil || r.Since != "2026-03-08" {
		t.Fatalf("DST range: %+v %v", r, e)
	}
}
func TestOptions(t *testing.T) {
	for _, args := range [][]string{{"--last", "0"}, {"--last", "2", "--since", "20260101"}, {"--timezone", "garbage"}, {"yearly"}, {"--since", "20260229"}} {
		if _, e := options(args, time.Now()); e == nil {
			t.Errorf("accepted %v", args)
		}
	}
	o, e := options([]string{"weekly", "--last", "2", "--timezone", "UTC"}, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if e != nil || o.Range.Since != "2025-12-22" {
		t.Fatalf("%+v %v", o, e)
	}
}

const sample = `{"daily":[{"agent":"all","period":"2026-09-01","totalTokens":180,"totalCost":3,"agents":[{"agent":"future-agent","totalTokens":100,"totalCost":2,"modelBreakdowns":[{"modelName":"shared-model","inputTokens":40,"outputTokens":10,"cacheReadTokens":40,"cacheCreationTokens":10,"cost":2}]},{"agent":"other","totalTokens":80,"totalCost":1,"modelBreakdowns":[{"modelName":"shared-model","inputTokens":30,"outputTokens":10,"cacheReadTokens":30,"cacheCreationTokens":10,"cost":1}]}],"modelBreakdowns":[{"modelName":"shared-model","inputTokens":70,"outputTokens":20,"cacheReadTokens":70,"cacheCreationTokens":20,"cost":3}]}],"weekly":[],"monthly":[],"session":[]}`

func TestParseAndFilters(t *testing.T) {
	s, e := parseSnapshot([]byte(sample))
	if e != nil {
		t.Fatal(e)
	}
	daily := s.Sections["daily"]
	r := filtered(daily, "future-agent", "shared-model")
	if len(r) != 1 || r[0].Usage.Tokens.Value != 100 || r[0].Usage.Cost.Value != 2 {
		t.Fatalf("intersection: %+v", r)
	}
	if len(filtered(daily, "unknown", "")) != 0 {
		t.Fatal("unknown agent matches")
	}
	a := rank(daily, "agents", "", "shared-model")
	if total(a).Cost.Value != 3 || len(a) != 2 {
		t.Fatalf("double counted: %+v", a)
	}
	if total(daily).Tokens.Value != 180 {
		t.Fatal("wrong total")
	}
	if len(names(s, "agent")) != 3 {
		t.Fatal("dynamic agents missing")
	}
}
func TestMissingMetrics(t *testing.T) {
	r, e := parseRow(json.RawMessage(`{"period":"a","inputTokens":0,"totalCost":null}`))
	if e != nil {
		t.Fatal(e)
	}
	if !r.Usage.Input.Known || r.Usage.Cost.Known || r.Usage.Tokens.Known {
		t.Fatal("missing metric became zero")
	}
	u := Usage{}
	u.add(r.Usage)
	u.add(Usage{Cost: known(1)})
	if !u.Cost.Partial || format(u.Cost, true) != "$1.0000 + ?" {
		t.Fatalf("partial: %+v", u.Cost)
	}
	if format(r.Usage.Cost, true) != "unavailable" {
		t.Fatal("missing cost label")
	}
}
func TestRejectOldShape(t *testing.T) {
	for _, s := range []string{`{"daily":[]}`, `no json`, `{"daily":[{"date":"2026-01-01"}],"weekly":[],"monthly":[],"session":[]}`} {
		if _, e := parseSnapshot([]byte(s)); e == nil {
			t.Fatal("unsupported shape accepted")
		}
	}
}
func TestBackendArgs(t *testing.T) {
	a := strings.Join(backendArgs(Range{"2026-01-01", ""}, "Pacific/Auckland"), " ")
	if strings.Contains(a, "--last") || strings.Contains(a, "--until") || !strings.Contains(a, "--timezone Pacific/Auckland") || !strings.Contains(a, "--sections daily,weekly,monthly,session") {
		t.Fatal(a)
	}
}
func TestMissingBackend(t *testing.T) {
	_, e := load(context.Background(), "/this/does/not/exist", Range{}, "UTC")
	if e == nil || !strings.Contains(e.Error(), "--demo") {
		t.Fatal(e)
	}
}
func TestDemoAggregates(t *testing.T) {
	s := demo(Range{"2026-09-01", "2026-09-30"}, time.UTC)
	daily := total(s.Sections["daily"])
	for _, g := range []string{"weekly", "monthly", "session"} {
		u := total(s.Sections[g])
		if u.Tokens.Value != daily.Tokens.Value {
			t.Fatalf("%s mismatch", g)
		}
	}
	for _, g := range []string{"daily", "weekly", "monthly"} {
		u := total(filtered(s.Sections[g], "codex", "gpt-5"))
		if u.Tokens.Value != total(filtered(s.Sections["daily"], "codex", "gpt-5")).Tokens.Value {
			t.Fatal("filter mismatch", g)
		}
	}
}
func fixtureModel() model {
	o := Options{Group: "daily", TZ: "UTC", Demo: true, Range: Range{"2026-09-01", "2026-09-30"}}
	m := newModel(context.Background(), o)
	m.s = demo(o.Range, time.UTC)
	return m
}
func key(m model, s string) model {
	v, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
	return v.(model)
}
func TestNavigationAndLayout(t *testing.T) {
	original := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(original)
	m := fixtureModel()
	m = key(m, "a")
	if m.agent == "" || m.modelFilter != "" {
		t.Fatal("filters conflated")
	}
	m = key(m, "f")
	if m.modelFilter == "" {
		t.Fatal("no model filter")
	}
	m = key(m, "x")
	m = key(m, "3")
	if m.view != 2 {
		t.Fatal("view key")
	}
	m = key(m, "c")
	if m.cost {
		t.Fatal("metric toggle")
	}
	m = key(m, "w")
	if m.o.Group != "weekly" {
		t.Fatal("group key")
	}
	for _, wh := range [][2]int{{50, 16}, {80, 24}, {120, 40}, {160, 48}} {
		m.width, m.height = wh[0], wh[1]
		for view := 0; view < 5; view++ {
			m.view = view
			text := ansi.Strip(m.View())
			if len(strings.Split(text, "\n")) > m.height {
				t.Fatalf("vertical overflow %v", wh)
			}
			for _, line := range strings.Split(text, "\n") {
				if ansi.StringWidth(line) > m.width {
					t.Fatalf("width overflow %v: %q", wh, line)
				}
			}
		}
	}
}
func TestStaleResultIgnored(t *testing.T) {
	m := fixtureModel()
	m.request = 2
	m.loading = true
	v, _ := m.Update(loadedMsg{id: 1, err: context.Canceled})
	n := v.(model)
	if !n.loading || n.err != "" {
		t.Fatal("stale result corrupted current load")
	}
}
func TestRealSnapshotOptIn(t *testing.T) {
	p := os.Getenv("TOKENLENS_TEST_SNAPSHOT")
	if p == "" {
		t.Skip("optional local integration fixture")
	}
	b, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	s, e := parseSnapshot(b)
	if e != nil {
		t.Fatal(e)
	}
	t.Logf("parsed %d daily rows and %d sessions", len(s.Sections["daily"]), len(s.Sections["session"]))
}
