package app

import (
	"fmt"
	"github.com/Kameleon21/tokenlens/internal/datefilter"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var activeTheme = "dark"

func applyTheme(name string) {
	activeTheme = name
	if name == "light" {
		ink = lipgloss.Color("#162638")
		muted = lipgloss.NewStyle().Foreground(lipgloss.Color("#526276"))
		accent = lipgloss.NewStyle().Foreground(lipgloss.Color("#087E70"))
		borderColor = lipgloss.Color("#A6B6C8")
		surface = lipgloss.Color("#E3EBF2")
	} else {
		ink = lipgloss.Color("#E3E9F3")
		muted = lipgloss.NewStyle().Foreground(lipgloss.Color("#8794A9"))
		accent = lipgloss.NewStyle().Foreground(lipgloss.Color("#80D8C3"))
		borderColor = lipgloss.Color("#394555")
		surface = lipgloss.Color("#19212C")
	}
	bright = lipgloss.NewStyle().Foreground(ink).Bold(true)
}
func themeRender(s, theme string, w, h int) string {
	if theme == "ascii" {
		s = strings.NewReplacer("╭", "+", "╮", "+", "╰", "+", "╯", "+", "│", "|", "─", "-", "━", "=", "█", "#", "▓", "#", "▒", ":", "░", ".", "●", "*", "○", "o", "◈", "*", "→", ">", "←", "<", "×", "x", "—", "-", "↑", "^", "↓", "v", "›", ">", "…", "~", "·", ".", "–", "-", "€", "EUR ", "£", "GBP ", "¥", "JPY ").Replace(s)
	}
	if theme == "ascii" {
		return fit(ansi.Strip(s), w, h)
	}
	style := lipgloss.NewStyle().Foreground(ink).Background(lipgloss.Color("#151D28"))
	if theme == "light" {
		style = style.Background(lipgloss.Color("#F4F7FA"))
	}
	out := style.Render(fit(s, w, h))
	if theme != "ascii" {
		sample := style.Render("x")
		prefix := strings.SplitN(sample, "x", 2)[0]
		if prefix != "" {
			out = prefix + strings.NewReplacer("\x1b[0m", "\x1b[0m"+prefix, "\x1b[m", "\x1b[m"+prefix).Replace(out) + "\x1b[0m"
		}
	}
	return out
}
func billingStart(now time.Time, day int) time.Time {
	start := billingDate(now.Year(), now.Month(), day, now.Location())
	if now.Before(start) {
		prev := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, now.Location())
		start = billingDate(prev.Year(), prev.Month(), day, now.Location())
	}
	return start
}
func billingDate(y int, mo time.Month, day int, loc *time.Location) time.Time {
	last := time.Date(y, mo+1, 0, 0, 0, 0, 0, loc).Day()
	return time.Date(y, mo, min(day, last), 0, 0, 0, 0, loc)
}
func (m model) presetRange(index int) datefilter.Range {
	loc, _ := time.LoadLocation(m.o.TZ)
	now := time.Now().In(loc)
	today := now.Format("2006-01-02")
	switch index {
	case 1:
		return datefilter.Range{Since: billingStart(now, m.o.BillingDay).Format("2006-01-02"), Until: today}
	case 2:
		return datefilter.Range{Since: now.AddDate(0, 0, -29).Format("2006-01-02"), Until: today}
	case 3:
		y := now.Year()
		if now.Month() < time.August {
			y--
		}
		return datefilter.Range{Since: fmt.Sprintf("%d-08-01", y), Until: today}
	}
	r, _ := datefilter.Resolve("", "", 0, m.o.Group, now, loc)
	return r
}
func (m model) planSummary() string {
	if m.o.PlanCost <= 0 {
		return "Configure --plan-cost 100 --plan-agent claude\nPlan amount uses your startup display currency."
	}
	if m.fx.Currency != m.o.PlanCurrency {
		return "Plan configured in " + m.o.PlanCurrency + "\nSelect that currency to compare."
	}
	agent := m.o.PlanAgent
	if agent == "" {
		agent = m.agent
	}
	rows := filtered(m.s.Sections["daily"], agent, m.modelFilter)
	u := total(rows)
	if !u.Cost.Known || u.Cost.Partial {
		return "Incomplete cost metrics; comparison unavailable"
	}
	amount := u.Cost.Value * m.fx.Rate
	label := agent
	if label == "" {
		label = "all agents"
	}
	return fmt.Sprintf("%.2f %s configured monthly plan · %s\n%.2f× plan price in API-equivalent usage\nSelected range only · not cash savings", m.o.PlanCost, m.o.PlanCurrency, label, amount/m.o.PlanCost)
}
