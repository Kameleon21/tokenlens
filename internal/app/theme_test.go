package app

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestThemePickerApplyAndCancel(t *testing.T) {
	defer applyTheme("dark")
	m := fixtureModel()
	m.o.Theme = "nord"
	m.agent, m.view = "codex", 2
	m = pickerKey(m, tea.KeyMsg{Type: tea.KeyCtrlT})
	if !m.choosingTheme || m.o.Theme != "nord" {
		t.Fatal("opening picker changed theme or failed")
	}
	m = pickerKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tnd")})
	if m.o.Theme != "tokyo-dark" || activeTheme != "tokyo-dark" {
		t.Fatal("fuzzy search did not preview Tokyo Night Dark")
	}
	m = pickerKey(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.choosingTheme || m.o.Theme != "nord" || activeTheme != "nord" {
		t.Fatal("cancel did not restore original theme")
	}
	m = pickerKey(m, tea.KeyMsg{Type: tea.KeyCtrlT})
	m = pickerKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("solarized dark")})
	m = pickerKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.choosingTheme || m.o.Theme != "solarized-dark" {
		t.Fatal("did not apply Solarized Dark")
	}
	if m.agent != "codex" || m.view != 2 {
		t.Fatal("picker changed navigation or filters")
	}
	if m.spin.Style.GetForeground() != accent.GetForeground() {
		t.Fatal("spinner kept previous theme")
	}
}

func pickerKey(m model, msg tea.KeyMsg) model {
	next, _ := m.Update(msg)
	return next.(model)
}

func TestThemePickerNavigationAndEmptyResults(t *testing.T) {
	defer applyTheme("dark")
	m := fixtureModel()
	m.o.Theme = "dark"
	m = pickerKey(m, tea.KeyMsg{Type: tea.KeyCtrlT})
	m = pickerKey(m, tea.KeyMsg{Type: tea.KeyUp})
	if m.o.Theme != "solarized-dark" {
		t.Fatal("up did not wrap")
	}
	m = pickerKey(m, tea.KeyMsg{Type: tea.KeyDown})
	if m.o.Theme != "dark" {
		t.Fatal("down did not wrap")
	}
	// q and application shortcuts must enter the search rather than leaking through.
	m = pickerKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("qz123")})
	if m.themeQuery.Value() != "qz123" || len(m.matchingThemes()) != 0 {
		t.Fatal("input leaked through picker")
	}
	m = pickerKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.choosingTheme || m.o.Theme != "dark" {
		t.Fatal("empty results applied a theme")
	}
	m = pickerKey(m, tea.KeyMsg{Type: tea.KeyCtrlU})
	if len(m.matchingThemes()) != len(themeNames) {
		t.Fatal("clearing search did not restore choices")
	}
	m = pickerKey(m, tea.KeyMsg{Type: tea.KeyEsc})
	m = pickerKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'T'}})
	if !m.choosingTheme {
		t.Fatal("legacy shortcut did not open picker")
	}
}

func TestThemePickerFits(t *testing.T) {
	original := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(original)
	defer applyTheme("dark")
	for _, name := range themeNames {
		for _, wh := range [][2]int{{20, 8}, {50, 16}, {80, 24}, {96, 32}, {160, 50}} {
			m := fixtureModel()
			m.o.Theme = name
			applyTheme(name)
			m.width, m.height = wh[0], wh[1]
			m = pickerKey(m, tea.KeyMsg{Type: tea.KeyCtrlT})
			for _, query := range []string{"", "no such theme"} {
				m.themeQuery.SetValue(query)
				m.themeCursor = 0
				rendered := m.View()
				lines := strings.Split(ansi.Strip(rendered), "\n")
				if len(lines) != m.height {
					t.Fatalf("height %s %v", name, wh)
				}
				for _, line := range lines {
					if ansi.StringWidth(line) > m.width {
						t.Fatalf("width %s %v", name, wh)
					}
				}
				if m.width >= 50 {
					for _, text := range []string{"CHOOSE THEME", "Search:", "Enter apply", "Esc cancel"} {
						if !strings.Contains(ansi.Strip(rendered), text) {
							t.Fatalf("missing %s at %v", text, wh)
						}
					}
				}
				if name == "ascii" && strings.ContainsAny(rendered, "\x1b╭╮╰╯│─") {
					t.Fatal("ASCII picker contains color or unicode borders")
				}
			}
		}
	}
}

func TestThemeStartupAndBackground(t *testing.T) {
	original := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(original)
	defer applyTheme("dark")
	for _, name := range themeNames {
		o, err := options([]string{"--theme", name}, time.Now())
		if err != nil || o.Theme != name {
			t.Fatalf("%s: %v", name, err)
		}
		if name == "ascii" {
			if strings.Contains(themeRender("hello", name, 50, 16), "\x1b") {
				t.Fatal("ASCII includes color codes")
			}
			continue
		}
		p, ok := themePalettes[name]
		if !ok {
			t.Fatalf("missing palette %s", name)
		}
		expected := lipgloss.NewStyle().Foreground(lipgloss.Color(p.foreground)).Background(lipgloss.Color(p.background)).Render("x")
		prefix := strings.SplitN(expected, "x", 2)[0]
		if prefix == "" || !strings.HasPrefix(themeRender("hello", name, 50, 16), prefix) {
			t.Fatalf("wrong background for %s", name)
		}
	}
}

func TestThemeIndicatorVisibleAtTop(t *testing.T) {
	defer applyTheme("dark")
	m := fixtureModel()
	for _, name := range themeNames {
		m.o.Theme = name
		applyTheme(name)
		for _, wh := range [][2]int{{50, 16}, {80, 24}, {96, 32}, {160, 50}} {
			m.width, m.height = wh[0], wh[1]
			for view := 0; view < 5; view++ {
				m.view = view
				lines := strings.Split(ansi.Strip(m.View()), "\n")
				top := strings.Join(lines[:min(4, len(lines))], "\n")
				if !strings.Contains(top, "Theme: "+themeLabel(name)) || !strings.Contains(top, "Ctrl+T choose") {
					t.Fatalf("missing visible theme indicator for %s at %v view %d: %s", name, wh, view, top)
				}
			}
		}
	}
}
