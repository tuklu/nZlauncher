package main

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/lipgloss"
)

// TUI theme, loaded from ~/.config/neZha/theme/nz-tui.toml when present.
// Colors accept ANSI indexes ("4"), 256-palette ints ("212"), or hex ("#89b4fa").
type tuiTheme struct {
	Accent          string `toml:"accent"`
	Text            string `toml:"text"`
	Muted           string `toml:"muted"`
	Border          string `toml:"border"`
	SelectionFg     string `toml:"selection_fg"`
	SelectionBg     string `toml:"selection_bg"`
	PromptIcon      string `toml:"prompt_icon"`
	PlaceholderText string `toml:"placeholder_text"`
}

func loadTheme() tuiTheme {
	t := tuiTheme{
		Accent:          "4",
		Muted:           "8",
		Border:          "8",
		SelectionFg:     "0",
		SelectionBg:     "4",
		PromptIcon:      "󰍉",
		PlaceholderText: "Search anything…",
	}
	path := filepath.Join(os.Getenv("HOME"), ".config/neZha/theme/nz-tui.toml")
	toml.DecodeFile(path, &t) // missing or partial file just keeps defaults
	return t
}

type tuiStyles struct {
	border      lipgloss.Style
	text        lipgloss.Style
	placeholder lipgloss.Style
	cursor      lipgloss.Style
	badge       lipgloss.Style
	selRow      lipgloss.Style
	marker      lipgloss.Style
	footer      lipgloss.Style

	promptIcon      string
	placeholderText string
}

func buildStyles(t tuiTheme) tuiStyles {
	text := lipgloss.NewStyle()
	if t.Text != "" {
		text = text.Foreground(lipgloss.Color(t.Text))
	}
	sel := lipgloss.NewStyle().Bold(true)
	if t.SelectionBg != "" {
		sel = sel.Background(lipgloss.Color(t.SelectionBg))
	}
	if t.SelectionFg != "" {
		sel = sel.Foreground(lipgloss.Color(t.SelectionFg))
	}

	return tuiStyles{
		border:      lipgloss.NewStyle().Foreground(lipgloss.Color(t.Border)),
		text:        text,
		placeholder: lipgloss.NewStyle().Foreground(lipgloss.Color(t.Muted)),
		cursor:      lipgloss.NewStyle().Foreground(lipgloss.Color(t.Accent)),
		badge:       lipgloss.NewStyle().Foreground(lipgloss.Color(t.Muted)),
		selRow:      sel,
		marker:      lipgloss.NewStyle().Foreground(lipgloss.Color(t.Accent)).Bold(true),
		footer:      lipgloss.NewStyle().Foreground(lipgloss.Color(t.Muted)),

		promptIcon:      t.PromptIcon,
		placeholderText: t.PlaceholderText,
	}
}
