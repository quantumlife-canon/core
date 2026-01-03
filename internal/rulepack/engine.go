// Package rulepack provides the Rule Pack Export engine.
//
// Phase 19.6: Rule Pack Export (Promotion Pipeline)
//
// CRITICAL INVARIANTS:
//   - RulePack does NOT apply itself
//   - No policy mutation
//   - No behavior change
//   - No raw identifiers in exports
//   - Deterministic: same inputs + clock => same hashes
//   - No goroutines, no time.Now()
//
// Reference: docs/ADR/ADR-0047-phase19-6-rulepack-export.md
package rulepack

import (
	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/rulepack"
	"quantumlife/pkg/domain/shadowgate"
	"quantumlife/pkg/domain/shadowllm"
)

// Engine builds RulePacks from PromotionIntents.
//
// CRITICAL: Does NOT apply packs. No behavior change.
type Engine struct {
	clk clock.Clock
}

// NewEngine creates a new RulePack engine.
func NewEngine(clk clock.Clock) *Engine {
	return &Engine{clk: clk}
}

// IntentSource provides access to promotion intents and candidates.
type IntentSource interface {
	GetPromotionIntents(periodKey string) []shadowgate.PromotionIntent
	GetCandidate(candidateID string) (*shadowgate.Candidate, bool)
}

// BuildInput contains inputs for pack building.
type BuildInput struct {
	// PeriodKey is the time bucket (YYYY-MM-DD).
	PeriodKey string

	// CircleID optionally filters to a specific circle.
	// Empty string means "all circles".
	CircleID identity.EntityID

	// IntentSource provides access to intents and candidates.
	IntentSource IntentSource
}

// BuildOutput contains the built pack.
type BuildOutput struct {
	// Pack is the built RulePack.
	Pack *rulepack.RulePack

	// TotalIntents is the number of intents considered.
	TotalIntents int

	// QualifiedIntents is the number that passed gating.
	QualifiedIntents int

	// SkippedIntents is the number that failed gating.
	SkippedIntents int
}

// Build creates a RulePack from promotion intents.
//
// CRITICAL: Deterministic - same inputs produce same outputs.
// CRITICAL: Does NOT apply the pack. No behavior change.
func (e *Engine) Build(input BuildInput) (*BuildOutput, error) {
	if input.PeriodKey == "" {
		input.PeriodKey = rulepack.PeriodKeyFromTime(e.clk.Now())
	}

	// Get promotion intents for the period
	intents := input.IntentSource.GetPromotionIntents(input.PeriodKey)

	totalIntents := len(intents)
	qualifiedIntents := 0
	skippedIntents := 0

	// Convert qualified intents to changes
	var changes []rulepack.RuleChange
	for _, intent := range intents {
		// Skip non-promote intents
		if intent.NoteCode != shadowgate.NotePromoteRule {
			skippedIntents++
			continue
		}

		// Get the candidate
		candidate, ok := input.IntentSource.GetCandidate(intent.CandidateID)
		if !ok {
			skippedIntents++
			continue
		}

		// Filter by circle if specified
		if input.CircleID != "" && candidate.CircleID != input.CircleID {
			skippedIntents++
			continue
		}

		// Apply gating criteria
		if !meetsGatingCriteria(candidate) {
			skippedIntents++
			continue
		}

		// Convert to rule change
		change := convertToRuleChange(candidate, &intent)
		changes = append(changes, change)
		qualifiedIntents++
	}

	// Sort changes deterministically
	rulepack.SortRuleChanges(changes)

	// Compute change IDs after sorting
	for i := range changes {
		changes[i].ChangeID = changes[i].ComputeID()
	}

	// Build the pack
	now := e.clk.Now()
	pack := &rulepack.RulePack{
		PeriodKey:           input.PeriodKey,
		CircleID:            input.CircleID,
		CreatedAtBucket:     rulepack.FiveMinuteBucket(now),
		ExportFormatVersion: rulepack.ExportFormatVersion,
		Changes:             changes,
		CreatedAt:           now,
	}

	// Compute ID and hash
	pack.PackID = pack.ComputeID()
	pack.PackHash = pack.ComputeHash()

	return &BuildOutput{
		Pack:             pack,
		TotalIntents:     totalIntents,
		QualifiedIntents: qualifiedIntents,
		SkippedIntents:   skippedIntents,
	}, nil
}

// meetsGatingCriteria checks if a candidate meets the minimum requirements.
//
// Gating thresholds (documented constants):
//   - Usefulness >= Medium
//   - Vote count >= 3
//   - Vote confidence >= Medium
func meetsGatingCriteria(c *shadowgate.Candidate) bool {
	// Check usefulness bucket
	if !isUsefulnessAtLeast(c.UsefulnessBucket, rulepack.MinUsefulnessBucket) {
		return false
	}

	// Check vote count
	totalVotes := c.VotesUseful + c.VotesUnnecessary
	if totalVotes < rulepack.MinVoteCount {
		return false
	}

	// Check vote confidence bucket
	if !isConfidenceAtLeast(c.VoteConfidenceBucket, rulepack.MinVoteConfidenceBucket) {
		return false
	}

	return true
}

// isUsefulnessAtLeast checks if usefulness meets minimum.
func isUsefulnessAtLeast(actual, min shadowgate.UsefulnessBucket) bool {
	order := map[shadowgate.UsefulnessBucket]int{
		shadowgate.UsefulnessUnknown: 0,
		shadowgate.UsefulnessLow:     1,
		shadowgate.UsefulnessMedium:  2,
		shadowgate.UsefulnessHigh:    3,
	}
	return order[actual] >= order[min]
}

// isConfidenceAtLeast checks if confidence meets minimum.
func isConfidenceAtLeast(actual, min shadowgate.VoteConfidenceBucket) bool {
	order := map[shadowgate.VoteConfidenceBucket]int{
		shadowgate.VoteConfidenceUnknown: 0,
		shadowgate.VoteConfidenceLow:     1,
		shadowgate.VoteConfidenceMedium:  2,
		shadowgate.VoteConfidenceHigh:    3,
	}
	return order[actual] >= order[min]
}

// convertToRuleChange converts a candidate and intent to a RuleChange.
func convertToRuleChange(c *shadowgate.Candidate, intent *shadowgate.PromotionIntent) rulepack.RuleChange {
	// Determine change kind based on origin
	changeKind := determineChangeKind(c.Origin)

	// Determine target scope based on category
	targetScope := determineTargetScope(c.Category)

	// Compute target hash (for privacy - never raw identifiers)
	targetHash := computeTargetHash(c)

	// Map novelty
	novelty := mapNovelty(c.Origin)

	// Map agreement (default to match since we don't have direct access)
	agreement := mapAgreement(c.Origin)

	return rulepack.RuleChange{
		CandidateHash:        c.Hash,
		IntentHash:           intent.IntentHash,
		CircleID:             c.CircleID,
		ChangeKind:           changeKind,
		TargetScope:          targetScope,
		TargetHash:           targetHash,
		Category:             c.Category,
		SuggestedDelta:       rulepack.DeltaFromUsefulness(c.UsefulnessBucket),
		UsefulnessBucket:     c.UsefulnessBucket,
		VoteConfidenceBucket: c.VoteConfidenceBucket,
		NoveltyBucket:        novelty,
		AgreementBucket:      agreement,
	}
}

// determineChangeKind infers the change kind from candidate origin.
func determineChangeKind(origin shadowgate.CandidateOrigin) rulepack.ChangeKind {
	switch origin {
	case shadowgate.OriginShadowOnly:
		// Shadow saw something canon didn't - suggest adjusting bias
		return rulepack.ChangeBiasAdjust
	case shadowgate.OriginCanonOnly:
		// Canon surfaced something shadow didn't - suggest threshold adjustment
		return rulepack.ChangeThresholdAdjust
	case shadowgate.OriginConflict:
		// Conflict - suggest suppression
		return rulepack.ChangeSuppressSuggest
	default:
		return rulepack.ChangeBiasAdjust
	}
}

// determineTargetScope infers the target scope from category.
func determineTargetScope(category shadowllm.AbstractCategory) rulepack.TargetScope {
	switch category {
	case shadowllm.CategoryMoney:
		return rulepack.ScopeCategory
	case shadowllm.CategoryTime:
		return rulepack.ScopeCategory
	case shadowllm.CategoryWork:
		return rulepack.ScopeCategory
	case shadowllm.CategoryPeople:
		return rulepack.ScopeCategory
	case shadowllm.CategoryHome:
		return rulepack.ScopeCategory
	default:
		return rulepack.ScopeUnknown
	}
}

// computeTargetHash computes a privacy-safe target hash.
// CRITICAL: Never contains raw identifiers.
func computeTargetHash(c *shadowgate.Candidate) string {
	// Use the first 16 chars of the candidate hash as target hash
	// This is deterministic and privacy-safe
	if len(c.Hash) >= 16 {
		return c.Hash[:16]
	}
	return c.Hash
}

// mapNovelty maps candidate origin to novelty bucket.
func mapNovelty(origin shadowgate.CandidateOrigin) rulepack.NoveltyBucket {
	switch origin {
	case shadowgate.OriginShadowOnly:
		return rulepack.NoveltyShadowOnly
	case shadowgate.OriginCanonOnly:
		return rulepack.NoveltyCanonOnly
	default:
		return rulepack.NoveltyNone
	}
}

// mapAgreement maps candidate origin to agreement bucket.
func mapAgreement(origin shadowgate.CandidateOrigin) rulepack.AgreementBucket {
	switch origin {
	case shadowgate.OriginConflict:
		return rulepack.AgreementConflict
	default:
		return rulepack.AgreementMatch
	}
}
