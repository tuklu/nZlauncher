// Package portable builds the command lines results run, from what this
// machine has. Each one is settled in three steps: the [commands] table in
// config.toml when it names one, then neZha's script or a common tool that
// is installed, then a fixed fallback. A result never names a program that
// is not there unless the user said so.
package portable

import (
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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

var overrides atomic.Pointer[map[string]string]

// Configure sets the [commands] table from config.toml.
func Configure(commands map[string]string) {
	overrides.Store(&commands)
}

// Keys are the commands [commands] can set, with the placeholders each takes.
var Keys = map[string]string{
	"app":             "{cmd}",
	"open":            "{target}",
	"terminal":        "{cmd}",
	"volume":          "{n}",
	"volume_up":       "{n}",
	"volume_down":     "{n}",
	"mute":            "",
	"unmute":          "",
	"brightness":      "{n}",
	"brightness_up":   "{n}",
	"brightness_down": "{n}",
	"remind":          "{minutes} {text}",
}

// A choice is one way to run a command: needs is the program that has to
// be installed for it ("" for none).
type choice struct {
	needs string
	cmd   string
}

// vars fill a template. {cmd} is a shell line already, numbers are numbers;
// {target} and {text} are quoted.
type vars map[string]string

func expand(template string, v vars, quote bool) string {
	for k, val := range v {
		if quote && (k == "target" || k == "text") {
			val = shell.Quote(val)
		}
		template = strings.ReplaceAll(template, "{"+k+"}", val)
	}
	return template
}

// resolve settles key: the user's template, else the first installed
// choice, else the last one. source says which won.
func resolve(key string, v vars, quote bool, choices ...choice) (cmd, source string) {
	if o := overrides.Load(); o != nil {
		if template, ok := (*o)[key]; ok && template != "" {
			return expand(template, v, quote), "config.toml"
		}
	}
	for _, c := range choices {
		if c.needs == "" || Have(c.needs) {
			src := c.needs
			if src == "" {
				src = "fallback"
			}
			return expand(c.cmd, v, quote), src
		}
	}
	if len(choices) == 0 {
		return "", ""
	}
	last := choices[len(choices)-1]
	return expand(last.cmd, v, quote), "fallback, " + last.needs + " not installed"
}

func number(n int) vars { return vars{"n": strconv.Itoa(n)} }

// The choices per key, in order. Built per call: $TERMINAL can change.
func choicesFor(key string) []choice {
	switch key {
	case "app":
		return []choice{{"uwsm-app", "uwsm-app -- {cmd}"}, {"", "{cmd}"}}
	case "open":
		return []choice{{"uwsm-app", "uwsm-app -- xdg-open {target}"}, {"", "xdg-open {target}"}}
	case "terminal":
		cs := []choice{{"launch-tui", "launch-tui {cmd}"}, {"xdg-terminal-exec", "xdg-terminal-exec {cmd}"}}
		if term := os.Getenv("TERMINAL"); term != "" {
			cs = append(cs, choice{strings.Fields(term)[0], term + " -e {cmd}"})
		}
		for _, t := range terminals {
			cs = append(cs, choice{t.name, t.run + " {cmd}"})
		}
		return cs
	case "volume":
		return []choice{{"wpctl", "wpctl set-volume @DEFAULT_AUDIO_SINK@ {n}%"}, {"pactl", "pactl set-sink-volume @DEFAULT_SINK@ {n}%"}}
	case "volume_up", "volume_down":
		sign, wp := "+", "+"
		if key == "volume_down" {
			sign, wp = "-", "-"
		}
		return []choice{
			{"osd", "osd --output-volume " + sign + "{n}"},
			{"swayosd-client", "swayosd-client --output-volume " + sign + "{n}"},
			{"wpctl", "wpctl set-volume -l 1.0 @DEFAULT_AUDIO_SINK@ {n}%" + wp},
			{"pactl", "pactl set-sink-volume @DEFAULT_SINK@ " + sign + "{n}%"},
		}
	case "mute", "unmute":
		flag := map[string]string{"mute": "1", "unmute": "0"}[key]
		return []choice{
			{"osd", "osd --output-volume " + key},
			{"swayosd-client", "swayosd-client --output-volume " + key},
			{"wpctl", "wpctl set-mute @DEFAULT_AUDIO_SINK@ " + flag},
			{"pactl", "pactl set-sink-mute @DEFAULT_SINK@ " + flag},
		}
	case "brightness":
		// neZha's script picks the focused monitor, backlight or DDC
		return []choice{{"brightness", "brightness {n}%"}, {"brightnessctl", "brightnessctl set {n}%"}}
	case "brightness_up", "brightness_down":
		sign := "+"
		if key == "brightness_down" {
			sign = "-"
		}
		return []choice{
			{"brightness", "brightness {n}%" + sign},
			{"swayosd-client", "swayosd-client --brightness " + sign + "{n}"},
			{"brightnessctl", "brightnessctl set {n}%" + sign},
		}
	case "remind":
		return []choice{
			{"reminder", "reminder {minutes} {text}"},
			{"systemd-run", "systemd-run --user --quiet --on-active={minutes}m --timer-property=AccuracySec=1s " +
				"notify-send -u critical Reminder {text} && notify-send -u low \"Reminder in {minutes} minutes\""},
		}
	}
	return nil
}

func command(key string, v vars) string {
	cmd, _ := resolve(key, v, true, choicesFor(key)...)
	return cmd
}

// terminals, for when neither launch-tui, xdg-terminal-exec nor $TERMINAL
// says which one: how each runs a command
var terminals = []struct{ name, run string }{
	{"ghostty", "ghostty -e"}, {"kitty", "kitty"}, {"foot", "foot"}, {"alacritty", "alacritty -e"},
	{"wezterm", "wezterm start --"}, {"konsole", "konsole -e"}, {"gnome-terminal", "gnome-terminal --"},
	{"xterm", "xterm -e"},
}

// App starts a program the way the session wants: as its own unit under
// uwsm when that is installed, directly otherwise.
func App(cmd string) string {
	return command("app", vars{"cmd": cmd})
}

// Open opens a file or URL with its default application.
func Open(target string) string {
	return command("open", vars{"target": target})
}

// Terminal runs a terminal program (a TUI, or a shell line through
// "bash -c") in a new terminal window. "" when there is no terminal.
func Terminal(cmd string) string {
	line, source := resolve("terminal", vars{"cmd": cmd}, true, choicesFor("terminal")...)
	if strings.HasPrefix(source, "fallback") {
		return ""
	}
	return line
}

// VolumeStep raises or lowers the output volume by percent (negative lowers).
func VolumeStep(percent int) string {
	if percent < 0 {
		return command("volume_down", number(-percent))
	}
	return command("volume_up", number(percent))
}

// Volume sets the output volume to percent.
func Volume(percent int) string {
	return command("volume", number(percent))
}

// Mute mutes (true) or unmutes the output.
func Mute(on bool) string {
	if on {
		return command("mute", nil)
	}
	return command("unmute", nil)
}

// Brightness sets the screen brightness: "40" to a level, "+10" or "-10"
// by a step, in percent.
func Brightness(level string) string {
	switch {
	case strings.HasPrefix(level, "+"):
		return command("brightness_up", vars{"n": level[1:]})
	case strings.HasPrefix(level, "-"):
		return command("brightness_down", vars{"n": level[1:]})
	}
	return command("brightness", vars{"n": level})
}

// Remind notifies after minutes.
func Remind(minutes int, text string) string {
	if text == "" {
		text = strconv.Itoa(minutes) + " minutes are up"
	}
	return command("remind", vars{"minutes": strconv.Itoa(minutes), "text": text})
}

// CanRemind reports whether reminders can be set here.
func CanRemind() bool {
	if o := overrides.Load(); o != nil && (*o)["remind"] != "" {
		return true
	}
	return Have("reminder") || Have("systemd-run") && Have("notify-send")
}

// Row is one command as `nzlauncher setup` shows it.
type Row struct {
	Key, Command, Source string
}

// Explain lists every command with its placeholders and what decided it.
func Explain() []Row {
	keys := make([]string, 0, len(Keys))
	for k := range Keys {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var rows []Row
	for _, k := range keys {
		v := vars{"cmd": "{cmd}", "target": "{target}", "n": "{n}", "minutes": "{minutes}", "text": "{text}"}
		cmd, source := resolve(k, v, false, choicesFor(k)...)
		rows = append(rows, Row{k, cmd, source})
	}
	return rows
}
