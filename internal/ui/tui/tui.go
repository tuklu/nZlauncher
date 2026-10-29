// Package tui is the terminal face: bubbletea, engine called in-process
// on every keystroke.
package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/tuklu/nZlauncher/internal/action"
	"github.com/tuklu/nZlauncher/internal/learn"
	"github.com/tuklu/nZlauncher/internal/server"
	"github.com/tuklu/nZlauncher/internal/shell"
)

// Look is driven by ~/.config/nzlauncher/theme/nz-tui.toml.
type tuiModel struct {
	query   string
	actions []action.Action
	cursor  int
	offset  int
	width   int
	height  int
	st      tuiStyles
	picked  string // command to run once the window is gone
}

func Run(initial string) error {
	m := &tuiModel{query: initial, width: 80, height: 16, st: buildStyles(loadTheme())}
	m.refresh()
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		return err
	}
	if m.picked != "" {
		return shell.LaunchAfterClose(m.picked)
	}
	return nil
}

func (m *tuiModel) refresh() {
	acts, err := server.Query(m.query, os.Getenv("NZ_FOCUSED"))
	if err != nil {
		acts = nil
	}
	m.actions = acts
	m.cursor = 0
	m.offset = 0
}

func (m *tuiModel) Init() tea.Cmd { return nil }

func (m *tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			if m.cursor < len(m.actions) {
				a := m.actions[m.cursor]
				learn.Record(a.ID, m.query)
				m.picked = a.Cmd
			}
			return m, tea.Quit
		case tea.KeyUp, tea.KeyCtrlP, tea.KeyCtrlK:
			if m.cursor > 0 {
				m.cursor--
			}
		case tea.KeyDown, tea.KeyCtrlN, tea.KeyCtrlJ:
			if m.cursor < len(m.actions)-1 {
				m.cursor++
			}
		case tea.KeyBackspace:
			if r := []rune(m.query); len(r) > 0 {
				m.query = string(r[:len(r)-1])
				m.refresh()
			}
		case tea.KeyCtrlU:
			m.query = ""
			m.refresh()
		case tea.KeySpace:
			m.query += " "
			m.refresh()
		case tea.KeyRunes:
			m.query += string(msg.Runes)
			m.refresh()
		}
		m.clamp()
	}
	return m, nil
}

func (m *tuiModel) visibleRows() int {
	rows := m.height - 6 // input box (3) + blank (1) + footer (2)
	if rows < 1 {
		rows = 1
	}
	return rows
}

func (m *tuiModel) clamp() {
	rows := m.visibleRows()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+rows {
		m.offset = m.cursor - rows + 1
	}
}

func (m *tuiModel) View() string {
	var b strings.Builder
	w := m.width - 2

	// search box
	content := m.st.cursor.Render(m.st.promptIcon) + "  "
	if m.query == "" {
		content += m.st.cursor.Render("█") + " " + m.st.placeholder.Render(m.st.placeholderText)
	} else {
		content += m.st.text.Render(m.query) + m.st.cursor.Render("█")
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.st.border.GetForeground()).
		Padding(0, 1).
		Width(w - 2).
		Render(content)
	b.WriteString(lipgloss.NewStyle().MarginLeft(1).Render(box) + "\n")

	if len(m.actions) == 0 {
		b.WriteString("\n   " + m.st.placeholder.Render("nothing here — keep typing or esc"))
		return b.String()
	}

	rows := m.visibleRows()
	end := m.offset + rows
	if end > len(m.actions) {
		end = len(m.actions)
	}

	for i := m.offset; i < end; i++ {
		a := m.actions[i]

		icon := a.Icon
		if icon == "" {
			icon = " "
		}
		name := a.Name
		badge := a.Badge()

		// room: marker(3) + icon col + gaps + badge + right margin
		avail := w - lipgloss.Width(icon) - lipgloss.Width(badge) - 9
		if lipgloss.Width(name) > avail && avail > 3 {
			r := []rune(name)
			cut := len(r)
			for cut > 0 && lipgloss.Width(string(r[:cut])) > avail-1 {
				cut--
			}
			name = string(r[:cut]) + "…"
		}

		pad := w - lipgloss.Width(icon) - lipgloss.Width(name) - lipgloss.Width(badge) - 8
		if pad < 1 {
			pad = 1
		}
		row := " " + icon + "  " + name + strings.Repeat(" ", pad) + badge + " "

		if i == m.cursor {
			b.WriteString(" " + m.st.marker.Render("▌") + m.st.selRow.Render(row) + "\n")
		} else {
			styled := " " + icon + "  " + m.st.text.Render(name) +
				strings.Repeat(" ", pad) + m.st.badge.Render(badge) + " "
			b.WriteString("  " + styled + "\n")
		}
	}

	// footer
	count := len(m.actions)
	left := "  "
	if count == 1 {
		left += "1 result"
	} else {
		left += fmt.Sprintf("%d results", count)
	}
	hints := "↑↓ navigate   ↵ run   esc close  "
	fpad := w - lipgloss.Width(left) - lipgloss.Width(hints)
	if fpad < 1 {
		fpad = 1
	}
	b.WriteString("\n" + m.st.footer.Render(left+strings.Repeat(" ", fpad)+hints))
	return b.String()
}
