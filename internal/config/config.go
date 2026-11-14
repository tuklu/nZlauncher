// Package config loads ~/.config/nzlauncher: config.toml for settings and
// catalog.toml for the static actions, both written on first run.
package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"

	"github.com/tuklu/nZlauncher/internal/action"
	"github.com/tuklu/nZlauncher/internal/vault"
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

// CacheDir holds what can be made again: decoded images, rates, the
// launcher's terminal override.
func CacheDir() string {
	return filepath.Join(xdg("XDG_CACHE_HOME", ".cache"), "nzlauncher")
}

// ThemeFile is a file in <config>/theme/, which a desktop theme may link in.
// Frontends fall back to their built-in look when it is missing.
func ThemeFile(name string) string {
	return filepath.Join(Dir(), "theme", name)
}

func CatalogPath() string {
	return filepath.Join(Dir(), "catalog.toml")
}

// SealedCatalogPath is the encrypted catalog; when it is there, it is the
// catalog and catalog.toml is not written.
func SealedCatalogPath() string {
	return CatalogPath() + ".enc"
}

// KeyFilePath is where [catalog] key = "file" keeps its key: with the data,
// not the config, so a synced or committed config never carries it.
func KeyFilePath() string {
	return filepath.Join(DataDir(), "catalog.key")
}

// One source per key setting for the life of the process, so a daemon asks
// for a passphrase or 1Password approval once, not on every reload.
var catalogSources = struct {
	sync.Mutex
	bySpec map[string]vault.Source
}{bySpec: map[string]vault.Source{}}

// CatalogSource is the key source config.toml's [catalog] key names.
func CatalogSource() (vault.Source, error) {
	settings, _ := LoadSettings()
	spec := settings.Catalog.Key
	catalogSources.Lock()
	defer catalogSources.Unlock()
	if src, ok := catalogSources.bySpec[spec]; ok {
		return src, nil
	}
	src, err := vault.ParseSource(spec, KeyFilePath())
	if err != nil {
		return nil, err
	}
	catalogSources.bySpec[spec] = src
	return src, nil
}

// CatalogText is the catalog's TOML, decrypted when it is sealed. The first
// run writes the default catalog in the clear.
func CatalogText() ([]byte, error) {
	if sealed, err := os.ReadFile(SealedCatalogPath()); err == nil {
		src, err := CatalogSource()
		if err != nil {
			return nil, err
		}
		plain, err := vault.Open(sealed, src)
		if err != nil {
			return nil, fmt.Errorf("encrypted catalog: %w", err)
		}
		return plain, nil
	}
	path := CatalogPath()
	if _, err := os.Stat(path); err != nil {
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, defaultCatalog, 0o644)
	}
	return os.ReadFile(path)
}

func Load() (Catalog, error) {
	text, err := CatalogText()
	if err != nil {
		return Catalog{}, fmt.Errorf("catalog: %w", err)
	}
	var c Catalog
	if _, err := toml.Decode(string(text), &c); err != nil {
		return Catalog{}, fmt.Errorf("catalog: %w", err)
	}
	for i, a := range c.Actions {
		if a.ID == "" || a.Name == "" || a.Cmd == "" {
			return Catalog{}, fmt.Errorf("catalog: action %d missing id, name, or cmd", i)
		}
	}
	return c, nil
}
