package engine

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tuklu/nZlauncher/internal/config"
)

func ids(t *testing.T, q string) []string {
	t.Helper()
	acts, err := Query(q)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, a := range acts {
		out = append(out, a.ID)
	}
	return out
}

func TestConfigToml(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local/share"))
	cache = state{}
	dir := filepath.Join(home, ".config/nzlauncher")
	write(t, filepath.Join(dir, "catalog.toml"),
		"[settings]\ninterface = \"gtk\"\n\n[[action]]\nid = \"a\"\nname = \"Alpha\"\ncmd = \"true\"\n")

	// first run writes a template that changes nothing
	if got := ids(t, "2+2"); !slices.Contains(got, "fallback:calc") || !slices.Contains(got, "fallback:search") {
		t.Fatalf("defaults: %v", got)
	}
	tmpl, err := os.ReadFile(filepath.Join(dir, "config.toml"))
	if err != nil {
		t.Fatal("config.toml template not written")
	}
	for _, line := range strings.Split(string(tmpl), "\n") {
		if l := strings.TrimSpace(line); l != "" && !strings.HasPrefix(l, "#") && !strings.HasPrefix(l, "[") {
			t.Fatalf("template sets a value: %q", line)
		}
	}

	// the legacy [settings] block still applies
	if s, _ := config.LoadSettings(); s.UI() != "gtk" {
		t.Fatalf("legacy interface lost: %q", s.Interface)
	}

	time.Sleep(10 * time.Millisecond)
	write(t, filepath.Join(dir, "config.toml"), `interface = "tui"
[search]
url = "https://duckduckgo.com/?q=%s"
[fallbacks]
calc = false
[providers]
emoji = false
`)
	got := ids(t, "2+2")
	if slices.Contains(got, "fallback:calc") {
		t.Fatalf("calc fallback still on: %v", got)
	}
	acts, _ := Query("hello there")
	if s := acts[len(acts)-1]; !strings.Contains(s.Cmd, "duckduckgo.com/?q=hello+there") {
		t.Fatalf("search url not applied: %s", s.Cmd)
	}
	if got := ids(t, "emoji fire"); slices.ContainsFunc(got, func(id string) bool { return strings.HasPrefix(id, "emoji") }) {
		t.Fatalf("disabled emoji provider answered: %v", got)
	}
	if s, _ := config.LoadSettings(); s.UI() != "tui" {
		t.Fatal("config.toml does not override the legacy block")
	}
}
