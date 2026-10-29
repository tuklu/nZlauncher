// Package config loads ~/.config/nzlauncher: config.toml for settings and
// catalog.toml for the static actions, both written on first run.
package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/tuklu/nZlauncher/internal/action"
)

//go:embed catalog.toml
var defaultCatalog []byte

type Catalog struct {
	Actions []action.Action `toml:"action"`
}

func ExpandHome(p string) string {
	if p == "~" {
		return os.Getenv("HOME")
	}
	if rest, ok := strings.CutPrefix(p, "~/"); ok {
		return filepath.Join(os.Getenv("HOME"), rest)
	}
	return p
}

func xdg(env, fallback string) string {
	if d := os.Getenv(env); d != "" {
		return d
	}
	return filepath.Join(os.Getenv("HOME"), fallback)
}

func Dir() string {
	return filepath.Join(xdg("XDG_CONFIG_HOME", ".config"), "nzlauncher")
}

// DataDir holds the learning log.
func DataDir() string {
	return filepath.Join(xdg("XDG_DATA_HOME", ".local/share"), "nzlauncher")
}

// ThemeFile is a file in <config>/theme/, which a desktop theme may link in.
// Frontends fall back to their built-in look when it is missing.
func ThemeFile(name string) string {
	return filepath.Join(Dir(), "theme", name)
}

func CatalogPath() string {
	return filepath.Join(Dir(), "catalog.toml")
}

func Load() (Catalog, error) {
	path := CatalogPath()
	if _, err := os.Stat(path); err != nil {
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, defaultCatalog, 0o644)
	}
	var c Catalog
	if _, err := toml.DecodeFile(path, &c); err != nil {
		return Catalog{}, fmt.Errorf("catalog: %w", err)
	}
	for i, a := range c.Actions {
		if a.ID == "" || a.Name == "" || a.Cmd == "" {
			return Catalog{}, fmt.Errorf("catalog: action %d missing id, name, or cmd", i)
		}
	}
	return c, nil
}
