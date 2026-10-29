package engine

import (
	"strings"
	"testing"

	"github.com/tuklu/nZlauncher/internal/intent"
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
	cases := map[string]string{
		"remind me in 20 minutes to stretch": "reminder 20 'stretch'",
		"remind me to stretch in 1h":         "reminder 60 'stretch'",
		"remind 10 tea":                      "reminder 10 'tea'",
		"timer 5m":                           "reminder 5 ''",
		"set a timer for half an hour":       "reminder 30 ''",
		"remind me":                          "reminder prompt",
	}
	for q, want := range cases {
		ws := intent.Words(q)
		acts := reminderIntent(ws)
		if len(acts) != 1 || !strings.HasPrefix(acts[0].Cmd, want) {
			t.Errorf("%q: got %+v, want %s", q, acts, want)
		}
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
