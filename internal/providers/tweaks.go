package providers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tuklu/nZlauncher/internal/action"
	"github.com/tuklu/nZlauncher/internal/intent"
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

// awakeMode is what a keep-awake row holds: the screen, sleep, or both.
type awakeMode struct {
	key   string // as nzlauncher tweak awake takes it
	name  string
	icon  string
	what  string // the hint's first half: what it keeps from happening
	holds []tweaks.Hold
}

var (
	modeScreen = awakeMode{"screen", "Keep Screen On", "󰅶", "no lock, no blanking", []tweaks.Hold{tweaks.HoldScreen}}
	modeSleep  = awakeMode{"sleep", "No Sleep", "󰤄", "may lock, never suspends, lid included", []tweaks.Hold{tweaks.HoldSleep}}
	modeAll    = awakeMode{"all", "Keep Everything On", "󰛨", "no lock, no blanking, no suspend", tweaks.Holds}
)

var holdRelease = map[tweaks.Hold]struct{ name, done string }{
	tweaks.HoldScreen: {"Allow Lock", "Screen can lock and blank again"},
	tweaks.HoldSleep:  {"Allow Sleep", "Machine can sleep again"},
}

// awake / caffeine: keep the screen on, keep the machine from sleeping, or
// both, for a while ("awake 2h", "awake sleep until 18:30") or until turned
// off. "awake off" shows what is held and lets it go. Held by the daemon.
func Awake(q string) []action.Action {
	if !On("awake") {
		return nil
	}
	if !tweaks.Available(tweaks.HoldScreen) && !tweaks.Available(tweaks.HoldSleep) {
		return unavailable("awake:none", "Keep Awake", tweaks.ErrNoIdle)
	}
	req := parseAwake(q)

	var acts []action.Action
	anyHeld := false
	for _, h := range tweaks.Holds {
		held, until := tweaks.Held(h)
		if !held {
			continue
		}
		anyHeld = true
		r := holdRelease[h]
		acts = append(acts, action.Action{ID: "awake:" + string(h) + ":off", Name: r.name, Icon: "󰒲", Group: "tweak",
			Hint: string(h) + " held " + untilHint(until), Cmd: tweak("awake "+string(h)+" off", r.done)})
	}
	if anyHeld {
		acts = append(acts, normalAction())
	}
	if req.off {
		if !anyHeld {
			return []action.Action{{ID: "awake:idle", Name: "Nothing Held", Icon: "󰒲", Group: "tweak",
				Hint: "lock and sleep work normally", Cmd: "true"}}
		}
		return acts
	}

	modes := []awakeMode{modeScreen, modeSleep, modeAll}
	if req.mode != nil {
		modes = []awakeMode{*req.mode}
	}
	durations := []time.Duration{0, time.Hour, 2 * time.Hour}
	if req.set {
		durations = []time.Duration{req.d}
	} else if req.mode == nil {
		durations = []time.Duration{0}
	}
	for _, m := range modes {
		if !modeAvailable(m) {
			continue
		}
		for _, d := range durations {
			acts = append(acts, keepAction(m, d))
		}
	}
	if !req.set && req.mode == nil && modeAvailable(modeScreen) {
		// the common timed case, without typing a duration
		acts = append(acts, keepAction(modeScreen, time.Hour))
	}
	return acts
}

// Normal: let go of every keep-awake hold and go back to the balanced power
// profile, the one TLP and power-profiles-daemon default to on battery.
func Normal(string) []action.Action {
	if !On("awake") {
		return nil
	}
	return []action.Action{normalAction()}
}

func normalAction() action.Action {
	return action.Action{ID: "awake:normal", Name: "Back to Normal", Icon: "󰒲", Group: "tweak",
		Hint: "lock and sleep as usual, balanced power", Cmd: tweak("normal", "Back to normal")}
}

func modeAvailable(m awakeMode) bool {
	for _, h := range m.holds {
		if !tweaks.Available(h) {
			return false
		}
	}
	return true
}

func keepAction(m awakeMode, d time.Duration) action.Action {
	name := m.name
	if d > 0 {
		name += " " + shortDuration(d)
	}
	return action.Action{ID: "awake:" + m.key + ":on", Name: name, Icon: m.icon, Group: "tweak",
		Hint: m.what + " · " + untilHint(timeAfter(d)),
		Cmd:  tweak(fmt.Sprintf("awake %s %d", m.key, int(d.Seconds())), name)}
}

func timeAfter(d time.Duration) time.Time {
	if d <= 0 {
		return time.Time{}
	}
	return time.Now().Add(d)
}

func untilHint(t time.Time) string {
	if t.IsZero() {
		return "until turned off"
	}
	return "until " + t.Format("15:04")
}

type awakeRequest struct {
	mode *awakeMode
	d    time.Duration
	set  bool // a duration, a clock time or forever was given
	off  bool
}

var awakeModeWords = map[string]*awakeMode{
	"screen": &modeScreen, "display": &modeScreen, "unlocked": &modeScreen, "lock": &modeScreen,
	"sleep": &modeSleep, "nosleep": &modeSleep, "lid": &modeSleep, "running": &modeSleep, "suspend": &modeSleep,
	"all": &modeAll, "everything": &modeAll, "full": &modeAll, "awakeall": &modeAll,
}

var foreverWords = map[string]bool{"forever": true, "always": true, "indefinitely": true, "permanently": true}

// parseAwake reads what the grammar or a typed "awake ..." passes on:
// a mode word, then a duration ("2h", "90 minutes"), a clock time
// ("until 18:30", "till 6pm") or "forever", or "off".
func parseAwake(q string) awakeRequest {
	var req awakeRequest
	var ws []intent.Word
	for _, w := range intent.Words(q) {
		// "all" is filler everywhere else, here it is a mode
		if !w.Filler || w.Canon == "all" {
			ws = append(ws, w)
		}
	}
	for i := 0; i < len(ws); i++ {
		c := ws[i].Canon
		if m, ok := awakeModeWords[c]; ok && req.mode == nil {
			req.mode = m
			continue
		}
		switch {
		case c == "off":
			req.off = true
		case foreverWords[c]:
			req.set, req.d = true, 0
		case c == "until" || c == "till" || c == "til":
			if d, n := clockUntil(ws[i+1:], time.Now()); n > 0 {
				req.set, req.d = true, d
				i += n
			}
		default:
			if d, n := intent.Duration(ws, i); n > 0 && d > 0 {
				req.set, req.d = true, d
				i += n - 1
			}
		}
	}
	return req
}

// clockUntil reads a clock time at the start of ws ("18:30", "6pm", "6:30 pm",
// "6.30pm") and returns how long until it next comes round, and how many
// words it used.
func clockUntil(ws []intent.Word, now time.Time) (time.Duration, int) {
	if len(ws) == 0 {
		return 0, 0
	}
	s, used := strings.ToLower(ws[0].Raw), 1
	if len(ws) > 1 && (ws[1].Canon == "am" || ws[1].Canon == "pm") {
		s, used = s+ws[1].Canon, 2
	}
	suffix := ""
	if t, ok := strings.CutSuffix(s, "am"); ok {
		s, suffix = t, "am"
	} else if t, ok := strings.CutSuffix(s, "pm"); ok {
		s, suffix = t, "pm"
	}
	hs, ms, hasMinutes := strings.Cut(strings.ReplaceAll(s, ".", ":"), ":")
	h, err := strconv.Atoi(hs)
	if err != nil {
		return 0, 0
	}
	m := 0
	if hasMinutes {
		if m, err = strconv.Atoi(ms); err != nil || m > 59 || m < 0 {
			return 0, 0
		}
	} else if suffix == "" {
		return 0, 0 // a bare number is not a time: "until 5" could be anything
	}
	switch suffix {
	case "am":
		if h < 1 || h > 12 {
			return 0, 0
		}
		h %= 12
	case "pm":
		if h < 1 || h > 12 {
			return 0, 0
		}
		h = h%12 + 12
	}
	if h > 23 || h < 0 {
		return 0, 0
	}
	at := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, now.Location())
	if !at.After(now) {
		at = at.AddDate(0, 0, 1)
	}
	return at.Sub(now), used
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
