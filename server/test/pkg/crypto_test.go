// Package pkg_test tests exported utilities in pkg.
package pkg_test

import (
	"strings"
	"testing"

	"opsmind/pkg/crypto"
)

const testEncryptionKey = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"

func resetCrypto(t *testing.T) {
	t.Helper()
	if err := crypto.Init(""); err != nil {
		t.Fatalf("reset crypto failed: %v", err)
	}
}

func TestCryptoPlaintextMode(t *testing.T) {
	resetCrypto(t)
	defer resetCrypto(t)

	encrypted, err := crypto.Encrypt("sk-plain")
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if encrypted != "sk-plain" {
		t.Fatalf("plaintext mode should keep original value, got %q", encrypted)
	}

	decrypted, err := crypto.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if decrypted != "sk-plain" {
		t.Fatalf("Decrypt = %q, want sk-plain", decrypted)
	}
}

func TestCryptoEncryptAddsCipherPrefixAndDecrypts(t *testing.T) {
	resetCrypto(t)
	defer resetCrypto(t)
	if err := crypto.Init(testEncryptionKey); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	encrypted, err := crypto.Encrypt("sk-secret")
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if !strings.HasPrefix(encrypted, "cipher:") {
		t.Fatalf("encrypted value should have cipher prefix, got %q", encrypted)
	}
	if encrypted == "sk-secret" {
		t.Fatal("encrypted value should not equal plaintext")
	}

	decrypted, err := crypto.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if decrypted != "sk-secret" {
		t.Fatalf("Decrypt = %q, want sk-secret", decrypted)
	}
}

func TestCryptoEncryptIsIdempotentForPrefixedCiphertext(t *testing.T) {
	resetCrypto(t)
	defer resetCrypto(t)
	if err := crypto.Init(testEncryptionKey); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	encrypted, err := crypto.Encrypt("sk-secret")
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	again, err := crypto.Encrypt(encrypted)
	if err != nil {
		t.Fatalf("Encrypt encrypted value failed: %v", err)
	}
	if again != encrypted {
		t.Fatalf("Encrypt should keep prefixed ciphertext unchanged, got %q want %q", again, encrypted)
	}
}

func TestCryptoDecryptSupportsLegacyUnprefixedCiphertext(t *testing.T) {
	resetCrypto(t)
	defer resetCrypto(t)
	if err := crypto.Init(testEncryptionKey); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	encrypted, err := crypto.Encrypt("sk-legacy")
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	legacy := strings.TrimPrefix(encrypted, "cipher:")
	decrypted, err := crypto.Decrypt(legacy)
	if err != nil {
		t.Fatalf("Decrypt legacy ciphertext failed: %v", err)
	}
	if decrypted != "sk-legacy" {
		t.Fatalf("Decrypt legacy = %q, want sk-legacy", decrypted)
	}
}

func TestCryptoDecryptKeepsPlaintextWhenEncryptionEnabled(t *testing.T) {
	resetCrypto(t)
	defer resetCrypto(t)
	if err := crypto.Init(testEncryptionKey); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	for _, value := range []string{"sk-plain", "deadbeef"} {
		decrypted, err := crypto.Decrypt(value)
		if err != nil {
			t.Fatalf("Decrypt plaintext %q failed: %v", value, err)
		}
		if decrypted != value {
			t.Fatalf("Decrypt plaintext = %q, want %q", decrypted, value)
		}
	}
}
