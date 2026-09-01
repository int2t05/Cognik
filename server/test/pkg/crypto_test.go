// Package pkg_test 测试 pkg 包中的加密工具。
package pkg_test

import (
	"strings"
	"testing"

	"opsmind/internal/pkg/crypto"
)

const testEncryptionKey = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"

func resetCrypto(t *testing.T) {
	t.Helper()
	if err := crypto.Init(""); err != nil {
		t.Fatalf("重置加密状态失败: %v", err)
	}
}

func TestCryptoPlaintextMode(t *testing.T) {
	resetCrypto(t)
	defer resetCrypto(t)

	encrypted, err := crypto.Encrypt("sk-plain")
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}
	if encrypted != "sk-plain" {
		t.Fatalf("明文模式应保持原值不变，得到 %q", encrypted)
	}

	decrypted, err := crypto.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("解密失败: %v", err)
	}
	if decrypted != "sk-plain" {
		t.Fatalf("解密结果 = %q，期望 sk-plain", decrypted)
	}
}

func TestCryptoEncryptAddsCipherPrefixAndDecrypts(t *testing.T) {
	resetCrypto(t)
	defer resetCrypto(t)
	if err := crypto.Init(testEncryptionKey); err != nil {
		t.Fatalf("初始化加密失败: %v", err)
	}

	encrypted, err := crypto.Encrypt("sk-secret")
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}
	if !strings.HasPrefix(encrypted, "cipher:") {
		t.Fatalf("加密结果应带 cipher 前缀，得到 %q", encrypted)
	}
	if encrypted == "sk-secret" {
		t.Fatal("加密结果不应等于明文")
	}

	decrypted, err := crypto.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("解密失败: %v", err)
	}
	if decrypted != "sk-secret" {
		t.Fatalf("解密结果 = %q，期望 sk-secret", decrypted)
	}
}

func TestCryptoEncryptIsIdempotentForPrefixedCiphertext(t *testing.T) {
	resetCrypto(t)
	defer resetCrypto(t)
	if err := crypto.Init(testEncryptionKey); err != nil {
		t.Fatalf("初始化加密失败: %v", err)
	}

	encrypted, err := crypto.Encrypt("sk-secret")
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}
	again, err := crypto.Encrypt(encrypted)
	if err != nil {
		t.Fatalf("二次加密失败: %v", err)
	}
	if again != encrypted {
		t.Fatalf("已带前缀的密文应保持不变，得到 %q 期望 %q", again, encrypted)
	}
}

func TestCryptoDecryptSupportsLegacyUnprefixedCiphertext(t *testing.T) {
	resetCrypto(t)
	defer resetCrypto(t)
	if err := crypto.Init(testEncryptionKey); err != nil {
		t.Fatalf("初始化加密失败: %v", err)
	}

	encrypted, err := crypto.Encrypt("sk-legacy")
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}
	legacy := strings.TrimPrefix(encrypted, "cipher:")
	decrypted, err := crypto.Decrypt(legacy)
	if err != nil {
		t.Fatalf("解密旧版密文失败: %v", err)
	}
	if decrypted != "sk-legacy" {
		t.Fatalf("旧版密文解密结果 = %q，期望 sk-legacy", decrypted)
	}
}

func TestCryptoDecryptKeepsPlaintextWhenEncryptionEnabled(t *testing.T) {
	resetCrypto(t)
	defer resetCrypto(t)
	if err := crypto.Init(testEncryptionKey); err != nil {
		t.Fatalf("初始化加密失败: %v", err)
	}

	for _, value := range []string{"sk-plain", "deadbeef"} {
		decrypted, err := crypto.Decrypt(value)
		if err != nil {
			t.Fatalf("解密明文 %q 失败: %v", value, err)
		}
		if decrypted != value {
			t.Fatalf("明文解密结果 = %q，期望 %q", decrypted, value)
		}
	}
}
