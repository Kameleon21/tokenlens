package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Kameleon21/tokenlens/internal/datefilter"
	tea "github.com/charmbracelet/bubbletea"
)

func press(m model, msg tea.KeyMsg) (model, tea.Cmd) {
	next, cmd := m.Update(msg)
	return next.(model), cmd
}

func TestQuitCancelsWorkFromAnyMode(t *testing.T) {
	for name, setup := range map[string]func(*model){
		"dashboard": func(*model) {},
		"theme":     func(m *model) { m.openThemePicker() },
		"editing":   func(m *model) { m.editing = "range" },
		"exporting": func(m *model) { m.exporting = true },
	} {
		m := fixtureModel()
		setup(&m)
		var cancelled, fxCancelled bool
		m.cancel = func() { cancelled = true }
		m.fxCancel = func() { fxCancelled = true }
		_, cmd := press(m, tea.KeyMsg{Type: tea.KeyCtrlC})
		if cmd == nil || !cancelled || !fxCancelled {
			t.Fatalf("%s: ctrl+c did not cancel and quit", name)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatalf("%s: ctrl+c did not quit", name)
		}
	}
	m := fixtureModel()
	var cancelled bool
	m.cancel = func() { cancelled = true }
	_, cmd := press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil || !cancelled {
		t.Fatal("q did not quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("q did not quit")
	}
}

func TestExportKeys(t *testing.T) {
	m := fixtureModel()
	m, _ = press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if !m.exporting {
		t.Fatal("o did not open export")
	}
	m, cmd := press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}})
	if !m.exporting || cmd != nil || m.view != 0 {
		t.Fatal("unknown export key leaked")
	}
	for _, k := range []tea.KeyMsg{{Type: tea.KeyEsc}, {Type: tea.KeyRunes, Runes: []rune{'q'}}} {
		m.exporting = true
		m, cmd = press(m, k)
		if m.exporting || cmd != nil {
			t.Fatalf("%s did not cancel export", k)
		}
	}
	m.exporting = true
	m, cmd = press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	if m.exporting || cmd == nil {
		t.Fatal("format key did not export")
	}
}

func TestRangeEditorInputs(t *testing.T) {
	now := time.Now().UTC()
	month, _ := datefilter.Resolve("", "", 0, "daily", now, time.UTC)
	last, _ := datefilter.Resolve("", "", 7, "daily", now, time.UTC)
	for input, want := range map[string]datefilter.Range{
		"month":                  month,
		"last 7":                 last,
		"* to 2026-09-07":        {Until: "2026-09-07"},
		"2026-09-06 → *":         {Since: "2026-09-06"},
		"2026-09-06  2026-09-07": {Since: "2026-09-06", Until: "2026-09-07"},
	} {
		m := newModel(context.Background(), Options{TZ: "UTC", Group: "daily"})
		m.editing = "range"
		m.input.SetValue(input)
		m, cmd := press(m, tea.KeyMsg{Type: tea.KeyEnter})
		if m.err != "" || m.editing != "" || cmd == nil || m.pending != want {
			t.Fatalf("%q: error %q, pending %+v", input, m.err, m.pending)
		}
	}
	for input, want := range map[string]string{
		"last 0":                "use last N with a positive integer",
		"last 07":               "use last N with a positive integer",
		"last x":                "use last N with a positive integer",
		"yesterday":             "enter two dates (use * for an open bound), month, or last N",
		"a b c":                 "enter two dates (use * for an open bound), month, or last N",
		"2026-13-01 2026-13-02": "",
	} {
		m := newModel(context.Background(), Options{TZ: "UTC", Group: "daily"})
		m.editing = "range"
		m.input.SetValue(input)
		m, cmd := press(m, tea.KeyMsg{Type: tea.KeyEnter})
		if cmd != nil || m.editing != "range" || m.err == "" || (want != "" && m.err != want) {
			t.Fatalf("%q: error %q", input, m.err)
		}
		m, _ = press(m, tea.KeyMsg{Type: tea.KeyEsc})
		if m.editing != "" || m.err == "" {
			t.Fatalf("%q: esc should close range editor and keep error", input)
		}
	}
	m := newModel(context.Background(), Options{TZ: "UTC", Group: "daily"})
	m.editing = "comparison"
	m.err = "bad"
	m, _ = press(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.editing != "" || m.err != "" {
		t.Fatal("esc should clear comparison editor error")
	}
	m.editing = "range"
	m.input.Focus()
	m, _ = press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if m.input.Value() != "q" || m.editing != "range" {
		t.Fatal("editor did not receive typed text")
	}
}

func TestEscapeDismissesInOrder(t *testing.T) {
	m := fixtureModel()
	m.help, m.comparing, m.details, m.info, m.notice, m.activityDetail, m.err = true, true, true, "i", "n", true, "e"
	m, _ = press(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.help || !m.comparing || !m.details {
		t.Fatal("esc should close help first")
	}
	m, _ = press(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.comparing || !m.details {
		t.Fatal("esc should close comparison second")
	}
	m, _ = press(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.details || m.info != "" || m.notice != "" || m.activityDetail || m.err != "" {
		t.Fatal("esc should clear transient state")
	}
}

func TestHelpScrollKeys(t *testing.T) {
	m := fixtureModel()
	m, _ = press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m, _ = press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.helpOffset != 0 {
		t.Fatal("help scrolled above top")
	}
	m, _ = press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m, _ = press(m, tea.KeyMsg{Type: tea.KeyDown})
	if m.helpOffset != 2 {
		t.Fatal("help did not scroll down")
	}
	m, _ = press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if m.agent != "" || m.helpOffset != 2 {
		t.Fatal("help leaked a dashboard key")
	}
	m, _ = press(m, tea.KeyMsg{Type: tea.KeyHome})
	if m.helpOffset != 0 {
		t.Fatal("home did not reset help")
	}
	m, _ = press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if m.help {
		t.Fatal("? did not close help")
	}
}

func TestDashboardKeys(t *testing.T) {
	m := fixtureModel()
	m.width, m.height = 120, 40
	periods := len(m.chartPeriods())
	m, cmd := press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if m.preset != 1 || m.notice != "This billing cycle" || cmd == nil {
		t.Fatal("preset key")
	}
	for _, step := range []struct {
		key   tea.KeyMsg
		check func(model) bool
	}{
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}}, func(m model) bool { return m.showPlan }},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}}, func(m model) bool { return m.info != "" }},
		{tea.KeyMsg{Type: tea.KeyLeft}, func(m model) bool { return m.dayCursor == 0 }},
		{tea.KeyMsg{Type: tea.KeyRight}, func(m model) bool { return m.dayCursor == 1 }},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}}, func(m model) bool { return m.widget == 3 }},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}}, func(m model) bool { return m.widget == 0 }},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}}, func(m model) bool { return m.cursor == len(m.rows())-1 }},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, func(m model) bool { return m.cursor == len(m.rows())-1 }},
		{tea.KeyMsg{Type: tea.KeyUp}, func(m model) bool { return m.cursor == len(m.rows())-2 }},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}, func(m model) bool { return m.cursor == 0 }},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}, func(m model) bool { return m.cursor == 0 }},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}, func(m model) bool { return m.sortMode == 1 }},
		{tea.KeyMsg{Type: tea.KeyShiftTab}, func(m model) bool { return m.view == 4 }},
		{tea.KeyMsg{Type: tea.KeyTab}, func(m model) bool { return m.view == 0 }},
	} {
		m, _ = press(m, step.key)
		if !step.check(m) {
			t.Fatalf("%s: unexpected state", step.key)
		}
	}
	m.dayCursor = periods + 5
	m, _ = press(m, tea.KeyMsg{Type: tea.KeyRight})
	if m.dayCursor != periods-1 {
		t.Fatal("right did not clamp")
	}
	m.loading, m.pending, m.request = true, datefilter.Range{Since: "2026-08-01"}, 3
	m, cmd = press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if m.request != 3 || m.pending.Since != "2026-08-01" || cmd != nil {
		t.Fatal("refresh restarted the pending range")
	}
	m.loading = false
	m, cmd = press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if m.request != 4 || m.pending != m.o.Range || cmd == nil {
		t.Fatal("refresh did not reload current range")
	}
}

func TestEnterOpensDashboardWidgets(t *testing.T) {
	for widget, want := range map[int]func(model) bool{
		1: func(m model) bool { return m.view == 2 && !m.activityDetail },
		2: func(m model) bool { return m.view == 4 && !m.activityDetail },
		3: func(m model) bool { return m.view == 0 && m.activityDetail && m.cursor == 0 },
	} {
		m := fixtureModel()
		m.width, m.height, m.widget, m.cursor = 120, 40, widget, 2
		m, _ = press(m, tea.KeyMsg{Type: tea.KeyEnter})
		if !want(m) || m.cursor != 0 {
			t.Fatalf("widget %d", widget)
		}
	}
	m := fixtureModel()
	m.width, m.height = 80, 24
	m, _ = press(m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.details || m.activityDetail {
		t.Fatal("narrow enter should toggle details")
	}
	m, _ = press(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.details {
		t.Fatal("enter should close details")
	}
}

func TestMouseSelectsChartDay(t *testing.T) {
	m := fixtureModel()
	m.width, m.height, m.widget = 120, 40, 2
	next, _ := m.Update(tea.MouseMsg{X: 17, Y: 19})
	m = next.(model)
	if m.dayCursor != 4 || m.widget != 0 {
		t.Fatalf("click selected %d", m.dayCursor)
	}
	next, _ = m.Update(tea.MouseMsg{X: 17, Y: 30})
	if next.(model).dayCursor != 4 {
		t.Fatal("click outside chart moved cursor")
	}
	m.comparing = true
	next, cmd := m.Update(tea.MouseMsg{X: 20, Y: 19})
	if next.(model).dayCursor != 4 || cmd != nil {
		t.Fatal("click under comparison moved cursor")
	}
}

func TestPriceAndNotificationMessages(t *testing.T) {
	m := newModel(context.Background(), Options{Demo: true})
	m.priceLoading = true
	next, cmd := m.Update(pricesMsg{err: errors.New("offline")})
	m = next.(model)
	if m.priceLoading || m.priceErr != "offline" || cmd != nil {
		t.Fatal("price failure")
	}
	p := testPrices()
	m.o.priceRevision = p.revision()
	m.s.PriceRevision = p.revision()
	next, cmd = m.Update(pricesMsg{catalog: p})
	m = next.(model)
	if m.priceErr != "" || cmd != nil || !m.s.PriceDate.Equal(p.Fetched) {
		t.Fatal("unchanged prices did not refresh price date")
	}
	next, _ = m.Update(exportedMsg{path: "/tmp/x.json"})
	if next.(model).notice != "Saved /tmp/x.json" {
		t.Fatal("export notice")
	}
	next, _ = m.Update(exportedMsg{err: errors.New("disk")})
	if next.(model).notice != "Export failed: disk" {
		t.Fatal("export failure notice")
	}
	next, _ = m.Update(tea.WindowSizeMsg{Width: 77, Height: 22})
	if next.(model).width != 77 || next.(model).height != 22 {
		t.Fatal("resize")
	}
}
