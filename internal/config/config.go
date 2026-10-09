// Package config loads ~/.config/neZha/catalog.toml: settings plus the
// static action catalog, written with embedded defaults on first run.
package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/tuklu/nZlauncher/internal/action"
)

//go:embed catalog.toml
var defaultCatalog []byte

type Settings struct {
	Interface   string `toml:"interface"`
	GtkMode     string `toml:"gtk_mode"`
	IconLibrary string `toml:"icon_library"` // name of ~/.config/neZha/icons/<name>.toml
}

// UI is "tui" unless the catalog asks for "gtk".
func (s Settings) UI() string {
	if s.Interface == "gtk" {
		return "gtk"
	}
	return "tui"
}

// GTKMode is "overlay" (layer-shell surface above everything, Spotlight-style)
// unless the catalog asks for "window" (a normal toplevel managed by Hyprland).
func (s Settings) GTKMode() string {
	if s.GtkMode == "window" {
		return "window"
	}
	return "overlay"
}

type Catalog struct {
	Settings Settings        `toml:"settings"`
	Actions  []action.Action `toml:"action"`
}

func Dir() string {
	return filepath.Join(os.Getenv("HOME"), ".config/neZha")
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
