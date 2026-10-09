// Package icons resolves TUI glyphs for applications from a TOML icon library.
package icons

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

//go:embed nerd.toml
var builtinIcons []byte

const defaultLibrary = "nerd"

// TUI app glyphs. The built-in Nerd Font library is always loaded first;
// the user's chosen library is layered on top, so a partial one only
// overrides what it names.
type Library struct {
	Apps       map[string]string `toml:"apps"`
	Categories map[string]string `toml:"categories"`
	Default    struct {
		App string `toml:"app"`
	} `toml:"default"`
}

// Load reads <dir>/<name>.toml over the built-in library; an empty name
// means the built-in "nerd". The built-in is written to dir on first use
// as a template to copy from.
func Load(dir, name string) *Library {
	lib := &Library{Apps: map[string]string{}, Categories: map[string]string{}}
	merge(lib, builtinIcons)

	builtinPath := filepath.Join(dir, defaultLibrary+".toml")
	if _, err := os.Stat(builtinPath); err != nil {
		os.MkdirAll(dir, 0o755)
		os.WriteFile(builtinPath, builtinIcons, 0o644)
	}

	if name == "" {
		name = defaultLibrary
	}
	if data, err := os.ReadFile(filepath.Join(dir, name+".toml")); err == nil {
		merge(lib, data)
	}
	return lib
}

func merge(into *Library, data []byte) {
	var lib Library
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
func (lib *Library) AppGlyph(candidates ...string) (string, bool) {
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

// CategoryGlyph takes categories already in priority order and returns
// the first one the library has a glyph for, else the default.
func (lib *Library) CategoryGlyph(ordered []string) string {
	for _, c := range ordered {
		if g, ok := lib.Categories[c]; ok {
			return g
		}
	}
	return lib.Default.App
}
