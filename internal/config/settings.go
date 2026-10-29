package config

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Written to config.toml on first run, every line commented, so the file
// documents every setting without overriding the defaults below.
//
//go:embed config.toml
var configTemplate []byte

// Settings is config.toml. Every key is optional; ConfigDefaults fills the rest.
type Settings struct {
	Interface   string `toml:"interface"`
	GtkMode     string `toml:"gtk_mode"`
	IconLibrary string `toml:"icon_library"`

	Wallpaper struct {
		Dir string `toml:"dir"`
		Cmd string `toml:"cmd"`
	} `toml:"wallpaper"`

	Search struct {
		URL string `toml:"url"`
	} `toml:"search"`

	Files struct {
		Roots   []string `toml:"roots"`
		Exclude []string `toml:"exclude"`
	} `toml:"files"`

	Providers map[string]bool `toml:"providers"`
	Fallbacks map[string]bool `toml:"fallbacks"`
}

// The [settings] block catalog.toml carried before config.toml existed.
type legacySettings struct {
	Interface    string `toml:"interface"`
	GtkMode      string `toml:"gtk_mode"`
	IconLibrary  string `toml:"icon_library"`
	WallpaperDir string `toml:"wallpaper_dir"`
	WallpaperCmd string `toml:"wallpaper_cmd"`
}

func ConfigPath() string {
	return filepath.Join(Dir(), "config.toml")
}

// LoadSettings reads config.toml over any legacy [settings] in catalog.toml.
// A missing config.toml is written as a commented template.
func LoadSettings() (Settings, error) {
	var s Settings

	var legacy struct {
		Settings legacySettings `toml:"settings"`
	}
	if _, err := toml.DecodeFile(CatalogPath(), &legacy); err == nil {
		l := legacy.Settings
		s.Interface, s.GtkMode, s.IconLibrary = l.Interface, l.GtkMode, l.IconLibrary
		s.Wallpaper.Dir, s.Wallpaper.Cmd = l.WallpaperDir, l.WallpaperCmd
	}

	path := ConfigPath()
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, configTemplate, 0o644)
	}
	if _, err := toml.DecodeFile(path, &s); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return s, fmt.Errorf("config: %w", err)
	}
	return s, nil
}

// UI is "tui" unless set to "gtk".
func (s Settings) UI() string {
	if s.Interface == "gtk" {
		return "gtk"
	}
	return "tui"
}

// GTKMode is "overlay" (layer-shell surface above everything) unless set to
// "window" (a normal toplevel managed by the compositor).
func (s Settings) GTKMode() string {
	if s.GtkMode == "window" {
		return "window"
	}
	return "overlay"
}

// Wallpapers default to ~/Pictures/Wallpapers, set with swaybg.
func (s Settings) Wallpapers() (dir, cmd string) {
	dir, cmd = s.Wallpaper.Dir, s.Wallpaper.Cmd
	if dir == "" {
		dir = "~/Pictures/Wallpapers"
	}
	if cmd == "" {
		cmd = "swaybg -m fill -i"
	}
	return ExpandHome(dir), cmd
}

// SearchURL has %s where the escaped query goes.
func (s Settings) SearchURL() string {
	if s.Search.URL != "" {
		return s.Search.URL
	}
	return "https://www.google.com/search?q=%s"
}

// Provider and Fallback are on unless config.toml sets them false.
func (s Settings) Provider(name string) bool {
	on, set := s.Providers[name]
	return !set || on
}

func (s Settings) Fallback(name string) bool {
	on, set := s.Fallbacks[name]
	return !set || on
}

// FileRoots and FileExclude are nil when unset, so the index keeps its defaults.
func (s Settings) FileRoots() []string {
	var roots []string
	for _, r := range s.Files.Roots {
		roots = append(roots, ExpandHome(r))
	}
	return roots
}
