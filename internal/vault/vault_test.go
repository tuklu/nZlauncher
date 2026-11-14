package vault

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// fixed is a source with a known secret, for the derived (stretched) path.
type fixed struct{ secret string }

func (f fixed) Secret(bool) ([]byte, error) { return []byte(f.secret), nil }
func (fixed) derivation() byte              { return derivedKey }
func (fixed) String() string                { return "a test secret" }

var catalog = []byte("[[action]]\nid = \"x\"\nname = \"X\"\ncmd = \"true\"\n")

func TestKeyFileRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys", "catalog.key")
	src := keyFile{path: path}
	sealed, err := Seal(catalog, src)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("cmd")) || !IsSealed(sealed) {
		t.Fatal("the sealed catalog shows its text, or lacks its header")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("key file %v, mode %v", err, info.Mode())
	}
	plain, err := Open(sealed, src)
	if err != nil || !bytes.Equal(plain, catalog) {
		t.Fatalf("got %q, %v", plain, err)
	}
	// a second Seal reuses the key: never a new one over a catalog's
	if _, err := Seal(catalog, src); err != nil {
		t.Fatal(err)
	}
	again, _ := os.ReadFile(path)
	if _, err := Open(sealed, keyFile{path: path}); err != nil {
		t.Errorf("the key changed under an existing catalog: %v (%s)", err, again)
	}
}

func TestOpeningNeedsTheKey(t *testing.T) {
	dir := t.TempDir()
	sealed, _ := Seal(catalog, keyFile{path: filepath.Join(dir, "a")})
	if _, err := Open(sealed, keyFile{path: filepath.Join(dir, "missing")}); err == nil {
		t.Error("opened without its key file")
	}
	other := keyFile{path: filepath.Join(dir, "b")}
	other.Secret(true)
	if _, err := Open(sealed, other); !errors.Is(err, ErrWrongKey) {
		t.Errorf("another key: %v", err)
	}
}

func TestTamperingIsRefused(t *testing.T) {
	src := keyFile{path: filepath.Join(t.TempDir(), "k")}
	sealed, _ := Seal(catalog, src)
	for _, at := range []int{len(magic), headerSize + 2, len(sealed) - 1} {
		bad := bytes.Clone(sealed)
		bad[at] ^= 1
		if _, err := Open(bad, src); err == nil {
			t.Errorf("a flipped byte at %d was accepted", at)
		}
	}
	if _, err := Open([]byte("[[action]]"), src); !errors.Is(err, ErrNotVault) {
		t.Errorf("plain text: %v", err)
	}
}

func TestDerivedKeys(t *testing.T) {
	sealed, err := Seal(catalog, fixed{"correct horse"})
	if err != nil {
		t.Fatal(err)
	}
	if plain, err := Open(sealed, fixed{"correct horse"}); err != nil || !bytes.Equal(plain, catalog) {
		t.Fatalf("got %q, %v", plain, err)
	}
	if _, err := Open(sealed, fixed{"battery staple"}); !errors.Is(err, ErrWrongKey) {
		t.Errorf("a wrong passphrase: %v", err)
	}
	// a stretched catalog does not open with a raw key, and the reverse
	if _, err := Open(sealed, keyFile{path: filepath.Join(t.TempDir(), "k")}); err == nil {
		t.Error("a derived catalog opened with a key file")
	}
}

func TestParseSource(t *testing.T) {
	for spec, want := range map[string]string{
		"":                    "/d/catalog.key",
		"file":                "/d/catalog.key",
		"file:/media/usb/key": "/media/usb/key",
		"keyring":             "the keyring",
		"passphrase":          "a passphrase",
		"op://Private/nz/key": "op://Private/nz/key",
	} {
		src, err := ParseSource(spec, "/d/catalog.key")
		if err != nil || src.String() != want {
			t.Errorf("%q: got %v, %v", spec, src, err)
		}
	}
	if _, err := ParseSource("tpm", "/d/k"); err == nil {
		t.Error("an unknown key source was accepted")
	}
}
