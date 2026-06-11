package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// The TUI face: bubbletea, engine called in-process on every keystroke.
// ANSI palette colors so the terminal theme stays in charge.
var (
	accent   = lipgloss.Color("4")
	dim      = lipgloss.Color("8")
	stPrompt = lipgloss.NewStyle().Foreground(accent).Bold(true)
	stCursor = lipgloss.NewStyle().Foreground(accent)
	stDim    = lipgloss.NewStyle().Foreground(dim)
	stSel    = lipgloss.NewStyle().Bold(true)
	stMarker = lipgloss.NewStyle().Foreground(accent).Bold(true)
)

type tuiModel struct {
	query   string
	actions []Action
	cursor  int
	offset  int
	width   int
	height  int
}

func runTUI(initial string) error {
	m := &tuiModel{query: initial, width: 80, height: 14}
	m.refresh()
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func (m *tuiModel) refresh() {
	acts, err := queryActions(m.query)
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
				recordLaunch(a.ID, m.query)
				launch(a.Cmd)
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
	rows := m.height - 3
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
	b.WriteString(" " + stPrompt.Render("❯ ") + m.query + stCursor.Render("█") + "\n\n")

	if len(m.actions) == 0 {
		b.WriteString("   " + stDim.Render("nothing — esc to close"))
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
		left := icon + "  " + a.Name
		badge := a.Group

		pad := m.width - lipgloss.Width(left) - lipgloss.Width(badge) - 6
		if pad < 1 {
			over := -pad + 1
			name := []rune(a.Name)
			if len(name) > over+3 {
				left = icon + "  " + string(name[:len(name)-over-3]) + "..."
			}
			pad = m.width - lipgloss.Width(left) - lipgloss.Width(badge) - 6
			if pad < 1 {
				pad = 1
			}
		}

		line := left + strings.Repeat(" ", pad) + stDim.Render(badge)
		if i == m.cursor {
			b.WriteString(" " + stMarker.Render("▌ ") + stSel.Render(line) + "\n")
		} else {
			b.WriteString("   " + line + "\n")
		}
	}
	return b.String()
}

// SUPER+SPACE toggles the launcher: close it if open, otherwise spawn.
func runBar(initial string) error {
	if exec.Command("pkill", "-f", `class=nz\.launcher`).Run() == nil {
		return nil
	}

	if uiInterface() == "gtk" {
		// reserved: GTK layer-shell frontend not built yet
		exec.Command("notify-send", "-u", "low", "nz", "GTK frontend not built yet, using TUI").Run()
	}

	self, err := os.Executable()
	if err != nil {
		self = "nz"
	}

	// capture what was focused before the launcher steals it: context for the event log
	focused := ""
	if out, err := exec.Command("hyprctl", "activewindow", "-j").Output(); err == nil {
		var w struct {
			Class string `json:"class"`
		}
		if json.Unmarshal(out, &w) == nil {
			focused = w.Class
		}
	}

	cmd := exec.Command("ghostty",
		"--class=nz.launcher",
		"--gtk-single-instance=false",
		"--window-padding-x=12",
		"--window-padding-y=10",
		"-e", self, "tui", initial)
	cmd.Env = append(os.Environ(), "NZ_FOCUSED="+focused)
	return startDetached(cmd)
}
