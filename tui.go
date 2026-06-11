package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// fzf in --disabled mode does no filtering of its own: every keystroke
// reloads `nz query <q>`, so all matching and ranking lives in Go.
func runTUI(initial string) error {
	self, err := os.Executable()
	if err != nil {
		self = "nz"
	}
	reload := fmt.Sprintf("reload:%s query {q}", self)

	cmd := exec.Command("fzf",
		"--disabled",
		"--print-query",
		"--delimiter", "\t",
		"--with-nth", "3..",
		"--bind", "start:"+reload,
		"--bind", "change:"+reload,
		"--layout", "reverse",
		"--info", "hidden",
		"--no-separator",
		"--prompt", "❯ ",
		"--pointer", "▌",
		"--highlight-line",
	)
	if initial != "" {
		cmd.Args = append(cmd.Args, "--query", initial)
	}
	cmd.Stdin = strings.NewReader("")
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil // dismissed with escape, or no match selected
	}

	// with --print-query: first line is the typed query, second the selection
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(lines) < 2 {
		return nil
	}
	query := lines[0]
	parts := strings.SplitN(lines[1], "\t", 3)
	if len(parts) < 3 || parts[0] == "" {
		return nil
	}

	if err := recordLaunch(parts[0], query); err != nil {
		return err
	}
	return launch(parts[1])
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
