// Package crypto 提供 AES-256-GCM 加密解密功能
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
)

// Encrypt 使用 AES-256-GCM 加密明文
// key 必须是 32 字节（256 位）
// 返回 hex(nonce + ciphertext) 格式的密文
func Encrypt(plaintext []byte, key []byte) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("crypto: key 长度必须为 32 字节，当前 %d 字节", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("crypto: 创建 AES cipher 失败: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypto: 创建 GCM 失败: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("crypto: 生成 nonce 失败: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return hex.EncodeToString(ciphertext), nil
}

// Decrypt 解密 hex(nonce + ciphertext) 格式的密文
func Decrypt(encoded string, key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("crypto: key 长度必须为 32 字节，当前 %d 字节", len(key))
	}

	ciphertext, err := hex.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("crypto: hex 解码失败: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: 创建 AES cipher 失败: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: 创建 GCM 失败: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("crypto: 密文长度过短")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("crypto: 解密失败: %w", err)
	}

	return plaintext, nil
}
