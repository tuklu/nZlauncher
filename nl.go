package main

import "strings"

// Natural language verb layer: deterministic grammar, no model.
// "set wallpaper to fg1", "turn off wifi", "open chromium", "connect mx master",
// "switch to workspace 3", "search rust traits", "find catalog".
// Filler words vanish, the verb routes, the rest is the argument.

var fillerWords = map[string]bool{
	"the": true, "a": true, "an": true, "my": true, "please": true,
	"to": true, "for": true, "of": true, "in": true,
}

func nlIntent(actions []Action, stats map[string]stat, ctx queryContext, q string) []Action {
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
		pool := keepGroups(append(actions, windowActions()...), "app", "window")
		res := rank(pool, stats, ctx, arg)
		if u, ok := urlAction(arg); ok {
			res = append(res, u)
		}
		return res

	case "switch", "go", "goto", "focus":
		if rest[0] == "workspace" || rest[0] == "ws" {
			return workspaceActions(strings.Join(rest[1:], " "))
		}
		return rank(windowActions(), stats, ctx, arg)

	case "connect", "disconnect":
		// device entries from both radios; state decides connect vs disconnect
		res := keepContaining(append(bluetoothActions(arg), wifiActions(arg)...), "connect")
		return res

	case "turn":
		if rest[0] == "on" || rest[0] == "off" {
			return toggleIntent(rest[0], strings.Join(rest[1:], " "), actions)
		}
		if last := rest[len(rest)-1]; last == "on" || last == "off" {
			return toggleIntent(last, strings.Join(rest[:len(rest)-1], " "), actions)
		}

	case "search", "google":
		return []Action{searchAction(arg)}

	case "find":
		return fileActions(arg)

	case "kill", "stop":
		return killActions(arg)
	}
	return nil
}

func domainProvider(domain string) func(string) []Action {
	switch domain {
	case "wallpaper", "background":
		return wallpaperActions
	case "volume", "sound":
		return volumeActions
	case "brightness":
		return brightnessActions
	case "workspace", "ws":
		return workspaceActions
	case "output", "audio", "speaker":
		return audioActions
	case "wifi", "network":
		return wifiActions
	case "bluetooth", "bt":
		return bluetoothActions
	case "monitor", "display":
		return monitorActions
	}
	return nil
}

func toggleIntent(state, domain string, actions []Action) []Action {
	if fn := domainProvider(domain); fn != nil {
		return keepContaining(fn(""), state)
	}
	// catalog toggles: nightlight, idle, screensaver, bar...
	var res []Action
	for _, a := range actions {
		if a.Group == "toggle" && strings.Contains(strings.ToLower(a.Name), domain) {
			res = append(res, a)
		}
	}
	return res
}

func keepGroups(actions []Action, groups ...string) []Action {
	keep := map[string]bool{}
	for _, g := range groups {
		keep[g] = true
	}
	var res []Action
	for _, a := range actions {
		if keep[a.Group] {
			res = append(res, a)
		}
	}
	return res
}

func keepContaining(actions []Action, sub string) []Action {
	var res []Action
	for _, a := range actions {
		if strings.Contains(strings.ToLower(a.Name), sub) {
			res = append(res, a)
		}
	}
	return res
}
