package app

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestComparisonCalendarAndFilters(t *testing.T) {
	m := fixtureModel()
	m.s.Sections["daily"] = []Row{
		{Name: "2026-03-28", Usage: Usage{Cost: known(99)}},
		{Name: "2026-03-29", Agents: []Row{{Agent: "codex", Models: []Row{{Name: "small", Usage: Usage{Cost: known(3)}}}}}},
		{Name: "2026-03-23", Usage: Usage{Cost: known(7)}},
	}
	m.displayLocation, _ = time.LoadLocation("Europe/Dublin")
	m.agent, m.modelFilter = "codex", "small"
	r, date, ok := m.comparisonBaseline("2026-03-30")
	if !ok || date != "2026-03-29" || r.Usage.Cost.Value != 3 {
		t.Fatalf("filtered baseline: %s %+v", date, r)
	}
	if _, _, ok = m.comparisonBaseline("2026-03-29"); ok {
		t.Fatal("missing filtered day treated as zero")
	}
	m.agent, m.modelFilter = "", ""
	m.compareWeekday = true
	if _, date, ok = m.comparisonBaseline("2026-03-30"); !ok || date != "2026-03-23" {
		t.Fatal(date, ok)
	}
	m.compareWeekday = false
	if _, date, ok = m.comparisonBaseline("2026-04-01"); ok || date != "2026-03-31" {
		t.Fatal("used nearest row", date, ok)
	}
	m.compareDate = "2026-03-28"
	if _, date, ok = m.comparisonBaseline("2026-04-01"); !ok || date != m.compareDate {
		t.Fatal("custom date ignored")
	}
}

func TestComparisonChanges(t *testing.T) {
	m := fixtureModel()
	for _, tc := range []struct {
		before, after Metric
		want          string
	}{
		{known(10), known(26), "+16 (+160.0%)"},
		{known(10), known(4), "−6 (-60.0%)"},
		{known(0), known(4), "+4 (from zero)"},
		{known(0), known(0), "0"},
		{Metric{}, known(4), "unavailable"},
		{Metric{Value: 3, Known: true, Partial: true}, known(4), "unavailable"},
		{known(4), Metric{Value: 3, Known: true, Partial: true}, "unavailable"},
	} {
		if got := m.comparisonDelta(tc.before, tc.after, false); got != tc.want {
			t.Fatalf("%s != %s", got, tc.want)
		}
	}
	m.fx = Exchange{Currency: "EUR", Rate: .9}
	if got := m.comparisonDelta(known(10), known(26), true); !strings.Contains(got, "14.4000") || !strings.Contains(got, "160.0%") {
		t.Fatal(got)
	}
	m.fx = Exchange{Currency: "EUR"}
	if got := m.comparisonDelta(known(10), known(26), true); got != "unavailable" {
		t.Fatal(got)
	}
}

func TestComparisonCustomDatesAndRefresh(t *testing.T) {
	m := fixtureModel()
	m.width, m.height = 160, 50
	m = key(m, "C")
	m = key(m, "t")
	if m.editing != "comparison" {
		t.Fatal("date picker not opened")
	}
	m.input.SetValue("2026-09-07 to 2026-09-03")
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(model)
	if cmd != nil || m.editing != "" || m.compareDate != "2026-09-07" || m.comparisonSelected().Name != "2026-09-03" {
		t.Fatal("reversed dates should be valid and local")
	}
	m = key(m, "t")
	m.input.SetValue("2026-02-30 to 2026-09-03")
	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(model)
	if cmd != nil || m.editing != "comparison" || m.err == "" {
		t.Fatal("invalid date accepted")
	}
	m.input.SetValue("2026-08-12 to 2026-09-03")
	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(model)
	if cmd == nil || m.pending.Since != "2026-08-12" || m.pending.Until != "2026-09-30" || !m.comparing {
		t.Fatalf("range not expanded: %+v", m.pending)
	}
	if m.cancel != nil {
		m.cancel()
	}
}

func TestComparisonNavigationAndCompactScrolling(t *testing.T) {
	for _, size := range [][2]int{{50, 16}, {80, 24}, {96, 32}, {160, 50}} {
		m := fixtureModel()
		m.width, m.height = size[0], size[1]
		m.dayCursor = 3
		if m.width < 96 {
			m.cursor = 3
		}
		var want string
		if m.width >= 96 {
			want = m.chartPeriods()[3].Name
		} else {
			want = m.rows()[3].Name
		}
		m = key(m, "C")
		if !m.comparing || m.comparisonSelected().Name != want {
			t.Fatal("wrong selected day")
		}
		m = key(m, "C")
		if !m.compareWeekday {
			t.Fatal("did not switch baseline")
		}
		m = key(m, "C")
		m.compareDate = "2026-09-02"
		m.compareSelected = "2026-09-03"
		first := ansi.Strip(m.View())
		for _, line := range strings.Split(first, "\n") {
			if ansi.StringWidth(line) > m.width {
				t.Fatal("width overflow")
			}
		}
		if len(strings.Split(first, "\n")) > m.height {
			t.Fatal("height overflow")
		}
		seen := first
		for i := 0; i < 15; i++ {
			m = pickerKey(m, tea.KeyMsg{Type: tea.KeyPgDown})
			seen += ansi.Strip(m.View())
		}
		if !strings.Contains(seen, "Cache write") || !strings.Contains(seen, "Missing/partial metrics") {
			t.Fatalf("unreachable content at %v: %s", size, seen)
		}
		m = key(m, "?")
		m = pickerKey(m, tea.KeyMsg{Type: tea.KeyEsc})
		if !m.comparing {
			t.Fatal("help lost comparison")
		}
		m = pickerKey(m, tea.KeyMsg{Type: tea.KeyEsc})
		if m.comparing {
			t.Fatal("escape did not close")
		}
	}
}

func TestThemeShortcutLeavesCtrlTUnbound(t *testing.T) {
	m := fixtureModel()
	m = pickerKey(m, tea.KeyMsg{Type: tea.KeyCtrlT})
	if m.choosingTheme {
		t.Fatal("Ctrl+T captured")
	}
	m = key(m, "T")
	if !m.choosingTheme {
		t.Fatal("Shift+T did not open picker")
	}
}

func TestComparisonSelectionSurvivesRefreshAndReopen(t *testing.T) {
	m := fixtureModel()
	m.width, m.height = 160, 50
	m.dayCursor = 3
	m = key(m, "C")
	want := m.comparisonSelected().Name
	next, _ := m.Update(loadedMsg{s: m.s, r: m.o.Range, id: m.request})
	m = next.(model)
	if m.comparisonSelected().Name != want {
		t.Fatal("refresh lost comparison day")
	}
	m = pickerKey(m, tea.KeyMsg{Type: tea.KeyEsc})
	m.dayCursor = 5
	m = key(m, "C")
	if m.comparisonSelected().Name != m.chartPeriods()[5].Name {
		t.Fatal("reopen used stale selected date")
	}
}
