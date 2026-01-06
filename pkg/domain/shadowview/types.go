// Package shadowview provides domain types for the shadow receipt primary proof.
//
// Phase 27: Real Shadow Receipt (Primary Proof of Intelligence, Zero Pressure)
//
// CRITICAL INVARIANTS:
//   - Shadow remains observation-only
//   - Shadow never alters runtime behavior
//   - Shadow output is abstract, bucketed, non-identifying
//   - User interaction is optional, single-shot, ignorable
//   - Silence remains the success state
//
// Reference: docs/ADR/ADR-0058-phase27-real-shadow-receipt-primary-proof.md
package shadowview

import (
	"crypto/sha256"
	"encoding/hex"
)

// =============================================================================
// Receipt Content Types (Phase 27)
// =============================================================================

// EvidenceAskedKind describes what we asked the model (generic only).
type EvidenceAskedKind string

const (
	// EvidenceAskedSurfaceCheck indicates we asked about surfacing.
	EvidenceAskedSurfaceCheck EvidenceAskedKind = "surface_check"

	// EvidenceAskedNone indicates no model was consulted.
	EvidenceAskedNone EvidenceAskedKind = "none"
)

// Validate checks if the kind is valid.
func (k EvidenceAskedKind) Validate() bool {
	switch k {
	case EvidenceAskedSurfaceCheck, EvidenceAskedNone:
		return true
	default:
		return false
	}
}

// ShadowReceiptEvidence represents "what we asked" section.
//
// CRITICAL: Contains ONLY generic phrasing - never specific queries.
type ShadowReceiptEvidence struct {
	// Kind is the type of question asked.
	Kind EvidenceAskedKind

	// Statement is the human-readable phrasing.
	// Example: "We asked whether anything should reach you today."
	Statement string
}

// CanonicalString returns the pipe-delimited canonical representation.
func (e *ShadowReceiptEvidence) CanonicalString() string {
	return "EVIDENCE|v1|" + string(e.Kind) + "|" + e.Statement
}

// =============================================================================
// Model Return Types
// =============================================================================

// HorizonBucket indicates urgency in abstract terms.
type HorizonBucket string

const (
	HorizonSoon    HorizonBucket = "soon"
	HorizonLater   HorizonBucket = "later"
	HorizonSomeday HorizonBucket = "someday"
	HorizonNone    HorizonBucket = "none" // Model returned nothing
)

// Validate checks if the horizon is valid.
func (h HorizonBucket) Validate() bool {
	switch h {
	case HorizonSoon, HorizonLater, HorizonSomeday, HorizonNone:
		return true
	default:
		return false
	}
}

// MagnitudeBucket indicates quantity in abstract terms.
type MagnitudeBucket string

const (
	MagnitudeNothing MagnitudeBucket = "nothing"
	MagnitudeAFew    MagnitudeBucket = "a_few"
	MagnitudeSeveral MagnitudeBucket = "several"
)

// Validate checks if the magnitude is valid.
func (m MagnitudeBucket) Validate() bool {
	switch m {
	case MagnitudeNothing, MagnitudeAFew, MagnitudeSeveral:
		return true
	default:
		return false
	}
}

// ConfidenceBucket indicates model confidence.
type ConfidenceBucket string

const (
	ConfidenceLow    ConfidenceBucket = "low"
	ConfidenceMedium ConfidenceBucket = "medium"
	ConfidenceHigh   ConfidenceBucket = "high"
	ConfidenceNA     ConfidenceBucket = "na" // No model consulted
)

// Validate checks if the confidence is valid.
func (c ConfidenceBucket) Validate() bool {
	switch c {
	case ConfidenceLow, ConfidenceMedium, ConfidenceHigh, ConfidenceNA:
		return true
	default:
		return false
	}
}

// ShadowReceiptModelReturn represents "what the model returned" section.
//
// CRITICAL: Contains ONLY abstract buckets - never raw counts or categories beyond canon.
type ShadowReceiptModelReturn struct {
	// Horizon indicates urgency bucket.
	Horizon HorizonBucket

	// Magnitude indicates quantity bucket.
	Magnitude MagnitudeBucket

	// Confidence indicates model confidence bucket.
	Confidence ConfidenceBucket

	// Statement is the human-readable summary.
	Statement string
}

// CanonicalString returns the pipe-delimited canonical representation.
func (r *ShadowReceiptModelReturn) CanonicalString() string {
	return "MODEL_RETURN|v1|" +
		string(r.Horizon) + "|" +
		string(r.Magnitude) + "|" +
		string(r.Confidence)
}

// =============================================================================
// Decision Types
// =============================================================================

// DecisionKind describes what action was taken (always restraint-forward).
type DecisionKind string

const (
	// DecisionNoSurface means nothing was surfaced.
	DecisionNoSurface DecisionKind = "no_surface"

	// DecisionNoModel means no model was consulted.
	DecisionNoModel DecisionKind = "no_model"
)

// Validate checks if the decision is valid.
func (d DecisionKind) Validate() bool {
	switch d {
	case DecisionNoSurface, DecisionNoModel:
		return true
	default:
		return false
	}
}

// ShadowReceiptDecision represents "what we did" section.
//
// CRITICAL: Always restraint-forward phrasing.
type ShadowReceiptDecision struct {
	// Kind is the type of decision made.
	Kind DecisionKind

	// Statement is the human-readable summary.
	// Example: "We chose not to surface anything."
	Statement string
}

// CanonicalString returns the pipe-delimited canonical representation.
func (d *ShadowReceiptDecision) CanonicalString() string {
	return "DECISION|v1|" + string(d.Kind) + "|" + d.Statement
}

// =============================================================================
// Reason Types
// =============================================================================

// ReasonKind explains why this didn't interrupt the user.
type ReasonKind string

const (
	// ReasonBelowThreshold means signal was below surfacing threshold.
	ReasonBelowThreshold ReasonKind = "below_threshold"

	// ReasonDefaultHold means default hold policy was in effect.
	ReasonDefaultHold ReasonKind = "default_hold"

	// ReasonExplicitActionRequired means explicit action is required.
	ReasonExplicitActionRequired ReasonKind = "explicit_action_required"

	// ReasonNoModelConsulted means no model was consulted.
	ReasonNoModelConsulted ReasonKind = "no_model_consulted"

	// ReasonShadowOnly means shadow mode is observation-only.
	ReasonShadowOnly ReasonKind = "shadow_only"
)

// Validate checks if the reason is valid.
func (r ReasonKind) Validate() bool {
	switch r {
	case ReasonBelowThreshold, ReasonDefaultHold, ReasonExplicitActionRequired,
		ReasonNoModelConsulted, ReasonShadowOnly:
		return true
	default:
		return false
	}
}

// ShadowReceiptReason represents "why this didn't interrupt you" section.
type ShadowReceiptReason struct {
	// Kind is the type of reason.
	Kind ReasonKind

	// Statement is the human-readable explanation.
	Statement string
}

// CanonicalString returns the pipe-delimited canonical representation.
func (r *ShadowReceiptReason) CanonicalString() string {
	return "REASON|v1|" + string(r.Kind) + "|" + r.Statement
}

// =============================================================================
// Provider Disclosure Types
// =============================================================================

// ProviderKind identifies the shadow provider (abstract disclosure only).
type ProviderKind string

const (
	// ProviderNone means no provider was configured/used.
	ProviderNone ProviderKind = "none"

	// ProviderStub means the deterministic stub provider was used.
	ProviderStub ProviderKind = "stub"

	// ProviderAzureOpenAIChat means Azure OpenAI chat was used.
	ProviderAzureOpenAIChat ProviderKind = "azure_openai_chat"
)

// Validate checks if the provider kind is valid.
func (p ProviderKind) Validate() bool {
	switch p {
	case ProviderNone, ProviderStub, ProviderAzureOpenAIChat:
		return true
	default:
		return false
	}
}

// ShadowReceiptProvider represents provider disclosure.
//
// CRITICAL: Never shows model names, deployment names, regions, endpoints, or keys.
type ShadowReceiptProvider struct {
	// Kind identifies the provider type.
	Kind ProviderKind

	// WasConsulted indicates if a model was actually consulted.
	WasConsulted bool

	// Statement is the human-readable disclosure.
	// If consulted: "A model was consulted (stub)" or "A model was consulted (azure_openai_chat)"
	// If not: "No model was consulted."
	Statement string
}

// CanonicalString returns the pipe-delimited canonical representation.
func (p *ShadowReceiptProvider) CanonicalString() string {
	consulted := "false"
	if p.WasConsulted {
		consulted = "true"
	}
	return "PROVIDER|v1|" + string(p.Kind) + "|" + consulted
}

// =============================================================================
// Vote Types
// =============================================================================

// VoteChoice represents the user's vote on restraint usefulness.
type VoteChoice string

const (
	// VoteUseful means the restraint was useful.
	VoteUseful VoteChoice = "useful"

	// VoteUnnecessary means the restraint was unnecessary.
	VoteUnnecessary VoteChoice = "unnecessary"

	// VoteSkip means the user chose to skip voting.
	VoteSkip VoteChoice = "skip"
)

// Validate checks if the vote is valid.
func (v VoteChoice) Validate() bool {
	switch v {
	case VoteUseful, VoteUnnecessary, VoteSkip:
		return true
	default:
		return false
	}
}

// ShadowReceiptVoteEligibility indicates if voting is available.
type ShadowReceiptVoteEligibility struct {
	// Eligible indicates if voting is available.
	Eligible bool

	// AlreadyVoted indicates if a vote was already recorded for this receipt.
	AlreadyVoted bool

	// ReceiptHash is the hash of the receipt being voted on.
	ReceiptHash string
}

// ShadowReceiptVote records a single vote.
//
// CRITICAL: Vote does NOT change behavior.
// CRITICAL: Vote feeds Phase 19 calibration only.
type ShadowReceiptVote struct {
	// ReceiptHash is the SHA256 hash of the receipt.
	ReceiptHash string

	// Choice is the user's vote.
	Choice VoteChoice

	// PeriodBucket is the day bucket (YYYY-MM-DD).
	PeriodBucket string
}

// CanonicalString returns the pipe-delimited canonical representation.
func (v *ShadowReceiptVote) CanonicalString() string {
	return "VOTE|v1|" + v.ReceiptHash + "|" + string(v.Choice) + "|" + v.PeriodBucket
}

// =============================================================================
// Primary Proof Page (Phase 27)
// =============================================================================

// ShadowReceiptPrimaryPage contains all data for the Phase 27 primary proof page.
//
// CRITICAL: Contains ONLY abstract buckets and hashes.
// CRITICAL: No raw content, no identifiable information.
// CRITICAL: This is proof, not marketing.
type ShadowReceiptPrimaryPage struct {
	// HasReceipt indicates if a shadow receipt exists.
	HasReceipt bool

	// Evidence is "What we asked" section.
	Evidence ShadowReceiptEvidence

	// ModelReturn is "What the model returned" section.
	ModelReturn ShadowReceiptModelReturn

	// Decision is "What we did" section.
	Decision ShadowReceiptDecision

	// Reason is "Why this didn't interrupt you" section.
	Reason ShadowReceiptReason

	// Provider is the provider disclosure section.
	Provider ShadowReceiptProvider

	// VoteEligibility indicates if voting is available.
	VoteEligibility ShadowReceiptVoteEligibility

	// StatusHash is the deterministic hash of the page content.
	StatusHash string

	// BackPath is the return URL.
	BackPath string
}

// CanonicalString returns the pipe-delimited canonical representation.
func (p *ShadowReceiptPrimaryPage) CanonicalString() string {
	return "SHADOW_RECEIPT_PRIMARY|v1|" +
		p.Evidence.CanonicalString() + "|" +
		p.ModelReturn.CanonicalString() + "|" +
		p.Decision.CanonicalString() + "|" +
		p.Reason.CanonicalString() + "|" +
		p.Provider.CanonicalString()
}

// ComputeStatusHash computes the deterministic hash of the page.
func (p *ShadowReceiptPrimaryPage) ComputeStatusHash() string {
	h := sha256.New()
	h.Write([]byte(p.CanonicalString()))
	return hex.EncodeToString(h.Sum(nil)[:16]) // 32 hex chars
}

// =============================================================================
// Whisper Cue (Phase 27)
// =============================================================================

// ShadowReceiptCue represents the whisper cue for the shadow receipt.
//
// CRITICAL: Subject to single-whisper rule.
// CRITICAL: Must be ignorable with zero cost.
type ShadowReceiptCue struct {
	// Available indicates if the cue should be shown.
	Available bool

	// CueText is the subtle text.
	// Phase 27 text: "We checked something — quietly."
	CueText string

	// LinkText is the link text.
	LinkText string

	// ReceiptHash for acknowledgement tracking.
	ReceiptHash string
}

// DefaultCueText is the Phase 27 whisper cue text.
const DefaultCueText = "We checked something — quietly."

// DefaultLinkText is the Phase 27 link text.
const DefaultLinkText = "proof"
