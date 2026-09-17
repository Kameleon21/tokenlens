package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func copyKey(m model) (model, tea.Cmd) {
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	return next.(model), cmd
}

func TestCopySelectedSession(t *testing.T) {
	for _, details := range []bool{false, true} {
		m := fixtureModel()
		m.view, m.details = 4, details
		name := strings.Repeat("long session ", 30) + "日本語 🦎"
		m.s.Sections["session"] = []Row{
			{Name: "excluded", Agent: "other"},
			{Name: name, Agent: "codex", Models: []Row{{Name: "wanted"}}},
			{Name: "a-first", Agent: "codex", Models: []Row{{Name: "wanted"}}},
			{Name: "excluded-model", Agent: "codex", Models: []Row{{Name: "other"}}},
		}
		m.agent, m.modelFilter = "codex", "wanted"
		m.o.preferences.SessionsSort, m.cursor = "name_asc", 1
		var got string
		m.clipboardWrite = func(ctx context.Context, s string) error {
			if _, ok := ctx.Deadline(); !ok {
				t.Error("missing clipboard deadline")
			}
			got = s
			return nil
		}
		next, cmd := copyKey(m)
		if cmd == nil || !next.copying || got != "" {
			t.Fatal("copy must be asynchronous and mark itself pending")
		}
		if _, duplicate := copyKey(next); duplicate != nil {
			t.Fatal("overlapping writes allowed")
		}
		next.cursor = 0
		result := cmd()
		if got != name {
			t.Fatalf("copied %q, want full selected name %q", got, name)
		}
		updated, _ := next.Update(result)
		next = updated.(model)
		if next.copying || next.notice != "Session name copied" {
			t.Fatalf("bad success state: %q", next.notice)
		}
	}
}

func TestCopySessionGuards(t *testing.T) {
	cases := map[string]func(*model){
		"other tab":    func(m *model) { m.view = 2 },
		"help":         func(m *model) { m.help = true },
		"export":       func(m *model) { m.exporting = true },
		"date input":   func(m *model) { m.editing = "range"; m.input.Focus() },
		"theme search": func(m *model) { m.openThemePicker() },
		"info":         func(m *model) { m.info = "info" },
		"error":        func(m *model) { m.err = "error" },
		"empty":        func(m *model) { m.s.Sections["session"] = nil },
		"past end":     func(m *model) { m.cursor = 9999 },
		"negative":     func(m *model) { m.cursor = -1 },
		"empty name":   func(m *model) { m.s.Sections["session"] = []Row{{}} },
		"NUL":          func(m *model) { m.s.Sections["session"] = []Row{{Name: "bad\x00name"}} },
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			m := fixtureModel()
			m.view = 4
			setup(&m)
			m.clipboardWrite = func(context.Context, string) error { t.Fatal("unexpected clipboard write"); return nil }
			next, _ := copyKey(m)
			if next.copying {
				t.Fatal("copy initiated outside a selected session")
			}
			if name == "date input" && next.input.Value() != "y" {
				t.Fatal("shortcut swallowed typed text")
			}
			if name == "theme search" && next.themeQuery.Value() != "y" {
				t.Fatal("shortcut swallowed theme search")
			}
		})
	}
}

func TestCopyFailureAndRendering(t *testing.T) {
	m := fixtureModel()
	m.view = 4
	m.clipboardWrite = func(context.Context, string) error { return errors.New("clipboard unavailable") }
	m, cmd := copyKey(m)
	updated, _ := m.Update(cmd())
	m = updated.(model)
	if m.copying || m.notice != "Copy failed: clipboard unavailable" {
		t.Fatal(m.notice)
	}
	for _, size := range [][2]int{{50, 16}, {80, 24}, {160, 50}} {
		m.width, m.height = size[0], size[1]
		for _, notice := range []string{"Copy failed: clipboard unavailable", "Session name copied"} {
			m.notice = notice
			output := ansi.Strip(m.View())
			if !strings.Contains(output, notice) || !strings.Contains(output, "y Copy name") {
				t.Fatalf("missing feedback at %v: %s", size, output)
			}
			if len(strings.Split(output, "\n")) > m.height {
				t.Fatal("height overflow")
			}
			for _, line := range strings.Split(output, "\n") {
				if ansi.StringWidth(line) > m.width {
					t.Fatal("width overflow")
				}
			}
		}
	}
	dismissed, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if dismissed.(model).notice != "" {
		t.Fatal("Escape did not dismiss notice")
	}
	if !strings.Contains(m.helpText(), "y (Sessions)") {
		t.Fatal("copy missing from help")
	}
}
