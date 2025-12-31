// Package execution provides v9 financial execution primitives.
//
// This file implements the v9.4 ApprovalBundle for multi-party financial execution.
//
// CRITICAL: This is the symmetry-verified approval bundle for multi-party execution.
// Every approver MUST receive an IDENTICAL bundle (verified by content hash).
//
// NON-NEGOTIABLE INVARIANTS:
// 1) No blanket/standing approvals - each approval binds to a specific ActionHash
// 2) Neutral approval language - no urgency/fear/shame/authority/optimization
// 3) Symmetry - every approver receives IDENTICAL payload (provable in audit)
// 4) Approval expiry enforced at verification time
// 5) Single-use approvals only
//
// Subordinate to:
// - docs/QUANTUMLIFE_CANON_V1.md
// - docs/CANON_ADDENDUM_V9_FINANCIAL_EXECUTION.md
// - docs/TECHNICAL_SPLIT_V9_EXECUTION.md
package execution

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ApprovalBundle is the canonical, immutable payload presented to all approvers.
// Every approver receives the EXACT same bundle (verified by ContentHash).
//
// This bundle contains ONLY what approvers need to make an informed decision.
// No hidden fields, no asymmetric information, no coercive language.
type ApprovalBundle struct {
	// EnvelopeID is the envelope requiring approval.
	EnvelopeID string `json:"envelope_id"`

	// ActionHash cryptographically binds this bundle to the action.
	ActionHash string `json:"action_hash"`

	// IntersectionID is the intersection context (required for multi-party).
	IntersectionID string `json:"intersection_id"`

	// PayeeID is the pre-defined payee identifier.
	PayeeID string `json:"payee_id"`

	// AmountCents is the exact amount in cents.
	AmountCents int64 `json:"amount_cents"`

	// Currency is the currency code.
	Currency string `json:"currency"`

	// ExecutionWindowStart is when execution may begin.
	ExecutionWindowStart time.Time `json:"execution_window_start"`

	// ExecutionWindowEnd is when execution window closes.
	ExecutionWindowEnd time.Time `json:"execution_window_end"`

	// Expiry is when this bundle expires.
	Expiry time.Time `json:"expiry"`

	// RevocationWindowStart is when revocation window opens.
	RevocationWindowStart time.Time `json:"revocation_window_start"`

	// RevocationWindowEnd is when revocation window closes.
	RevocationWindowEnd time.Time `json:"revocation_window_end"`

	// RevocationWaived indicates if revocation window is waived for this action.
	// CRITICAL: Can only be waived per-action, never default.
	RevocationWaived bool `json:"revocation_waived"`

	// ViewHash is the ContentHash of the v8 SharedFinancialView (if available).
	ViewHash string `json:"view_hash,omitempty"`

	// ViewContentHash is the hash of view content (for verification).
	ViewContentHash string `json:"view_content_hash,omitempty"`

	// NeutralityAttestation contains neutrality check results.
	NeutralityAttestation NeutralityAttestation `json:"neutrality_attestation"`

	// Description is a neutral, factual description of the action.
	// MUST NOT contain urgency/fear/shame/authority/optimization language.
	Description string `json:"description"`

	// CreatedAt is when this bundle was created.
	CreatedAt time.Time `json:"created_at"`

	// ContentHash is computed from all fields for symmetry verification.
	// This is NOT included in the hash computation itself.
	ContentHash string `json:"-"`
}

// NeutralityAttestation records the neutrality verification result.
type NeutralityAttestation struct {
	// Verified indicates if neutrality check passed.
	Verified bool `json:"verified"`

	// Reason explains the result.
	Reason string `json:"reason"`

	// CheckedAt is when the check was performed.
	CheckedAt time.Time `json:"checked_at"`
}

// ComputeContentHash computes the deterministic hash of the bundle.
// This is used to verify all approvers received identical bundles.
func (b *ApprovalBundle) ComputeContentHash() string {
	// Serialize to canonical JSON for deterministic hashing
	canonical := b.canonicalJSON()
	h := sha256.New()
	h.Write([]byte(canonical))
	return hex.EncodeToString(h.Sum(nil))
}

// canonicalJSON produces a deterministic JSON representation.
func (b *ApprovalBundle) canonicalJSON() string {
	// Build canonical representation with sorted keys
	parts := []string{
		fmt.Sprintf(`"action_hash":"%s"`, b.ActionHash),
		fmt.Sprintf(`"amount_cents":%d`, b.AmountCents),
		fmt.Sprintf(`"created_at":"%s"`, b.CreatedAt.UTC().Format(time.RFC3339Nano)),
		fmt.Sprintf(`"currency":"%s"`, b.Currency),
		fmt.Sprintf(`"description":"%s"`, escapeJSON(b.Description)),
		fmt.Sprintf(`"envelope_id":"%s"`, b.EnvelopeID),
		fmt.Sprintf(`"execution_window_end":"%s"`, b.ExecutionWindowEnd.UTC().Format(time.RFC3339Nano)),
		fmt.Sprintf(`"execution_window_start":"%s"`, b.ExecutionWindowStart.UTC().Format(time.RFC3339Nano)),
		fmt.Sprintf(`"expiry":"%s"`, b.Expiry.UTC().Format(time.RFC3339Nano)),
		fmt.Sprintf(`"intersection_id":"%s"`, b.IntersectionID),
		fmt.Sprintf(`"neutrality_attestation":{"checked_at":"%s","reason":"%s","verified":%t}`,
			b.NeutralityAttestation.CheckedAt.UTC().Format(time.RFC3339Nano),
			escapeJSON(b.NeutralityAttestation.Reason),
			b.NeutralityAttestation.Verified),
		fmt.Sprintf(`"payee_id":"%s"`, b.PayeeID),
		fmt.Sprintf(`"revocation_waived":%t`, b.RevocationWaived),
		fmt.Sprintf(`"revocation_window_end":"%s"`, b.RevocationWindowEnd.UTC().Format(time.RFC3339Nano)),
		fmt.Sprintf(`"revocation_window_start":"%s"`, b.RevocationWindowStart.UTC().Format(time.RFC3339Nano)),
	}

	// Optional fields
	if b.ViewHash != "" {
		parts = append(parts, fmt.Sprintf(`"view_hash":"%s"`, b.ViewHash))
	}
	if b.ViewContentHash != "" {
		parts = append(parts, fmt.Sprintf(`"view_content_hash":"%s"`, b.ViewContentHash))
	}

	sort.Strings(parts)
	return "{" + strings.Join(parts, ",") + "}"
}

// escapeJSON escapes a string for JSON.
func escapeJSON(s string) string {
	b, _ := json.Marshal(s)
	// Remove surrounding quotes
	return string(b[1 : len(b)-1])
}

// Seal computes and sets the ContentHash.
func (b *ApprovalBundle) Seal() {
	b.ContentHash = b.ComputeContentHash()
}

// VerifyHash checks if the stored hash matches the computed hash.
func (b *ApprovalBundle) VerifyHash() bool {
	return b.ContentHash == b.ComputeContentHash()
}

// ApproverBundleHash records the hash each approver received.
type ApproverBundleHash struct {
	// ApproverCircleID is the circle that received the bundle.
	ApproverCircleID string

	// ContentHash is the hash of the bundle they received.
	ContentHash string

	// PresentedAt is when the bundle was presented.
	PresentedAt time.Time
}

// SymmetryProof proves all approvers received identical bundles.
type SymmetryProof struct {
	// ProofID uniquely identifies this proof.
	ProofID string

	// BundleContentHash is the expected hash for all approvers.
	BundleContentHash string

	// ApproverHashes contains the hash each approver received.
	ApproverHashes []ApproverBundleHash

	// Symmetric is true if all hashes match.
	Symmetric bool

	// VerifiedAt is when symmetry was verified.
	VerifiedAt time.Time

	// Violations lists any asymmetric approvers.
	Violations []SymmetryViolation
}

// SymmetryViolation records a single asymmetry.
type SymmetryViolation struct {
	ApproverCircleID string
	ExpectedHash     string
	ReceivedHash     string
}

// SymmetryVerifier verifies that all approvers received identical bundles.
type SymmetryVerifier struct {
	idGenerator func() string
}

// NewSymmetryVerifier creates a new symmetry verifier.
func NewSymmetryVerifier(idGen func() string) *SymmetryVerifier {
	return &SymmetryVerifier{
		idGenerator: idGen,
	}
}

// Verify checks that all approver hashes match the expected bundle hash.
func (v *SymmetryVerifier) Verify(bundle *ApprovalBundle, approverHashes []ApproverBundleHash) *SymmetryProof {
	now := time.Now()
	expectedHash := bundle.ComputeContentHash()

	proof := &SymmetryProof{
		ProofID:           v.idGenerator(),
		BundleContentHash: expectedHash,
		ApproverHashes:    approverHashes,
		Symmetric:         true,
		VerifiedAt:        now,
		Violations:        make([]SymmetryViolation, 0),
	}

	for _, ah := range approverHashes {
		if ah.ContentHash != expectedHash {
			proof.Symmetric = false
			proof.Violations = append(proof.Violations, SymmetryViolation{
				ApproverCircleID: ah.ApproverCircleID,
				ExpectedHash:     expectedHash,
				ReceivedHash:     ah.ContentHash,
			})
		}
	}

	return proof
}

// ForbiddenLanguagePatterns are patterns that indicate non-neutral language.
// These MUST NOT appear in approval prompts or descriptions.
var ForbiddenLanguagePatterns = []string{
	// Urgency
	"urgent", "urgently", "immediately", "now", "asap", "hurry",
	"time-sensitive", "act now", "don't wait", "limited time",
	// Fear
	"miss out", "lose", "penalty", "fine", "expire soon",
	"last chance", "avoid", "prevent", "before it's too late",
	// Shame
	"should have", "could have", "failed to", "neglected",
	"irresponsible", "careless", "mistake",
	// Authority
	"required by", "mandated", "must comply", "obligation",
	"you must", "you need to", "you have to",
	// Optimization/Persuasion
	"best", "optimal", "recommended", "suggested", "smart",
	"save money", "save time", "efficient", "maximize",
	"approve now to avoid", "approve to save",
}

// NeutralityChecker verifies language neutrality.
type NeutralityChecker struct{}

// NewNeutralityChecker creates a new neutrality checker.
func NewNeutralityChecker() *NeutralityChecker {
	return &NeutralityChecker{}
}

// Check verifies that text contains no forbidden language patterns.
func (c *NeutralityChecker) Check(text string) NeutralityAttestation {
	now := time.Now()
	lowerText := strings.ToLower(text)

	for _, pattern := range ForbiddenLanguagePatterns {
		if strings.Contains(lowerText, strings.ToLower(pattern)) {
			return NeutralityAttestation{
				Verified:  false,
				Reason:    fmt.Sprintf("forbidden pattern detected: %q", pattern),
				CheckedAt: now,
			}
		}
	}

	return NeutralityAttestation{
		Verified:  true,
		Reason:    "no forbidden patterns detected",
		CheckedAt: now,
	}
}

// BuildApprovalBundle creates an ApprovalBundle from an envelope and policy.
func BuildApprovalBundle(
	envelope *ExecutionEnvelope,
	payeeID string,
	description string,
	expirySeconds int,
	idGen func() string,
) (*ApprovalBundle, error) {
	now := time.Now()

	// Check neutrality of description
	checker := NewNeutralityChecker()
	neutrality := checker.Check(description)
	if !neutrality.Verified {
		return nil, fmt.Errorf("description violates neutrality: %s", neutrality.Reason)
	}

	expiry := now.Add(time.Duration(expirySeconds) * time.Second)
	if expirySeconds <= 0 {
		expiry = envelope.Expiry
	}

	bundle := &ApprovalBundle{
		EnvelopeID:            envelope.EnvelopeID,
		ActionHash:            envelope.ActionHash,
		IntersectionID:        envelope.IntersectionID,
		PayeeID:               payeeID,
		AmountCents:           envelope.ActionSpec.AmountCents,
		Currency:              envelope.ActionSpec.Currency,
		ExecutionWindowStart:  now,
		ExecutionWindowEnd:    envelope.Expiry,
		Expiry:                expiry,
		RevocationWindowStart: envelope.RevocationWindowStart,
		RevocationWindowEnd:   envelope.RevocationWindowEnd,
		RevocationWaived:      envelope.RevocationWaived,
		ViewHash:              envelope.ViewHash,
		NeutralityAttestation: neutrality,
		Description:           description,
		CreatedAt:             now,
	}

	bundle.Seal()
	return bundle, nil
}

// MultiPartyApprovalArtifact extends ApprovalArtifact with bundle hash.
type MultiPartyApprovalArtifact struct {
	ApprovalArtifact

	// BundleContentHash is the hash of the bundle this approval is for.
	BundleContentHash string

	// Used indicates if this approval has been consumed.
	// CRITICAL: Single-use only. Once used, cannot be reused.
	Used bool

	// UsedAt is when the approval was consumed.
	UsedAt time.Time
}

// Errors for approval bundle operations.
var (
	// ErrNeutralityViolation is returned when language neutrality check fails.
	ErrNeutralityViolation = errors.New("neutrality violation: forbidden language detected")

	// ErrAsymmetricBundle is returned when approvers received different bundles.
	ErrAsymmetricBundle = errors.New("asymmetric bundle: approvers received different content")

	// ErrInsufficientApprovals is returned when threshold is not met.
	ErrInsufficientMultiPartyApprovals = errors.New("insufficient approvals for multi-party execution")

	// ErrApprovalReuse is returned when attempting to reuse a consumed approval.
	ErrApprovalReuse = errors.New("approval reuse: single-use approval has already been consumed")

	// ErrBundleExpired is returned when the approval bundle has expired.
	ErrBundleExpired = errors.New("approval bundle has expired")

	// ErrBundleHashMismatch is returned when approval doesn't match bundle hash.
	ErrBundleHashMismatch = errors.New("approval bundle hash mismatch")

	// ErrApprovalHashMismatch is returned when approval action hash doesn't match.
	ErrApprovalHashMismatch = errors.New("approval action hash mismatch")
)
