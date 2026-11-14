package vault

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/tuklu/nZlauncher/internal/prompt"
)

// ParseSource reads [catalog] key: "file" (the default) or "file:<path>",
// "keyring", "passphrase", or an op:// reference. defaultKeyFile is where
// "file" keeps its key.
func ParseSource(spec, defaultKeyFile string) (Source, error) {
	switch {
	case spec == "" || spec == "file":
		return remembered(keyFile{path: defaultKeyFile}), nil
	case strings.HasPrefix(spec, "file:"):
		return remembered(keyFile{path: expandHome(strings.TrimPrefix(spec, "file:"))}), nil
	case spec == "keyring":
		return remembered(keyring{}), nil
	case spec == "passphrase":
		return remembered(passphrase{}), nil
	case strings.HasPrefix(spec, "op://"):
		return remembered(onePassword{ref: spec}), nil
	}
	return nil, fmt.Errorf(`[catalog] key is "file", "file:<path>", "keyring", "passphrase" or an op:// reference, not %q`, spec)
}

// remembered asks a source once per process: a daemon unlocks the catalog
// when it first reads it, then never prompts again.
func remembered(s Source) Source {
	return &memo{Source: s}
}

type memo struct {
	Source
	mu     sync.Mutex
	secret []byte
}

func (m *memo) Secret(create bool) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.secret != nil {
		return m.secret, nil
	}
	secret, err := m.Source.Secret(create)
	if err != nil {
		return nil, err
	}
	m.secret = secret
	return secret, nil
}

// ── key file ───────────────────────────────────────────────────────────────

// keyFile is the native way: nzlauncher makes a random key and keeps it in a
// file of its own, readable by you alone and outside the config directory,
// so the catalog can be copied, synced or committed without its key.
type keyFile struct{ path string }

func (keyFile) derivation() byte { return rawKey }
func (k keyFile) String() string { return k.path }

func (k keyFile) Secret(create bool) ([]byte, error) {
	encoded, err := os.ReadFile(k.path)
	if errors.Is(err, os.ErrNotExist) && create {
		return k.generate()
	}
	if err != nil {
		return nil, fmt.Errorf("the catalog key: %w", err)
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("%s is not a catalog key", k.path)
	}
	return key, nil
}

func (k keyFile) generate() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(k.path), 0o700); err != nil {
		return nil, err
	}
	// O_EXCL: never overwrite a key some catalog may still need
	f, err := os.OpenFile(k.path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if _, err := fmt.Fprintln(f, base64.StdEncoding.EncodeToString(key)); err != nil {
		return nil, err
	}
	return key, f.Sync()
}

func expandHome(path string) string {
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, rest)
		}
	}
	return path
}

// ── keyring ────────────────────────────────────────────────────────────────

// keyring keeps a random key in the Secret Service's default collection.
type keyring struct{}

func (keyring) derivation() byte { return rawKey }
func (keyring) String() string   { return "the keyring" }

const secretsBus = "org.freedesktop.secrets"

var keyAttributes = map[string]string{"application": "nzlauncher", "purpose": "catalog-key"}

// secret is the Secret Service's (oayays) Secret struct.
type secret struct {
	Session     dbus.ObjectPath
	Parameters  []byte
	Value       []byte
	ContentType string
}

func (keyring) Secret(create bool) ([]byte, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil, err
	}
	service := conn.Object(secretsBus, "/org/freedesktop/secrets")
	var ignored dbus.Variant
	var session dbus.ObjectPath
	if err := service.Call("org.freedesktop.Secret.Service.OpenSession", 0, "plain", dbus.MakeVariant("")).
		Store(&ignored, &session); err != nil {
		return nil, fmt.Errorf("no keyring answers (gnome-keyring, KWallet, KeePassXC): %w", err)
	}
	defer conn.Object(secretsBus, session).Call("org.freedesktop.Secret.Session.Close", 0)

	var unlocked, locked []dbus.ObjectPath
	if err := service.Call("org.freedesktop.Secret.Service.SearchItems", 0, keyAttributes).
		Store(&unlocked, &locked); err != nil {
		return nil, err
	}
	if len(unlocked)+len(locked) == 0 {
		if !create {
			return nil, errors.New("the keyring holds no nzlauncher catalog key")
		}
		return createKey(conn, service, session)
	}
	if len(locked) > 0 {
		var now []dbus.ObjectPath
		var pending dbus.ObjectPath
		if err := service.Call("org.freedesktop.Secret.Service.Unlock", 0, locked).Store(&now, &pending); err != nil {
			return nil, err
		}
		if err := waitForPrompt(conn, pending); err != nil {
			return nil, err
		}
	}
	item := append(unlocked, locked...)[0]
	var secrets map[dbus.ObjectPath]secret
	if err := service.Call("org.freedesktop.Secret.Service.GetSecrets", 0, []dbus.ObjectPath{item}, session).
		Store(&secrets); err != nil {
		return nil, err
	}
	return secrets[item].Value, nil
}

func createKey(conn *dbus.Conn, service dbus.BusObject, session dbus.ObjectPath) ([]byte, error) {
	var collection dbus.ObjectPath
	if err := service.Call("org.freedesktop.Secret.Service.ReadAlias", 0, "default").Store(&collection); err != nil {
		return nil, err
	}
	if collection == "/" {
		return nil, errors.New("the keyring has no default collection to keep the key in")
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	properties := map[string]dbus.Variant{
		"org.freedesktop.Secret.Item.Label":      dbus.MakeVariant("nzlauncher catalog key"),
		"org.freedesktop.Secret.Item.Attributes": dbus.MakeVariant(keyAttributes),
	}
	var item, pending dbus.ObjectPath
	err := conn.Object(secretsBus, collection).Call("org.freedesktop.Secret.Collection.CreateItem", 0,
		properties, secret{session, []byte{}, key, "application/octet-stream"}, false).Store(&item, &pending)
	if err != nil {
		return nil, err
	}
	if err := waitForPrompt(conn, pending); err != nil {
		return nil, err
	}
	return key, nil
}

// waitForPrompt shows the keyring's own unlock prompt, if it has one, and
// waits for the answer.
func waitForPrompt(conn *dbus.Conn, pending dbus.ObjectPath) error {
	if pending == "/" || pending == "" {
		return nil
	}
	if err := conn.AddMatchSignal(dbus.WithMatchObjectPath(pending),
		dbus.WithMatchInterface("org.freedesktop.Secret.Prompt"), dbus.WithMatchMember("Completed")); err != nil {
		return err
	}
	signals := make(chan *dbus.Signal, 1)
	conn.Signal(signals)
	defer conn.RemoveSignal(signals)
	if err := conn.Object(secretsBus, pending).Call("org.freedesktop.Secret.Prompt.Prompt", 0, "").Err; err != nil {
		return err
	}
	timeout := time.After(2 * time.Minute)
	for {
		select {
		case s := <-signals:
			if s.Path != pending {
				continue
			}
			if dismissed, _ := s.Body[0].(bool); dismissed {
				return errors.New("the keyring stayed locked")
			}
			return nil
		case <-timeout:
			return errors.New("the keyring prompt was not answered")
		}
	}
}

// ── 1Password ──────────────────────────────────────────────────────────────

// onePassword reads any secret at an op:// reference; the key is stretched
// from it, so a generated password item is all it needs.
type onePassword struct{ ref string }

func (onePassword) derivation() byte { return derivedKey }
func (o onePassword) String() string { return o.ref }

func (o onePassword) Secret(bool) ([]byte, error) {
	out, err := exec.Command("op", "read", "--no-newline", o.ref).Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return nil, fmt.Errorf("op could not read %s: %s", o.ref, bytes.TrimSpace(exit.Stderr))
		}
		return nil, fmt.Errorf("reading %s needs the 1Password CLI (op): %w", o.ref, err)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s is empty", o.ref)
	}
	return out, nil
}

// ── passphrase ─────────────────────────────────────────────────────────────

type passphrase struct{}

func (passphrase) derivation() byte { return derivedKey }
func (passphrase) String() string   { return "a passphrase" }

// Secret asks for the passphrase; a new one is asked twice.
func (passphrase) Secret(create bool) ([]byte, error) {
	first, err := prompt.Secret("Catalog passphrase")
	if err != nil {
		return nil, err
	}
	if create {
		again, err := prompt.Secret("Catalog passphrase, again")
		if err != nil {
			return nil, err
		}
		if again != first {
			return nil, errors.New("the passphrases differ")
		}
	}
	return []byte(first), nil
}
