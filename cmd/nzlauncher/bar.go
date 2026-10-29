package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/tuklu/nZlauncher/internal/config"
	"github.com/tuklu/nZlauncher/internal/hypr"
	"github.com/tuklu/nZlauncher/internal/identity"
	"github.com/tuklu/nZlauncher/internal/server"
	"github.com/tuklu/nZlauncher/internal/shell"
)

// SUPER+SPACE toggles the launcher: close it if open, otherwise open it.
func runBar(initial string) error {
	self, err := os.Executable()
	if err != nil {
		self = "nzlauncher"
	}

	if loadSettings().UI() == "gtk" {
		// a resident face shows and hides itself; only the first press starts it
		focused := focusedClass()
		if server.ToggleFace(initial, focused) == nil {
			return nil
		}
		gtkBin, err := gtkBinary()
		if err != nil {
			return fmt.Errorf("interface = \"gtk\" needs nzlauncher-gtk: %w", err)
		}
		cmd := exec.Command(gtkBin, "--resident", initial)
		cmd.Env = append(os.Environ(), "NZ_FOCUSED="+focused)
		return shell.StartDetached(cmd)
	}

	// no leading dashes, or pkill reads the pattern as an option
	if exec.Command("pkill", "-f", "class="+regexp.QuoteMeta(identity.TUIClass)).Run() == nil {
		return nil
	}
	cmd := exec.Command("ghostty",
		"--class="+identity.TUIClass,
		"--gtk-single-instance=false",
		solidConfig(),
		"--window-padding-x=12",
		"--window-padding-y=10",
		"-e", self, "tui", initial)
	cmd.Env = append(os.Environ(), "NZ_FOCUSED="+focusedClass())
	return shell.StartDetached(cmd)
}

// The window the user came from, captured before the launcher takes focus:
// context for ranking and the event log.
func focusedClass() string {
	out, err := hypr.JSON("activewindow")
	if err != nil {
		return ""
	}
	var w struct {
		Class string `json:"class"`
	}
	json.Unmarshal(out, &w)
	return w.Class
}

// solidConfig makes the launcher's terminal window solid, whatever the
// terminal theme says: text behind a launcher makes it unreadable. A plain
// --background-opacity loses to a theme pulled in with config-file, which
// Ghostty reads after the flags; a config-file given here is read after the
// theme, so it wins.
func solidConfig() string {
	path := filepath.Join(config.CacheDir(), "ghostty-solid.conf")
	const body = "background-opacity = 1\n"
	if current, err := os.ReadFile(path); err != nil || string(current) != body {
		os.MkdirAll(filepath.Dir(path), 0o700)
		os.WriteFile(path, []byte(body), 0o600)
	}
	return "--config-file=" + path
}
