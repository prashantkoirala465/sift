package security

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func testKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, KeySize)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generate test key: %v", err)
	}
	return key
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := testKey(t)
	plaintext := []byte("ya29.a0AfH6SMC...refresh-token-material")

	sealed, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if bytes.Equal(sealed, plaintext) {
		t.Fatal("ciphertext equals plaintext, encryption did nothing")
	}

	got, err := Decrypt(key, sealed)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("got %q, want %q", got, plaintext)
	}
}

func TestDecryptWrongKeyFails(t *testing.T) {
	sealed, err := Encrypt(testKey(t), []byte("secret"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	if _, err := Decrypt(testKey(t), sealed); err == nil {
		t.Fatal("decrypt succeeded with the wrong key")
	}
}

func TestDecryptTamperedCiphertextFails(t *testing.T) {
	key := testKey(t)
	sealed, err := Encrypt(key, []byte("secret"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	sealed[len(sealed)-1] ^= 0xFF // flip the last byte of the GCM tag

	if _, err := Decrypt(key, sealed); err == nil {
		t.Fatal("decrypt succeeded on tampered ciphertext")
	}
}

func TestEncryptRejectsWrongKeySize(t *testing.T) {
	if _, err := Encrypt([]byte("too short"), []byte("secret")); err == nil {
		t.Fatal("encrypt accepted a key of the wrong size")
	}
}
