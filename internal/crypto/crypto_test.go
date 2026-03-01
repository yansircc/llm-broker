package crypto

import (
	"strings"
	"testing"
)

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	c := New("test-secret-key-12345")
	plaintext := "hello world, this is a secret token"
	encrypted, err := c.Encrypt(plaintext, "salt1")
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if encrypted == plaintext {
		t.Error("encrypted should differ from plaintext")
	}

	decrypted, err := c.Decrypt(encrypted, "salt1")
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if decrypted != plaintext {
		t.Errorf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestDifferentSalts(t *testing.T) {
	c := New("test-secret-key-12345")
	plaintext := "same plaintext"
	enc1, _ := c.Encrypt(plaintext, "salt1")
	enc2, _ := c.Encrypt(plaintext, "salt2")

	// Different salts mean different derived keys → different ciphertexts (structurally different)
	// Note: even with the same salt, random IV makes them different,
	// but with different salts the keys are fundamentally different
	dec1, _ := c.Decrypt(enc1, "salt1")
	dec2, _ := c.Decrypt(enc2, "salt2")
	if dec1 != dec2 {
		t.Error("both should decrypt to same plaintext")
	}

	// Cross-salt decryption: wrong key should produce wrong plaintext or error
	dec3, err := c.Decrypt(enc1, "salt2")
	if err == nil && dec3 == plaintext {
		t.Error("decrypting with wrong salt should not produce correct plaintext")
	}
}

func TestRandomIV(t *testing.T) {
	c := New("test-secret-key-12345")
	enc1, _ := c.Encrypt("same", "salt")
	enc2, _ := c.Encrypt("same", "salt")
	if enc1 == enc2 {
		t.Error("same plaintext should produce different ciphertexts due to random IV")
	}
}

func TestDecrypt_InvalidFormat(t *testing.T) {
	c := New("test-secret-key-12345")
	// Derive key for salt first
	c.DeriveKey("salt")

	// Missing colon
	_, err := c.Decrypt("nodelimiter", "salt")
	if err == nil {
		t.Error("missing colon should fail")
	}

	// Invalid hex
	_, err = c.Decrypt("zzzz:xxxx", "salt")
	if err == nil {
		t.Error("invalid hex should fail")
	}

	// Valid hex but wrong length IV
	_, err = c.Decrypt("abcd:abcd", "salt")
	if err == nil || !strings.Contains(err.Error(), "iv length") {
		t.Error("short IV should fail with iv length error")
	}
}
