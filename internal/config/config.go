// Package config loads ~/.config/nzlauncher/catalog.toml: settings plus the
// static action catalog, written with embedded defaults on first run.
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

type Settings struct {
	Interface    string `toml:"interface"`
	GtkMode      string `toml:"gtk_mode"`
	IconLibrary  string `toml:"icon_library"`  // name of ~/.config/nzlauncher/icons/<name>.toml
	WallpaperDir string `toml:"wallpaper_dir"` // images the wallpaper provider lists
	WallpaperCmd string `toml:"wallpaper_cmd"` // run with the chosen image appended
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

// Wallpapers default to ~/Pictures/Wallpapers, set with swaybg.
func (s Settings) Wallpapers() (dir, cmd string) {
	dir, cmd = s.WallpaperDir, s.WallpaperCmd
	if dir == "" {
		dir = "~/Pictures/Wallpapers"
	}
	if cmd == "" {
		cmd = "swaybg -m fill -i"
	}
	return ExpandHome(dir), cmd
}

func ExpandHome(p string) string {
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
