// Package vault keeps the catalog encrypted at rest, for those who do not
// want every script running as them to read their commands. The file is
// AES-256-GCM, so a changed or corrupted file is refused, not misread.
//
// The key never sits next to the file. It comes from one of four places,
// picked by [catalog] key in config.toml:
//
//	file         the default and needs nothing installed: a random key in a
//	             file of its own (0600, outside the config directory), so the
//	             catalog can be synced or committed without it. Scripts
//	             running as you can read it, as they can the keyring.
//	keyring      a random key in the Secret Service (gnome-keyring, KWallet,
//	             KeePassXC). No prompt; protects backups, sync and other
//	             users, not scripts running as you, which can ask too.
//	op://...     a secret in 1Password, read with op. Reading it needs
//	             1Password's approval, so a script trying it shows a prompt.
//	passphrase   typed at first use. Nothing on disk opens without you.
//
// A key built into the program would protect nothing: the program is public,
// so everyone would hold the same key.
package vault

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
)

// The file: magic, how the key is made, salt, nonce, then the sealed data.
const (
	magic      = "nzcat1"
	saltSize   = 16
	headerSize = len(magic) + 1 + saltSize
	// PBKDF2-SHA256 at OWASP's 2023 count: a passphrase guess costs real time
	iterations = 600_000
)

// How the 32-byte key comes out of the source's secret.
const (
	rawKey     byte = 0 // the secret is the key (file, keyring)
	derivedKey byte = 1 // the secret is stretched with the salt (op, passphrase)
)

var (
	ErrNotVault = errors.New("not an encrypted catalog")
	ErrWrongKey = errors.New("the key does not open this catalog, or the file was changed")
)

// Source hands out the secret a key is made from.
type Source interface {
	// Secret returns the secret, creating one first when create is set and
	// none exists yet (only the keyring can).
	Secret(create bool) ([]byte, error)
	derivation() byte
	String() string
}

// Seal encrypts plain with a key from src.
func Seal(plain []byte, src Source) ([]byte, error) {
	secret, err := src.Secret(true)
	if err != nil {
		return nil, err
	}
	header := make([]byte, headerSize)
	copy(header, magic)
	header[len(magic)] = src.derivation()
	if _, err := rand.Read(header[len(magic)+1:]); err != nil {
		return nil, err
	}
	aead, err := newAEAD(secret, header)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	out := append(header, nonce...)
	// the header is authenticated too: changing how the key is made fails
	return aead.Seal(out, nonce, plain, header), nil
}

// Open decrypts what Seal wrote.
func Open(sealed []byte, src Source) ([]byte, error) {
	if len(sealed) < headerSize || !bytes.HasPrefix(sealed, []byte(magic)) {
		return nil, ErrNotVault
	}
	header := sealed[:headerSize]
	if header[len(magic)] != src.derivation() {
		return nil, fmt.Errorf("the catalog was encrypted for another kind of key than %s", src)
	}
	secret, err := src.Secret(false)
	if err != nil {
		return nil, err
	}
	aead, err := newAEAD(secret, header)
	if err != nil {
		return nil, err
	}
	rest := sealed[headerSize:]
	if len(rest) < aead.NonceSize() {
		return nil, ErrWrongKey
	}
	plain, err := aead.Open(nil, rest[:aead.NonceSize()], rest[aead.NonceSize():], header)
	if err != nil {
		return nil, ErrWrongKey
	}
	return plain, nil
}

// IsSealed reports whether data is an encrypted catalog.
func IsSealed(data []byte) bool {
	return bytes.HasPrefix(data, []byte(magic))
}

func newAEAD(secret, header []byte) (cipher.AEAD, error) {
	key := secret
	if header[len(magic)] == derivedKey {
		salt := header[len(magic)+1 : headerSize]
		var err error
		if key, err = stretch(secret, salt); err != nil {
			return nil, err
		}
	} else if len(key) != 32 {
		return nil, fmt.Errorf("the stored key is %d bytes, not 32", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// Stretching is the slow part; a daemon pays it once per secret and salt.
var stretched struct {
	sync.Mutex
	keys map[string][]byte
}

func stretch(secret, salt []byte) ([]byte, error) {
	id := string(secret) + "\x00" + string(salt)
	stretched.Lock()
	defer stretched.Unlock()
	if key, ok := stretched.keys[id]; ok {
		return key, nil
	}
	key, err := pbkdf2.Key(sha256.New, string(secret), salt, iterations, 32)
	if err != nil {
		return nil, err
	}
	if stretched.keys == nil {
		stretched.keys = map[string][]byte{}
	}
	stretched.keys[id] = key
	return key, nil
}
