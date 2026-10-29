package providers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tuklu/nZlauncher/internal/action"
	"github.com/tuklu/nZlauncher/internal/shell"
	"github.com/tuklu/nZlauncher/internal/tweaks"
)

// tweak runs nzlauncher tweak and says how it went.
func tweak(args, done string) string {
	return "nzlauncher tweak " + args + " && notify-send -u low " + shell.Quote(done) +
		" || notify-send " + shell.Quote("Could not: "+strings.ToLower(done))
}

// unavailable is a row that says why a tweak cannot work here.
func unavailable(id, name string, err error) []action.Action {
	return []action.Action{{ID: id, Name: name, Icon: "󰀦", Group: "tweak", Hint: "not available",
		Cmd: "notify-send " + shell.Quote(name) + " " + shell.Quote(err.Error())}}
}

var profileIcons = map[string]string{"power-saver": "󰾆", "balanced": "󰾅", "performance": "󰓅"}

// profile / powermode: the power profiles, the current one marked.
// "profile perf" narrows them.
func Power(q string) []action.Action {
	if !On("power") {
		return nil
	}
	state, ok := tweaks.Power()
	if !ok {
		return unavailable("power:none", "Power Profiles", tweaks.ErrNoPower)
	}
	q = strings.ToLower(q)
	var acts []action.Action
	for _, p := range state.Profiles {
		if q != "" && !strings.Contains(p, q) {
			continue
		}
		hint := state.Daemon
		if p == state.Active {
			hint = "current · " + state.Daemon
		}
		name := "Power " + titleWords(strings.ReplaceAll(p, "-", " "))
		acts = append(acts, action.Action{ID: "power:" + p, Name: name, Icon: profileIcons[p], Group: "tweak", Hint: hint,
			Cmd: tweak("power "+shell.Quote(p), name)})
	}
	return acts
}

// dnd: do not disturb on the notification daemon.
func DND(q string) []action.Action {
	if !On("dnd") {
		return nil
	}
	on, daemon, ok := tweaks.DND()
	if !ok {
		return unavailable("dnd:none", "Do Not Disturb", tweaks.ErrNoDND)
	}
	if on {
		return []action.Action{{ID: "dnd:off", Name: "Do Not Disturb Off", Icon: "󰂚", Group: "tweak",
			Hint: "on · " + daemon, Cmd: tweak("dnd off", "Do not disturb off")}}
	}
	// no notification for turning it on: it would be the last one let through
	return []action.Action{{ID: "dnd:on", Name: "Do Not Disturb On", Icon: "󰂛", Group: "tweak",
		Hint: "off · " + daemon, Cmd: "nzlauncher tweak dnd on || notify-send 'Could not turn on do not disturb'"}}
}

// nightlight: warm the screen; "nightlight 3500" picks the temperature.
func Nightlight(q string) []action.Action {
	if !On("nightlight") {
		return nil
	}
	kelvin, ok := tweaks.Nightlight()
	if !ok {
		return unavailable("nightlight:none", "Nightlight", tweaks.ErrNoNightlight)
	}
	q = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(q)), "k")
	if k, err := strconv.Atoi(q); err == nil && k >= 1000 && k <= 10000 {
		return []action.Action{{ID: "nightlight:set", Name: fmt.Sprintf("Nightlight %d K", k), Icon: "󰖔", Group: "tweak",
			Cmd: tweak(fmt.Sprintf("nightlight %d", k), fmt.Sprintf("Nightlight %d K", k))}}
	}
	if tweaks.NightlightOn(kelvin) {
		return []action.Action{{ID: "nightlight:off", Name: "Nightlight Off", Icon: "󰖨", Group: "tweak",
			Hint: fmt.Sprintf("on · %d K", kelvin), Cmd: tweak("nightlight off", "Nightlight off")}}
	}
	return []action.Action{{ID: "nightlight:on", Name: "Nightlight On", Icon: "󰖔", Group: "tweak",
		Hint: "off · 4000 K", Cmd: tweak("nightlight 4000", "Nightlight on")}}
}

// awake / caffeine: keep the screen from locking and blanking, for a while
// ("awake 2h") or until turned off. Held by the daemon.
func Awake(q string) []action.Action {
	if !On("awake") {
		return nil
	}
	if !tweaks.IdleAvailable() {
		return unavailable("awake:none", "Keep Awake", tweaks.ErrNoIdle)
	}
	if held, until := tweaks.Awake(); held {
		hint := "awake until turned off"
		if !until.IsZero() {
			hint = "awake until " + until.Format("15:04")
		}
		return []action.Action{{ID: "awake:off", Name: "Allow Sleep", Icon: "󰒲", Group: "tweak", Hint: hint,
			Cmd: tweak("awake off", "Screen can sleep again")}}
	}
	keep := func(d time.Duration) action.Action {
		name, hint := "Keep Awake", "until turned off"
		if d > 0 {
			name = "Keep Awake " + shortDuration(d)
			hint = "until " + time.Now().Add(d).Format("15:04")
		}
		return action.Action{ID: "awake:on", Name: name, Icon: "󰅶", Group: "tweak", Hint: hint,
			Cmd: tweak(fmt.Sprintf("awake %d", int(d.Seconds())), name)}
	}
	if d, err := time.ParseDuration(strings.ReplaceAll(strings.TrimSpace(q), " ", "")); err == nil && d > 0 {
		return []action.Action{keep(d)}
	}
	return []action.Action{keep(0), keep(time.Hour), keep(2 * time.Hour)}
}

func shortDuration(d time.Duration) string {
	h, m := int(d.Hours()), int(d.Minutes())%60
	switch {
	case h == 0:
		return fmt.Sprintf("%dm", m)
	case m == 0:
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh %dm", h, m)
}

func titleWords(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}
