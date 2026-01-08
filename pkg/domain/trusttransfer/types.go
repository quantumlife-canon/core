// Package trusttransfer provides domain types for Phase 44: Cross-Circle Trust Transfer (HOLD-only).
//
// Trust Transfer allows a primary circle (From) to delegate HOLD-only restraint to a
// secondary circle (To) without adding any ability to surface/interrupt/deliver/execute.
// This is delegation of silence, not action.
//
// CRITICAL INVARIANTS:
//   - HOLD-only outcomes: engine returns ONLY NO_EFFECT, HOLD, or QUEUE_PROOF.
//     NEVER SURFACE, NEVER INTERRUPT_CANDIDATE, NEVER DELIVER, NEVER EXECUTE.
//   - Commerce excluded: commerce pressure is never escalated, even under scope_all.
//   - Deterministic: canonical strings + SHA256 hashes. Same inputs + clock => same hashes.
//   - No time.Now() in pkg/ or internal/. Use clock injection.
//   - No goroutines in pkg/ or internal/.
//   - stdlib only (no cloud SDKs).
//   - Hash-only storage: persist only hashes + enum buckets; never store identifiers.
//   - Bounded retention: 30 days OR 200 records, FIFO eviction.
//   - One active trust-transfer contract per FromCircle per period.
//   - UI is calm: no IDs shown, no free text, no urgency.
//
// Reference: docs/ADR/ADR-0081-phase44-cross-circle-trust-transfer-hold-only.md
package trusttransfer

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// ============================================================================
// Constants
// ============================================================================

// Retention limits
const (
	MaxRetentionDays = 30
	MaxRecords       = 200
	MaxTripDays      = 7
)

// Fixed UX copy
const (
	DefaultTitle        = "Shared holding."
	DefaultProofTitle   = "Trust, transferred."
	DefaultCueText      = "Shared holding is active — quietly."
	DefaultPath         = "/delegate/transfer"
	DefaultProofPath    = "/proof/transfer"
	DefaultProposePath  = "/delegate/transfer/propose"
	DefaultAcceptPath   = "/delegate/transfer/accept"
	DefaultRevokePath   = "/delegate/transfer/revoke"
)

// ============================================================================
// Enums: TransferScope
// ============================================================================

// TransferScope represents what type of pressure the transfer applies to.
type TransferScope string

const (
	// ScopeHuman matches pressure from human circles.
	ScopeHuman TransferScope = "human"
	// ScopeInstitution matches pressure from institution circles.
	ScopeInstitution TransferScope = "institution"
	// ScopeAll matches all non-commerce pressure (human + institution).
	// CRITICAL: Commerce is NEVER included, even under scope_all.
	ScopeAll TransferScope = "all"
)

// AllTransferScopes returns all scopes in deterministic order.
func AllTransferScopes() []TransferScope {
	return []TransferScope{ScopeHuman, ScopeInstitution, ScopeAll}
}

// Validate checks if the scope is valid.
func (s TransferScope) Validate() error {
	switch s {
	case ScopeHuman, ScopeInstitution, ScopeAll:
		return nil
	default:
		return fmt.Errorf("invalid transfer scope: %s", s)
	}
}

// CanonicalString returns the canonical string representation.
func (s TransferScope) CanonicalString() string {
	return string(s)
}

// DisplayText returns human-readable text.
func (s TransferScope) DisplayText() string {
	switch s {
	case ScopeHuman:
		return "People"
	case ScopeInstitution:
		return "Institutions"
	case ScopeAll:
		return "All (except commerce)"
	default:
		return "Unknown"
	}
}

// MatchesCircleType checks if this scope matches the given circle type.
// CRITICAL: Commerce never matches, even for ScopeAll.
func (s TransferScope) MatchesCircleType(circleType string) bool {
	// Commerce is NEVER matched
	if circleType == "commerce" {
		return false
	}
	switch s {
	case ScopeHuman:
		return circleType == "human"
	case ScopeInstitution:
		return circleType == "institution"
	case ScopeAll:
		return circleType == "human" || circleType == "institution"
	default:
		return false
	}
}

// ============================================================================
// Enums: TransferMode
// ============================================================================

// TransferMode represents the mode of trust transfer.
// CRITICAL: Only HOLD_ONLY is allowed. No SURFACE, INTERRUPT, DELIVER, or EXECUTE.
type TransferMode string

const (
	// ModeHoldOnly means the transfer can only bias toward HOLD.
	// This is the ONLY allowed mode.
	ModeHoldOnly TransferMode = "hold_only"
)

// AllTransferModes returns all modes in deterministic order.
func AllTransferModes() []TransferMode {
	return []TransferMode{ModeHoldOnly}
}

// Validate checks if the mode is valid.
func (m TransferMode) Validate() error {
	switch m {
	case ModeHoldOnly:
		return nil
	default:
		return fmt.Errorf("invalid transfer mode: %s", m)
	}
}

// CanonicalString returns the canonical string representation.
func (m TransferMode) CanonicalString() string {
	return string(m)
}

// DisplayText returns human-readable text.
func (m TransferMode) DisplayText() string {
	switch m {
	case ModeHoldOnly:
		return "Hold only"
	default:
		return "Unknown"
	}
}

// ============================================================================
// Enums: TransferState
// ============================================================================

// TransferState represents the state of a trust transfer contract.
type TransferState string

const (
	// StateProposed means the transfer has been proposed but not yet accepted.
	StateProposed TransferState = "proposed"
	// StateActive means the transfer is currently in effect.
	StateActive TransferState = "active"
	// StateRevoked means the transfer was explicitly revoked.
	StateRevoked TransferState = "revoked"
	// StateExpired means the transfer has passed its duration.
	StateExpired TransferState = "expired"
)

// AllTransferStates returns all states in deterministic order.
func AllTransferStates() []TransferState {
	return []TransferState{StateProposed, StateActive, StateRevoked, StateExpired}
}

// Validate checks if the state is valid.
func (s TransferState) Validate() error {
	switch s {
	case StateProposed, StateActive, StateRevoked, StateExpired:
		return nil
	default:
		return fmt.Errorf("invalid transfer state: %s", s)
	}
}

// CanonicalString returns the canonical string representation.
func (s TransferState) CanonicalString() string {
	return string(s)
}

// DisplayText returns human-readable text.
func (s TransferState) DisplayText() string {
	switch s {
	case StateProposed:
		return "Proposed"
	case StateActive:
		return "Active"
	case StateRevoked:
		return "Revoked"
	case StateExpired:
		return "Expired"
	default:
		return "Unknown"
	}
}

// ============================================================================
// Enums: TransferDuration
// ============================================================================

// TransferDuration represents how long the transfer is active.
type TransferDuration string

const (
	// DurationHour expires after 1 hour bucket.
	DurationHour TransferDuration = "hour"
	// DurationDay expires after 1 day bucket.
	DurationDay TransferDuration = "day"
	// DurationTrip requires manual revocation; auto-expires after 7 days.
	DurationTrip TransferDuration = "trip"
)

// AllTransferDurations returns all durations in deterministic order.
func AllTransferDurations() []TransferDuration {
	return []TransferDuration{DurationHour, DurationDay, DurationTrip}
}

// Validate checks if the duration is valid.
func (d TransferDuration) Validate() error {
	switch d {
	case DurationHour, DurationDay, DurationTrip:
		return nil
	default:
		return fmt.Errorf("invalid transfer duration: %s", d)
	}
}

// CanonicalString returns the canonical string representation.
func (d TransferDuration) CanonicalString() string {
	return string(d)
}

// DisplayText returns human-readable text.
func (d TransferDuration) DisplayText() string {
	switch d {
	case DurationHour:
		return "One hour"
	case DurationDay:
		return "One day"
	case DurationTrip:
		return "Until revoked"
	default:
		return "Unknown"
	}
}

// BucketCount returns the number of time buckets for expiry calculation.
func (d TransferDuration) BucketCount() int {
	switch d {
	case DurationHour:
		return 1
	case DurationDay:
		return 24 // 24 hours
	case DurationTrip:
		return 168 // 7 days * 24 hours
	default:
		return 0
	}
}

// ============================================================================
// Enums: TransferDecision
// ============================================================================

// TransferDecision represents the outcome of applying a trust transfer.
// CRITICAL: Only NO_EFFECT, HOLD, and QUEUE_PROOF are allowed.
// NEVER SURFACE, INTERRUPT_CANDIDATE, DELIVER, or EXECUTE.
type TransferDecision string

const (
	// DecisionNoEffect means the transfer does not apply.
	DecisionNoEffect TransferDecision = "no_effect"
	// DecisionHold means the pressure should be held (suppressed).
	DecisionHold TransferDecision = "hold"
	// DecisionQueueProof means hold and queue proof for later display.
	DecisionQueueProof TransferDecision = "queue_proof"
)

// AllTransferDecisions returns all decisions in deterministic order.
func AllTransferDecisions() []TransferDecision {
	return []TransferDecision{DecisionNoEffect, DecisionHold, DecisionQueueProof}
}

// Validate checks if the decision is valid.
func (d TransferDecision) Validate() error {
	switch d {
	case DecisionNoEffect, DecisionHold, DecisionQueueProof:
		return nil
	default:
		return fmt.Errorf("invalid transfer decision: %s", d)
	}
}

// CanonicalString returns the canonical string representation.
func (d TransferDecision) CanonicalString() string {
	return string(d)
}

// IsHoldOutcome returns true if this is a HOLD-type outcome.
func (d TransferDecision) IsHoldOutcome() bool {
	return d == DecisionHold || d == DecisionQueueProof
}

// ============================================================================
// Enums: ProposalReason (allowlisted buckets only)
// ============================================================================

// ProposalReason represents why the trust transfer is being proposed.
// CRITICAL: Only allowlisted buckets - no free text.
type ProposalReason string

const (
	ReasonTravel   ProposalReason = "travel"
	ReasonWork     ProposalReason = "work"
	ReasonHealth   ProposalReason = "health"
	ReasonOverload ProposalReason = "overload"
	ReasonFamily   ProposalReason = "family"
	ReasonUnknown  ProposalReason = "unknown"
)

// AllProposalReasons returns all reasons in deterministic order.
func AllProposalReasons() []ProposalReason {
	return []ProposalReason{ReasonTravel, ReasonWork, ReasonHealth, ReasonOverload, ReasonFamily, ReasonUnknown}
}

// Validate checks if the reason is valid.
func (r ProposalReason) Validate() error {
	switch r {
	case ReasonTravel, ReasonWork, ReasonHealth, ReasonOverload, ReasonFamily, ReasonUnknown:
		return nil
	default:
		return fmt.Errorf("invalid proposal reason: %s", r)
	}
}

// CanonicalString returns the canonical string representation.
func (r ProposalReason) CanonicalString() string {
	return string(r)
}

// DisplayText returns human-readable text.
func (r ProposalReason) DisplayText() string {
	switch r {
	case ReasonTravel:
		return "Traveling"
	case ReasonWork:
		return "Focus time"
	case ReasonHealth:
		return "Health"
	case ReasonOverload:
		return "Overloaded"
	case ReasonFamily:
		return "Family time"
	case ReasonUnknown:
		return "Other"
	default:
		return "Unknown"
	}
}

// ============================================================================
// Enums: RevokeReason (allowlisted buckets only)
// ============================================================================

// RevokeReason represents why the trust transfer is being revoked.
// CRITICAL: Only allowlisted buckets - no free text.
type RevokeReason string

const (
	RevokeReasonDone        RevokeReason = "done"
	RevokeReasonTooMuch     RevokeReason = "too_much"
	RevokeReasonChangedMind RevokeReason = "changed_mind"
	RevokeReasonTrustReset  RevokeReason = "trust_reset"
	RevokeReasonUnknown     RevokeReason = "unknown"
)

// AllRevokeReasons returns all reasons in deterministic order.
func AllRevokeReasons() []RevokeReason {
	return []RevokeReason{RevokeReasonDone, RevokeReasonTooMuch, RevokeReasonChangedMind, RevokeReasonTrustReset, RevokeReasonUnknown}
}

// Validate checks if the reason is valid.
func (r RevokeReason) Validate() error {
	switch r {
	case RevokeReasonDone, RevokeReasonTooMuch, RevokeReasonChangedMind, RevokeReasonTrustReset, RevokeReasonUnknown:
		return nil
	default:
		return fmt.Errorf("invalid revoke reason: %s", r)
	}
}

// CanonicalString returns the canonical string representation.
func (r RevokeReason) CanonicalString() string {
	return string(r)
}

// DisplayText returns human-readable text.
func (r RevokeReason) DisplayText() string {
	switch r {
	case RevokeReasonDone:
		return "Done with this"
	case RevokeReasonTooMuch:
		return "Too much held"
	case RevokeReasonChangedMind:
		return "Changed mind"
	case RevokeReasonTrustReset:
		return "Trust reset"
	case RevokeReasonUnknown:
		return "Other"
	default:
		return "Unknown"
	}
}

// ============================================================================
// Structs: TrustTransferProposal
// ============================================================================

// TrustTransferProposal represents a proposed trust transfer.
// CRITICAL: Contains only hashes, enums, and buckets - no raw identifiers.
type TrustTransferProposal struct {
	// ProposalHash is the deterministic hash of this proposal.
	ProposalHash string

	// FromCircleHash identifies the source circle (the one delegating).
	FromCircleHash string

	// ToCircleHash identifies the target circle (the one receiving delegation).
	ToCircleHash string

	// Scope determines what type of pressure this transfer applies to.
	Scope TransferScope

	// Mode is always hold_only.
	Mode TransferMode

	// Duration determines how long the transfer is active.
	Duration TransferDuration

	// Reason is the allowlisted reason bucket.
	Reason ProposalReason

	// PeriodKey is the bucketed period when this was proposed.
	// Format: "YYYY-MM-DD-HH" (hour bucket)
	PeriodKey string
}

// Validate checks if the proposal is valid.
func (p *TrustTransferProposal) Validate() error {
	if p.ProposalHash == "" {
		return errors.New("missing proposal_hash")
	}
	if p.FromCircleHash == "" {
		return errors.New("missing from_circle_hash")
	}
	if p.ToCircleHash == "" {
		return errors.New("missing to_circle_hash")
	}
	if p.FromCircleHash == p.ToCircleHash {
		return errors.New("from and to circles must be different")
	}
	if err := p.Scope.Validate(); err != nil {
		return err
	}
	if err := p.Mode.Validate(); err != nil {
		return err
	}
	if err := p.Duration.Validate(); err != nil {
		return err
	}
	if err := p.Reason.Validate(); err != nil {
		return err
	}
	if p.PeriodKey == "" {
		return errors.New("missing period_key")
	}
	return nil
}

// CanonicalString returns the pipe-delimited canonical form.
func (p *TrustTransferProposal) CanonicalString() string {
	var b strings.Builder
	b.WriteString("TTP|v1|")
	b.WriteString(p.FromCircleHash)
	b.WriteString("|")
	b.WriteString(p.ToCircleHash)
	b.WriteString("|")
	b.WriteString(p.Scope.CanonicalString())
	b.WriteString("|")
	b.WriteString(p.Mode.CanonicalString())
	b.WriteString("|")
	b.WriteString(p.Duration.CanonicalString())
	b.WriteString("|")
	b.WriteString(p.Reason.CanonicalString())
	b.WriteString("|")
	b.WriteString(p.PeriodKey)
	return b.String()
}

// ComputeHash computes the deterministic hash of this proposal.
func (p *TrustTransferProposal) ComputeHash() string {
	h := sha256.Sum256([]byte(p.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// ============================================================================
// Structs: TrustTransferContract
// ============================================================================

// TrustTransferContract represents an active trust transfer agreement.
// CRITICAL: Contains only hashes, enums, and buckets - no raw identifiers.
type TrustTransferContract struct {
	// ContractHash is the deterministic hash of this contract.
	ContractHash string

	// FromCircleHash identifies the source circle (the one delegating).
	FromCircleHash string

	// ToCircleHash identifies the target circle (the one receiving delegation).
	ToCircleHash string

	// Scope determines what type of pressure this transfer applies to.
	Scope TransferScope

	// Mode is always hold_only.
	Mode TransferMode

	// Duration determines how long the transfer is active.
	Duration TransferDuration

	// Reason is the allowlisted reason bucket.
	Reason ProposalReason

	// State is the current state of the contract.
	State TransferState

	// CreatedPeriodKey is when this contract was created.
	// Format: "YYYY-MM-DD-HH" (hour bucket)
	CreatedPeriodKey string

	// StatusHash is a deterministic hash for verification.
	StatusHash string
}

// Validate checks if the contract is valid.
func (c *TrustTransferContract) Validate() error {
	if c.ContractHash == "" {
		return errors.New("missing contract_hash")
	}
	if c.FromCircleHash == "" {
		return errors.New("missing from_circle_hash")
	}
	if c.ToCircleHash == "" {
		return errors.New("missing to_circle_hash")
	}
	if c.FromCircleHash == c.ToCircleHash {
		return errors.New("from and to circles must be different")
	}
	if err := c.Scope.Validate(); err != nil {
		return err
	}
	if err := c.Mode.Validate(); err != nil {
		return err
	}
	if err := c.Duration.Validate(); err != nil {
		return err
	}
	if err := c.Reason.Validate(); err != nil {
		return err
	}
	if err := c.State.Validate(); err != nil {
		return err
	}
	if c.CreatedPeriodKey == "" {
		return errors.New("missing created_period_key")
	}
	return nil
}

// CanonicalString returns the pipe-delimited canonical form.
func (c *TrustTransferContract) CanonicalString() string {
	var b strings.Builder
	b.WriteString("TTC|v1|")
	b.WriteString(c.FromCircleHash)
	b.WriteString("|")
	b.WriteString(c.ToCircleHash)
	b.WriteString("|")
	b.WriteString(c.Scope.CanonicalString())
	b.WriteString("|")
	b.WriteString(c.Mode.CanonicalString())
	b.WriteString("|")
	b.WriteString(c.Duration.CanonicalString())
	b.WriteString("|")
	b.WriteString(c.Reason.CanonicalString())
	b.WriteString("|")
	b.WriteString(c.State.CanonicalString())
	b.WriteString("|")
	b.WriteString(c.CreatedPeriodKey)
	return b.String()
}

// ComputeHash computes the deterministic hash of this contract.
func (c *TrustTransferContract) ComputeHash() string {
	h := sha256.Sum256([]byte(c.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// IsActive returns true if the contract is in active state.
func (c *TrustTransferContract) IsActive() bool {
	return c.State == StateActive
}

// ============================================================================
// Structs: TrustTransferRevocation
// ============================================================================

// TrustTransferRevocation represents a revocation of a trust transfer.
type TrustTransferRevocation struct {
	// RevocationHash is the deterministic hash of this revocation.
	RevocationHash string

	// ContractHash identifies the contract being revoked.
	ContractHash string

	// FromCircleHash identifies the source circle.
	FromCircleHash string

	// Reason is the allowlisted revoke reason bucket.
	Reason RevokeReason

	// PeriodKey is when this revocation occurred.
	// Format: "YYYY-MM-DD-HH" (hour bucket)
	PeriodKey string
}

// Validate checks if the revocation is valid.
func (r *TrustTransferRevocation) Validate() error {
	if r.RevocationHash == "" {
		return errors.New("missing revocation_hash")
	}
	if r.ContractHash == "" {
		return errors.New("missing contract_hash")
	}
	if r.FromCircleHash == "" {
		return errors.New("missing from_circle_hash")
	}
	if err := r.Reason.Validate(); err != nil {
		return err
	}
	if r.PeriodKey == "" {
		return errors.New("missing period_key")
	}
	return nil
}

// CanonicalString returns the pipe-delimited canonical form.
func (r *TrustTransferRevocation) CanonicalString() string {
	var b strings.Builder
	b.WriteString("TTR|v1|")
	b.WriteString(r.ContractHash)
	b.WriteString("|")
	b.WriteString(r.FromCircleHash)
	b.WriteString("|")
	b.WriteString(r.Reason.CanonicalString())
	b.WriteString("|")
	b.WriteString(r.PeriodKey)
	return b.String()
}

// ComputeHash computes the deterministic hash of this revocation.
func (r *TrustTransferRevocation) ComputeHash() string {
	h := sha256.Sum256([]byte(r.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// ============================================================================
// Structs: TrustTransferEffect
// ============================================================================

// TrustTransferEffect represents the effect of applying a trust transfer to pressure.
// CRITICAL: Decision can ONLY be NO_EFFECT, HOLD, or QUEUE_PROOF.
type TrustTransferEffect struct {
	// Decision is the outcome of applying the transfer.
	// CRITICAL: Must be NO_EFFECT, HOLD, or QUEUE_PROOF only.
	Decision TransferDecision

	// ContractHash identifies the contract that was applied (if any).
	ContractHash string

	// OriginalDecision was the upstream decision before clamping.
	// Used for proof purposes only.
	OriginalDecision string

	// WasClamped indicates if the original decision was clamped to HOLD.
	WasClamped bool

	// EffectHash is a deterministic hash of this effect.
	EffectHash string
}

// Validate checks if the effect is valid.
func (e *TrustTransferEffect) Validate() error {
	if err := e.Decision.Validate(); err != nil {
		return err
	}
	if e.EffectHash == "" {
		return errors.New("missing effect_hash")
	}
	return nil
}

// CanonicalString returns the pipe-delimited canonical form.
func (e *TrustTransferEffect) CanonicalString() string {
	var b strings.Builder
	b.WriteString("TTE|v1|")
	b.WriteString(e.Decision.CanonicalString())
	b.WriteString("|")
	b.WriteString(e.ContractHash)
	b.WriteString("|")
	b.WriteString(e.OriginalDecision)
	b.WriteString("|")
	if e.WasClamped {
		b.WriteString("clamped")
	} else {
		b.WriteString("unchanged")
	}
	return b.String()
}

// ComputeHash computes the deterministic hash of this effect.
func (e *TrustTransferEffect) ComputeHash() string {
	h := sha256.Sum256([]byte(e.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// ============================================================================
// Structs: TrustTransferProofPage
// ============================================================================

// TrustTransferProofPage represents the proof page for trust transfers.
type TrustTransferProofPage struct {
	// Title is the page title.
	Title string

	// Lines contains explanatory text lines.
	Lines []string

	// HasContract indicates if a contract exists.
	HasContract bool

	// IsActive indicates if the contract is currently active.
	IsActive bool

	// Scope is the transfer scope (if contract exists).
	Scope TransferScope

	// Duration is the transfer duration (if contract exists).
	Duration TransferDuration

	// Reason is the proposal reason bucket (if contract exists).
	Reason ProposalReason

	// CreatedBucket is when the contract was created.
	CreatedBucket string

	// ContractHashPrefix is the first 16 chars of the contract hash.
	ContractHashPrefix string

	// StatusHashPrefix is the first 16 chars of the status hash.
	StatusHashPrefix string

	// BackLink is the link to go back.
	BackLink string
}

// Validate checks if the page is valid.
func (p *TrustTransferProofPage) Validate() error {
	if p.Title == "" {
		return errors.New("missing title")
	}
	if p.HasContract {
		if err := p.Scope.Validate(); err != nil {
			return err
		}
		if err := p.Duration.Validate(); err != nil {
			return err
		}
		if err := p.Reason.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// CanonicalString returns the pipe-delimited canonical form.
func (p *TrustTransferProofPage) CanonicalString() string {
	var b strings.Builder
	b.WriteString("TTPP|v1|")
	b.WriteString(p.Title)
	b.WriteString("|")
	if p.HasContract {
		b.WriteString("has_contract")
	} else {
		b.WriteString("no_contract")
	}
	b.WriteString("|")
	if p.IsActive {
		b.WriteString("active")
	} else {
		b.WriteString("inactive")
	}
	b.WriteString("|")
	b.WriteString(p.Scope.CanonicalString())
	b.WriteString("|")
	b.WriteString(p.Duration.CanonicalString())
	b.WriteString("|")
	b.WriteString(p.Reason.CanonicalString())
	b.WriteString("|")
	b.WriteString(p.CreatedBucket)
	return b.String()
}

// ComputeHash computes the deterministic hash of this page.
func (p *TrustTransferProofPage) ComputeHash() string {
	h := sha256.Sum256([]byte(p.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// ============================================================================
// Structs: TrustTransferStatusPage
// ============================================================================

// TrustTransferStatusPage represents the status page for trust transfers.
type TrustTransferStatusPage struct {
	// Title is the page title.
	Title string

	// Lines contains explanatory text lines.
	Lines []string

	// HasActiveContract indicates if a contract is active.
	HasActiveContract bool

	// ActiveContract is the current active contract (if any).
	ActiveContract *TrustTransferContract

	// CanPropose indicates if a new proposal can be made.
	CanPropose bool

	// BlockedReason is why proposal is blocked (if not CanPropose).
	BlockedReason string

	// ProposePath is the form action for proposing.
	ProposePath string

	// AcceptPath is the form action for accepting.
	AcceptPath string

	// RevokePath is the form action for revoking.
	RevokePath string

	// ProofPath is the link to the proof page.
	ProofPath string

	// BackLink is the link to go back.
	BackLink string

	// StatusHash is a deterministic hash for verification.
	StatusHash string
}

// CanonicalString returns the pipe-delimited canonical form.
func (p *TrustTransferStatusPage) CanonicalString() string {
	var b strings.Builder
	b.WriteString("TTSP|v1|")
	b.WriteString(p.Title)
	b.WriteString("|")
	if p.HasActiveContract {
		b.WriteString("has_active")
	} else {
		b.WriteString("no_active")
	}
	b.WriteString("|")
	if p.CanPropose {
		b.WriteString("can_propose")
	} else {
		b.WriteString("cannot_propose")
	}
	b.WriteString("|")
	b.WriteString(p.BlockedReason)
	if p.ActiveContract != nil {
		b.WriteString("|")
		b.WriteString(p.ActiveContract.ContractHash)
	}
	return b.String()
}

// ComputeHash computes the deterministic hash of this page.
func (p *TrustTransferStatusPage) ComputeHash() string {
	h := sha256.Sum256([]byte(p.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// ============================================================================
// Structs: TrustTransferCue
// ============================================================================

// TrustTransferCue represents the cue shown on /today.
type TrustTransferCue struct {
	// Available indicates if the cue should be shown.
	Available bool

	// CueText is the text to display.
	CueText string

	// Path is the path to navigate to.
	Path string

	// StatusHash must match the page StatusHash.
	StatusHash string
}

// ============================================================================
// Structs: Phase32DecisionInput
// ============================================================================

// Phase32DecisionInput represents an upstream decision from Phase 32.
// This is what ApplyTransfer receives and potentially clamps.
type Phase32DecisionInput struct {
	// CircleHash identifies the circle this decision is about.
	CircleHash string

	// CircleType is the type of circle (human/institution/commerce).
	CircleType string

	// Decision is the upstream decision string.
	// CRITICAL: If this is SURFACE, INTERRUPT_CANDIDATE, DELIVER, or EXECUTE,
	// it must be clamped to HOLD.
	Decision string

	// SourceHash is the hash of the upstream decision record.
	SourceHash string
}

// IsCommerce returns true if this is a commerce circle.
// CRITICAL: Commerce is NEVER affected by trust transfer.
func (p *Phase32DecisionInput) IsCommerce() bool {
	return p.CircleType == "commerce"
}

// IsForbiddenDecision returns true if the decision is forbidden under HOLD-only mode.
// Forbidden decisions: SURFACE, INTERRUPT_CANDIDATE, DELIVER, EXECUTE.
func (p *Phase32DecisionInput) IsForbiddenDecision() bool {
	switch p.Decision {
	case "surface", "interrupt_candidate", "deliver", "execute",
		"SURFACE", "INTERRUPT_CANDIDATE", "DELIVER", "EXECUTE":
		return true
	default:
		return false
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

// ComputeContractHash computes a deterministic contract hash from proposal.
func ComputeContractHash(fromCircle, toCircle, scope, duration, reason, periodKey string) string {
	var b strings.Builder
	b.WriteString("CONTRACT|")
	b.WriteString(fromCircle)
	b.WriteString("|")
	b.WriteString(toCircle)
	b.WriteString("|")
	b.WriteString(scope)
	b.WriteString("|")
	b.WriteString(duration)
	b.WriteString("|")
	b.WriteString(reason)
	b.WriteString("|")
	b.WriteString(periodKey)
	h := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(h[:16])
}

// NewDefaultStatusPage creates an empty status page.
func NewDefaultStatusPage() *TrustTransferStatusPage {
	page := &TrustTransferStatusPage{
		Title:       DefaultTitle,
		Lines:       []string{"No shared holding is active."},
		CanPropose:  true,
		ProposePath: DefaultProposePath,
		AcceptPath:  DefaultAcceptPath,
		RevokePath:  DefaultRevokePath,
		ProofPath:   DefaultProofPath,
		BackLink:    "/today",
	}
	page.StatusHash = page.ComputeHash()
	return page
}

// NewDefaultProofPage creates an empty proof page.
func NewDefaultProofPage() *TrustTransferProofPage {
	return &TrustTransferProofPage{
		Title:       DefaultProofTitle,
		Lines:       []string{"No shared holding recorded."},
		HasContract: false,
		BackLink:    DefaultPath,
	}
}

// BuildProofFromContract creates a proof page from a contract.
func BuildProofFromContract(contract *TrustTransferContract) *TrustTransferProofPage {
	if contract == nil {
		return NewDefaultProofPage()
	}

	contractPrefix := contract.ContractHash
	if len(contractPrefix) > 16 {
		contractPrefix = contractPrefix[:16]
	}

	statusPrefix := contract.StatusHash
	if len(statusPrefix) > 16 {
		statusPrefix = statusPrefix[:16]
	}

	page := &TrustTransferProofPage{
		Title:              DefaultProofTitle,
		Lines:              []string{"A shared holding agreement exists."},
		HasContract:        true,
		IsActive:           contract.IsActive(),
		Scope:              contract.Scope,
		Duration:           contract.Duration,
		Reason:             contract.Reason,
		CreatedBucket:      contract.CreatedPeriodKey,
		ContractHashPrefix: contractPrefix,
		StatusHashPrefix:   statusPrefix,
		BackLink:           DefaultPath,
	}

	return page
}
