// Package shadowgate provides the candidate engine for Shadow Gating.
//
// Phase 19.5: Shadow Gating + Promotion Candidates (NO behavior change)
//
// CRITICAL INVARIANTS:
//   - Shadow does NOT affect behavior
//   - No canon thresholds/policies changed
//   - No obligation rules changed
//   - No drafts generated from shadow
//   - No execution boundaries touched
//   - Deterministic: same inputs + clock => same candidates
//   - Privacy: only abstract buckets and generic reason strings
//
// Reference: docs/ADR/ADR-0046-phase19-5-shadow-gating-and-promotion-candidates.md
package shadowgate

import (
	"time"

	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/shadowdiff"
	"quantumlife/pkg/domain/shadowgate"
	"quantumlife/pkg/domain/shadowllm"
)

// Engine computes promotion candidates from diffs and votes.
//
// CRITICAL: Does NOT affect any runtime behavior.
type Engine struct {
	clk          clock.Clock
	privacyGuard *PrivacyGuard
}

// NewEngine creates a new candidate engine.
func NewEngine(clk clock.Clock) *Engine {
	return &Engine{
		clk:          clk,
		privacyGuard: NewPrivacyGuard(),
	}
}

// DiffSource provides access to diffs for a period.
type DiffSource interface {
	ListDiffsByPeriod(periodKey string) []*shadowdiff.DiffResult
	GetVoteForDiff(diffID string) (shadowdiff.CalibrationVote, bool)
}

// ComputeInput contains inputs for candidate computation.
type ComputeInput struct {
	// PeriodKey is the time bucket (YYYY-MM-DD).
	PeriodKey string

	// CircleID is the circle to compute candidates for.
	CircleID identity.EntityID

	// DiffSource provides access to diffs and votes.
	DiffSource DiffSource
}

// ComputeOutput contains the computed candidates.
type ComputeOutput struct {
	// PeriodKey is the time bucket.
	PeriodKey string

	// Candidates are the computed promotion candidates.
	Candidates []shadowgate.Candidate

	// TotalDiffs is the number of diffs considered.
	TotalDiffs int

	// TotalVotes is the number of votes available.
	TotalVotes int
}

// Compute generates promotion candidates from diffs and votes.
//
// CRITICAL: Deterministic - same inputs produce same outputs.
// CRITICAL: Privacy-safe - only abstract buckets and generic reasons.
func (e *Engine) Compute(input ComputeInput) (*ComputeOutput, error) {
	if input.PeriodKey == "" {
		input.PeriodKey = shadowgate.PeriodKeyFromTime(e.clk.Now())
	}

	// Get diffs for the period
	diffs := input.DiffSource.ListDiffsByPeriod(input.PeriodKey)
	if len(diffs) == 0 {
		return &ComputeOutput{
			PeriodKey:  input.PeriodKey,
			Candidates: nil,
			TotalDiffs: 0,
			TotalVotes: 0,
		}, nil
	}

	// Group diffs by candidate signature
	signatures := make(map[string]*candidateSignature)
	totalVotes := 0

	for _, diff := range diffs {
		// Skip diffs that don't match the circle (if specified)
		if input.CircleID != "" && diff.CircleID != input.CircleID {
			continue
		}

		// Skip diffs without novelty (both canon and shadow agreed)
		if diff.NoveltyType == shadowdiff.NoveltyNone && diff.Agreement == shadowdiff.AgreementMatch {
			continue
		}

		sig := buildSignature(diff)
		key := sig.Key()

		existing, ok := signatures[key]
		if !ok {
			existing = sig
			signatures[key] = existing
		}

		// Update with this diff
		existing.DiffCount++
		existing.LastSeenBucket = input.PeriodKey

		// Get vote if available
		if vote, hasVote := input.DiffSource.GetVoteForDiff(diff.DiffID); hasVote {
			totalVotes++
			switch vote {
			case shadowdiff.VoteUseful:
				existing.VotesUseful++
			case shadowdiff.VoteUnnecessary:
				existing.VotesUnnecessary++
			}
		}
	}

	// Convert signatures to candidates
	now := e.clk.Now()
	candidates := make([]shadowgate.Candidate, 0, len(signatures))

	for _, sig := range signatures {
		candidate := sig.ToCandidate(input.PeriodKey, now, e.privacyGuard)
		candidates = append(candidates, candidate)
	}

	// Sort candidates in deterministic order
	shadowgate.SortCandidates(candidates)

	return &ComputeOutput{
		PeriodKey:  input.PeriodKey,
		Candidates: candidates,
		TotalDiffs: len(diffs),
		TotalVotes: totalVotes,
	}, nil
}

// candidateSignature groups related diffs into a candidate.
type candidateSignature struct {
	CircleID        identity.EntityID
	Origin          shadowgate.CandidateOrigin
	Category        shadowllm.AbstractCategory
	HorizonBucket   shadowllm.Horizon
	MagnitudeBucket shadowllm.MagnitudeBucket

	DiffCount        int
	VotesUseful      int
	VotesUnnecessary int
	FirstSeenBucket  string
	LastSeenBucket   string
}

// buildSignature creates a signature from a diff.
func buildSignature(diff *shadowdiff.DiffResult) *candidateSignature {
	sig := &candidateSignature{
		CircleID:        diff.CircleID,
		FirstSeenBucket: diff.PeriodBucket,
		LastSeenBucket:  diff.PeriodBucket,
	}

	// Determine origin from novelty and agreement
	hasConflict := diff.Agreement == shadowdiff.AgreementConflict
	sig.Origin = shadowgate.OriginFromNovelty(diff.NoveltyType, hasConflict)

	// Extract category and buckets from the signals
	if diff.ShadowSignal != nil {
		sig.Category = diff.ShadowSignal.Key.Category
		sig.HorizonBucket = diff.ShadowSignal.Horizon
		sig.MagnitudeBucket = diff.ShadowSignal.Magnitude
	} else if diff.CanonSignal != nil {
		sig.Category = diff.CanonSignal.Key.Category
		sig.HorizonBucket = diff.CanonSignal.Horizon
		sig.MagnitudeBucket = diff.CanonSignal.Magnitude
	} else {
		// Default fallback
		sig.Category = shadowllm.CategoryMoney
		sig.HorizonBucket = shadowllm.HorizonSoon
		sig.MagnitudeBucket = shadowllm.MagnitudeAFew
	}

	return sig
}

// Key returns a unique key for this signature.
func (s *candidateSignature) Key() string {
	return string(s.CircleID) + "|" +
		string(s.Origin) + "|" +
		string(s.Category) + "|" +
		string(s.HorizonBucket) + "|" +
		string(s.MagnitudeBucket)
}

// ToCandidate converts a signature to a candidate.
func (s *candidateSignature) ToCandidate(periodKey string, now time.Time, guard *PrivacyGuard) shadowgate.Candidate {
	// Compute usefulness
	totalVotes := s.VotesUseful + s.VotesUnnecessary
	usefulnessPct := 0
	if totalVotes > 0 {
		usefulnessPct = (s.VotesUseful * 100) / totalVotes
	}

	// Determine buckets
	usefulnessBucket := shadowgate.UsefulnessBucketFromPct(usefulnessPct)
	if totalVotes == 0 {
		usefulnessBucket = shadowgate.UsefulnessUnknown
		usefulnessPct = -1 // Signal unknown
	}
	voteConfidenceBucket := shadowgate.VoteConfidenceBucketFromCount(totalVotes)

	// Generate privacy-safe why
	whyGeneric := SelectReasonPhrase(string(s.Category))
	whyGeneric = guard.SanitizeWhyGeneric(whyGeneric)

	return shadowgate.Candidate{
		PeriodKey:            periodKey,
		CircleID:             s.CircleID,
		Origin:               s.Origin,
		Category:             s.Category,
		HorizonBucket:        s.HorizonBucket,
		MagnitudeBucket:      s.MagnitudeBucket,
		WhyGeneric:           whyGeneric,
		UsefulnessPct:        usefulnessPct,
		UsefulnessBucket:     usefulnessBucket,
		VoteConfidenceBucket: voteConfidenceBucket,
		VotesUseful:          s.VotesUseful,
		VotesUnnecessary:     s.VotesUnnecessary,
		FirstSeenBucket:      s.FirstSeenBucket,
		LastSeenBucket:       s.LastSeenBucket,
		CreatedAt:            now,
	}
}
