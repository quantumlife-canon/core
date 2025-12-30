// Package impl_inmem provides in-memory implementation of the token broker.
// This file provides placeholder encryption for stored tokens.
//
// CRITICAL: This is a PLACEHOLDER implementation for demo/testing only.
// Production MUST use proper envelope encryption with Azure Key Vault or similar.
//
// Reference: docs/TECHNOLOGY_SELECTION_V1.md §6 Identity & Crypto Posture
package impl_inmem

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

// PlaceholderEncryptionAlgorithm identifies this as placeholder crypto.
const PlaceholderEncryptionAlgorithm = "AES-256-GCM-PLACEHOLDER"

// TokenEncryptor provides placeholder encryption for stored tokens.
// PLACEHOLDER: Production should use envelope encryption with HSM/Key Vault.
type TokenEncryptor struct {
	// key is the encryption key (derived from TOKEN_ENC_KEY or generated).
	key []byte

	// algorithm identifies the encryption algorithm.
	algorithm string
}

// NewTokenEncryptor creates a new token encryptor.
// If encryptionKey is empty, a deterministic placeholder key is used.
//
// CRITICAL: The deterministic key is for demo only. Production MUST
// provide a proper key from a key management system.
func NewTokenEncryptor(encryptionKey string) *TokenEncryptor {
	var key []byte
	if encryptionKey != "" {
		// Derive a 256-bit key from the provided key
		hash := sha256.Sum256([]byte(encryptionKey))
		key = hash[:]
	} else {
		// PLACEHOLDER: Deterministic key for demo (NOT SECURE)
		hash := sha256.Sum256([]byte("quantumlife-demo-placeholder-key-not-for-production"))
		key = hash[:]
	}

	return &TokenEncryptor{
		key:       key,
		algorithm: PlaceholderEncryptionAlgorithm,
	}
}

// Encrypt encrypts the plaintext using AES-256-GCM.
// Returns base64-encoded ciphertext.
func (e *TokenEncryptor) Encrypt(plaintext []byte) (string, error) {
	block, err := aes.NewCipher(e.key)
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

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64-encoded ciphertext.
func (e *TokenEncryptor) Decrypt(ciphertextB64 string) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// Algorithm returns the encryption algorithm identifier.
func (e *TokenEncryptor) Algorithm() string {
	return e.algorithm
}

// EncryptString encrypts a string and returns base64-encoded ciphertext.
func (e *TokenEncryptor) EncryptString(plaintext string) (string, error) {
	return e.Encrypt([]byte(plaintext))
}

// DecryptString decrypts base64-encoded ciphertext to a string.
func (e *TokenEncryptor) DecryptString(ciphertextB64 string) (string, error) {
	plaintext, err := e.Decrypt(ciphertextB64)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
