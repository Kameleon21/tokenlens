package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func (m *model) openThemePicker() tea.Cmd {
	m.choosingTheme = true
	m.themeOriginal = m.o.Theme
	m.themeCursor = 0
	m.themeQuery = textinput.New()
	m.themeQuery.Prompt = "Search: "
	m.themeQuery.Placeholder = "type a theme name"
	m.themeQuery.CharLimit = 64
	m.themeQuery.Focus()
	for i, name := range themeNames {
		if name == m.o.Theme {
			m.themeCursor = i
			break
		}
	}
	return textinput.Blink
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

func (m model) matchingThemes() []string {
	var matches []string
	for _, name := range themeNames {
		if fuzzyThemeMatch(m.themeQuery.Value(), themeLabel(name)) || fuzzyThemeMatch(m.themeQuery.Value(), name) {
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
	matches := m.matchingThemes()
	var cmd tea.Cmd
	switch msg.String() {
	case "esc":
		m.previewTheme(m.themeOriginal)
		m.choosingTheme = false
		m.themeQuery.Blur()
		return m, nil
	case "enter":
		if len(matches) == 0 {
			return m, nil
		}
		m.previewTheme(matches[m.themeCursor])
		m.savePreference(func(p *Preferences) { p.Theme = m.o.Theme })
		m.choosingTheme = false
		m.themeQuery.Blur()
		return m, nil
	case "down", "ctrl+n", "tab":
		if len(matches) > 0 {
			m.themeCursor = (m.themeCursor + 1) % len(matches)
		}
	case "up", "ctrl+p", "shift+tab":
		if len(matches) > 0 {
			m.themeCursor = (m.themeCursor + len(matches) - 1) % len(matches)
		}
	default:
		before := m.themeQuery.Value()
		m.themeQuery, cmd = m.themeQuery.Update(msg)
		if before != m.themeQuery.Value() {
			m.themeCursor = 0
		}
	}
	matches = m.matchingThemes()
	if len(matches) > 0 {
		m.previewTheme(matches[m.themeCursor])
	}
	return m, cmd
}

func (m model) View() string {
	if m.help && !m.choosingTheme {
		return m.helpView()
	}
	base := m.dashboardView()
	if !m.choosingTheme {
		return base
	}
	// Use a centered modal over the real dashboard so navigation previews the palette.
	w := max(1, min(62, m.width-4))
	inner := max(1, w-4)
	slots := max(1, min(len(themeNames), m.height-10))
	matches := m.matchingThemes()
	start := max(0, m.themeCursor-slots+1)
	lines := []string{
		bright.Render("CHOOSE THEME"),
		muted.Render("Applied: " + themeLabel(m.themeOriginal)),
	}
	m.themeQuery.Width = max(1, inner-9)
	m.themeQuery.PromptStyle = accent
	m.themeQuery.TextStyle = lipgloss.NewStyle().Foreground(ink)
	m.themeQuery.PlaceholderStyle = muted
	lines = append(lines, m.themeQuery.View(), muted.Render(strings.Repeat("─", inner)))
	for i := start; i < start+slots; i++ {
		row := ""
		if i < len(matches) {
			name := matches[i]
			row = "  " + themeLabel(name)
			if name == m.themeOriginal {
				row += " (current)"
			}
			if i == m.themeCursor {
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
