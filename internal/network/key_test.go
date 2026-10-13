// internal/network/key_test.go - Tests for keys

package network

import (
	"encoding/hex"
	"testing"
)

// R-KEY-4: public keys are derived from private keys with X25519. The
// vector is from RFC 7748, section 6.1.
func TestPublicKeyRFC7748(t *testing.T) {
	var priv, want Key
	mustHex(t, priv[:], "77076d0a7318a57d3c16c17251b26645df4c2f87ebc0992ab177fba51db92c2a")
	mustHex(t, want[:], "8520f0098930a754748b7ddcb43ef75a0dbf3a0d26381af4eba4a98eaa9b4e6a")
	if got := priv.PublicKey(); got != want {
		t.Errorf("PublicKey() = %s, want %s", got, want)
	}
}

func mustHex(t *testing.T, dst []byte, s string) {
	t.Helper()
	if _, err := hex.Decode(dst, []byte(s)); err != nil {
		t.Fatal(err)
	}
}

// R-CLI-12: generated private keys are clamped.
func TestGeneratePrivateKeyClamped(t *testing.T) {
	for range 20 {
		k, err := GeneratePrivateKey()
		if err != nil {
			t.Fatal(err)
		}
		if k[0]&7 != 0 || k[31]&128 != 0 || k[31]&64 == 0 {
			t.Fatalf("key %x is not clamped", k)
		}
	}
}

func TestGenerateKeyIsRandom(t *testing.T) {
	a, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	b, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Error("two generated keys are equal")
	}
}

// R-KEY-1
func TestParseKey(t *testing.T) {
	k := testKey(7)
	got, err := ParseKey(k.String())
	if err != nil || got != k {
		t.Errorf("ParseKey(%s) = %s, %v; want the same key", k, got, err)
	}

	for _, s := range []string{
		"",
		"not base64!",
		"AAAA", // too short
		"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", // too long
	} {
		if _, err := ParseKey(s); err == nil {
			t.Errorf("ParseKey(%q) succeeded, want an error", s)
		}
	}
}

// testKey returns a private key with every byte set to b. Any 32 bytes are
// a valid X25519 private key.
func testKey(b byte) Key {
	var k Key
	for i := range k {
		k[i] = b
	}
	return k
}
