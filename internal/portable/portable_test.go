package portable

import (
	"strings"
	"testing"
)

func TestOverridesWin(t *testing.T) {
	Configure(map[string]string{
		"terminal":    "foot -e {cmd}",
		"volume_up":   "pamixer --increase {n}",
		"volume_down": "pamixer --decrease {n}",
		"open":        "handlr open {target}",
		"remind":      "my-reminder {minutes} {text}",
	})
	defer Configure(nil)
	cases := map[string]string{
		Terminal("htop"):           "foot -e htop",
		VolumeStep(10):             "pamixer --increase 10",
		VolumeStep(-5):             "pamixer --decrease 5",
		Open("/tmp/it's here.pdf"): `handlr open '/tmp/it'\''s here.pdf'`, // quoted, quote included
		Remind(20, "stretch now"):  "my-reminder 20 'stretch now'",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
	if !CanRemind() {
		t.Errorf("a configured reminder counts")
	}
}

func TestDetectedAndFallback(t *testing.T) {
	Configure(nil)
	t.Setenv("PATH", t.TempDir()) // nothing installed
	have.found = nil
	have.at = have.at.AddDate(-1, 0, 0)
	if Terminal("htop") != "" {
		t.Errorf("no terminal installed, still got one")
	}
	if got := App("firefox"); got != "firefox" {
		t.Errorf("no uwsm: run directly, got %q", got)
	}
	rows := Explain()
	if len(rows) != len(Keys) {
		t.Fatalf("explain lists %d of %d", len(rows), len(Keys))
	}
	for _, r := range rows {
		if r.Key == "volume_up" && !strings.HasPrefix(r.Source, "fallback") {
			t.Errorf("volume_up with nothing installed: %+v", r)
		}
	}
}
