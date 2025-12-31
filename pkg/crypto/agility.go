// Package crypto provides cryptographic primitives with algorithm agility.
//
// This file implements crypto agility for post-quantum readiness.
// All operations use algorithm identifiers, not hardcoded algorithms.
//
// CRITICAL: Uses only Go stdlib. No external dependencies.
//
// Reference: docs/POST_QUANTUM_CRYPTO_V1.md
package crypto

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// AlgorithmID identifies a cryptographic algorithm.
// This enables algorithm agility for post-quantum migration.
type AlgorithmID string

// Supported algorithm identifiers.
const (
	// AlgEd25519 is the Ed25519 signature algorithm (current default).
	AlgEd25519 AlgorithmID = "Ed25519"

	// AlgSHA256 is SHA-256 hashing algorithm.
	AlgSHA256 AlgorithmID = "SHA-256"

	// AlgAES256GCM is AES-256-GCM authenticated encryption.
	AlgAES256GCM AlgorithmID = "AES-256-GCM"

	// Future PQC algorithms (not yet implemented, reserved IDs)
	// AlgMLDSA65 is ML-DSA-65 (Dilithium) for post-quantum signatures.
	AlgMLDSA65 AlgorithmID = "ML-DSA-65"

	// AlgMLKEM768 is ML-KEM-768 (Kyber) for post-quantum key encapsulation.
	AlgMLKEM768 AlgorithmID = "ML-KEM-768"

	// AlgHybridSig is dual-signature (Ed25519 + ML-DSA) for transition.
	AlgHybridSig AlgorithmID = "Ed25519+ML-DSA"
)

// SignatureRecord contains a signature with algorithm metadata.
// This format supports crypto agility and dual-signature for PQC transition.
type SignatureRecord struct {
	// Algorithm identifies which algorithm created this signature.
	Algorithm AlgorithmID `json:"algorithm"`

	// KeyID identifies which key signed the data.
	KeyID string `json:"key_id"`

	// Signature is the raw signature bytes.
	Signature []byte `json:"signature"`

	// SignedAt is when the signature was created.
	SignedAt time.Time `json:"signed_at"`

	// DataHash is the hash of the data that was signed.
	// This enables verification without the original data.
	DataHash []byte `json:"data_hash"`

	// PQCAlgorithm is the post-quantum algorithm, if dual-signature.
	PQCAlgorithm *AlgorithmID `json:"pqc_algorithm,omitempty"`

	// PQCSignature is the post-quantum signature bytes, if dual-signature.
	PQCSignature []byte `json:"pqc_signature,omitempty"`
}

// SealRecord contains encrypted data with algorithm metadata.
// This format supports crypto agility for post-quantum migration.
type SealRecord struct {
	// Algorithm identifies which algorithm encrypted this data.
	Algorithm AlgorithmID `json:"algorithm"`

	// KeyID identifies which key encrypted the data.
	KeyID string `json:"key_id"`

	// Nonce is the nonce/IV used for encryption.
	Nonce []byte `json:"nonce"`

	// Ciphertext is the encrypted data.
	Ciphertext []byte `json:"ciphertext"`

	// Tag is the authentication tag (for AEAD).
	Tag []byte `json:"tag"`

	// SealedAt is when the data was encrypted.
	SealedAt time.Time `json:"sealed_at"`

	// PQCAlgorithm is the post-quantum KEM, if hybrid encryption.
	PQCAlgorithm *AlgorithmID `json:"pqc_algorithm,omitempty"`

	// PQCCiphertext is the PQC-encapsulated key, if hybrid encryption.
	PQCCiphertext []byte `json:"pqc_ciphertext,omitempty"`
}

// Errors.
var (
	ErrUnsupportedAlgorithm = errors.New("unsupported algorithm")
	ErrInvalidSignature     = errors.New("invalid signature")
	ErrInvalidKeySize       = errors.New("invalid key size")
	ErrSigningFailed        = errors.New("signing failed")
)

// CanonicalHash computes the SHA-256 hash of data.
// This is the standard hashing function for all signature operations.
func CanonicalHash(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

// CanonicalJSON serializes a value to canonical JSON for signing.
// Go's json.Marshal sorts map keys, providing deterministic output.
func CanonicalJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// CanonicalHashJSON hashes the canonical JSON representation of a value.
func CanonicalHashJSON(v interface{}) ([]byte, error) {
	data, err := CanonicalJSON(v)
	if err != nil {
		return nil, fmt.Errorf("canonical json: %w", err)
	}
	return CanonicalHash(data), nil
}

// Ed25519Signer implements the Signer interface using Ed25519.
type Ed25519Signer struct {
	keyID      string
	privateKey ed25519.PrivateKey
}

// NewEd25519Signer creates a new Ed25519 signer with the given key.
func NewEd25519Signer(keyID string, privateKey ed25519.PrivateKey) (*Ed25519Signer, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("%w: expected %d bytes, got %d",
			ErrInvalidKeySize, ed25519.PrivateKeySize, len(privateKey))
	}
	return &Ed25519Signer{
		keyID:      keyID,
		privateKey: privateKey,
	}, nil
}

// Sign creates an Ed25519 signature for the given data.
func (s *Ed25519Signer) Sign(ctx context.Context, data []byte) ([]byte, error) {
	signature := ed25519.Sign(s.privateKey, data)
	return signature, nil
}

// KeyID returns the key identifier.
func (s *Ed25519Signer) KeyID() string {
	return s.keyID
}

// Algorithm returns the algorithm identifier.
func (s *Ed25519Signer) Algorithm() string {
	return string(AlgEd25519)
}

// SignWithRecord creates a SignatureRecord for the given data.
func (s *Ed25519Signer) SignWithRecord(ctx context.Context, data []byte, now time.Time) (SignatureRecord, error) {
	hash := CanonicalHash(data)
	signature, err := s.Sign(ctx, hash)
	if err != nil {
		return SignatureRecord{}, err
	}

	return SignatureRecord{
		Algorithm: AlgEd25519,
		KeyID:     s.keyID,
		Signature: signature,
		SignedAt:  now,
		DataHash:  hash,
	}, nil
}

// Ed25519Verifier implements the Verifier interface using Ed25519.
type Ed25519Verifier struct {
	keyID     string
	publicKey ed25519.PublicKey
}

// NewEd25519Verifier creates a new Ed25519 verifier with the given key.
func NewEd25519Verifier(keyID string, publicKey ed25519.PublicKey) (*Ed25519Verifier, error) {
	if len(publicKey) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("%w: expected %d bytes, got %d",
			ErrInvalidKeySize, ed25519.PublicKeySize, len(publicKey))
	}
	return &Ed25519Verifier{
		keyID:     keyID,
		publicKey: publicKey,
	}, nil
}

// Verify checks if the signature is valid for the given data.
func (v *Ed25519Verifier) Verify(ctx context.Context, data []byte, signature []byte) error {
	if !ed25519.Verify(v.publicKey, data, signature) {
		return ErrInvalidSignature
	}
	return nil
}

// KeyID returns the key identifier.
func (v *Ed25519Verifier) KeyID() string {
	return v.keyID
}

// Algorithm returns the algorithm identifier.
func (v *Ed25519Verifier) Algorithm() string {
	return string(AlgEd25519)
}

// VerifyRecord verifies a SignatureRecord against the original data.
func (v *Ed25519Verifier) VerifyRecord(ctx context.Context, data []byte, record SignatureRecord) error {
	// Verify algorithm matches
	if record.Algorithm != AlgEd25519 {
		return fmt.Errorf("%w: expected %s, got %s",
			ErrUnsupportedAlgorithm, AlgEd25519, record.Algorithm)
	}

	// Verify key ID matches
	if record.KeyID != v.keyID {
		return fmt.Errorf("key ID mismatch: expected %s, got %s", v.keyID, record.KeyID)
	}

	// Compute hash and verify
	hash := CanonicalHash(data)
	return v.Verify(ctx, hash, record.Signature)
}

// Ed25519KeyPair holds an Ed25519 key pair.
type Ed25519KeyPair struct {
	KeyID      string
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
	CreatedAt  time.Time
}

// GenerateEd25519KeyPair generates a new Ed25519 key pair.
func GenerateEd25519KeyPair(keyID string, now time.Time) (*Ed25519KeyPair, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}

	return &Ed25519KeyPair{
		KeyID:      keyID,
		PublicKey:  publicKey,
		PrivateKey: privateKey,
		CreatedAt:  now,
	}, nil
}

// Signer returns a signer for this key pair.
func (kp *Ed25519KeyPair) Signer() (*Ed25519Signer, error) {
	return NewEd25519Signer(kp.KeyID, kp.PrivateKey)
}

// Verifier returns a verifier for this key pair.
func (kp *Ed25519KeyPair) Verifier() (*Ed25519Verifier, error) {
	return NewEd25519Verifier(kp.KeyID, kp.PublicKey)
}

// Metadata returns KeyMetadata for this key pair.
func (kp *Ed25519KeyPair) Metadata() KeyMetadata {
	return KeyMetadata{
		KeyID:            kp.KeyID,
		Algorithm:        string(AlgEd25519),
		AlgorithmVersion: 1,
		CreatedAt:        kp.CreatedAt,
		IsActive:         true,
		PQExtension:      false, // Ed25519 is not PQC
	}
}

// IsSupportedAlgorithm checks if an algorithm is supported.
func IsSupportedAlgorithm(alg AlgorithmID) bool {
	switch alg {
	case AlgEd25519, AlgSHA256, AlgAES256GCM:
		return true
	case AlgMLDSA65, AlgMLKEM768, AlgHybridSig:
		// Reserved but not yet implemented
		return false
	default:
		return false
	}
}

// IsPQCAlgorithm checks if an algorithm is post-quantum.
func IsPQCAlgorithm(alg AlgorithmID) bool {
	switch alg {
	case AlgMLDSA65, AlgMLKEM768, AlgHybridSig:
		return true
	default:
		return false
	}
}
