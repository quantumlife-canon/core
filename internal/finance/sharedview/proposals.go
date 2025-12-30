// Package sharedview - Proposal Generator for Shared Financial Views
//
// CRITICAL: Proposals are OPTIONAL suggestions only.
// - No execution, payment, or automation
// - No urgency, fear, shame, authority, or optimization language
// - Dismissal is permanent and respected
// - Silence is a valid response
//
// Reference: v8.6 Family Financial Intersections

package sharedview

import (
	"fmt"
	"sort"
	"time"
)

// ProposalGenerator creates neutral financial proposals from shared views.
//
// CRITICAL: All language must be calm and non-judgmental.
// Proposals are suggestions only - never demands or requirements.
type ProposalGenerator struct {
	idGenerator func() string
}

// NewProposalGenerator creates a new proposal generator.
func NewProposalGenerator(idGen func() string) *ProposalGenerator {
	return &ProposalGenerator{
		idGenerator: idGen,
	}
}

// SharedProposal represents a proposal generated from shared financial data.
// These are identical for all parties when RequireSymmetry=true.
//
// CRITICAL: No urgency, no demands, dismissal permanent.
type SharedProposal struct {
	// ID uniquely identifies this proposal.
	ID string

	// IntersectionID is the intersection this proposal belongs to.
	IntersectionID string

	// ViewID is the shared view that generated this proposal.
	ViewID string

	// Type categorizes the proposal.
	Type ProposalType

	// Summary is a neutral, calm description.
	// CRITICAL: Must follow v8 language guidelines.
	Summary string

	// Details provides additional context (optional).
	Details string

	// RelatedCategory is the category this proposal relates to.
	RelatedCategory string

	// RelatedCurrency is the currency context.
	RelatedCurrency string

	// GeneratedAt is when this proposal was created.
	GeneratedAt time.Time

	// ExpiresAt is when this proposal should no longer be shown.
	// After expiry, proposal is silently removed.
	ExpiresAt time.Time

	// Priority indicates relative importance (lower = more important).
	// Used for ordering only - not for urgency language.
	Priority int

	// DismissedBy tracks which circles have dismissed this.
	// Once dismissed by a circle, never shown to that circle again.
	DismissedBy []string

	// ActionType is the kind of action this proposes.
	ActionType ProposalActionType

	// CRITICAL: No execution authority. These are suggestions only.
}

// ProposalType categorizes proposals.
type ProposalType string

const (
	// ProposalTypeObservation is a neutral observation.
	ProposalTypeObservation ProposalType = "observation"

	// ProposalTypeConversationStarter is a discussion prompt.
	ProposalTypeConversationStarter ProposalType = "conversation_starter"

	// ProposalTypeCategoryReview suggests reviewing a category together.
	ProposalTypeCategoryReview ProposalType = "category_review"

	// ProposalTypeBudgetDiscussion suggests discussing shared budgets.
	ProposalTypeBudgetDiscussion ProposalType = "budget_discussion"
)

// ProposalActionType indicates what kind of action the proposal suggests.
type ProposalActionType string

const (
	// ActionTypeDiscuss suggests having a conversation.
	ActionTypeDiscuss ProposalActionType = "discuss"

	// ActionTypeReview suggests reviewing data together.
	ActionTypeReview ProposalActionType = "review"

	// ActionTypeAcknowledge just asks for acknowledgment.
	ActionTypeAcknowledge ProposalActionType = "acknowledge"

	// ActionTypeNone requires no action.
	ActionTypeNone ProposalActionType = "none"
)

// GenerateRequest contains parameters for generating proposals.
type GenerateRequest struct {
	// View is the shared financial view to analyze.
	View *SharedFinancialView

	// MaxProposals limits how many proposals to generate.
	// Default: 3. More proposals = more noise.
	MaxProposals int

	// ExpiryDuration is how long proposals remain valid.
	// Default: 7 days.
	ExpiryDuration time.Duration

	// ExistingDismissals maps proposal types to circles that dismissed them.
	// Used to avoid regenerating dismissed proposals.
	ExistingDismissals map[ProposalType][]string
}

// GenerateResult contains generated proposals.
type GenerateResult struct {
	// Proposals is the list of generated proposals.
	Proposals []SharedProposal

	// Suppressed is the count of proposals not generated due to dismissals.
	Suppressed int

	// Reason is set if no proposals were generated.
	Reason string
}

// Generate creates proposals from a shared financial view.
// All proposals use neutral, calm language without urgency.
func (g *ProposalGenerator) Generate(req GenerateRequest) GenerateResult {
	if req.View == nil {
		return GenerateResult{Reason: "no_view_provided"}
	}

	if !req.View.Policy.ProposalAllowed {
		return GenerateResult{Reason: "proposals_disabled"}
	}

	if req.MaxProposals <= 0 {
		req.MaxProposals = 3
	}
	if req.ExpiryDuration == 0 {
		req.ExpiryDuration = 7 * 24 * time.Hour
	}

	candidates := g.generateCandidates(req.View, req.ExpiryDuration)
	if len(candidates) == 0 {
		return GenerateResult{Reason: "no_observations"}
	}

	// Sort by priority
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Priority < candidates[j].Priority
	})

	// Filter suppressed and limit
	var proposals []SharedProposal
	suppressed := 0

	for _, p := range candidates {
		if len(proposals) >= req.MaxProposals {
			break
		}

		// Check if suppressed by dismissals
		if req.ExistingDismissals != nil {
			if dismissers, ok := req.ExistingDismissals[p.Type]; ok {
				if len(dismissers) >= req.View.Provenance.ContributorCount {
					suppressed++
					continue
				}
			}
		}

		proposals = append(proposals, p)
	}

	return GenerateResult{
		Proposals:  proposals,
		Suppressed: suppressed,
	}
}

// generateCandidates creates candidate proposals from the view.
func (g *ProposalGenerator) generateCandidates(view *SharedFinancialView, expiry time.Duration) []SharedProposal {
	var candidates []SharedProposal
	now := time.Now().UTC()
	expiresAt := now.Add(expiry)

	// Category distribution observations
	for currency, categories := range view.SpendByCategory {
		// Find top categories
		type catSpend struct {
			name    string
			percent float64
			bucket  AmountBucket
		}
		var sorted []catSpend
		for name, cs := range categories {
			sorted = append(sorted, catSpend{name, cs.PercentOfTotal, cs.Bucket})
		}
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].percent > sorted[j].percent
		})

		// If one category is dominant (>40%), suggest reviewing
		if len(sorted) > 0 && sorted[0].percent > 40 {
			candidates = append(candidates, SharedProposal{
				ID:              g.idGenerator(),
				IntersectionID:  view.IntersectionID,
				ViewID:          view.ViewID,
				Type:            ProposalTypeCategoryReview,
				Summary:         g.neutralCategorySummary(sorted[0].name, sorted[0].percent),
				RelatedCategory: sorted[0].name,
				RelatedCurrency: currency,
				GeneratedAt:     now,
				ExpiresAt:       expiresAt,
				Priority:        2,
				ActionType:      ActionTypeReview,
			})
		}

		// If there are many small categories, suggest consolidation discussion
		smallCount := 0
		for _, c := range sorted {
			if c.percent < 5 {
				smallCount++
			}
		}
		if smallCount > 5 {
			candidates = append(candidates, SharedProposal{
				ID:              g.idGenerator(),
				IntersectionID:  view.IntersectionID,
				ViewID:          view.ViewID,
				Type:            ProposalTypeObservation,
				Summary:         "Spending is spread across many categories.",
				Details:         fmt.Sprintf("There are %d categories with less than 5%% of spending each.", smallCount),
				RelatedCurrency: currency,
				GeneratedAt:     now,
				ExpiresAt:       expiresAt,
				Priority:        3,
				ActionType:      ActionTypeNone,
			})
		}
	}

	// Multi-contributor observation
	if view.Provenance.ContributorCount > 1 {
		candidates = append(candidates, SharedProposal{
			ID:             g.idGenerator(),
			IntersectionID: view.IntersectionID,
			ViewID:         view.ViewID,
			Type:           ProposalTypeConversationStarter,
			Summary:        fmt.Sprintf("This view includes data from %d contributors.", view.Provenance.ContributorCount),
			Details:        "Consider discussing shared financial priorities when convenient.",
			GeneratedAt:    now,
			ExpiresAt:      expiresAt,
			Priority:       4,
			ActionType:     ActionTypeDiscuss,
		})
	}

	// Stale data observation
	if view.Provenance.DataFreshness == FreshnessStale {
		candidates = append(candidates, SharedProposal{
			ID:             g.idGenerator(),
			IntersectionID: view.IntersectionID,
			ViewID:         view.ViewID,
			Type:           ProposalTypeObservation,
			Summary:        "Some financial data may not reflect recent changes.",
			Details:        "Data will update automatically when available.",
			GeneratedAt:    now,
			ExpiresAt:      expiresAt,
			Priority:       1,
			ActionType:     ActionTypeAcknowledge,
		})
	}

	return candidates
}

// neutralCategorySummary generates neutral language for category observations.
// CRITICAL: No urgency, judgment, or optimization language.
func (g *ProposalGenerator) neutralCategorySummary(category string, percent float64) string {
	// Round percent for cleaner display
	rounded := int(percent)

	// NEUTRAL language patterns (from v8 acceptance tests):
	// - No "too much", "excessive", "overspending"
	// - No "should", "must", "need to"
	// - No "concerning", "worrying", "alarming"
	// - Use observational language only

	return fmt.Sprintf("%s represents approximately %d%% of shared spending.", category, rounded)
}

// DismissProposal records that a circle has dismissed a proposal.
// Once dismissed, the proposal is never shown to that circle again.
func (g *ProposalGenerator) DismissProposal(proposal *SharedProposal, circleID string) {
	for _, d := range proposal.DismissedBy {
		if d == circleID {
			return // Already dismissed
		}
	}
	proposal.DismissedBy = append(proposal.DismissedBy, circleID)
}

// IsDismissedBy checks if a proposal has been dismissed by a circle.
func (g *ProposalGenerator) IsDismissedBy(proposal *SharedProposal, circleID string) bool {
	for _, d := range proposal.DismissedBy {
		if d == circleID {
			return true
		}
	}
	return false
}

// ProposalLanguageGuidelines documents the language requirements.
// This is for documentation and testing - not runtime enforcement.
var ProposalLanguageGuidelines = []string{
	"No urgency words: urgent, immediately, now, asap, critical",
	"No fear words: concerning, worrying, alarming, dangerous",
	"No shame words: excessive, too much, overspending, wasteful",
	"No authority words: must, should, need to, have to, required",
	"No optimization words: optimize, maximize, improve, efficient",
	"Use observational language: represents, approximately, currently",
	"Use optional framing: consider, when convenient, if helpful",
	"Acknowledge silence: no response is a valid response",
}

// ForbiddenWords lists words that must never appear in proposals.
var ForbiddenWords = []string{
	// Urgency
	"urgent", "immediately", "now", "asap", "critical", "deadline",
	// Fear
	"concerning", "worrying", "alarming", "dangerous", "risk",
	// Shame
	"excessive", "too much", "overspending", "wasteful", "unnecessary",
	// Authority
	"must", "should", "need to", "have to", "required", "mandatory",
	// Optimization
	"optimize", "maximize", "efficient", "better", "improve",
}
