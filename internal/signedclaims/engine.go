// Package signedclaims provides the verification engine for Phase 50.
//
// CRITICAL INVARIANTS:
// - NO POWER: This engine MUST NOT change decisions, outcomes, interrupts,
//   delivery, or execution. It only verifies signatures and creates records.
// - PURE FUNCTIONS: All verification is deterministic.
// - CLOCK INJECTION: Time is injected, never uses time.Now() directly.
// - HASH-ONLY OUTPUT: Produces records with only hashes/fingerprints.
// - NO NETWORK: No external calls. No storage. Pure verification.
//
// Reference: docs/ADR/ADR-0088-phase50-signed-vendor-claims-and-pack-manifests.md
package signedclaims

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"time"

	domain "quantumlife/pkg/domain/signedclaims"
)

// Engine verifies signed claims and manifests.
// It is stateless and produces hash-only records.
type Engine struct {
	clock func() time.Time
}

// NewEngine creates a new verification engine with injected clock.
func NewEngine(clock func() time.Time) *Engine {
	return &Engine{clock: clock}
}

// ============================================================================
// Claim Verification
// ============================================================================

// VerifyClaimResult is the result of claim verification.
type VerifyClaimResult struct {
	Status         domain.VerificationStatus
	Record         domain.SignedClaimRecord
	KeyFingerprint domain.KeyFingerprint
	ClaimHash      domain.SafeRefHash
}

// VerifyClaim verifies a signed vendor claim and produces a record.
// Returns the verification status and a hash-only record for storage.
//
// This function has NO POWER - it only checks cryptographic validity.
func (e *Engine) VerifyClaim(
	claim domain.SignedVendorClaim,
	signatureB64 domain.SignatureB64,
	publicKeyB64 domain.PublicKeyB64,
) VerifyClaimResult {
	// Step 1: Validate claim format
	if err := claim.Validate(); err != nil {
		return VerifyClaimResult{
			Status: domain.VerifiedBadFormat,
		}
	}

	// Step 2: Validate and decode public key
	if err := publicKeyB64.Validate(); err != nil {
		return VerifyClaimResult{
			Status: domain.VerifiedBadFormat,
		}
	}
	pubKeyBytes, err := publicKeyB64.Bytes()
	if err != nil {
		return VerifyClaimResult{
			Status: domain.VerifiedBadFormat,
		}
	}

	// Step 3: Compute key fingerprint
	fingerprint := domain.NewKeyFingerprint(pubKeyBytes)

	// Step 4: Compute claim hash
	claimHash := claim.Hash()

	// Step 5: Validate and decode signature
	if err := signatureB64.Validate(); err != nil {
		return VerifyClaimResult{
			Status:         domain.VerifiedBadFormat,
			KeyFingerprint: fingerprint,
			ClaimHash:      claimHash,
		}
	}
	sigBytes, err := signatureB64.Bytes()
	if err != nil {
		return VerifyClaimResult{
			Status:         domain.VerifiedBadFormat,
			KeyFingerprint: fingerprint,
			ClaimHash:      claimHash,
		}
	}

	// Step 6: Get message bytes for verification
	messageBytes := claim.MessageBytes()

	// Step 7: Verify Ed25519 signature
	if !ed25519.Verify(pubKeyBytes, messageBytes, sigBytes) {
		return VerifyClaimResult{
			Status:         domain.VerifiedBadSig,
			KeyFingerprint: fingerprint,
			ClaimHash:      claimHash,
			Record: domain.SignedClaimRecord{
				ClaimHash:      claimHash,
				KeyFingerprint: fingerprint,
				Status:         domain.VerifiedBadSig,
				Provenance:     claim.Provenance,
				Kind:           claim.Kind,
				PeriodKey:      claim.PeriodKey,
				CircleIDHash:   claim.CircleIDHash,
				CreatedBucket:  e.CurrentPeriodKey(),
			},
		}
	}

	// Step 8: Build verified record
	record := domain.SignedClaimRecord{
		ClaimHash:      claimHash,
		KeyFingerprint: fingerprint,
		Status:         domain.VerifiedOK,
		Provenance:     claim.Provenance,
		Kind:           claim.Kind,
		PeriodKey:      claim.PeriodKey,
		CircleIDHash:   claim.CircleIDHash,
		CreatedBucket:  e.CurrentPeriodKey(),
	}

	return VerifyClaimResult{
		Status:         domain.VerifiedOK,
		Record:         record,
		KeyFingerprint: fingerprint,
		ClaimHash:      claimHash,
	}
}

// ============================================================================
// Manifest Verification
// ============================================================================

// VerifyManifestResult is the result of manifest verification.
type VerifyManifestResult struct {
	Status         domain.VerificationStatus
	Record         domain.SignedManifestRecord
	KeyFingerprint domain.KeyFingerprint
	ManifestHash   domain.SafeRefHash
}

// VerifyManifest verifies a signed pack manifest and produces a record.
// Returns the verification status and a hash-only record for storage.
//
// This function has NO POWER - it only checks cryptographic validity.
func (e *Engine) VerifyManifest(
	manifest domain.SignedPackManifest,
	signatureB64 domain.SignatureB64,
	publicKeyB64 domain.PublicKeyB64,
) VerifyManifestResult {
	// Step 1: Validate manifest format
	if err := manifest.Validate(); err != nil {
		return VerifyManifestResult{
			Status: domain.VerifiedBadFormat,
		}
	}

	// Step 2: Validate and decode public key
	if err := publicKeyB64.Validate(); err != nil {
		return VerifyManifestResult{
			Status: domain.VerifiedBadFormat,
		}
	}
	pubKeyBytes, err := publicKeyB64.Bytes()
	if err != nil {
		return VerifyManifestResult{
			Status: domain.VerifiedBadFormat,
		}
	}

	// Step 3: Compute key fingerprint
	fingerprint := domain.NewKeyFingerprint(pubKeyBytes)

	// Step 4: Compute manifest hash
	manifestHash := manifest.Hash()

	// Step 5: Validate and decode signature
	if err := signatureB64.Validate(); err != nil {
		return VerifyManifestResult{
			Status:         domain.VerifiedBadFormat,
			KeyFingerprint: fingerprint,
			ManifestHash:   manifestHash,
		}
	}
	sigBytes, err := signatureB64.Bytes()
	if err != nil {
		return VerifyManifestResult{
			Status:         domain.VerifiedBadFormat,
			KeyFingerprint: fingerprint,
			ManifestHash:   manifestHash,
		}
	}

	// Step 6: Get message bytes for verification
	messageBytes := manifest.MessageBytes()

	// Step 7: Verify Ed25519 signature
	if !ed25519.Verify(pubKeyBytes, messageBytes, sigBytes) {
		return VerifyManifestResult{
			Status:         domain.VerifiedBadSig,
			KeyFingerprint: fingerprint,
			ManifestHash:   manifestHash,
			Record: domain.SignedManifestRecord{
				ManifestHash:   manifestHash,
				KeyFingerprint: fingerprint,
				Status:         domain.VerifiedBadSig,
				Provenance:     manifest.Provenance,
				PeriodKey:      manifest.PeriodKey,
				CircleIDHash:   manifest.CircleIDHash,
				PackHash:       manifest.PackHash,
				CreatedBucket:  e.CurrentPeriodKey(),
			},
		}
	}

	// Step 8: Build verified record
	record := domain.SignedManifestRecord{
		ManifestHash:   manifestHash,
		KeyFingerprint: fingerprint,
		Status:         domain.VerifiedOK,
		Provenance:     manifest.Provenance,
		PeriodKey:      manifest.PeriodKey,
		CircleIDHash:   manifest.CircleIDHash,
		PackHash:       manifest.PackHash,
		CreatedBucket:  e.CurrentPeriodKey(),
	}

	return VerifyManifestResult{
		Status:         domain.VerifiedOK,
		Record:         record,
		KeyFingerprint: fingerprint,
		ManifestHash:   manifestHash,
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

// CurrentPeriodKey returns the current period key based on the injected clock.
// Uses daily period (YYYY-MM-DD format).
func (e *Engine) CurrentPeriodKey() string {
	return e.clock().UTC().Format("2006-01-02")
}

// FingerprintPublicKey computes the fingerprint of a public key.
func FingerprintPublicKey(pub ed25519.PublicKey) domain.KeyFingerprint {
	return domain.NewKeyFingerprint(pub)
}

// HashCanonical computes SHA256 hex of a canonical string.
func HashCanonical(line string) string {
	hash := sha256.Sum256([]byte(line))
	return hex.EncodeToString(hash[:])
}

// ============================================================================
// Proof Display Helpers
// ============================================================================

// ProofDisplayData contains data for displaying proofs to users.
// Contains ONLY hashes and fingerprints - no raw keys or signatures.
type ProofDisplayData struct {
	// Claims is a list of claim records for display.
	Claims []domain.SignedClaimRecord
	// Manifests is a list of manifest records for display.
	Manifests []domain.SignedManifestRecord
	// PeriodKey is the current period.
	PeriodKey string
	// HasVerifiedClaims is true if any claims are verified.
	HasVerifiedClaims bool
	// HasVerifiedManifests is true if any manifests are verified.
	HasVerifiedManifests bool
	// HasUnverifiedClaims is true if any claims failed verification.
	HasUnverifiedClaims bool
	// HasUnverifiedManifests is true if any manifests failed verification.
	HasUnverifiedManifests bool
}

// BuildProofDisplayData builds display data from records.
func (e *Engine) BuildProofDisplayData(
	claims []domain.SignedClaimRecord,
	manifests []domain.SignedManifestRecord,
	periodKey string,
) ProofDisplayData {
	data := ProofDisplayData{
		Claims:    claims,
		Manifests: manifests,
		PeriodKey: periodKey,
	}

	for _, c := range claims {
		if c.Status == domain.VerifiedOK {
			data.HasVerifiedClaims = true
		} else {
			data.HasUnverifiedClaims = true
		}
	}

	for _, m := range manifests {
		if m.Status == domain.VerifiedOK {
			data.HasVerifiedManifests = true
		} else {
			data.HasUnverifiedManifests = true
		}
	}

	return data
}

// ============================================================================
// Dedup Key Helpers
// ============================================================================

// ClaimDedupKey returns a dedup key for a claim (circleIDHash|periodKey|claimHash).
func ClaimDedupKey(circleIDHash domain.SafeRefHash, periodKey string, claimHash domain.SafeRefHash) string {
	return string(circleIDHash) + "|" + periodKey + "|" + string(claimHash)
}

// ManifestDedupKey returns a dedup key for a manifest (circleIDHash|periodKey|manifestHash).
func ManifestDedupKey(circleIDHash domain.SafeRefHash, periodKey string, manifestHash domain.SafeRefHash) string {
	return string(circleIDHash) + "|" + periodKey + "|" + string(manifestHash)
}
