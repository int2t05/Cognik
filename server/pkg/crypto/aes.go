// Package crypto 提供 AES-256-GCM 加密工具。
//
// 用于 LLM 配置 API Key 等敏感字段的加密存储。
// 密钥通过环境变量 OPSMIND_ENCRYPTION_KEY 注入（32 字节 hex 编码）。
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"strings"
)

var encKey []byte

const cipherPrefix = "cipher:"

// Init 初始化加密密钥（32 字节 = AES-256）。
//
// keyHex 为 64 字符 hex 编码字符串，空字符串表示禁用加密（明文存储）。
func Init(keyHex string) error {
	if keyHex == "" {
		encKey = nil
		return nil
	}
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return err
	}
	if len(key) != 32 {
		return errors.New("加密密钥必须为 32 字节（64 字符 hex）")
	}
	encKey = key
	return nil
}

// Encrypt 使用 AES-256-GCM 加密明文。
//
// 返回带 cipher: 前缀的 hex 编码密文（hex 内容含 12 字节 nonce 前缀）。
// 若密钥未初始化，返回明文（向后兼容无加密环境）。
func Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if strings.HasPrefix(plaintext, cipherPrefix) {
		return plaintext, nil
	}
	if encKey == nil {
		return plaintext, nil
	}

	block, err := aes.NewCipher(encKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return cipherPrefix + hex.EncodeToString(ciphertext), nil
}

// Decrypt 使用 AES-256-GCM 解密密文。
//
// 接受 hex 编码的密文。若密钥未初始化，返回原文（明文存储兼容）。
func Decrypt(cipherHex string) (string, error) {
	if cipherHex == "" {
		return "", nil
	}
	if encKey == nil {
		return cipherHex, nil
	}

	prefixed := strings.HasPrefix(cipherHex, cipherPrefix)
	raw := strings.TrimPrefix(cipherHex, cipherPrefix)
	ciphertext, err := hex.DecodeString(raw)
	if err != nil {
		if prefixed {
			return "", err
		}
		return cipherHex, nil
	}
	block, err := aes.NewCipher(encKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		if prefixed {
			return "", errors.New("密文过短")
		}
		return cipherHex, nil
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		if prefixed {
			return "", err
		}
		return cipherHex, nil
	}
	return string(plaintext), nil
}
