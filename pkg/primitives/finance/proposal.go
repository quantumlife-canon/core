// Package finance provides financial proposal types.
//
// CRITICAL: Proposals are NON-BINDING suggestions for human review.
// They NEVER trigger actions. They NEVER auto-execute.
// Language must be neutral and optional.
//
// Required disclaimers in all proposals:
// - "This is informational only."
// - "No action has been taken."
// - "You may dismiss this at any time."
package finance

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"
)

// FinancialProposal is a non-binding suggestion for human review.
// Proposals are OPTIONAL insights that humans may freely ignore.
//
// CRITICAL constraints:
// - No deadlines that trigger action
// - No escalation if ignored
// - Dismissal is immediate and complete
// - No guilt language on dismissal
type FinancialProposal struct {
	// ProposalID uniquely identifies this proposal.
	ProposalID string

	// OwnerType is "circle" or "intersection".
	OwnerType string

	// OwnerID is the circle or intersection ID.
	OwnerID string

	// Type describes what kind of proposal this is.
	Type ProposalType

	// Title is a brief summary (neutral language).
	Title string

	// Description is the proposal text (neutral, optional framing).
	// MUST include: "This is informational only. No action has been taken."
	Description string

	// Rationale explains why this proposal was generated.
	Rationale string

	// SourceObservationIDs links to the observations that triggered this.
	SourceObservationIDs []string

	// Basis lists the data points that produced this proposal.
	Basis []string

	// Assumptions lists any assumptions made.
	Assumptions []string

	// Limitations lists what this proposal does not account for.
	Limitations []string

	// CreatedAt is when this proposal was created.
	CreatedAt time.Time

	// SchemaVersion is the version of this schema.
	SchemaVersion string

	// TraceID links to the operation that created this proposal.
	TraceID string

	// Fingerprint is a stable hash for deduplication/dismissal.
	Fingerprint string

	// Disclaimers contains required disclaimer text.
	Disclaimers ProposalDisclaimers

	// Status is the proposal's current status.
	Status ProposalStatus

	// DismissedAt is when this proposal was dismissed (if applicable).
	DismissedAt *time.Time

	// DismissedBy is who dismissed this proposal (if applicable).
	DismissedBy string
}

// ProposalType describes the kind of proposal.
type ProposalType string

const (
	// ProposalReviewSpending suggests reviewing spending in a category.
	ProposalReviewSpending ProposalType = "review_spending"

	// ProposalReviewRecurring suggests reviewing a recurring charge.
	ProposalReviewRecurring ProposalType = "review_recurring"

	// ProposalReviewLargeTransaction suggests reviewing a large transaction.
	ProposalReviewLargeTransaction ProposalType = "review_large_transaction"

	// ProposalReviewCashflow suggests reviewing overall cashflow.
	ProposalReviewCashflow ProposalType = "review_cashflow"

	// ProposalReviewBalance suggests reviewing account balance.
	ProposalReviewBalance ProposalType = "review_balance"

	// ProposalAcknowledgeDataQuality notes data quality issues.
	ProposalAcknowledgeDataQuality ProposalType = "acknowledge_data_quality"
)

// ProposalStatus describes the proposal's lifecycle state.
type ProposalStatus string

const (
	// StatusActive means the proposal is visible.
	StatusActive ProposalStatus = "active"

	// StatusDismissed means the human dismissed this proposal.
	StatusDismissed ProposalStatus = "dismissed"

	// StatusDecayed means the proposal aged out.
	StatusDecayed ProposalStatus = "decayed"

	// StatusSuppressed means the proposal was suppressed (similar existing).
	StatusSuppressed ProposalStatus = "suppressed"
)

// ProposalDisclaimers contains required disclaimer text.
type ProposalDisclaimers struct {
	// Informational is the "informational only" disclaimer.
	Informational string

	// NoAction is the "no action taken" disclaimer.
	NoAction string

	// Dismissible is the "you may dismiss" disclaimer.
	Dismissible string
}

// StandardDisclaimers returns the required disclaimer text.
func StandardDisclaimers() ProposalDisclaimers {
	return ProposalDisclaimers{
		Informational: "This is informational only.",
		NoAction:      "No action has been taken.",
		Dismissible:   "You may dismiss this at any time.",
	}
}

// ComputeProposalFingerprint generates a stable fingerprint for deduplication.
// The fingerprint is based on proposal type, owner, and key factors.
func ComputeProposalFingerprint(proposalType ProposalType, ownerType, ownerID string, factors map[string]string) string {
	// Sort factor keys for determinism
	keys := make([]string, 0, len(factors))
	for k := range factors {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build canonical string
	data := fmt.Sprintf("proposal:%s:%s:%s", proposalType, ownerType, ownerID)
	for _, k := range keys {
		data += fmt.Sprintf(":%s=%s", k, factors[k])
	}

	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16])
}

// IsDismissed returns true if the proposal has been dismissed.
func (p *FinancialProposal) IsDismissed() bool {
	return p.Status == StatusDismissed
}

// IsActive returns true if the proposal is active.
func (p *FinancialProposal) IsActive() bool {
	return p.Status == StatusActive
}

// ProposalBatch contains proposals from an analysis.
type ProposalBatch struct {
	// BatchID uniquely identifies this batch.
	BatchID string

	// OwnerType is "circle" or "intersection".
	OwnerType string

	// OwnerID is the circle or intersection ID.
	OwnerID string

	// Proposals contains the generated proposals.
	Proposals []FinancialProposal

	// CreatedAt is when this batch was created.
	CreatedAt time.Time

	// TraceID links to the operation that created this batch.
	TraceID string

	// SuppressedCount is how many proposals were suppressed.
	SuppressedCount int

	// SilenceApplied indicates if silence policy resulted in zero proposals.
	SilenceApplied bool

	// SilenceReason explains why silence was applied (if applicable).
	SilenceReason string
}

// IsEmpty returns true if no proposals were generated.
func (b *ProposalBatch) IsEmpty() bool {
	return len(b.Proposals) == 0
}
