// Package signedclaims provides domain types for Phase 50: Signed Vendor Claims + Pack Manifests.
//
// CRITICAL INVARIANTS:
// - NO POWER: This package MUST NOT change decisions, outcomes, interrupts,
//   delivery, or execution. It only adds verifiable authenticity metadata.
// - HASH-ONLY STORAGE: Never store raw public keys, signatures, vendor names,
//   emails, or URLs in persistence. Only store hashes and fingerprints.
// - PIPE-DELIMITED: All canonical strings use pipe-delimited format, NOT JSON.
// - ED25519 ONLY: 32-byte public keys, 64-byte signatures.
//
// Reference: docs/ADR/ADR-0088-phase50-signed-vendor-claims-and-pack-manifests.md
package signedclaims

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// ============================================================================
// Enums
// ============================================================================

// ClaimKind identifies what kind of claim is being made.
type ClaimKind string

const (
	// ClaimVendorCap asserts the vendor caps themselves at a pressure level.
	ClaimVendorCap ClaimKind = "claim_vendor_cap"
	// ClaimPackManifest asserts the contents/bindings of a pack.
	ClaimPackManifest ClaimKind = "claim_pack_manifest"
	// ClaimObserverBindingIntent asserts intended observer bindings.
	ClaimObserverBindingIntent ClaimKind = "claim_observer_binding_intent"
)

// Validate checks if the ClaimKind is valid.
func (k ClaimKind) Validate() error {
	switch k {
	case ClaimVendorCap, ClaimPackManifest, ClaimObserverBindingIntent:
		return nil
	default:
		return fmt.Errorf("invalid ClaimKind: %q", k)
	}
}

// String returns the string representation.
func (k ClaimKind) String() string {
	return string(k)
}

// ============================================================================

// Provenance identifies where a claim originated.
// NOTE: Provenance has NO POWER - it is metadata only for display/audit.
type Provenance string

const (
	// ProvenanceUserSupplied means the user submitted the claim directly.
	ProvenanceUserSupplied Provenance = "provenance_user_supplied"
	// ProvenanceMarketplace means the claim came via marketplace flow.
	ProvenanceMarketplace Provenance = "provenance_marketplace"
	// ProvenanceAdmin means the claim came from admin tooling.
	ProvenanceAdmin Provenance = "provenance_admin"
)

// Validate checks if the Provenance is valid.
func (p Provenance) Validate() error {
	switch p {
	case ProvenanceUserSupplied, ProvenanceMarketplace, ProvenanceAdmin:
		return nil
	default:
		return fmt.Errorf("invalid Provenance: %q", p)
	}
}

// String returns the string representation.
func (p Provenance) String() string {
	return string(p)
}

// ============================================================================

// VerificationStatus is the result of signature verification.
type VerificationStatus string

const (
	// VerifiedOK means the signature is valid.
	VerifiedOK VerificationStatus = "verified_ok"
	// VerifiedBadSig means the signature did not verify.
	VerifiedBadSig VerificationStatus = "verified_bad_sig"
	// VerifiedBadFormat means the claim/manifest format is invalid.
	VerifiedBadFormat VerificationStatus = "verified_bad_format"
	// VerifiedUnknownKey means we cannot verify (key not known/trusted).
	VerifiedUnknownKey VerificationStatus = "verified_unknown_key"
)

// Validate checks if the VerificationStatus is valid.
func (v VerificationStatus) Validate() error {
	switch v {
	case VerifiedOK, VerifiedBadSig, VerifiedBadFormat, VerifiedUnknownKey:
		return nil
	default:
		return fmt.Errorf("invalid VerificationStatus: %q", v)
	}
}

// String returns the string representation.
func (v VerificationStatus) String() string {
	return string(v)
}

// IsVerified returns true if status is VerifiedOK.
func (v VerificationStatus) IsVerified() bool {
	return v == VerifiedOK
}

// ============================================================================

// VendorScope identifies the scope of a vendor claim.
type VendorScope string

const (
	// ScopeHuman is for human/individual vendors.
	ScopeHuman VendorScope = "scope_human"
	// ScopeInstitution is for institutional vendors.
	ScopeInstitution VendorScope = "scope_institution"
	// ScopeCommerce is for commerce/commercial vendors.
	ScopeCommerce VendorScope = "scope_commerce"
)

// Validate checks if the VendorScope is valid.
func (s VendorScope) Validate() error {
	switch s {
	case ScopeHuman, ScopeInstitution, ScopeCommerce:
		return nil
	default:
		return fmt.Errorf("invalid VendorScope: %q", s)
	}
}

// String returns the string representation.
func (s VendorScope) String() string {
	return string(s)
}

// ============================================================================

// PressureCap is the self-declared pressure cap in a vendor claim.
type PressureCap string

const (
	// AllowHoldOnly means vendor will only use HOLD pressure.
	AllowHoldOnly PressureCap = "allow_hold_only"
	// AllowSurfaceOnly means vendor will use at most SURFACE pressure.
	AllowSurfaceOnly PressureCap = "allow_surface_only"
)

// Validate checks if the PressureCap is valid.
func (c PressureCap) Validate() error {
	switch c {
	case AllowHoldOnly, AllowSurfaceOnly:
		return nil
	default:
		return fmt.Errorf("invalid PressureCap: %q", c)
	}
}

// String returns the string representation.
func (c PressureCap) String() string {
	return string(c)
}

// ============================================================================

// PackVersionBucket is the version bucket for a pack manifest.
type PackVersionBucket string

const (
	// PackVersionV0 is the initial version.
	PackVersionV0 PackVersionBucket = "v0"
	// PackVersionV1 is version 1.
	PackVersionV1 PackVersionBucket = "v1"
	// PackVersionV1_1 is version 1.1.
	PackVersionV1_1 PackVersionBucket = "v1_1"
)

// Validate checks if the PackVersionBucket is valid.
func (v PackVersionBucket) Validate() error {
	switch v {
	case PackVersionV0, PackVersionV1, PackVersionV1_1:
		return nil
	default:
		return fmt.Errorf("invalid PackVersionBucket: %q", v)
	}
}

// String returns the string representation.
func (v PackVersionBucket) String() string {
	return string(v)
}

// ============================================================================
// Value Types
// ============================================================================

// SafeRefHash is a SHA256 hex string for referring to other records.
// Used for contract hash, pack hash, market signal hash, etc.
type SafeRefHash string

// Validate checks if the SafeRefHash is valid (64 hex characters).
func (h SafeRefHash) Validate() error {
	if len(h) != 64 {
		return fmt.Errorf("SafeRefHash must be 64 hex characters, got %d", len(h))
	}
	for _, c := range h {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return fmt.Errorf("SafeRefHash must be lowercase hex, got invalid char %q", c)
		}
	}
	return nil
}

// String returns the string representation.
func (h SafeRefHash) String() string {
	return string(h)
}

// ComputeSafeRefHash computes a SafeRefHash from arbitrary data.
func ComputeSafeRefHash(data []byte) SafeRefHash {
	hash := sha256.Sum256(data)
	return SafeRefHash(hex.EncodeToString(hash[:]))
}

// ============================================================================

// KeyFingerprint is the SHA256 of public key bytes.
// We store ONLY the fingerprint, never the raw public key.
type KeyFingerprint string

// NewKeyFingerprint computes the fingerprint from raw public key bytes.
func NewKeyFingerprint(publicKeyBytes []byte) KeyFingerprint {
	hash := sha256.Sum256(publicKeyBytes)
	return KeyFingerprint(hex.EncodeToString(hash[:]))
}

// Validate checks if the KeyFingerprint is valid (64 hex characters).
func (f KeyFingerprint) Validate() error {
	if len(f) != 64 {
		return fmt.Errorf("KeyFingerprint must be 64 hex characters, got %d", len(f))
	}
	for _, c := range f {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return fmt.Errorf("KeyFingerprint must be lowercase hex, got invalid char %q", c)
		}
	}
	return nil
}

// String returns the string representation.
func (f KeyFingerprint) String() string {
	return string(f)
}

// CanonicalString returns the canonical form for hashing.
func (f KeyFingerprint) CanonicalString() string {
	return string(f)
}

// Short returns a shortened fingerprint for display (first 16 chars).
func (f KeyFingerprint) Short() string {
	if len(f) >= 16 {
		return string(f)[:16]
	}
	return string(f)
}

// ============================================================================

// SignatureB64 is a base64-encoded Ed25519 signature.
// Used only for transport/verification, NEVER stored.
type SignatureB64 string

// Validate checks if the SignatureB64 is valid base64 and correct length.
func (s SignatureB64) Validate() error {
	if s == "" {
		return errors.New("SignatureB64 cannot be empty")
	}
	decoded, err := base64.StdEncoding.DecodeString(string(s))
	if err != nil {
		return fmt.Errorf("SignatureB64 invalid base64: %w", err)
	}
	// Ed25519 signatures are 64 bytes
	if len(decoded) != 64 {
		return fmt.Errorf("SignatureB64 must decode to 64 bytes, got %d", len(decoded))
	}
	return nil
}

// Bytes decodes and returns the raw signature bytes.
func (s SignatureB64) Bytes() ([]byte, error) {
	return base64.StdEncoding.DecodeString(string(s))
}

// String returns the string representation.
func (s SignatureB64) String() string {
	return string(s)
}

// ============================================================================

// PublicKeyB64 is a base64-encoded Ed25519 public key.
// Used only for transport/verification, NEVER stored.
type PublicKeyB64 string

// Validate checks if the PublicKeyB64 is valid base64 and correct length.
func (p PublicKeyB64) Validate() error {
	if p == "" {
		return errors.New("PublicKeyB64 cannot be empty")
	}
	decoded, err := base64.StdEncoding.DecodeString(string(p))
	if err != nil {
		return fmt.Errorf("PublicKeyB64 invalid base64: %w", err)
	}
	// Ed25519 public keys are 32 bytes
	if len(decoded) != 32 {
		return fmt.Errorf("PublicKeyB64 must decode to 32 bytes, got %d", len(decoded))
	}
	return nil
}

// Bytes decodes and returns the raw public key bytes.
func (p PublicKeyB64) Bytes() ([]byte, error) {
	return base64.StdEncoding.DecodeString(string(p))
}

// Fingerprint computes the KeyFingerprint from the public key.
func (p PublicKeyB64) Fingerprint() (KeyFingerprint, error) {
	bytes, err := p.Bytes()
	if err != nil {
		return "", err
	}
	return NewKeyFingerprint(bytes), nil
}

// String returns the string representation.
func (p PublicKeyB64) String() string {
	return string(p)
}

// ============================================================================
// Signed Vendor Claim
// ============================================================================

// SignedVendorClaim represents a vendor's self-declaration.
// Used for verification, then only hashes/fingerprints are stored.
type SignedVendorClaim struct {
	// Kind identifies what kind of claim this is.
	Kind ClaimKind
	// Scope identifies the vendor scope.
	Scope VendorScope
	// Cap is the self-declared pressure cap.
	Cap PressureCap
	// RefHash is a reference to related record (contract, signal, etc).
	RefHash SafeRefHash
	// Provenance is where this claim originated (metadata only, no power).
	Provenance Provenance
	// CircleIDHash is SHA256 of the circle ID this claim applies to.
	CircleIDHash SafeRefHash
	// PeriodKey is the time bucket (e.g., "2025-01-09").
	PeriodKey string
}

// Validate checks all fields of the claim.
func (c SignedVendorClaim) Validate() error {
	if err := c.Kind.Validate(); err != nil {
		return fmt.Errorf("claim Kind: %w", err)
	}
	if err := c.Scope.Validate(); err != nil {
		return fmt.Errorf("claim Scope: %w", err)
	}
	if err := c.Cap.Validate(); err != nil {
		return fmt.Errorf("claim Cap: %w", err)
	}
	if err := c.RefHash.Validate(); err != nil {
		return fmt.Errorf("claim RefHash: %w", err)
	}
	if err := c.Provenance.Validate(); err != nil {
		return fmt.Errorf("claim Provenance: %w", err)
	}
	if err := c.CircleIDHash.Validate(); err != nil {
		return fmt.Errorf("claim CircleIDHash: %w", err)
	}
	if c.PeriodKey == "" {
		return errors.New("claim PeriodKey cannot be empty")
	}
	return nil
}

// CanonicalString returns the canonical pipe-delimited string for hashing.
// Order: kind|scope|cap|refHash|provenance|circleIDHash|periodKey
// CRITICAL: Does NOT include signature itself.
func (c SignedVendorClaim) CanonicalString() string {
	parts := []string{
		string(c.Kind),
		string(c.Scope),
		string(c.Cap),
		string(c.RefHash),
		string(c.Provenance),
		string(c.CircleIDHash),
		c.PeriodKey,
	}
	return strings.Join(parts, "|")
}

// MessageBytes returns the bytes to sign/verify.
// Format: QL|phase50|vendor_claim| + CanonicalString()
func (c SignedVendorClaim) MessageBytes() []byte {
	return []byte("QL|phase50|vendor_claim|" + c.CanonicalString())
}

// Hash returns the SHA256 of the message bytes.
func (c SignedVendorClaim) Hash() SafeRefHash {
	hash := sha256.Sum256(c.MessageBytes())
	return SafeRefHash(hex.EncodeToString(hash[:]))
}

// ============================================================================
// Signed Pack Manifest
// ============================================================================

// SignedPackManifest represents a pack author's assertion about pack contents.
// Used for verification, then only hashes/fingerprints are stored.
type SignedPackManifest struct {
	// PackHash is SHA256 of the pack content.
	PackHash SafeRefHash
	// Version is the version bucket.
	Version PackVersionBucket
	// BindingsHash is SHA256 of intended observer bindings.
	BindingsHash SafeRefHash
	// Provenance is where this manifest originated (metadata only, no power).
	Provenance Provenance
	// CircleIDHash is SHA256 of the circle ID this manifest applies to.
	CircleIDHash SafeRefHash
	// PeriodKey is the time bucket (e.g., "2025-01-09").
	PeriodKey string
}

// Validate checks all fields of the manifest.
func (m SignedPackManifest) Validate() error {
	if err := m.PackHash.Validate(); err != nil {
		return fmt.Errorf("manifest PackHash: %w", err)
	}
	if err := m.Version.Validate(); err != nil {
		return fmt.Errorf("manifest Version: %w", err)
	}
	if err := m.BindingsHash.Validate(); err != nil {
		return fmt.Errorf("manifest BindingsHash: %w", err)
	}
	if err := m.Provenance.Validate(); err != nil {
		return fmt.Errorf("manifest Provenance: %w", err)
	}
	if err := m.CircleIDHash.Validate(); err != nil {
		return fmt.Errorf("manifest CircleIDHash: %w", err)
	}
	if m.PeriodKey == "" {
		return errors.New("manifest PeriodKey cannot be empty")
	}
	return nil
}

// CanonicalString returns the canonical pipe-delimited string for hashing.
// Order: packHash|version|bindingsHash|provenance|circleIDHash|periodKey
// CRITICAL: Does NOT include signature itself.
func (m SignedPackManifest) CanonicalString() string {
	parts := []string{
		string(m.PackHash),
		string(m.Version),
		string(m.BindingsHash),
		string(m.Provenance),
		string(m.CircleIDHash),
		m.PeriodKey,
	}
	return strings.Join(parts, "|")
}

// MessageBytes returns the bytes to sign/verify.
// Format: QL|phase50|pack_manifest| + CanonicalString()
func (m SignedPackManifest) MessageBytes() []byte {
	return []byte("QL|phase50|pack_manifest|" + m.CanonicalString())
}

// Hash returns the SHA256 of the message bytes.
func (m SignedPackManifest) Hash() SafeRefHash {
	hash := sha256.Sum256(m.MessageBytes())
	return SafeRefHash(hex.EncodeToString(hash[:]))
}

// ============================================================================
// Persisted Records (Hash-Only)
// ============================================================================

// SignedClaimRecord is what we persist for a verified vendor claim.
// CRITICAL: Hash-only storage - no raw keys, signatures, vendor names.
type SignedClaimRecord struct {
	// ClaimHash is SHA256 of the message bytes.
	ClaimHash SafeRefHash
	// KeyFingerprint is SHA256 of the public key that signed it.
	KeyFingerprint KeyFingerprint
	// Status is the verification result.
	Status VerificationStatus
	// Provenance is where this claim originated.
	Provenance Provenance
	// Kind is the claim kind.
	Kind ClaimKind
	// PeriodKey is the time bucket.
	PeriodKey string
	// CircleIDHash is SHA256 of the circle ID.
	CircleIDHash SafeRefHash
	// CreatedBucket is the creation time bucket.
	CreatedBucket string
}

// CanonicalString returns the canonical pipe-delimited string for the record.
func (r SignedClaimRecord) CanonicalString() string {
	parts := []string{
		string(r.ClaimHash),
		string(r.KeyFingerprint),
		string(r.Status),
		string(r.Provenance),
		string(r.Kind),
		r.PeriodKey,
		string(r.CircleIDHash),
		r.CreatedBucket,
	}
	return strings.Join(parts, "|")
}

// ============================================================================

// SignedManifestRecord is what we persist for a verified pack manifest.
// CRITICAL: Hash-only storage - no raw keys, signatures, pack content.
type SignedManifestRecord struct {
	// ManifestHash is SHA256 of the message bytes.
	ManifestHash SafeRefHash
	// KeyFingerprint is SHA256 of the public key that signed it.
	KeyFingerprint KeyFingerprint
	// Status is the verification result.
	Status VerificationStatus
	// Provenance is where this manifest originated.
	Provenance Provenance
	// PeriodKey is the time bucket.
	PeriodKey string
	// CircleIDHash is SHA256 of the circle ID.
	CircleIDHash SafeRefHash
	// PackHash is the hash of pack content (for reference).
	PackHash SafeRefHash
	// CreatedBucket is the creation time bucket.
	CreatedBucket string
}

// CanonicalString returns the canonical pipe-delimited string for the record.
func (r SignedManifestRecord) CanonicalString() string {
	parts := []string{
		string(r.ManifestHash),
		string(r.KeyFingerprint),
		string(r.Status),
		string(r.Provenance),
		r.PeriodKey,
		string(r.CircleIDHash),
		string(r.PackHash),
		r.CreatedBucket,
	}
	return strings.Join(parts, "|")
}

// ============================================================================
// Proof Acknowledgment
// ============================================================================

// ProofAckKind is the kind of acknowledgment for a proof.
type ProofAckKind string

const (
	// ProofAckViewed means the proof was viewed.
	ProofAckViewed ProofAckKind = "viewed"
	// ProofAckDismissed means the proof cue was dismissed.
	ProofAckDismissed ProofAckKind = "dismissed"
)

// Validate checks if the ProofAckKind is valid.
func (k ProofAckKind) Validate() error {
	switch k {
	case ProofAckViewed, ProofAckDismissed:
		return nil
	default:
		return fmt.Errorf("invalid ProofAckKind: %q", k)
	}
}

// String returns the string representation.
func (k ProofAckKind) String() string {
	return string(k)
}

// SignedClaimProofAck records user acknowledgment of a claim proof.
type SignedClaimProofAck struct {
	// AckKind is viewed or dismissed.
	AckKind ProofAckKind
	// CircleIDHash is SHA256 of the circle ID.
	CircleIDHash SafeRefHash
	// PeriodKey is the time bucket.
	PeriodKey string
}

// CanonicalString returns the canonical pipe-delimited string.
func (a SignedClaimProofAck) CanonicalString() string {
	parts := []string{
		string(a.AckKind),
		string(a.CircleIDHash),
		a.PeriodKey,
	}
	return strings.Join(parts, "|")
}

// Validate checks all fields of the ack.
func (a SignedClaimProofAck) Validate() error {
	if err := a.AckKind.Validate(); err != nil {
		return fmt.Errorf("ack AckKind: %w", err)
	}
	if err := a.CircleIDHash.Validate(); err != nil {
		return fmt.Errorf("ack CircleIDHash: %w", err)
	}
	if a.PeriodKey == "" {
		return errors.New("ack PeriodKey cannot be empty")
	}
	return nil
}
