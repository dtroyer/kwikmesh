// internal/network/key.go - WireGuard keys

package network

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
)

// KeyLen is the length in bytes of a WireGuard key (R-KEY-1).
const KeyLen = 32

// Key is a Curve25519 private, public or preshared key.
type Key [KeyLen]byte

var errBadKey = errors.New("must be a base64-encoded 32-byte key")

// ParseKey decodes a base64-encoded key (R-KEY-1).
func ParseKey(s string) (Key, error) {
	var k Key
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil || len(b) != KeyLen {
		return k, errBadKey
	}
	copy(k[:], b)
	return k, nil
}

// String returns the key in base64, as wg(8) prints it.
func (k Key) String() string {
	return base64.StdEncoding.EncodeToString(k[:])
}

// GenerateKey returns a random key, as wg genpsk does.
func GenerateKey() (Key, error) {
	var k Key
	if _, err := rand.Read(k[:]); err != nil {
		return k, fmt.Errorf("generating key: %w", err)
	}
	return k, nil
}

// GeneratePrivateKey returns a random private key clamped as Curve25519
// requires, as wg genkey does (R-CLI-12).
func GeneratePrivateKey() (Key, error) {
	k, err := GenerateKey()
	if err != nil {
		return k, err
	}
	k[0] &= 248
	k[31] &= 127
	k[31] |= 64
	return k, nil
}

// PublicKey derives the public key for private key k.
func (k Key) PublicKey() Key {
	priv, err := ecdh.X25519().NewPrivateKey(k[:])
	if err != nil {
		// NewPrivateKey only rejects keys of the wrong length.
		panic(err)
	}
	var pub Key
	copy(pub[:], priv.PublicKey().Bytes())
	return pub
}

// readKeyFile reads a private key from a file containing it in base64.
func readKeyFile(path string) (Key, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Key{}, fmt.Errorf("private_key_file: %w", err)
	}
	k, err := ParseKey(strings.TrimSpace(string(data)))
	if err != nil {
		return Key{}, fmt.Errorf("private_key_file %s: %w", path, err)
	}
	return k, nil
}
