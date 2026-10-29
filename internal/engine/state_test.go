package engine

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/tuklu/nZlauncher/internal/learn"
)

func names(t *testing.T, q string) []string {
	t.Helper()
	acts, err := Query(q)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, a := range acts {
		out = append(out, a.Name)
	}
	return out
}

func write(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The cache must never serve stale results: each source changes between two
// queries and the second query has to see it.
func TestCacheFollowsChanges(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local/share"))
	cache = state{}

	catalog := filepath.Join(home, ".config/nzlauncher/catalog.toml")
	write(t, catalog, "[[action]]\nid = \"one\"\nname = \"Zebra One\"\ncmd = \"true\"\n")
	if got := names(t, "zebra"); !slices.Contains(got, "Zebra One") {
		t.Fatalf("first catalog not served: %v", got)
	}

	time.Sleep(10 * time.Millisecond) // a distinct mtime
	write(t, catalog, "[[action]]\nid = \"two\"\nname = \"Zebra Two\"\ncmd = \"true\"\n")
	if got := names(t, "zebra"); !slices.Contains(got, "Zebra Two") || slices.Contains(got, "Zebra One") {
		t.Fatalf("catalog edit not picked up: %v", got)
	}

	write(t, filepath.Join(home, ".local/share/applications/qux.desktop"),
		"[Desktop Entry]\nType=Application\nName=Quxapp\nExec=quxapp\n")
	if got := names(t, "quxapp"); !slices.Contains(got, "Quxapp") {
		t.Fatalf("new .desktop file not picked up: %v", got)
	}

	before := cache.stats["two"].Hits
	if err := learn.Record("two", "zebra"); err != nil {
		t.Fatal(err)
	}
	names(t, "zebra")
	if after := cache.stats["two"].Hits; after != before+1 {
		t.Fatalf("recorded launch not counted: %d -> %d", before, after)
	}
}
