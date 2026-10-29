package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"

	"github.com/tuklu/nZlauncher/internal/hypr"
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
	gtkBin, gtkErr := gtkBinary()
	// anchored to the binary path so an unrelated command line never matches
	if gtkErr == nil && exec.Command("pkill", "-f", "^"+regexp.QuoteMeta(gtkBin)+"( |$)").Run() == nil {
		killed = true
	}
	if killed {
		return nil
	}

	// capture what was focused before the launcher steals it: context for the event log
	focused := ""
	if out, err := hypr.JSON("activewindow"); err == nil {
		var w struct {
			Class string `json:"class"`
		}
		if json.Unmarshal(out, &w) == nil {
			focused = w.Class
		}
	}

	var cmd *exec.Cmd
	if loadSettings().UI() == "gtk" {
		if gtkErr != nil {
			return fmt.Errorf("interface = \"gtk\" needs nzlauncher-gtk: %w", gtkErr)
		}
		cmd = exec.Command(gtkBin, initial)
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
