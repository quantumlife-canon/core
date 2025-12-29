// Package crypto defines interfaces for cryptographic operations.
// Implementations must support algorithm agility for post-quantum readiness.
//
// Reference: docs/TECHNOLOGY_SELECTION_V1.md §6 Identity & Crypto Posture
package crypto

import (
	"context"
	"time"
)

// Signer signs data using a private key.
type Signer interface {
	// Sign creates a signature for the given data.
	// Returns the signature bytes and any error.
	Sign(ctx context.Context, data []byte) ([]byte, error)

	// KeyID returns the identifier of the signing key.
	KeyID() string

	// Algorithm returns the signing algorithm identifier.
	Algorithm() string
}

// Verifier verifies signatures using a public key.
type Verifier interface {
	// Verify checks if the signature is valid for the given data.
	// Returns nil if valid, error otherwise.
	Verify(ctx context.Context, data []byte, signature []byte) error

	// KeyID returns the identifier of the verification key.
	KeyID() string

	// Algorithm returns the verification algorithm identifier.
	Algorithm() string
}

// KeyManager manages cryptographic keys with rotation support.
type KeyManager interface {
	// GetSigner returns a signer for the specified key.
	GetSigner(ctx context.Context, keyID string) (Signer, error)

	// GetVerifier returns a verifier for the specified key.
	GetVerifier(ctx context.Context, keyID string) (Verifier, error)

	// RotateKey creates a new version of the specified key.
	RotateKey(ctx context.Context, keyID string) (KeyMetadata, error)

	// ListKeys returns metadata for all keys.
	ListKeys(ctx context.Context) ([]KeyMetadata, error)
}

// KeyMetadata contains metadata about a cryptographic key.
// No key material is included in this struct.
type KeyMetadata struct {
	// KeyID uniquely identifies this key.
	KeyID string

	// Algorithm identifies the cryptographic algorithm.
	// Examples: "Ed25519", "RSA-4096", "Kyber-768"
	Algorithm string

	// AlgorithmVersion tracks algorithm version for agility.
	AlgorithmVersion int

	// CreatedAt is when this key was created.
	CreatedAt time.Time

	// ExpiresAt is when this key expires (zero if no expiry).
	ExpiresAt time.Time

	// IsActive indicates if this key is currently active.
	IsActive bool

	// PQExtension indicates if post-quantum extension is available.
	PQExtension bool
}

// Encryptor encrypts data using a public key or symmetric key.
type Encryptor interface {
	// Encrypt encrypts the plaintext.
	Encrypt(ctx context.Context, plaintext []byte) ([]byte, error)

	// KeyID returns the identifier of the encryption key.
	KeyID() string
}

// Decryptor decrypts data using a private key or symmetric key.
type Decryptor interface {
	// Decrypt decrypts the ciphertext.
	Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error)

	// KeyID returns the identifier of the decryption key.
	KeyID() string
}
