// Package portable builds the command lines results run, from what this
// machine has: neZha's own scripts when they are installed, the common
// tools otherwise. A result never names a program that is not there.
package portable

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/tuklu/nZlauncher/internal/shell"
)

// Something installed while the daemon runs shows up within a minute.
const haveFor = time.Minute

var have struct {
	sync.Mutex
	at    time.Time
	found map[string]bool
}

// Have reports whether a program is on PATH.
func Have(name string) bool {
	have.Lock()
	defer have.Unlock()
	if time.Since(have.at) > haveFor {
		have.found, have.at = map[string]bool{}, time.Now()
	}
	found, seen := have.found[name]
	if !seen {
		_, err := exec.LookPath(name)
		found = err == nil
		have.found[name] = found
	}
	return found
}

// App starts a program the way the session wants: as its own unit under
// uwsm when that runs the session, directly otherwise.
func App(cmd string) string {
	if Have("uwsm-app") {
		return "uwsm-app -- " + cmd
	}
	return cmd
}

// Open opens a file or URL with its default application.
func Open(target string) string {
	return App("xdg-open " + shell.Quote(target))
}

// terminals, for when neither launch-tui, xdg-terminal-exec nor $TERMINAL
// says which one: how each runs a command
var terminals = []struct{ name, run string }{
	{"ghostty", "ghostty -e"}, {"kitty", "kitty"}, {"foot", "foot"}, {"alacritty", "alacritty -e"},
	{"wezterm", "wezterm start --"}, {"konsole", "konsole -e"}, {"gnome-terminal", "gnome-terminal --"},
	{"xterm", "xterm -e"},
}

// Terminal runs a terminal program (a TUI, or a shell line through
// "bash -c") in a new terminal window. "" when there is no terminal.
func Terminal(cmd string) string {
	switch {
	case Have("launch-tui"):
		return "launch-tui " + cmd
	case Have("xdg-terminal-exec"):
		return App("xdg-terminal-exec " + cmd)
	}
	if term := os.Getenv("TERMINAL"); term != "" && Have(strings.Fields(term)[0]) {
		return App(term + " -e " + cmd)
	}
	for _, t := range terminals {
		if Have(t.name) {
			return App(t.run + " " + cmd)
		}
	}
	return ""
}

// VolumeStep raises or lowers the output volume by percent (negative
// lowers), with an on-screen display when one is installed.
func VolumeStep(percent int) string {
	sign := "+"
	if percent < 0 {
		sign, percent = "-", -percent
	}
	switch {
	case Have("osd"):
		return fmt.Sprintf("osd --output-volume %s%d", sign, percent)
	case Have("swayosd-client"):
		return fmt.Sprintf("swayosd-client --output-volume %s%d", sign, percent)
	case Have("wpctl"):
		return fmt.Sprintf("wpctl set-volume -l 1.0 @DEFAULT_AUDIO_SINK@ %d%%%s", percent, sign)
	}
	return fmt.Sprintf("pactl set-sink-volume @DEFAULT_SINK@ %s%d%%", sign, percent)
}

// Volume sets the output volume to percent.
func Volume(percent int) string {
	if Have("wpctl") {
		return fmt.Sprintf("wpctl set-volume @DEFAULT_AUDIO_SINK@ %d%%", percent)
	}
	return fmt.Sprintf("pactl set-sink-volume @DEFAULT_SINK@ %d%%", percent)
}

// Mute mutes (true) or unmutes the output.
func Mute(on bool) string {
	word, flag := "unmute", "0"
	if on {
		word, flag = "mute", "1"
	}
	switch {
	case Have("osd"):
		return "osd --output-volume " + word
	case Have("swayosd-client"):
		return "swayosd-client --output-volume " + word
	case Have("wpctl"):
		return "wpctl set-mute @DEFAULT_AUDIO_SINK@ " + flag
	}
	return "pactl set-sink-mute @DEFAULT_SINK@ " + flag
}

// Brightness sets the screen brightness: "40" to a level, "+10" or "-10"
// by a step, in percent.
func Brightness(level string) string {
	step := strings.HasPrefix(level, "+") || strings.HasPrefix(level, "-")
	switch {
	case Have("brightness"):
		// neZha's script picks the focused monitor, backlight or DDC
		if step {
			return "brightness " + level[1:] + "%" + level[:1]
		}
		return "brightness " + level + "%"
	case step && Have("swayosd-client"):
		return "swayosd-client --brightness " + level
	case step:
		return "brightnessctl set " + level[1:] + "%" + level[:1]
	}
	return "brightnessctl set " + level + "%"
}

// CanSetBrightness reports whether anything here sets the backlight.
func CanSetBrightness() bool {
	return Have("brightness") || Have("brightnessctl") || Have("swayosd-client")
}

// Remind notifies after minutes, with neZha's reminder script (which lists
// and clears them too) or a systemd user timer.
func Remind(minutes int, text string) string {
	if Have("reminder") {
		return fmt.Sprintf("reminder %d %s", minutes, shell.Quote(text))
	}
	title := "Reminder"
	if text == "" {
		text = fmt.Sprintf("%d minutes are up", minutes)
	}
	return fmt.Sprintf("systemd-run --user --quiet --on-active=%dm --timer-property=AccuracySec=1s notify-send -u critical %s %s && notify-send -u low %s",
		minutes, shell.Quote(title), shell.Quote(text), shell.Quote(fmt.Sprintf("Reminder in %d minutes", minutes)))
}

// CanRemind reports whether reminders can be set here.
func CanRemind() bool {
	return Have("reminder") || Have("systemd-run") && Have("notify-send")
}
