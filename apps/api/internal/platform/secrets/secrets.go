// Package secrets provides AES-256-GCM encryption for values stored at rest
// (channel access tokens, etc.). The cipher key is held in env as
// TOKEN_ENCRYPTION_KEY (base64 of 32 raw bytes). Generate one with:
//
//	openssl rand -base64 32
//
// Rotating the key requires re-encrypting every stored ciphertext. For L1 a
// single key is fine; key rotation is a separate routine when we add it.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

const RequiredKeyBytes = 32 // AES-256

type Cipher struct {
	aead cipher.AEAD
}

// New parses a base64-encoded 32-byte key and returns a ready cipher.
func New(keyBase64 string) (*Cipher, error) {
	if keyBase64 == "" {
		return nil, errors.New("empty TOKEN_ENCRYPTION_KEY")
	}
	key, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		return nil, fmt.Errorf("decode key: %w", err)
	}
	if len(key) != RequiredKeyBytes {
		return nil, fmt.Errorf("key must be %d bytes (got %d) — generate with `openssl rand -base64 32`", RequiredKeyBytes, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes new: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	return &Cipher{aead: aead}, nil
}

// Encrypt returns base64(nonce || ciphertext || tag).
func (c *Cipher) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("nonce: %w", err)
	}
	ct := c.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

// Decrypt parses the base64 input and returns the original plaintext.
func (c *Cipher) Decrypt(b64 string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", fmt.Errorf("decode b64: %w", err)
	}
	ns := c.aead.NonceSize()
	if len(data) < ns {
		return "", errors.New("ciphertext too short")
	}
	nonce, ct := data[:ns], data[ns:]
	pt, err := c.aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(pt), nil
}

// GenerateDataKey returns a fresh base64-encoded 32-byte AES-256 key, in the
// same format New() accepts — the per-secret "data key" of an envelope
// encryption scheme: a value is encrypted with its own data key, and the
// data key itself is encrypted with the caller's master Cipher (see
// EncryptWithDataKey/DecryptWithDataKey below). Compromising one secret's
// data key never exposes any other secret, unlike every secret sharing one
// static key.
func GenerateDataKey() (string, error) {
	key := make([]byte, RequiredKeyBytes)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return "", fmt.Errorf("generate data key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

// EncryptWithDataKey encrypts plaintext under dataKeyBase64 (as returned by
// GenerateDataKey), independent of any Cipher instance — used to encrypt a
// secret with its own one-off data key rather than the shared master key.
func EncryptWithDataKey(dataKeyBase64, plaintext string) (string, error) {
	c, err := New(dataKeyBase64)
	if err != nil {
		return "", fmt.Errorf("data key cipher: %w", err)
	}
	return c.Encrypt(plaintext)
}

// DecryptWithDataKey reverses EncryptWithDataKey.
func DecryptWithDataKey(dataKeyBase64, ciphertext string) (string, error) {
	c, err := New(dataKeyBase64)
	if err != nil {
		return "", fmt.Errorf("data key cipher: %w", err)
	}
	return c.Decrypt(ciphertext)
}
