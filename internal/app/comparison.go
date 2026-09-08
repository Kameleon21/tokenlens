package app

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Kameleon21/tokenlens/internal/datefilter"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Compare calendar dates, never adjacent report rows: gaps are not zero usage.
func (m model) comparisonBaseline(selected string) (Row, string, bool) {
	date, err := time.Parse("2006-01-02", selected)
	if err != nil {
		return Row{}, "", false
	}
	days := -1
	if m.compareWeekday {
		days = -7
	}
	target := date.AddDate(0, 0, days).Format("2006-01-02")
	if m.compareDate != "" {
		target = m.compareDate
	}
	for _, r := range filtered(m.s.Sections["daily"], m.agent, m.modelFilter) {
		if r.Name == target {
			return r, target, true
		}
	}
	return Row{}, target, false
}

func (m model) comparisonDelta(before, after Metric, cost bool) string {
	if !before.Known || !after.Known || before.Partial || after.Partial || (cost && !m.fx.available()) {
		return "unavailable"
	}
	delta := after.Value - before.Value
	sign := "+"
	if delta < 0 {
		sign = "−"
	}
	value := m.comparisonValue(known(math.Abs(delta)), cost)
	if delta == 0 {
		sign = ""
	}
	result := sign + value
	if before.Value > 0 {
		result += fmt.Sprintf(" (%+.1f%%)", 100*delta/before.Value)
	} else if after.Value > 0 {
		result += " (from zero)"
	}
	return result
}

func (m model) comparisonValue(v Metric, cost bool) string {
	if cost {
		return m.fx.format(v)
	}
	return format(v, false)
}

func (m model) comparisonLines(w int) []string {
	selected := m.comparisonSelected()
	if selected.Name == "" {
		return wrapComparison([]string{m.formatPeriod(m.compareSelected) + ": selected day unavailable for these filters.", "[t] choose comparison dates."}, w)
	}
	before, date, found := m.comparisonBaseline(selected.Name)
	label := "Previous day"
	if m.compareWeekday {
		label = "Same weekday last week"
	}
	if m.compareDate != "" {
		label = "Custom comparison"
	}
	lines := []string{m.formatPeriod(selected.Name) + " · " + label, "Baseline: " + m.formatPeriod(date)}
	// Current-day totals cannot be matched to the same time yesterday with daily data.
	loc := m.displayLocation
	if loc == nil {
		loc = time.UTC
	}
	if selected.Name >= time.Now().In(loc).Format("2006-01-02") || date >= time.Now().In(loc).Format("2006-01-02") {
		lines = append(lines, "Comparison includes an incomplete day (so far).")
	}
	if !found {
		return wrapComparison(append(lines, "Baseline unavailable in loaded filtered report.", "[t] choose dates or [a/f/x] change filters."), w)
	}
	if w >= 100 {
		lines = append(lines, fmt.Sprintf("%-24s %20s %20s  %s", "Metric", "Before", "Selected", "Change"))
	} else {
		lines = append(lines, "Before → selected · change", "")
	}
	for _, v := range []struct {
		name          string
		before, after Metric
		cost          bool
	}{
		{"Estimated cost · " + m.fx.Currency, before.Usage.Cost, selected.Usage.Cost, true},
		{"Total tokens", before.Usage.Tokens, selected.Usage.Tokens, false},
		{"Input", before.Usage.Input, selected.Usage.Input, false},
		{"Output", before.Usage.Output, selected.Usage.Output, false},
		{"Cache read", before.Usage.Read, selected.Usage.Read, false},
		{"Cache write", before.Usage.Write, selected.Usage.Write, false},
	} {
		if w >= 100 {
			lines = append(lines, fmt.Sprintf("%-24s %20s %20s  %s", v.name, m.comparisonValue(v.before, v.cost), m.comparisonValue(v.after, v.cost), m.comparisonDelta(v.before, v.after, v.cost)))
		} else {
			lines = append(lines, v.name, m.comparisonValue(v.before, v.cost)+" → "+m.comparisonValue(v.after, v.cost), "Change: "+m.comparisonDelta(v.before, v.after, v.cost), "")
		}
	}
	lines = append(lines, "Same snapshot pricing and exchange rate.", "Missing/partial metrics have no calculated change.")
	return wrapComparison(lines, w)
}

// Wrap before scrolling, so all values remain reachable on compact terminals.
func wrapComparison(lines []string, w int) []string {
	wrapped := []string{}
	for _, line := range lines {
		wrapped = append(wrapped, strings.Split(ansi.Wrap(line, max(1, w), ""), "\n")...)
	}
	return wrapped
}

func (m model) comparisonContent(w, h int) string {
	lines := m.comparisonLines(w)
	start := min(m.compareOffset, max(0, len(lines)-max(1, h)))
	return strings.Join(lines[start:min(len(lines), start+max(1, h))], "\n")
}

func (m model) comparisonSelected() Row {
	rows := m.rows()
	if m.compareSelected != "" {
		for _, r := range rows {
			if r.Name == m.compareSelected {
				return r
			}
		}
		return Row{}
	}
	if len(rows) == 0 {
		return Row{}
	}
	return rows[min(m.cursor, len(rows)-1)]
}

func (m model) dateEditorTitle() string {
	if m.editing == "comparison" {
		return "Compare dates"
	}
	return "Change date range"
}
func (m model) dateEditorHelp() string {
	if m.editing == "comparison" {
		return "Baseline to selected: YYYY-MM-DD to YYYY-MM-DD\nEither date may come first. Loaded range expands if needed."
	}
	return m.rangeHelp()
}
func (m model) applyComparisonDates() (tea.Model, tea.Cmd) {
	parts := strings.Split(strings.ReplaceAll(m.input.Value(), " to ", "→"), "→")
	if len(parts) != 2 {
		parts = strings.Fields(m.input.Value())
	}
	if len(parts) != 2 {
		m.err = "Enter baseline to selected: YYYY-MM-DD to YYYY-MM-DD"
		return m, nil
	}
	loc := m.displayLocation
	if loc == nil {
		loc = time.UTC
	}
	dates := [2]string{}
	for i, part := range parts {
		date, err := datefilter.Parse(m.canonicalDate(strings.TrimSpace(part)), loc)
		if err != nil {
			m.err = err.Error()
			return m, nil
		}
		dates[i] = date.Format("2006-01-02")
	}
	m.compareDate, m.compareSelected = dates[0], dates[1]
	m.compareOffset = 0
	m.editing, m.err = "", ""
	m.input.Blur()
	r := m.o.Range
	if m.loading {
		r = m.pending
	}
	for _, date := range dates {
		if r.Since != "" && date < r.Since {
			r.Since = date
		}
		if r.Until != "" && date > r.Until {
			r.Until = date
		}
	}
	if r != m.o.Range || m.loading {
		return m, m.refresh(r)
	}
	return m, nil
}

func (m model) compactComparisonView() string {
	w := m.width - 4
	badge := ""
	if m.o.Demo {
		badge = " · SYNTHETIC"
	}
	header := bright.Render("TOKENLENS · Compare day"+badge) + "\n" + m.themeIndicator() + "\n"
	agent, modelName := m.agent, m.modelFilter
	if agent == "" {
		agent = "all"
	}
	if modelName == "" {
		modelName = "all"
	}
	header += clip("Agent: "+safe(agent)+" · Model: "+safe(modelName)+" · "+m.o.TZ, w) + "\n"
	status := "snapshot " + m.formatTimestamp(m.s.Loaded)
	if m.cached {
		status += " · cached"
	}
	if m.loading {
		status = "Loading comparison dates…"
	}
	header += clip(status, w) + "\n"
	bodyH := max(1, m.height-8)
	body := m.comparisonContent(w, bodyH)
	footer := "t dates · C baseline · PgUp/Dn · ↑↓ · esc"
	if m.editing == "comparison" {
		m.input.Width = max(1, w-2)
		body = m.dateEditorTitle() + "\n" + m.input.View() + "\n" + strings.Join(wrapComparison([]string{m.dateEditorHelp(), m.err}, w), "\n")
		footer = "enter apply · esc cancel"
	}
	content := fit(header, w, 4) + "\n" + fit(body, w, bodyH) + "\n" + clip(footer, w)
	return themeRender(lipgloss.NewStyle().Foreground(ink).Padding(1, 2).Render(content), m.o.Theme, m.width, m.height)
}
