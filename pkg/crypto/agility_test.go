package crypto

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"testing"
	"time"
)

func TestCanonicalHash_Deterministic(t *testing.T) {
	// Same input must always produce same hash
	data := []byte("test data for hashing")

	hash1 := CanonicalHash(data)
	hash2 := CanonicalHash(data)

	if !bytes.Equal(hash1, hash2) {
		t.Error("CanonicalHash is not deterministic")
	}

	// Hash must be 32 bytes (SHA-256)
	if len(hash1) != 32 {
		t.Errorf("expected 32 bytes, got %d", len(hash1))
	}
}

func TestCanonicalHash_DifferentInputs(t *testing.T) {
	data1 := []byte("input one")
	data2 := []byte("input two")

	hash1 := CanonicalHash(data1)
	hash2 := CanonicalHash(data2)

	if bytes.Equal(hash1, hash2) {
		t.Error("different inputs produced same hash")
	}
}

func TestCanonicalJSON_Deterministic(t *testing.T) {
	// Maps should serialize with sorted keys
	data := map[string]interface{}{
		"zebra": 1,
		"alpha": 2,
		"beta":  3,
	}

	json1, err := CanonicalJSON(data)
	if err != nil {
		t.Fatalf("CanonicalJSON failed: %v", err)
	}

	json2, err := CanonicalJSON(data)
	if err != nil {
		t.Fatalf("CanonicalJSON failed: %v", err)
	}

	if !bytes.Equal(json1, json2) {
		t.Error("CanonicalJSON is not deterministic")
	}

	// Verify keys are sorted
	expected := `{"alpha":2,"beta":3,"zebra":1}`
	if string(json1) != expected {
		t.Errorf("expected %s, got %s", expected, string(json1))
	}
}

func TestCanonicalHashJSON_Deterministic(t *testing.T) {
	data := map[string]interface{}{
		"action": "transfer",
		"amount": 100,
	}

	hash1, err := CanonicalHashJSON(data)
	if err != nil {
		t.Fatalf("CanonicalHashJSON failed: %v", err)
	}

	hash2, err := CanonicalHashJSON(data)
	if err != nil {
		t.Fatalf("CanonicalHashJSON failed: %v", err)
	}

	if !bytes.Equal(hash1, hash2) {
		t.Error("CanonicalHashJSON is not deterministic")
	}
}

func TestEd25519Signer_SignAndVerify(t *testing.T) {
	ctx := context.Background()

	// Generate key pair
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	signer, err := NewEd25519Signer("test-key", privateKey)
	if err != nil {
		t.Fatalf("NewEd25519Signer failed: %v", err)
	}

	verifier, err := NewEd25519Verifier("test-key", publicKey)
	if err != nil {
		t.Fatalf("NewEd25519Verifier failed: %v", err)
	}

	// Sign data
	data := []byte("test data to sign")
	signature, err := signer.Sign(ctx, data)
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}

	// Verify signature
	if err := verifier.Verify(ctx, data, signature); err != nil {
		t.Errorf("Verify failed: %v", err)
	}

	// Verify with wrong data should fail
	wrongData := []byte("wrong data")
	if err := verifier.Verify(ctx, wrongData, signature); err == nil {
		t.Error("Verify should fail with wrong data")
	}

	// Verify algorithm and key ID
	if signer.Algorithm() != string(AlgEd25519) {
		t.Errorf("expected algorithm %s, got %s", AlgEd25519, signer.Algorithm())
	}
	if signer.KeyID() != "test-key" {
		t.Errorf("expected key ID test-key, got %s", signer.KeyID())
	}
}

func TestEd25519Signer_SignWithRecord(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	// Generate key pair
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	signer, err := NewEd25519Signer("test-key", privateKey)
	if err != nil {
		t.Fatalf("NewEd25519Signer failed: %v", err)
	}

	verifier, err := NewEd25519Verifier("test-key", publicKey)
	if err != nil {
		t.Fatalf("NewEd25519Verifier failed: %v", err)
	}

	// Sign with record
	data := []byte("test data for record signing")
	record, err := signer.SignWithRecord(ctx, data, now)
	if err != nil {
		t.Fatalf("SignWithRecord failed: %v", err)
	}

	// Verify record contents
	if record.Algorithm != AlgEd25519 {
		t.Errorf("expected algorithm %s, got %s", AlgEd25519, record.Algorithm)
	}
	if record.KeyID != "test-key" {
		t.Errorf("expected key ID test-key, got %s", record.KeyID)
	}
	if !record.SignedAt.Equal(now) {
		t.Errorf("expected signed at %v, got %v", now, record.SignedAt)
	}
	if len(record.DataHash) != 32 {
		t.Errorf("expected 32-byte hash, got %d", len(record.DataHash))
	}

	// Verify signature via record
	if err := verifier.VerifyRecord(ctx, data, record); err != nil {
		t.Errorf("VerifyRecord failed: %v", err)
	}

	// Verify with wrong data should fail
	wrongData := []byte("wrong data")
	if err := verifier.VerifyRecord(ctx, wrongData, record); err == nil {
		t.Error("VerifyRecord should fail with wrong data")
	}
}

func TestEd25519Signer_TamperDetection(t *testing.T) {
	ctx := context.Background()

	// Generate key pair
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	signer, err := NewEd25519Signer("test-key", privateKey)
	if err != nil {
		t.Fatalf("NewEd25519Signer failed: %v", err)
	}

	verifier, err := NewEd25519Verifier("test-key", publicKey)
	if err != nil {
		t.Fatalf("NewEd25519Verifier failed: %v", err)
	}

	// Sign data
	data := []byte("original data")
	signature, err := signer.Sign(ctx, data)
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}

	// Tamper with signature
	tamperedSig := make([]byte, len(signature))
	copy(tamperedSig, signature)
	tamperedSig[0] ^= 0xFF

	if err := verifier.Verify(ctx, data, tamperedSig); err == nil {
		t.Error("Verify should fail with tampered signature")
	}

	// Tamper with data
	tamperedData := []byte("tampered data")
	if err := verifier.Verify(ctx, tamperedData, signature); err == nil {
		t.Error("Verify should fail with tampered data")
	}
}

func TestGenerateEd25519KeyPair(t *testing.T) {
	now := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	kp, err := GenerateEd25519KeyPair("test-key-1", now)
	if err != nil {
		t.Fatalf("GenerateEd25519KeyPair failed: %v", err)
	}

	if kp.KeyID != "test-key-1" {
		t.Errorf("expected key ID test-key-1, got %s", kp.KeyID)
	}
	if !kp.CreatedAt.Equal(now) {
		t.Errorf("expected created at %v, got %v", now, kp.CreatedAt)
	}
	if len(kp.PublicKey) != ed25519.PublicKeySize {
		t.Errorf("expected public key size %d, got %d", ed25519.PublicKeySize, len(kp.PublicKey))
	}
	if len(kp.PrivateKey) != ed25519.PrivateKeySize {
		t.Errorf("expected private key size %d, got %d", ed25519.PrivateKeySize, len(kp.PrivateKey))
	}

	// Test signer/verifier creation
	signer, err := kp.Signer()
	if err != nil {
		t.Fatalf("Signer failed: %v", err)
	}
	verifier, err := kp.Verifier()
	if err != nil {
		t.Fatalf("Verifier failed: %v", err)
	}

	// Test round-trip
	ctx := context.Background()
	data := []byte("test data")
	sig, err := signer.Sign(ctx, data)
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}
	if err := verifier.Verify(ctx, data, sig); err != nil {
		t.Errorf("Verify failed: %v", err)
	}
}

func TestEd25519KeyPair_Metadata(t *testing.T) {
	now := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	kp, err := GenerateEd25519KeyPair("test-key", now)
	if err != nil {
		t.Fatalf("GenerateEd25519KeyPair failed: %v", err)
	}

	meta := kp.Metadata()

	if meta.KeyID != "test-key" {
		t.Errorf("expected key ID test-key, got %s", meta.KeyID)
	}
	if meta.Algorithm != string(AlgEd25519) {
		t.Errorf("expected algorithm %s, got %s", AlgEd25519, meta.Algorithm)
	}
	if meta.AlgorithmVersion != 1 {
		t.Errorf("expected version 1, got %d", meta.AlgorithmVersion)
	}
	if !meta.CreatedAt.Equal(now) {
		t.Errorf("expected created at %v, got %v", now, meta.CreatedAt)
	}
	if !meta.IsActive {
		t.Error("expected key to be active")
	}
	if meta.PQExtension {
		t.Error("Ed25519 should not have PQ extension")
	}
}

func TestNewEd25519Signer_InvalidKeySize(t *testing.T) {
	_, err := NewEd25519Signer("test-key", []byte("too short"))
	if err == nil {
		t.Error("expected error for invalid key size")
	}
}

func TestNewEd25519Verifier_InvalidKeySize(t *testing.T) {
	_, err := NewEd25519Verifier("test-key", []byte("too short"))
	if err == nil {
		t.Error("expected error for invalid key size")
	}
}

func TestIsSupportedAlgorithm(t *testing.T) {
	tests := []struct {
		alg       AlgorithmID
		supported bool
	}{
		{AlgEd25519, true},
		{AlgSHA256, true},
		{AlgAES256GCM, true},
		{AlgMLDSA65, false},      // Reserved but not implemented
		{AlgMLKEM768, false},     // Reserved but not implemented
		{AlgHybridSig, false},    // Reserved but not implemented
		{"unknown", false},       // Unknown algorithm
	}

	for _, tt := range tests {
		t.Run(string(tt.alg), func(t *testing.T) {
			got := IsSupportedAlgorithm(tt.alg)
			if got != tt.supported {
				t.Errorf("IsSupportedAlgorithm(%s) = %v, want %v", tt.alg, got, tt.supported)
			}
		})
	}
}

func TestIsPQCAlgorithm(t *testing.T) {
	tests := []struct {
		alg   AlgorithmID
		isPQC bool
	}{
		{AlgEd25519, false},
		{AlgSHA256, false},
		{AlgAES256GCM, false},
		{AlgMLDSA65, true},
		{AlgMLKEM768, true},
		{AlgHybridSig, true},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.alg), func(t *testing.T) {
			got := IsPQCAlgorithm(tt.alg)
			if got != tt.isPQC {
				t.Errorf("IsPQCAlgorithm(%s) = %v, want %v", tt.alg, got, tt.isPQC)
			}
		})
	}
}

func TestVerifyRecord_AlgorithmMismatch(t *testing.T) {
	ctx := context.Background()

	// Generate key pair
	publicKey, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	verifier, err := NewEd25519Verifier("test-key", publicKey)
	if err != nil {
		t.Fatalf("NewEd25519Verifier failed: %v", err)
	}

	// Create record with wrong algorithm
	record := SignatureRecord{
		Algorithm: AlgMLDSA65, // Wrong algorithm
		KeyID:     "test-key",
		Signature: []byte("fake signature"),
	}

	data := []byte("test data")
	err = verifier.VerifyRecord(ctx, data, record)
	if err == nil {
		t.Error("expected error for algorithm mismatch")
	}
}

func TestVerifyRecord_KeyIDMismatch(t *testing.T) {
	ctx := context.Background()

	// Generate key pair
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	signer, err := NewEd25519Signer("key-a", privateKey)
	if err != nil {
		t.Fatalf("NewEd25519Signer failed: %v", err)
	}

	verifier, err := NewEd25519Verifier("key-b", publicKey) // Different key ID
	if err != nil {
		t.Fatalf("NewEd25519Verifier failed: %v", err)
	}

	// Sign with signer
	now := time.Now()
	data := []byte("test data")
	record, err := signer.SignWithRecord(ctx, data, now)
	if err != nil {
		t.Fatalf("SignWithRecord failed: %v", err)
	}

	// Verify should fail due to key ID mismatch
	err = verifier.VerifyRecord(ctx, data, record)
	if err == nil {
		t.Error("expected error for key ID mismatch")
	}
}
