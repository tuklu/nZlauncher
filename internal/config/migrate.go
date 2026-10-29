package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Migrate copies a setup from the pre-rename ~/.config/neZha and
// ~/.local/share/neZha. Each half runs only while its new directory does not
// exist yet, and the old files are left where they are.
func Migrate() {
	home := os.Getenv("HOME")
	oldConfig := filepath.Join(home, ".config/neZha")
	oldData := filepath.Join(home, ".local/share/neZha")

	if missing(Dir()) && !missing(filepath.Join(oldConfig, "catalog.toml")) {
		copyFile(filepath.Join(oldConfig, "catalog.toml"), CatalogPath())
		copyFiles(filepath.Join(oldConfig, "icons"), filepath.Join(Dir(), "icons"))
		copyFiles(filepath.Join(oldConfig, "providers.d"), filepath.Join(Dir(), "providers.d"))
		fmt.Fprintf(os.Stderr, "nzlauncher: copied your config from %s to %s\n", oldConfig, Dir())
	}
	if missing(DataDir()) && !missing(filepath.Join(oldData, "events.jsonl")) {
		copyFile(filepath.Join(oldData, "events.jsonl"), filepath.Join(DataDir(), "events.jsonl"))
		copyFile(filepath.Join(oldData, "frecency.json"), filepath.Join(DataDir(), "frecency.json"))
	}
}

func missing(path string) bool {
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}

// Regular files only, keeping their mode so plugins stay executable.
func copyFiles(src, dst string) {
	entries, err := os.ReadDir(src)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.Type().IsRegular() || e.Type()&os.ModeSymlink != 0 {
			copyFile(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name()))
		}
	}
}

func copyFile(src, dst string) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil || info.IsDir() {
		return
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return
	}
	defer out.Close()
	io.Copy(out, in)
}
