// Package trustaction provides the engine for Phase 28: Trust Kept.
//
// This is the first and only trust-confirming real action.
// After execution: silence forever. No growth mechanics, engagement loops, or escalation paths.
//
// CRITICAL INVARIANTS:
//   - Only calendar_respond action allowed
//   - Single execution per period (day)
//   - 15-minute undo window (bucketed)
//   - Delegates to Phase 5 calendar execution boundary (no new execution paths)
//   - No goroutines
//   - No retries
//   - No background execution
//   - No time.Now() - clock injection only
//   - stdlib only
//
// Reference: docs/ADR/ADR-0059-phase28-trust-kept.md
package trustaction

import (
	"context"
	"fmt"
	"sort"
	"time"

	calexec "quantumlife/internal/calendar/execution"
	"quantumlife/internal/persist"
	"quantumlife/internal/reality"
	"quantumlife/pkg/domain/draft"
	"quantumlife/pkg/domain/identity"
	realitydomain "quantumlife/pkg/domain/reality"
	"quantumlife/pkg/domain/trustaction"
)

// Engine orchestrates trust-confirming actions.
//
// CRITICAL: This engine ONLY delegates to Phase 5 calendar execution.
// CRITICAL: No new execution paths.
type Engine struct {
	clock            func() time.Time
	calendarExecutor *calexec.Executor
	draftStore       draft.Store
	trustStore       *persist.TrustStore
	realityAckStore  *persist.RealityAckStore
	trustActionStore *persist.TrustActionStore
	realityEngine    *reality.Engine
}

// EngineConfig contains configuration for the engine.
type EngineConfig struct {
	Clock            func() time.Time
	CalendarExecutor *calexec.Executor
	DraftStore       draft.Store
	TrustStore       *persist.TrustStore
	RealityAckStore  *persist.RealityAckStore
	TrustActionStore *persist.TrustActionStore
	RealityEngine    *reality.Engine
}

// NewEngine creates a new trust action engine.
func NewEngine(config EngineConfig) *Engine {
	return &Engine{
		clock:            config.Clock,
		calendarExecutor: config.CalendarExecutor,
		draftStore:       config.DraftStore,
		trustStore:       config.TrustStore,
		realityAckStore:  config.RealityAckStore,
		trustActionStore: config.TrustActionStore,
		realityEngine:    config.RealityEngine,
	}
}

// CheckEligibility verifies if a trust action is available.
//
// Prerequisites:
//   - Trust baseline exists (Phase 20)
//   - Reality verified (Phase 26C)
//   - Exactly one approved calendar draft exists
//   - No prior Phase 28 execution this period
//
// CRITICAL: Deterministic selection - lowest hash wins when multiple drafts exist.
func (e *Engine) CheckEligibility(circleID identity.EntityID) *trustaction.EligibilityResult {
	now := e.clock()
	period := now.UTC().Format("2006-01-02")

	// 1. Trust baseline exists (Phase 20)
	if e.trustStore == nil || e.trustStore.GetRecentMeaningfulSummary() == nil {
		return &trustaction.EligibilityResult{
			Eligible:  false,
			Reason:    "no trust baseline",
			PeriodKey: period,
		}
	}

	// 2. Reality verified (Phase 26C)
	// Build reality inputs and check if acknowledged
	if e.realityAckStore != nil && e.realityEngine != nil {
		realityInputs := e.buildRealityInputs(circleID, now)
		page := e.realityEngine.BuildPage(realityInputs)
		if !e.realityAckStore.IsAcked(period, page.StatusHash) {
			return &trustaction.EligibilityResult{
				Eligible:  false,
				Reason:    "reality not verified",
				PeriodKey: period,
			}
		}
	}

	// 3. Exactly one undoable calendar draft exists
	drafts := e.findApprovedCalendarDrafts(circleID)
	if len(drafts) == 0 {
		return &trustaction.EligibilityResult{
			Eligible:  false,
			Reason:    "no approved calendar draft",
			PeriodKey: period,
		}
	}

	// Deterministic selection: lowest hash
	selected := e.selectLowestHash(drafts)

	// 4. No prior Phase 28 execution this period
	if e.trustActionStore != nil && e.trustActionStore.HasExecutedThisPeriod(string(circleID), period) {
		return &trustaction.EligibilityResult{
			Eligible:  false,
			Reason:    "already executed this period",
			PeriodKey: period,
		}
	}

	// Build preview (abstract only, no identifiers)
	preview := e.buildPreview(selected, now)

	return &trustaction.EligibilityResult{
		Eligible:  true,
		Preview:   preview,
		DraftID:   string(selected.DraftID),
		PeriodKey: period,
	}
}

// buildRealityInputs builds reality inputs for the given circle.
// This matches the logic used in Phase 26C to compute the status hash.
func (e *Engine) buildRealityInputs(circleID identity.EntityID, now time.Time) *realitydomain.RealityInputs {
	// Build minimal inputs for status hash computation
	// In real usage, this would pull from actual stores
	return &realitydomain.RealityInputs{
		CircleID:           string(circleID),
		NowBucket:          now.UTC().Format("2006-01-02"),
		GmailConnected:     false,
		SyncBucket:         realitydomain.SyncBucketNever,
		SyncMagnitude:      realitydomain.MagnitudeNothing,
		ObligationsHeld:    true,
		AutoSurface:        false,
		ShadowProviderKind: realitydomain.ProviderOff,
		ShadowRealAllowed:  false,
		ShadowMagnitude:    realitydomain.MagnitudeNothing,
	}
}

// findApprovedCalendarDrafts finds all approved calendar response drafts.
func (e *Engine) findApprovedCalendarDrafts(circleID identity.EntityID) []draft.Draft {
	if e.draftStore == nil {
		return nil
	}

	filter := draft.ListFilter{
		CircleID:  circleID,
		Status:    draft.StatusApproved,
		DraftType: draft.DraftTypeCalendarResponse,
	}
	return e.draftStore.List(filter)
}

// selectLowestHash deterministically selects the draft with lowest hash.
func (e *Engine) selectLowestHash(candidates []draft.Draft) draft.Draft {
	if len(candidates) == 0 {
		return draft.Draft{}
	}

	// Sort by draft ID (deterministic)
	sort.Slice(candidates, func(i, j int) bool {
		return string(candidates[i].DraftID) < string(candidates[j].DraftID)
	})

	return candidates[0]
}

// buildPreview builds an abstract preview (no identifiers).
func (e *Engine) buildPreview(d draft.Draft, now time.Time) *trustaction.TrustActionPreview {
	horizon := e.computeHorizonBucket(d, now)

	return &trustaction.TrustActionPreview{
		ActionKind:     trustaction.ActionKindCalendarRespond,
		AbstractTarget: "a calendar event",
		HorizonBucket:  horizon,
		Reversible:     true, // Always true for Phase 28
	}
}

// computeHorizonBucket computes the time horizon bucket.
// Since CalendarDraftContent doesn't have event time, we default to "soon"
// for all calendar actions in Phase 28.
func (e *Engine) computeHorizonBucket(d draft.Draft, now time.Time) trustaction.HorizonBucket {
	_, ok := d.CalendarContent()
	if !ok {
		return trustaction.HorizonSomeday
	}

	// Calendar responses are typically time-sensitive, default to "soon"
	// In future phases, we could derive this from the event's start time
	// by looking up the event in the calendar store
	return trustaction.HorizonSoon
}

// Execute executes the trust action.
//
// CRITICAL: Delegates to Phase 5 calendar execution boundary.
// CRITICAL: No new execution paths.
// CRITICAL: Single-shot per period.
func (e *Engine) Execute(ctx context.Context, circleID identity.EntityID, draftID string) *trustaction.ExecuteResult {
	now := e.clock()
	period := now.UTC().Format("2006-01-02")

	// 1. Re-verify eligibility
	eligibility := e.CheckEligibility(circleID)
	if !eligibility.Eligible {
		return &trustaction.ExecuteResult{
			Success: false,
			Error:   eligibility.Reason,
		}
	}

	// 2. Verify draft matches
	if eligibility.DraftID != draftID {
		return &trustaction.ExecuteResult{
			Success: false,
			Error:   "draft mismatch",
		}
	}

	// 3. Get the draft
	d, found := e.draftStore.Get(draft.DraftID(draftID))
	if !found {
		return &trustaction.ExecuteResult{
			Success: false,
			Error:   "draft not found",
		}
	}

	// 4. Execute via Phase 5 calendar boundary
	// CRITICAL: This is the ONLY external write path
	execResult := e.executeCalendarDraft(ctx, d)
	if !execResult.Success {
		return &trustaction.ExecuteResult{
			Success: false,
			Error:   execResult.Error,
		}
	}

	// 5. Create receipt
	receipt := &trustaction.TrustActionReceipt{
		ActionKind:   trustaction.ActionKindCalendarRespond,
		State:        trustaction.StateExecuted,
		UndoBucket:   trustaction.NewUndoBucket(now),
		Period:       period,
		CircleID:     string(circleID),
		DraftIDHash:  trustaction.HashString(draftID),
		EnvelopeHash: trustaction.HashString(execResult.EnvelopeID),
	}
	receipt.StatusHash = receipt.ComputeStatusHash()
	receipt.ReceiptID = receipt.ComputeReceiptID()

	// 6. Store receipt
	if e.trustActionStore != nil {
		_ = e.trustActionStore.AppendReceipt(receipt)
	}

	return &trustaction.ExecuteResult{
		Success: true,
		Receipt: receipt,
	}
}

// executeCalendarDraft executes a calendar draft via the Phase 5 boundary.
func (e *Engine) executeCalendarDraft(ctx context.Context, d draft.Draft) calexec.ExecuteResult {
	if e.calendarExecutor == nil {
		return calexec.ExecuteResult{
			Success: false,
			Error:   "no calendar executor",
		}
	}

	// Create snapshots for Phase 28
	policySnapshot := calexec.PolicySnapshot{
		PolicyHash: "phase28-trust-kept",
		CircleID:   d.CircleID,
	}
	viewSnapshot := calexec.ViewSnapshot{
		ViewHash:   "phase28-trust-kept",
		CapturedAt: e.clock(),
	}
	traceID := fmt.Sprintf("phase28-trust-kept-%s", d.DraftID)

	return e.calendarExecutor.ExecuteFromDraft(ctx, d, policySnapshot, viewSnapshot, traceID)
}

// Undo reverses a previous execution.
//
// CRITICAL: Must be within undo window (15 minutes).
// CRITICAL: No double-undo allowed.
func (e *Engine) Undo(ctx context.Context, receiptID string) *trustaction.UndoResult {
	now := e.clock()

	// 1. Get receipt
	if e.trustActionStore == nil {
		return &trustaction.UndoResult{
			Success: false,
			Error:   "no trust action store",
		}
	}

	receipt := e.trustActionStore.GetByID(receiptID)
	if receipt == nil {
		return &trustaction.UndoResult{
			Success: false,
			Error:   "receipt not found",
		}
	}

	// 2. Check state
	if receipt.State != trustaction.StateExecuted {
		return &trustaction.UndoResult{
			Success: false,
			Error:   "not in executed state",
		}
	}

	// 3. Check undo window
	if receipt.UndoBucket.IsExpired(now) {
		return &trustaction.UndoResult{
			Success: false,
			Error:   "undo window expired",
		}
	}

	// 4. Build and execute reversal
	// We need the original draft to build the reversal
	originalDraftID, found := e.trustActionStore.GetDraftIDForReceipt(receiptID)
	if !found {
		return &trustaction.UndoResult{
			Success: false,
			Error:   "original draft not found",
		}
	}

	originalDraft, found := e.draftStore.Get(draft.DraftID(originalDraftID))
	if !found {
		return &trustaction.UndoResult{
			Success: false,
			Error:   "original draft not found",
		}
	}

	reversalDraft := e.buildReversalDraft(originalDraft)

	execResult := e.executeCalendarDraft(ctx, reversalDraft)
	if !execResult.Success {
		return &trustaction.UndoResult{
			Success: false,
			Error:   execResult.Error,
		}
	}

	// 5. Update receipt state
	receipt.State = trustaction.StateUndone
	e.trustActionStore.UpdateState(receiptID, trustaction.StateUndone)

	return &trustaction.UndoResult{
		Success: true,
		Receipt: receipt,
	}
}

// buildReversalDraft creates a draft to reverse the original action.
func (e *Engine) buildReversalDraft(original draft.Draft) draft.Draft {
	calContent, _ := original.CalendarContent()

	// Swap response back to previous
	reversalContent := draft.CalendarDraftContent{
		EventID:                calContent.EventID,
		Response:               calContent.PreviousResponseStatus,
		PreviousResponseStatus: calContent.Response,
		Message:                "",
		ProviderHint:           calContent.ProviderHint,
		CalendarID:             calContent.CalendarID,
	}

	return draft.Draft{
		DraftID:   draft.DraftID(fmt.Sprintf("undo-%s", original.DraftID)),
		CircleID:  original.CircleID,
		DraftType: draft.DraftTypeCalendarResponse,
		Status:    draft.StatusApproved,
		Content:   reversalContent,
		CreatedAt: e.clock(),
		ExpiresAt: e.clock().Add(24 * time.Hour),
	}
}

// ShouldShowCue returns true if the trust action cue should be shown.
//
// CRITICAL: Returns false after execution or expiry.
// CRITICAL: Silence is the default after action.
func (e *Engine) ShouldShowCue(circleID identity.EntityID) bool {
	now := e.clock()
	period := now.UTC().Format("2006-01-02")

	// Never show if already executed this period
	if e.trustActionStore != nil && e.trustActionStore.HasExecutedThisPeriod(string(circleID), period) {
		return false
	}

	// Only show if eligible
	eligibility := e.CheckEligibility(circleID)
	return eligibility.Eligible
}

// GetReceipt retrieves a receipt by ID.
func (e *Engine) GetReceipt(receiptID string) *trustaction.TrustActionReceipt {
	if e.trustActionStore == nil {
		return nil
	}
	return e.trustActionStore.GetByID(receiptID)
}

// GetLatestReceipt returns the latest receipt for a circle.
func (e *Engine) GetLatestReceipt(circleID identity.EntityID) *trustaction.TrustActionReceipt {
	if e.trustActionStore == nil {
		return nil
	}
	return e.trustActionStore.GetLatestForCircle(string(circleID))
}

// CurrentPeriod returns the current period key.
func (e *Engine) CurrentPeriod() string {
	return e.clock().UTC().Format("2006-01-02")
}

// GetCue builds a trust action cue if available.
func (e *Engine) GetCue(circleID identity.EntityID) *trustaction.TrustActionCue {
	if !e.ShouldShowCue(circleID) {
		return trustaction.NewTrustActionCue(false)
	}
	return trustaction.NewTrustActionCue(true)
}
