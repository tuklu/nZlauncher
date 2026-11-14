package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tuklu/nZlauncher/internal/config"
)

// The whole cycle against a throwaway home: encrypt, load, decrypt.
func TestCatalogEncryptLoadDecrypt(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))

	before, err := config.Load() // writes the default catalog
	if err != nil {
		t.Fatal(err)
	}
	if err := catalogEncrypt(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(config.CatalogPath()); !os.IsNotExist(err) {
		t.Error("the plain catalog is still there")
	}
	sealedText, _ := os.ReadFile(config.SealedCatalogPath())
	if strings.Contains(string(sealedText), "[[action]]") {
		t.Error("the encrypted catalog is readable")
	}
	if info, _ := os.Stat(config.SealedCatalogPath()); info.Mode().Perm() != 0o600 {
		t.Errorf("encrypted catalog mode %v", info.Mode().Perm())
	}

	after, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Actions) != len(before.Actions) || len(after.Actions) == 0 {
		t.Fatalf("%d actions before, %d after", len(before.Actions), len(after.Actions))
	}
	if err := catalogEncrypt(); err == nil {
		t.Error("encrypted twice")
	}

	if err := catalogDecrypt(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(config.SealedCatalogPath()); !os.IsNotExist(err) {
		t.Error("the encrypted catalog is still there")
	}
	if again, err := config.Load(); err != nil || len(again.Actions) != len(before.Actions) {
		t.Errorf("after decrypting: %v", err)
	}
}
