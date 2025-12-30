// Package propose generates deterministic, non-binding financial proposals.
//
// CRITICAL: Proposals are NON-BINDING and OPTIONAL.
// - No urgency language
// - No fear language
// - No shame language
// - No authority language
// - Silence is success when nothing material changed
//
// Reference: docs/ACCEPTANCE_TESTS_V8_FINANCIAL_READ.md §B, §D, §F
package propose

import (
	"fmt"
	"time"

	"quantumlife/pkg/primitives/finance"
)

// Generator creates financial proposals from observations.
type Generator struct {
	config      Config
	dismissals  DismissalStore
	clockFunc   func() time.Time
	idGenerator func() string
}

// Config configures proposal generation.
type Config struct {
	// MinIntervalDays is the minimum days between similar proposals.
	// Default: 7
	MinIntervalDays int

	// DecayDays is how long until a proposal ages out.
	// Default: 30
	DecayDays int

	// SuppressDismissedDays is how long to suppress dismissed proposals.
	// Default: 90
	SuppressDismissedDays int

	// MaxProposalsPerBatch limits proposals per generation.
	// Default: 5
	MaxProposalsPerBatch int
}

// DefaultConfig returns default configuration.
func DefaultConfig() Config {
	return Config{
		MinIntervalDays:       7,
		DecayDays:             30,
		SuppressDismissedDays: 90,
		MaxProposalsPerBatch:  5,
	}
}

// DismissalStore provides access to dismissal records.
type DismissalStore interface {
	// IsDismissed checks if a proposal fingerprint is dismissed.
	IsDismissed(fingerprint string) bool

	// GetLastGenerated returns when a fingerprint was last generated.
	GetLastGenerated(fingerprint string) *time.Time

	// RecordGenerated records that a fingerprint was generated.
	RecordGenerated(fingerprint string, at time.Time)
}

// NewGenerator creates a new proposal generator.
func NewGenerator(config Config, dismissals DismissalStore, clockFunc func() time.Time, idGen func() string) *Generator {
	if clockFunc == nil {
		clockFunc = time.Now
	}
	return &Generator{
		config:      config,
		dismissals:  dismissals,
		clockFunc:   clockFunc,
		idGenerator: idGen,
	}
}

// Generate creates proposals from observations.
// Returns empty slice if silence policy applies (no material changes).
func (g *Generator) Generate(ownerType, ownerID string, observations []finance.FinancialObservation, traceID string) *finance.ProposalBatch {
	now := g.clockFunc()

	batch := &finance.ProposalBatch{
		BatchID:   g.idGenerator(),
		OwnerType: ownerType,
		OwnerID:   ownerID,
		CreatedAt: now,
		TraceID:   traceID,
	}

	// Check if any observations warrant proposals
	if len(observations) == 0 {
		batch.SilenceApplied = true
		batch.SilenceReason = "No observations to generate proposals from"
		return batch
	}

	// Filter to notable observations
	notable := g.filterNotable(observations)
	if len(notable) == 0 {
		batch.SilenceApplied = true
		batch.SilenceReason = "No observations exceed threshold for proposals"
		return batch
	}

	// Generate proposals for each notable observation
	var proposals []finance.FinancialProposal
	suppressedCount := 0

	for _, obs := range notable {
		proposal := g.observationToProposal(ownerType, ownerID, obs, now, traceID)

		// Check suppression rules
		if g.shouldSuppress(proposal, now) {
			suppressedCount++
			continue
		}

		proposals = append(proposals, proposal)

		// Record generation
		if g.dismissals != nil {
			g.dismissals.RecordGenerated(proposal.Fingerprint, now)
		}

		// Enforce batch limit
		if len(proposals) >= g.config.MaxProposalsPerBatch {
			break
		}
	}

	batch.Proposals = proposals
	batch.SuppressedCount = suppressedCount

	// Check if all proposals were suppressed
	if len(proposals) == 0 && suppressedCount > 0 {
		batch.SilenceApplied = true
		batch.SilenceReason = fmt.Sprintf("%d proposals suppressed (dismissed or recent)", suppressedCount)
	}

	return batch
}

// filterNotable filters to observations worth proposing about.
func (g *Generator) filterNotable(observations []finance.FinancialObservation) []finance.FinancialObservation {
	var notable []finance.FinancialObservation

	for _, obs := range observations {
		// Only propose for notable or significant observations
		if obs.Severity == finance.SeverityNotable || obs.Severity == finance.SeveritySignificant {
			notable = append(notable, obs)
		}
	}

	return notable
}

// shouldSuppress checks if a proposal should be suppressed.
func (g *Generator) shouldSuppress(proposal finance.FinancialProposal, now time.Time) bool {
	if g.dismissals == nil {
		return false
	}

	// Check if dismissed
	if g.dismissals.IsDismissed(proposal.Fingerprint) {
		return true
	}

	// Check if generated too recently
	lastGen := g.dismissals.GetLastGenerated(proposal.Fingerprint)
	if lastGen != nil {
		minInterval := time.Duration(g.config.MinIntervalDays) * 24 * time.Hour
		if now.Sub(*lastGen) < minInterval {
			return true
		}
	}

	return false
}

// observationToProposal converts an observation to a proposal.
func (g *Generator) observationToProposal(ownerType, ownerID string, obs finance.FinancialObservation, now time.Time, traceID string) finance.FinancialProposal {
	proposalType := g.mapObservationToProposalType(obs.Type)

	// Compute fingerprint
	factors := map[string]string{
		"observation_type": string(obs.Type),
		"category":         obs.Category,
	}
	fingerprint := finance.ComputeProposalFingerprint(proposalType, ownerType, ownerID, factors)

	// Generate neutral, optional language
	title := g.generateTitle(obs)
	description := g.generateDescription(obs)
	rationale := g.generateRationale(obs)

	return finance.FinancialProposal{
		ProposalID:           g.idGenerator(),
		OwnerType:            ownerType,
		OwnerID:              ownerID,
		Type:                 proposalType,
		Title:                title,
		Description:          description,
		Rationale:            rationale,
		SourceObservationIDs: []string{obs.ObservationID},
		Basis:                obs.Basis,
		Assumptions:          obs.Assumptions,
		Limitations:          obs.Limitations,
		CreatedAt:            now,
		SchemaVersion:        "1.0",
		TraceID:              traceID,
		Fingerprint:          fingerprint,
		Disclaimers:          finance.StandardDisclaimers(),
		Status:               finance.StatusActive,
	}
}

// mapObservationToProposalType maps observation types to proposal types.
func (g *Generator) mapObservationToProposalType(obsType finance.ObservationType) finance.ProposalType {
	switch obsType {
	case finance.ObservationCategoryShift,
		finance.ObservationSpendingIncrease,
		finance.ObservationSpendingDecrease,
		finance.ObservationUnusualSpending:
		return finance.ProposalReviewSpending

	case finance.ObservationRecurringDetected:
		return finance.ProposalReviewRecurring

	case finance.ObservationLargeTransaction:
		return finance.ProposalReviewLargeTransaction

	case finance.ObservationNegativeCashflow,
		finance.ObservationPositiveCashflow,
		finance.ObservationCashflowShift:
		return finance.ProposalReviewCashflow

	case finance.ObservationBalanceChange,
		finance.ObservationBalanceIncrease,
		finance.ObservationBalanceDecrease,
		finance.ObservationLowBalance,
		finance.ObservationHighBalance:
		return finance.ProposalReviewBalance

	case finance.ObservationStaleData,
		finance.ObservationPartialData:
		return finance.ProposalAcknowledgeDataQuality

	default:
		return finance.ProposalReviewSpending
	}
}

// generateTitle creates a neutral title.
// CRITICAL: No urgency, fear, shame, or authority language.
func (g *Generator) generateTitle(obs finance.FinancialObservation) string {
	// Use observation title if available, otherwise generate
	if obs.Title != "" {
		return obs.Title
	}

	switch obs.Type {
	case finance.ObservationCategoryShift:
		return fmt.Sprintf("%s spending pattern", obs.Category)
	case finance.ObservationSpendingIncrease:
		return fmt.Sprintf("%s spending this month", obs.Category)
	case finance.ObservationLargeTransaction:
		return "Notable transaction"
	case finance.ObservationRecurringDetected:
		return "Recurring transaction detected"
	case finance.ObservationNegativeCashflow:
		return "Monthly cash flow summary"
	case finance.ObservationBalanceChange:
		return "Account balance change"
	default:
		return "Financial observation"
	}
}

// generateDescription creates a neutral description.
// CRITICAL: Must include disclaimers. No manipulative language.
func (g *Generator) generateDescription(obs finance.FinancialObservation) string {
	base := obs.Description
	if base == "" {
		base = "A pattern was observed in your financial data."
	}

	// Add mandatory disclaimers
	disclaimers := finance.StandardDisclaimers()
	return fmt.Sprintf("%s\n\n%s %s %s",
		base,
		disclaimers.Informational,
		disclaimers.NoAction,
		disclaimers.Dismissible,
	)
}

// generateRationale explains why this proposal was generated.
func (g *Generator) generateRationale(obs finance.FinancialObservation) string {
	return fmt.Sprintf("This observation was generated because: %s", obs.Reason)
}
