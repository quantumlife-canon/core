// Package drafts implements the draft generation orchestration.
//
// CRITICAL: This engine generates DRAFTS ONLY. NO external writes.
// CRITICAL: Deterministic. Same inputs + clock = same drafts.
package drafts

import (
	"time"

	"quantumlife/pkg/domain/draft"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/obligation"
)

// Engine orchestrates draft generation from obligations.
type Engine struct {
	generators   []draft.DraftGenerator
	store        draft.Store
	policy       draft.DraftPolicy
	quotaTracker *draft.DraftQuotaTracker
}

// NewEngine creates a new draft orchestration engine.
func NewEngine(store draft.Store, policy draft.DraftPolicy, generators ...draft.DraftGenerator) *Engine {
	return &Engine{
		generators:   generators,
		store:        store,
		policy:       policy,
		quotaTracker: draft.NewDraftQuotaTracker(policy),
	}
}

// ProcessResult contains the result of processing an obligation.
type ProcessResult struct {
	// DraftID is the ID of the generated draft (empty if not generated).
	DraftID draft.DraftID

	// Generated indicates a new draft was created.
	Generated bool

	// Deduplicated indicates an existing draft was found and returned.
	Deduplicated bool

	// Skipped indicates processing was skipped.
	Skipped bool

	// SkipReason explains why processing was skipped.
	SkipReason string

	// Error indicates a processing failure.
	Error error
}

// ProcessOptions contains optional parameters for draft processing.
type ProcessOptions struct {
	// PolicySnapshotHash binds the draft to a specific policy state.
	PolicySnapshotHash string

	// ViewSnapshotHash binds the draft to a specific view state.
	ViewSnapshotHash string
}

// Process generates a draft from an obligation.
func (e *Engine) Process(
	circleID identity.EntityID,
	intersectionID identity.EntityID,
	obl *obligation.Obligation,
	now time.Time,
) ProcessResult {
	return e.ProcessWithOptions(circleID, intersectionID, obl, now, ProcessOptions{})
}

// ProcessWithOptions generates a draft from an obligation with additional options.
func (e *Engine) ProcessWithOptions(
	circleID identity.EntityID,
	intersectionID identity.EntityID,
	obl *obligation.Obligation,
	now time.Time,
	opts ProcessOptions,
) ProcessResult {
	// Check rate limit
	if !e.quotaTracker.CanCreate(circleID, now) {
		return ProcessResult{
			Skipped:    true,
			SkipReason: "rate limit exceeded for circle",
		}
	}

	// Build generation context
	ctx := draft.GenerationContext{
		CircleID:           circleID,
		IntersectionID:     intersectionID,
		Obligation:         obl,
		Now:                now,
		Policy:             e.policy,
		PolicySnapshotHash: opts.PolicySnapshotHash,
		ViewSnapshotHash:   opts.ViewSnapshotHash,
	}

	// Find a generator that can handle this obligation
	var result draft.GenerationResult
	handled := false
	for _, gen := range e.generators {
		if gen.CanHandle(obl) {
			result = gen.Generate(ctx)
			handled = true
			break
		}
	}

	if !handled {
		return ProcessResult{
			Skipped:    true,
			SkipReason: "no generator found for obligation type",
		}
	}

	// Handle generation result
	if result.Error != nil {
		return ProcessResult{
			Error: result.Error,
		}
	}

	if result.Skipped {
		return ProcessResult{
			Skipped:    true,
			SkipReason: result.SkipReason,
		}
	}

	if result.Draft == nil {
		return ProcessResult{
			Skipped:    true,
			SkipReason: "generator returned nil draft",
		}
	}

	// Check for deduplication
	dedupKey := result.Draft.DedupKey()
	if existing, found := e.store.GetByDedupKey(dedupKey); found {
		// If existing draft is still active (proposed), return it
		if existing.Status == draft.StatusProposed {
			return ProcessResult{
				DraftID:      existing.DraftID,
				Deduplicated: true,
			}
		}
		// If existing is terminal, allow new draft (user may have changed mind)
	}

	// Store the draft
	if err := e.store.Put(*result.Draft); err != nil {
		return ProcessResult{
			Error: err,
		}
	}

	// Update quota
	e.quotaTracker.Increment(circleID, now)

	return ProcessResult{
		DraftID:   result.Draft.DraftID,
		Generated: true,
	}
}

// ProcessBatch processes multiple obligations deterministically.
func (e *Engine) ProcessBatch(
	circleID identity.EntityID,
	intersectionID identity.EntityID,
	obligations []*obligation.Obligation,
	now time.Time,
) []ProcessResult {
	// Sort obligations for determinism
	sorted := make([]*obligation.Obligation, len(obligations))
	copy(sorted, obligations)
	obligation.SortObligations(sorted)

	results := make([]ProcessResult, len(sorted))
	for i, obl := range sorted {
		results[i] = e.Process(circleID, intersectionID, obl, now)
	}

	return results
}

// MarkExpiredDrafts marks all expired drafts as expired.
func (e *Engine) MarkExpiredDrafts(now time.Time) int {
	return e.store.MarkExpired(now)
}

// GetPendingDrafts returns all pending drafts for a circle.
func (e *Engine) GetPendingDrafts(circleID identity.EntityID) []draft.Draft {
	return e.store.List(draft.ListFilter{
		CircleID: circleID,
		Status:   draft.StatusProposed,
	})
}

// GetDraft retrieves a draft by ID.
func (e *Engine) GetDraft(id draft.DraftID) (draft.Draft, bool) {
	return e.store.Get(id)
}

// Stats returns engine statistics.
type Stats struct {
	TotalDrafts    int
	PendingDrafts  int
	ApprovedDrafts int
	RejectedDrafts int
	ExpiredDrafts  int
	EmailDrafts    int
	CalendarDrafts int
	CommerceDrafts int
}

// GetStats returns current engine statistics.
func (e *Engine) GetStats() Stats {
	allDrafts := e.store.List(draft.ListFilter{IncludeExpired: true})

	stats := Stats{
		TotalDrafts: len(allDrafts),
	}

	for _, d := range allDrafts {
		switch d.Status {
		case draft.StatusProposed:
			stats.PendingDrafts++
		case draft.StatusApproved:
			stats.ApprovedDrafts++
		case draft.StatusRejected:
			stats.RejectedDrafts++
		case draft.StatusExpired:
			stats.ExpiredDrafts++
		}

		switch d.DraftType {
		case draft.DraftTypeEmailReply:
			stats.EmailDrafts++
		case draft.DraftTypeCalendarResponse:
			stats.CalendarDrafts++
		}

		// Count commerce drafts
		if draft.IsCommerceDraft(d.DraftType) {
			stats.CommerceDrafts++
		}
	}

	return stats
}
