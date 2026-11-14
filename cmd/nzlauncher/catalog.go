package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/tuklu/nZlauncher/internal/config"
	"github.com/tuklu/nZlauncher/internal/vault"
)

// runCatalog switches the catalog between plain text and encrypted, and
// edits it either way.
func runCatalog(args []string) error {
	if len(args) != 1 {
		return misuse("catalog", "catalog takes status, encrypt, decrypt or edit")
	}
	switch args[0] {
	case "status":
		return catalogStatus()
	case "encrypt":
		return catalogEncrypt()
	case "decrypt":
		return catalogDecrypt()
	case "edit":
		return catalogEdit()
	}
	return misuse("catalog", "no catalog action %q", args[0])
}

func sealed() bool {
	_, err := os.Stat(config.SealedCatalogPath())
	return err == nil
}

func catalogStatus() error {
	if !sealed() {
		fmt.Println("plain", config.CatalogPath())
		return nil
	}
	src, err := config.CatalogSource()
	if err != nil {
		return err
	}
	fmt.Printf("encrypted %s, key from %s\n", config.SealedCatalogPath(), src)
	return nil
}

func catalogEncrypt() error {
	if sealed() {
		return errors.New("the catalog is encrypted already")
	}
	// a dotfiles setup links the catalog in from a repo: encrypting the link
	// would leave the original in plain text, and wiping it would wipe the repo's
	if target, err := os.Readlink(config.CatalogPath()); err == nil {
		return fmt.Errorf("%s links to %s, which would stay in plain text: "+
			"replace the link with a copy of the file first", config.CatalogPath(), target)
	}
	plain, err := os.ReadFile(config.CatalogPath())
	if err != nil {
		return err
	}
	if err := validCatalog(plain); err != nil {
		return err
	}
	src, err := config.CatalogSource()
	if err != nil {
		return err
	}
	if err := writeSealed(plain, src); err != nil {
		return err
	}
	if err := removePlain(config.CatalogPath()); err != nil {
		return err
	}
	fmt.Printf("encrypted %s, key from %s\n", config.SealedCatalogPath(), src)
	if src.String() == config.KeyFilePath() {
		fmt.Printf("keep a copy of %s somewhere safe: without it the catalog cannot be opened\n", config.KeyFilePath())
	}
	return nil
}

func catalogDecrypt() error {
	if !sealed() {
		return errors.New("the catalog is not encrypted")
	}
	if _, err := os.Stat(config.CatalogPath()); err == nil {
		return fmt.Errorf("%s exists: move it away first", config.CatalogPath())
	}
	plain, err := config.CatalogText()
	if err != nil {
		return err
	}
	if err := os.WriteFile(config.CatalogPath(), plain, 0o644); err != nil {
		return err
	}
	if err := os.Remove(config.SealedCatalogPath()); err != nil {
		return err
	}
	fmt.Println("plain", config.CatalogPath())
	return nil
}

// catalogEdit opens the catalog in $VISUAL or $EDITOR. An encrypted one is
// decrypted to a file only you can read in $XDG_RUNTIME_DIR (memory, not
// disk), encrypted again on save, and the file removed.
func catalogEdit() error {
	if !sealed() {
		return editor(config.CatalogPath())
	}
	plain, err := config.CatalogText()
	if err != nil {
		return err
	}
	runtime := os.Getenv("XDG_RUNTIME_DIR")
	if runtime == "" {
		return errors.New("editing an encrypted catalog needs XDG_RUNTIME_DIR, which never touches disk")
	}
	f, err := os.CreateTemp(runtime, "nzlauncher-catalog-*.toml")
	if err != nil {
		return err
	}
	draft := f.Name()
	_, werr := f.Write(plain)
	cerr := f.Close()
	if err := errors.Join(werr, cerr); err != nil {
		os.Remove(draft)
		return err
	}
	if err := editor(draft); err != nil {
		os.Remove(draft)
		return err
	}
	edited, err := os.ReadFile(draft)
	if err != nil {
		return err
	}
	if bytes.Equal(edited, plain) {
		return os.Remove(draft)
	}
	if err := validCatalog(edited); err != nil {
		// the edit is not lost: it stays where it was written
		return fmt.Errorf("%w; your edit is kept in %s", err, draft)
	}
	src, err := config.CatalogSource()
	if err != nil {
		return fmt.Errorf("%w; your edit is kept in %s", err, draft)
	}
	if err := writeSealed(edited, src); err != nil {
		return fmt.Errorf("%w; your edit is kept in %s", err, draft)
	}
	return os.Remove(draft)
}

func editor(path string) error {
	name := os.Getenv("VISUAL")
	if name == "" {
		name = os.Getenv("EDITOR")
	}
	if name == "" {
		name = "vi"
	}
	cmd := exec.Command("sh", "-c", name+` "$1"`, "editor", path)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

func validCatalog(text []byte) error {
	var c config.Catalog
	if _, err := toml.Decode(string(text), &c); err != nil {
		return fmt.Errorf("the catalog is not valid TOML: %w", err)
	}
	return nil
}

// writeSealed replaces the encrypted catalog in one step, so a reader sees
// the old one or the new one, never half of either.
func writeSealed(plain []byte, src vault.Source) error {
	data, err := vault.Seal(plain, src)
	if err != nil {
		return err
	}
	path := config.SealedCatalogPath()
	tmp, err := os.CreateTemp(filepath.Dir(path), ".catalog-*.enc")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := errors.Join(tmp.Sync(), tmp.Close()); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// removePlain overwrites the plain catalog before removing it. On a
// copy-on-write filesystem (btrfs) or an SSD the old blocks may survive.
func removePlain(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if f, err := os.OpenFile(path, os.O_WRONLY, 0); err == nil {
		f.Write(make([]byte, info.Size()))
		f.Sync()
		f.Close()
	}
	return os.Remove(path)
}
