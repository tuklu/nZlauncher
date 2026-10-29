package engine

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/tuklu/nZlauncher/internal/action"
	"github.com/tuklu/nZlauncher/internal/intent"
	"github.com/tuklu/nZlauncher/internal/learn"
	"github.com/tuklu/nZlauncher/internal/providers"
	"github.com/tuklu/nZlauncher/internal/shell"
)

// Natural language verb layer: deterministic grammar, no model.
// "set wallpaper to fg1", "wifi off", "disable bluetooth", "launch chromium",
// "connect mx master", "go to workspace 3", "volume up 10", "louder",
// "remind me in 20 minutes to stretch". intent.Words resolves synonyms and
// typos; the first word routes, the rest is the argument as typed.

func nlIntent(actions []action.Action, stats map[string]learn.Stat, ctx queryContext, q string) []action.Action {
	all := intent.Words(q)
	words := intent.Content(all)
	if len(words) == 0 {
		return nil
	}
	verb, rest := words[0].Canon, words[1:]
	arg := intent.RawJoin(rest)

	// a level word routes with or without "set": "volume 40%", "louder"
	if verb == "volume" || verb == "brightness" {
		if level := levelArg(rest); level != "" {
			return domainProvider(verb)(level)
		}
	}
	// "wifi off", "off wifi", "turn bluetooth on", "disable nightlight"
	if state, domain, ok := toggleWords(words); ok {
		return toggleIntent(state, domain, actions)
	}
	// "remind me in 20 minutes to stretch", "set a timer for 10m"
	if verb == "remind" || verb == "set" && len(rest) > 0 && rest[0].Canon == "remind" {
		return reminderIntent(all)
	}
	// "performance mode", "power saver", "switch to balanced"
	if profile, ok := profileWords(words); ok {
		return providers.Power(profile)
	}
	// "keep awake for 2 hours", "awake 45m"
	if verb == "awake" && len(rest) > 0 {
		if d, n := intent.Duration(rest, 0); n > 0 {
			return providers.Awake(d.String())
		}
	}
	if len(words) == 1 && verb != "connect" && verb != "disconnect" {
		// "do not disturb" is one word by now, but no prefix types it
		if fn := domainProvider(verb); fn != nil && words[0].Raw != verb {
			return fn("")
		}
		return nil
	}

	switch verb {
	case "set":
		if len(rest) > 0 {
			if fn := domainProvider(rest[0].Canon); fn != nil {
				if level := levelArg(rest[1:]); level != "" {
					return fn(level)
				}
				return fn(intent.RawJoin(rest[1:]))
			}
		}

	case "open":
		pool := keepGroups(append(actions, cache.windowList()...), "app", "window")
		res := rank(pool, stats, ctx, arg)
		if u, ok := urlAction(arg); ok {
			res = append(res, u)
		}
		return res

	case "switch", "go", "focus":
		if rest[0].Canon == "workspace" {
			return providers.Workspace(intent.RawJoin(rest[1:]))
		}
		return rank(cache.windowList(), stats, ctx, arg)

	case "connect", "disconnect":
		// device entries from both radios; state decides connect vs disconnect
		res := keepContaining(append(providers.Bluetooth(arg), providers.Wifi(arg)...), "connect")
		if verb == "disconnect" {
			res = keepContaining(res, "disconnect")
		}
		return res

	case "search":
		return []action.Action{searchAction(arg, ctx.settings)}

	case "find":
		return providers.Files(arg)

	case "kill":
		return providers.Kill(arg)

	case "nightlight", "powermode", "dnd", "awake":
		// "night light 3500", "power mode balanced"
		return domainProvider(verb)(arg)
	}
	return nil
}

// levelArg turns the words after "volume" or "brightness" into what the
// provider takes: "40", "+10", "-5", "mute"; "" when they say no level.
func levelArg(ws []intent.Word) string {
	sign, amount := "", ""
	for _, w := range ws {
		switch c := w.Canon; {
		case c == "up" || c == "down":
			sign = map[string]string{"up": "+", "down": "-"}[c]
		case c == "mute" || c == "unmute":
			return c
		case c == "%" || c == "by" || c == "at":
		default:
			c = strings.TrimSuffix(c, "%")
			if strings.HasPrefix(c, "+") || strings.HasPrefix(c, "-") {
				sign, c = c[:1], c[1:]
			}
			n, ok := intent.Number(c)
			if !ok || n > 100 || !isDigits(strings.ReplaceAll(c, ".", "")) {
				return ""
			}
			amount = fmt.Sprint(int(n))
		}
	}
	switch {
	case sign != "" && amount == "":
		return sign + "5"
	case sign != "":
		return sign + amount
	}
	return amount
}

var toggleDomains = map[string]bool{"wifi": true, "bluetooth": true, "dnd": true, "nightlight": true, "awake": true}

// toggleWords finds a state and what it applies to in any order. The state
// must open or close the sentence ("on" in the middle is usually a date).
func toggleWords(ws []intent.Word) (state, domain string, ok bool) {
	if len(ws) < 2 {
		return "", "", false
	}
	if ws[0].Canon == "turn" || ws[0].Canon == "switch" {
		ws = ws[1:]
	}
	if len(ws) < 2 {
		return "", "", false
	}
	first, last := ws[0].Canon, ws[len(ws)-1].Canon
	switch {
	case first == "on" || first == "off":
		return first, intent.RawJoin(ws[1:]), true
	case last == "on" || last == "off":
		return last, intent.RawJoin(ws[:len(ws)-1]), true
	}
	return "", "", false
}

func reminderIntent(ws []intent.Word) []action.Action {
	var d time.Duration
	var msg []intent.Word
	for i := 0; i < len(ws); i++ {
		if got, n := intent.Duration(ws, i); n > 0 && d == 0 {
			d = got
			i += n - 1
			continue
		}
		// a bare number is minutes, as before: "remind 10 tea"
		if c := ws[i].Canon; d == 0 && isDigits(c) && c != "0" {
			n, _ := intent.Number(c)
			d = time.Duration(n) * time.Minute
			continue
		}
		if c := ws[i].Canon; ws[i].Filler || c == "remind" || c == "set" || c == "about" || c == "after" || c == "that" {
			continue
		}
		msg = append(msg, ws[i])
	}
	if d == 0 {
		return []action.Action{{ID: "set-reminder", Name: "Set Reminder…", Icon: "󰔛", Group: "info",
			Cmd: "reminder prompt"}}
	}
	mins := int(math.Ceil(d.Minutes()))
	text := intent.RawJoin(msg)
	name := "Remind in " + shortDuration(mins)
	if text != "" {
		name += ": " + text
	}
	return []action.Action{{ID: "remind", Name: name, Icon: "󰔛", Group: "info",
		Hint: time.Now().Add(time.Duration(mins) * time.Minute).Format("15:04"),
		Cmd:  fmt.Sprintf("reminder %d %s", mins, shell.Quote(text))}}
}

func shortDuration(mins int) string {
	if mins < 60 {
		return fmt.Sprintf("%dm", mins)
	}
	if mins%60 == 0 {
		return fmt.Sprintf("%dh", mins/60)
	}
	return fmt.Sprintf("%dh %dm", mins/60, mins%60)
}

func domainProvider(domain string) func(string) []action.Action {
	switch domain {
	case "wallpaper":
		return providers.Wallpaper
	case "volume":
		return providers.Volume
	case "brightness":
		return providers.Brightness
	case "workspace":
		return providers.Workspace
	case "output":
		return providers.Audio
	case "wifi":
		return providers.Wifi
	case "bluetooth":
		return providers.Bluetooth
	case "monitor":
		return providers.Monitor
	case "dnd":
		return providers.DND
	case "nightlight":
		return providers.Nightlight
	case "awake":
		return providers.Awake
	case "powermode":
		return providers.Power
	}
	return nil
}

var profileNames = map[string]bool{"power-saver": true, "balanced": true, "performance": true}

// profileWords finds a power profile in a short sentence about power:
// "performance mode", "power saver", "set powermode to balanced".
func profileWords(ws []intent.Word) (string, bool) {
	profile := ""
	for _, w := range ws {
		switch c := w.Canon; {
		case profileNames[c]:
			profile = c
		case c == "mode" || c == "powermode" || c == "power" || c == "set" || c == "switch" || c == "on" || c == "go":
		default:
			return "", false
		}
	}
	return profile, profile != ""
}

func toggleIntent(state, domain string, actions []action.Action) []action.Action {
	canon := intent.Content(intent.Words(domain))
	if len(canon) == 1 && toggleDomains[canon[0].Canon] {
		all := domainProvider(canon[0].Canon)("")
		// already in that state: show what there is, its hint says so
		if kept := keepWord(all, state); len(kept) > 0 {
			return kept
		}
		return all
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

// keepWord keeps actions whose name has word as a whole word: "Wi-Fi Off"
// for "off", not "Connect Pixel" for "on".
func keepWord(actions []action.Action, word string) []action.Action {
	var res []action.Action
	for _, a := range actions {
		for _, w := range strings.Fields(strings.ToLower(a.Name)) {
			if w == word {
				res = append(res, a)
				break
			}
		}
	}
	return res
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}
