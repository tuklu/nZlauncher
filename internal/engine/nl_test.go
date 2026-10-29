package engine

import (
	"testing"

	"github.com/tuklu/nZlauncher/internal/intent"
	"github.com/tuklu/nZlauncher/internal/portable"
)

func TestLevelArg(t *testing.T) {
	cases := map[string]string{
		"40": "40", "40%": "40", "40 percent": "40", "to 40": "40", "up": "+5", "down 10": "-10",
		"+10": "+10", "-5": "-5", "by 20 up": "+20", "mute": "mute", "unmute": "unmute", "wiremix": "", "500": "",
	}
	for q, want := range cases {
		ws := intent.Content(intent.Words(q))
		if len(ws) > 0 && ws[0].Canon == "volume" { // "mute" reads "volume mute"
			ws = ws[1:]
		}
		if got := levelArg(ws); got != want {
			t.Errorf("%q: got %q, want %q", q, got, want)
		}
	}
}

func TestToggleWords(t *testing.T) {
	cases := map[string]string{
		"wifi off": "off wifi", "off wifi": "off wifi", "turn bluetooth on": "on bluetooth",
		"disable bluetooth": "off bluetooth", "enable night light": "on night light",
		"pdfs on monday": "", "firefox": "",
		"turn on do not disturb": "on do not disturb", "nightlight off": "off nightlight",
	}
	for q, want := range cases {
		state, domain, ok := toggleWords(intent.Content(intent.Words(q)))
		got := ""
		if ok {
			got = state + " " + domain
		}
		if got != want {
			t.Errorf("%q: got %q, want %q", q, got, want)
		}
	}
}

func TestReminderIntent(t *testing.T) {
	// the command depends on what is installed: compare with what this
	// machine would run
	cases := map[string]struct {
		minutes int
		text    string
	}{
		"remind me in 20 minutes to stretch": {20, "stretch"},
		"remind me to stretch in 1h":         {60, "stretch"},
		"remind 10 tea":                      {10, "tea"},
		"timer 5m":                           {5, ""},
		"set a timer for half an hour":       {30, ""},
	}
	for q, want := range cases {
		acts := reminderIntent(intent.Words(q))
		if !portable.CanRemind() {
			if len(acts) != 0 {
				t.Errorf("%q: no way to remind, yet %+v", q, acts)
			}
			continue
		}
		if len(acts) != 1 || acts[0].Cmd != portable.Remind(want.minutes, want.text) {
			t.Errorf("%q: got %+v, want %s", q, acts, portable.Remind(want.minutes, want.text))
		}
	}
	// without a duration only neZha's script can ask for one
	if acts := reminderIntent(intent.Words("remind me")); portable.Have("reminder") != (len(acts) == 1) {
		t.Errorf("remind me: %+v", acts)
	}
}

func TestTypoMatch(t *testing.T) {
	words := []string{"firefox"}
	for q, want := range map[string]bool{"fierfox": true, "firfox": true, "fxzqw": false, "fire": false} {
		if typoPrefix(words, q) != want {
			t.Errorf("%q: want %v", q, want)
		}
	}
}

func TestProfileWords(t *testing.T) {
	cases := map[string]string{
		"performance mode": "performance", "power saver": "power-saver", "set power mode to balanced": "balanced",
		"switch to performance": "performance", "power off": "", "performance review pdf": "",
	}
	for q, want := range cases {
		got, _ := profileWords(intent.Content(intent.Words(q)))
		if got != want {
			t.Errorf("%q: got %q, want %q", q, got, want)
		}
	}
}
