package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"regexp"

	"github.com/tuklu/nZlauncher/internal/identity"
	"github.com/tuklu/nZlauncher/internal/shell"
)

// SUPER+SPACE toggles the launcher: close it if open, otherwise spawn.
func runBar(initial string) error {
	self, err := os.Executable()
	if err != nil {
		self = "nzlauncher"
	}

	// no leading dashes, or pkill reads the pattern as an option
	killed := exec.Command("pkill", "-f", "class="+regexp.QuoteMeta(identity.TUIClass)).Run() == nil
	// anchored to our own binary so an unrelated command line never matches
	if exec.Command("pkill", "-f", "^"+regexp.QuoteMeta(self)+" gtk").Run() == nil {
		killed = true
	}
	if killed {
		return nil
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

	var cmd *exec.Cmd
	if loadSettings().UI() == "gtk" {
		cmd = exec.Command(self, "gtk", initial)
	} else {
		cmd = exec.Command("ghostty",
			"--class="+identity.TUIClass,
			"--gtk-single-instance=false",
			"--window-padding-x=12",
			"--window-padding-y=10",
			"-e", self, "tui", initial)
	}
	cmd.Env = append(os.Environ(), "NZ_FOCUSED="+focused)
	return shell.StartDetached(cmd)
}
