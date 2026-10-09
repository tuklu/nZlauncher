package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

//go:embed catalog.toml
var defaultCatalog []byte

type Action struct {
	ID       string   `toml:"id"`
	Name     string   `toml:"name"`
	Icon     string   `toml:"icon"`
	IconName string   `toml:"icon_name"`
	Group    string   `toml:"group"`
	Cmd      string   `toml:"cmd"`
	Keywords []string `toml:"keywords"`
}

func (a Action) display() string {
	if a.Icon != "" {
		return a.Icon + "  " + a.Name
	}
	return a.Name
}

type settings struct {
	Interface   string `toml:"interface"`
	GtkMode     string `toml:"gtk_mode"`
	IconLibrary string `toml:"icon_library"` // name of ~/.config/neZha/icons/<name>.toml
}

var loadedSettings settings

func uiInterface() string {
	if loadedSettings.Interface == "" {
		loadCatalog(catalogPath())
	}
	if loadedSettings.Interface == "gtk" {
		return "gtk"
	}
	return "tui"
}

// overlay: layer-shell surface above everything (Spotlight-style).
// window: a normal toplevel managed by Hyprland rules and animations.
func gtkMode() string {
	if loadedSettings.GtkMode == "" {
		loadCatalog(catalogPath())
	}
	if loadedSettings.GtkMode == "window" {
		return "window"
	}
	return "overlay"
}

func loadCatalog(path string) ([]Action, error) {
	if _, err := os.Stat(path); err != nil {
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, defaultCatalog, 0o644)
	}
	var c struct {
		Settings settings `toml:"settings"`
		Action   []Action `toml:"action"`
	}
	if _, err := toml.DecodeFile(path, &c); err != nil {
		return nil, fmt.Errorf("catalog: %w", err)
	}
	if c.Settings != (settings{}) {
		loadedSettings = c.Settings
	}
	for i, a := range c.Action {
		if a.ID == "" || a.Name == "" || a.Cmd == "" {
			return nil, fmt.Errorf("catalog: action %d missing id, name, or cmd", i)
		}
	}
	return c.Action, nil
}
