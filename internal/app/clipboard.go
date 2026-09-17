package app

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type copiedSessionMsg struct{ err error }

func (m *model) copySessionCmd() tea.Cmd {
	if m.view != 4 || m.copying || m.info != "" || m.err != "" || m.comparing {
		return nil
	}
	rows := m.rows()
	if m.cursor < 0 || m.cursor >= len(rows) || rows[m.cursor].Name == "" {
		m.notice = "No session name to copy"
		return nil
	}
	// Capture the full identity before navigation, filtering, or refresh changes it.
	name := rows[m.cursor].Name
	if strings.ContainsRune(name, '\x00') {
		m.notice = "Copy failed: session name contains a NUL character"
		return nil
	}
	write := m.clipboardWrite
	if write == nil {
		write = writeClipboard
	}
	ctx := m.ctx
	m.copying = true
	m.notice = "Copying session name…"
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		return copiedSessionMsg{err: write(ctx, name)}
	}
}
