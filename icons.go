package main

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

//go:embed icons/nerd.toml
var builtinIcons []byte

const defaultIconLibrary = "nerd"

// TUI app glyphs. The built-in Nerd Font library is always loaded first;
// the user's chosen library is layered on top, so a partial one only
// overrides what it names.
type iconLibrary struct {
	Apps       map[string]string `toml:"apps"`
	Categories map[string]string `toml:"categories"`
	Default    struct {
		App string `toml:"app"`
	} `toml:"default"`
}

func iconLibraryDir() string {
	return filepath.Join(os.Getenv("HOME"), ".config/neZha/icons")
}

var loadedIcons *iconLibrary

func icons() *iconLibrary {
	if loadedIcons != nil {
		return loadedIcons
	}
	lib := &iconLibrary{Apps: map[string]string{}, Categories: map[string]string{}}
	mergeIcons(lib, builtinIcons)

	builtinPath := filepath.Join(iconLibraryDir(), defaultIconLibrary+".toml")
	if _, err := os.Stat(builtinPath); err != nil {
		os.MkdirAll(iconLibraryDir(), 0o755)
		os.WriteFile(builtinPath, builtinIcons, 0o644)
	}

	name := loadedSettings.IconLibrary
	if name == "" {
		name = defaultIconLibrary
	}
	if data, err := os.ReadFile(filepath.Join(iconLibraryDir(), name+".toml")); err == nil {
		mergeIcons(lib, data)
	}
	loadedIcons = lib
	return lib
}

func mergeIcons(into *iconLibrary, data []byte) {
	var lib iconLibrary
	if _, err := toml.Decode(string(data), &lib); err != nil {
		return
	}
	for k, v := range lib.Apps {
		into.Apps[strings.ToLower(k)] = v
	}
	for k, v := range lib.Categories {
		into.Categories[k] = v
	}
	if lib.Default.App != "" {
		into.Default.App = lib.Default.App
	}
}

// candidates in priority order: desktop id, its last dotted segment,
// Icon=, StartupWMClass, Exec binary. An Icon= that is a file path
// matches by its basename without extension.
func (lib *iconLibrary) appGlyph(candidates ...string) (string, bool) {
	for _, c := range candidates {
		if strings.ContainsRune(c, '/') {
			c = strings.TrimSuffix(filepath.Base(c), filepath.Ext(c))
		}
		c = strings.ToLower(strings.TrimSpace(c))
		if c == "" {
			continue
		}
		if g, ok := lib.Apps[c]; ok {
			return g, true
		}
		if dot := strings.LastIndexByte(c, '.'); dot >= 0 {
			if g, ok := lib.Apps[c[dot+1:]]; ok {
				return g, true
			}
		}
	}
	return "", false
}

// Table order wins over the desktop file's own order, so Network;Email;
// resolves to Email. Categories the table doesn't know come after.
func (lib *iconLibrary) categoryGlyph(desktopCategories []string) string {
	for _, c := range categories {
		if containsString(desktopCategories, c.match) {
			if g, ok := lib.Categories[c.match]; ok {
				return g
			}
		}
	}
	for _, c := range desktopCategories {
		if g, ok := lib.Categories[c]; ok {
			return g
		}
	}
	return lib.Default.App
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
