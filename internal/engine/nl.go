package engine

import (
	"strings"

	"github.com/tuklu/nZlauncher/internal/action"
	"github.com/tuklu/nZlauncher/internal/learn"
	"github.com/tuklu/nZlauncher/internal/providers"
	"github.com/tuklu/nZlauncher/internal/shell"
)

// Natural language verb layer: deterministic grammar, no model.
// "set wallpaper to fg1", "turn off wifi", "open chromium", "connect mx master",
// "switch to workspace 3", "search rust traits", "find catalog".
// Filler words vanish, the verb routes, the rest is the argument.

var fillerWords = map[string]bool{
	"the": true, "a": true, "an": true, "my": true, "please": true,
	"to": true, "for": true, "of": true, "in": true, "me": true,
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}

func nlIntent(actions []action.Action, stats map[string]learn.Stat, ctx queryContext, q string) []action.Action {
	var words []string
	for _, w := range strings.Fields(q) {
		if !fillerWords[w] {
			words = append(words, w)
		}
	}
	if len(words) < 2 {
		return nil
	}
	verb, rest := words[0], words[1:]
	arg := strings.Join(rest, " ")

	switch verb {
	case "set", "change", "make":
		if fn := domainProvider(rest[0]); fn != nil {
			return fn(strings.Join(rest[1:], " "))
		}

	case "open", "launch", "start":
		pool := keepGroups(append(actions, cache.windowList()...), "app", "window")
		res := rank(pool, stats, ctx, arg)
		if u, ok := urlAction(arg); ok {
			res = append(res, u)
		}
		return res

	case "switch", "go", "goto", "focus":
		if rest[0] == "workspace" || rest[0] == "ws" {
			return providers.Workspace(strings.Join(rest[1:], " "))
		}
		return rank(cache.windowList(), stats, ctx, arg)

	case "connect", "disconnect":
		// device entries from both radios; state decides connect vs disconnect
		res := keepContaining(append(providers.Bluetooth(arg), providers.Wifi(arg)...), "connect")
		return res

	case "turn":
		if rest[0] == "on" || rest[0] == "off" {
			return toggleIntent(rest[0], strings.Join(rest[1:], " "), actions)
		}
		if last := rest[len(rest)-1]; last == "on" || last == "off" {
			return toggleIntent(last, strings.Join(rest[:len(rest)-1], " "), actions)
		}

	case "search", "google":
		return []action.Action{searchAction(arg, ctx.settings)}

	case "find":
		return providers.Files(arg)

	case "kill", "stop":
		return providers.Kill(arg)

	case "remind", "remember":
		if isDigits(rest[0]) {
			mins, msg := rest[0], strings.Join(rest[1:], " ")
			name := "Remind in " + mins + "m"
			if msg != "" {
				name += ": " + msg
			}
			return []action.Action{{ID: "remind", Name: name, Icon: "󰔛", Group: "info",
				Cmd: "reminder " + mins + " " + shell.Quote(msg)}}
		}
		return []action.Action{{ID: "set-reminder", Name: "Set Reminder…", Icon: "󰔛", Group: "info",
			Cmd: "reminder prompt"}}
	}
	return nil
}

func domainProvider(domain string) func(string) []action.Action {
	switch domain {
	case "wallpaper", "background":
		return providers.Wallpaper
	case "volume", "sound":
		return providers.Volume
	case "brightness":
		return providers.Brightness
	case "workspace", "ws":
		return providers.Workspace
	case "output", "audio", "speaker":
		return providers.Audio
	case "wifi", "network":
		return providers.Wifi
	case "bluetooth", "bt":
		return providers.Bluetooth
	case "monitor", "display":
		return providers.Monitor
	}
	return nil
}

func toggleIntent(state, domain string, actions []action.Action) []action.Action {
	if fn := domainProvider(domain); fn != nil {
		return keepContaining(fn(""), state)
	}
	// catalog toggles: nightlight, idle, screensaver, bar...
	var res []action.Action
	for _, a := range actions {
		if a.Group == "toggle" && strings.Contains(strings.ToLower(a.Name), domain) {
			res = append(res, a)
		}
	}
	return res
}

func keepGroups(actions []action.Action, groups ...string) []action.Action {
	keep := map[string]bool{}
	for _, g := range groups {
		keep[g] = true
	}
	var res []action.Action
	for _, a := range actions {
		if keep[a.Group] {
			res = append(res, a)
		}
	}
	return res
}

func keepContaining(actions []action.Action, sub string) []action.Action {
	var res []action.Action
	for _, a := range actions {
		if strings.Contains(strings.ToLower(a.Name), sub) {
			res = append(res, a)
		}
	}
	return res
}
