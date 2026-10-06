package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type themePicker struct {
	active   bool
	original string
	cursor   int
	query    textinput.Model
}

func newThemePicker(current string) themePicker {
	p := themePicker{active: true, original: current, query: textinput.New()}
	p.query.Prompt = "Search: "
	p.query.Placeholder = "type a theme name"
	p.query.CharLimit = 64
	p.query.Focus()
	for i, name := range themeNames {
		if name == current {
			p.cursor = i
			break
		}
	}
	return p
}

func (m *model) openThemePicker() tea.Cmd {
	m.theme = newThemePicker(m.o.Theme)
	return textinput.Blink
}

func (p *themePicker) close() {
	p.active = false
	p.query.Blur()
}

func (p *themePicker) move(step, count int) {
	if count > 0 {
		p.cursor = (p.cursor + count + step) % count
	}
}

// A subsequence match supports short queries such as "tnd" for Tokyo Night Dark.
func fuzzyThemeMatch(query, candidate string) bool {
	remaining := []rune(strings.ToLower(strings.TrimSpace(query)))
	for _, c := range strings.ToLower(candidate) {
		if len(remaining) > 0 && remaining[0] == c {
			remaining = remaining[1:]
		}
	}
	return len(remaining) == 0
}

func (p themePicker) matches() []string {
	var matches []string
	for _, name := range themeNames {
		if fuzzyThemeMatch(p.query.Value(), themeLabel(name)) || fuzzyThemeMatch(p.query.Value(), name) {
			matches = append(matches, name)
		}
	}
	return matches
}

func (m *model) previewTheme(name string) {
	m.o.Theme = name
	applyTheme(name)
	m.spin.Style = accent
}

func (m model) updateThemePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	matches := m.theme.matches()
	var cmd tea.Cmd
	switch msg.String() {
	case "esc":
		m.previewTheme(m.theme.original)
		m.theme.close()
		return m, nil
	case "enter":
		if len(matches) == 0 {
			return m, nil
		}
		m.previewTheme(matches[m.theme.cursor])
		m.savePreference(func(p *Preferences) { p.Theme = m.o.Theme })
		m.theme.close()
		return m, nil
	case "down", "ctrl+n", "tab":
		m.theme.move(1, len(matches))
	case "up", "ctrl+p", "shift+tab":
		m.theme.move(-1, len(matches))
	default:
		before := m.theme.query.Value()
		m.theme.query, cmd = m.theme.query.Update(msg)
		if before != m.theme.query.Value() {
			m.theme.cursor = 0
		}
	}
	matches = m.theme.matches()
	if len(matches) > 0 {
		m.previewTheme(matches[m.theme.cursor])
	}
	return m, cmd
}

func (m model) View() string {
	base := m.dashboardView()
	if !m.theme.active {
		return base
	}
	// Use a centered modal over the real dashboard so navigation previews the palette.
	w := max(1, min(62, m.width-4))
	inner := max(1, w-4)
	slots := max(1, min(len(themeNames), m.height-10))
	matches := m.theme.matches()
	start := max(0, m.theme.cursor-slots+1)
	lines := []string{
		bright.Render("CHOOSE THEME"),
		muted.Render("Applied: " + themeLabel(m.theme.original)),
	}
	m.theme.query.Width = max(1, inner-9)
	m.theme.query.PromptStyle = accent
	m.theme.query.TextStyle = lipgloss.NewStyle().Foreground(ink)
	m.theme.query.PlaceholderStyle = muted
	lines = append(lines, m.theme.query.View(), muted.Render(strings.Repeat("─", inner)))
	for i := start; i < start+slots; i++ {
		row := ""
		if i < len(matches) {
			name := matches[i]
			row = "  " + themeLabel(name)
			if name == m.theme.original {
				row += " (current)"
			}
			if i == m.theme.cursor {
				row = accent.Bold(true).Background(surface).Render(fit("> "+strings.TrimSpace(row), inner, 1))
			}
		} else if i == start && len(matches) == 0 {
			row = "No matching themes"
		}
		lines = append(lines, row)
	}
	lines = append(lines, muted.Render(fmt.Sprintf("%d matches · live preview", len(matches))), muted.Render("↑/↓ move · Enter apply · Esc cancel"))
	panel := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(paletteFor(m.o.Theme).accent)).Background(lipgloss.Color(paletteFor(m.o.Theme).background)).Padding(0, 1).Render(fit(strings.Join(lines, "\n"), inner, len(lines)))
	panelLines := strings.Split(panel, "\n")
	baseLines := strings.Split(fit(base, m.width, m.height), "\n")
	x, y := max(0, (m.width-w)/2), max(0, (m.height-len(panelLines))/2)
	for i, row := range panelLines {
		if y+i >= len(baseLines) {
			break
		}
		baseLines[y+i] = ansi.Cut(baseLines[y+i], 0, x) + row + ansi.Cut(baseLines[y+i], x+w, m.width)
	}
	return themeRender(strings.Join(baseLines, "\n"), m.o.Theme, m.width, m.height)
}
