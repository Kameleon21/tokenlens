package app

import (
	"fmt"
	"github.com/Kameleon21/tokenlens/internal/datefilter"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Keep CLI validation, theme picker, and palettes in the same order.
var themeNames = []string{"dark", "light", "ascii", "nord", "gruvbox", "tokyo-light", "dracula", "catppuccin", "solarized-light", "tokyo-dark", "solarized-dark"}

type themePalette struct {
	label, background, foreground, muted, accent, border, surface string
	series                                                        [6]string
}

var themePalettes = map[string]themePalette{
	"dark":            {"Dark", "#151D28", "#E3E9F3", "#8794A9", "#80D8C3", "#394555", "#19212C", [6]string{"#80D8C3", "#AFABED", "#E5B887", "#8CBFE5", "#D9A1BA", "#B8C990"}},
	"light":           {"Light", "#F4F7FA", "#162638", "#526276", "#087E70", "#A6B6C8", "#E3EBF2", [6]string{"#087E70", "#7156B2", "#9B601C", "#286D9B", "#A04773", "#5D7524"}},
	"nord":            {"Nord", "#2E3440", "#ECEFF4", "#D8DEE9", "#88C0D0", "#4C566A", "#3B4252", [6]string{"#88C0D0", "#B48EAD", "#EBCB8B", "#81A1C1", "#D08770", "#A3BE8C"}},
	"gruvbox":         {"Gruvbox", "#282828", "#EBDBB2", "#A89984", "#8EC07C", "#665C54", "#3C3836", [6]string{"#8EC07C", "#D3869B", "#FABD2F", "#83A598", "#FE8019", "#B8BB26"}},
	"tokyo-light":     {"Tokyo Night Light", "#E1E2E7", "#3760BF", "#6172B0", "#007197", "#A8AECb", "#D0D5E3", [6]string{"#007197", "#7847BD", "#8C6C3E", "#2E7DE9", "#B15C00", "#587539"}},
	"dracula":         {"Dracula", "#282A36", "#F8F8F2", "#BDBDCB", "#BD93F9", "#6272A4", "#44475A", [6]string{"#8BE9FD", "#BD93F9", "#F1FA8C", "#FF79C6", "#FFB86C", "#50FA7B"}},
	"catppuccin":      {"Catppuccin Mocha", "#1E1E2E", "#CDD6F4", "#A6ADC8", "#CBA6F7", "#585B70", "#313244", [6]string{"#94E2D5", "#CBA6F7", "#F9E2AF", "#89B4FA", "#F5C2E7", "#A6E3A1"}},
	"solarized-light": {"Solarized Light", "#FDF6E3", "#586E75", "#657B83", "#007F78", "#93A1A1", "#EEE8D5", [6]string{"#007F78", "#6C71C4", "#9A7000", "#268BD2", "#D33682", "#738000"}},
	"tokyo-dark":      {"Tokyo Night Dark", "#1A1B26", "#C0CAF5", "#A9B1D6", "#7AA2F7", "#545C7E", "#292E42", [6]string{"#7DCFFF", "#BB9AF7", "#E0AF68", "#7AA2F7", "#FF9E64", "#9ECE6A"}},
	"solarized-dark":  {"Solarized Dark", "#002B36", "#839496", "#93A1A1", "#2AA198", "#586E75", "#073642", [6]string{"#2AA198", "#6C71C4", "#B58900", "#268BD2", "#D33682", "#859900"}},
}

var activeTheme = "dark"

func paletteFor(name string) themePalette {
	if p, ok := themePalettes[name]; ok {
		return p
	}
	return themePalettes["dark"]
}

func themeLabel(name string) string {
	if name == "ascii" {
		return "ASCII"
	}
	return paletteFor(name).label
}

// themeIndicator stays visible above the content, including in compact terminals.
func (m model) themeIndicator() string {
	return accent.Bold(true).Render("Theme: "+themeLabel(m.o.Theme)) + muted.Render("  Ctrl+T choose")
}

func applyTheme(name string) {
	activeTheme = name
	p := paletteFor(name)
	ink = lipgloss.Color(p.foreground)
	muted = lipgloss.NewStyle().Foreground(lipgloss.Color(p.muted))
	accent = lipgloss.NewStyle().Foreground(lipgloss.Color(p.accent))
	borderColor = lipgloss.Color(p.border)
	surface = lipgloss.Color(p.surface)
	bright = lipgloss.NewStyle().Foreground(ink).Bold(true)
}
func themeRender(s, theme string, w, h int) string {
	if theme == "ascii" {
		s = strings.NewReplacer("╭", "+", "╮", "+", "╰", "+", "╯", "+", "│", "|", "─", "-", "━", "=", "█", "#", "▓", "#", "▒", ":", "░", ".", "●", "*", "○", "o", "◈", "*", "→", ">", "←", "<", "×", "x", "—", "-", "↑", "^", "↓", "v", "›", ">", "…", "~", "·", ".", "–", "-", "€", "EUR ", "£", "GBP ", "¥", "JPY ").Replace(s)
	}
	if theme == "ascii" {
		return fit(ansi.Strip(s), w, h)
	}
	p := paletteFor(theme)
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(p.foreground)).Background(lipgloss.Color(p.background))
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
