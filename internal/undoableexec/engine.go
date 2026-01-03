// Package undoableexec provides the undoable execution engine.
//
// Phase 25: First Undoable Execution (Opt-In, Single-Shot)
//
// This engine orchestrates the first real external write that is undoable.
// Phase 25 ONLY supports calendar_respond because:
//   - Email send is not truly undoable
//   - Finance is not undoable
//   - Calendar RSVP can be reversed by applying previous response
//
// CRITICAL INVARIANTS:
//   - Only calendar execution boundary is called
//   - Single-shot per period (max one execution)
//   - Undo window is bounded (bucketed time)
//   - Undo is a first-class flow, not "best effort"
//   - No goroutines
//   - No retries
//   - No background execution
//   - No time.Now() - clock injection only.
//   - Stdlib only.
//
// Reference: docs/ADR/ADR-0055-phase25-first-undoable-execution.md
package undoableexec

import (
	"context"
	"fmt"
	"sort"
	"time"

	calexec "quantumlife/internal/calendar/execution"
	"quantumlife/internal/persist"
	"quantumlife/pkg/domain/draft"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/undoableexec"
)

// Engine orchestrates undoable execution.
type Engine struct {
	clock            func() time.Time
	calendarExecutor *calexec.Executor
	draftStore       draft.Store
	undoStore        *persist.UndoableExecStore
}

// EngineConfig contains configuration for the engine.
type EngineConfig struct {
	Clock            func() time.Time
	CalendarExecutor *calexec.Executor
	DraftStore       draft.Store
	UndoStore        *persist.UndoableExecStore
}

// NewEngine creates a new undoable execution engine.
func NewEngine(config EngineConfig) *Engine {
	return &Engine{
		clock:            config.Clock,
		calendarExecutor: config.CalendarExecutor,
		draftStore:       config.DraftStore,
		undoStore:        config.UndoStore,
	}
}

// EligibleAction checks if there's an eligible undoable action.
// Returns the eligibility status and selected draft (if eligible).
//
// CRITICAL: Selection is deterministic - lowest hash wins.
func (e *Engine) EligibleAction(ctx context.Context, circleID identity.EntityID) *undoableexec.ActionEligibility {
	now := e.clock()
	periodKey := undoableexec.PeriodKeyFromTime(now)

	// Check if already executed this period
	if e.undoStore != nil && e.undoStore.HasExecutedThisPeriod(circleID, periodKey) {
		return &undoableexec.ActionEligibility{
			Eligible:  false,
			Reason:    "already executed this period",
			PeriodKey: periodKey,
			CircleID:  string(circleID),
		}
	}

	// Find approved calendar response drafts
	candidates := e.findApprovedCalendarDrafts(circleID)
	if len(candidates) == 0 {
		return &undoableexec.ActionEligibility{
			Eligible:  false,
			Reason:    "no approved calendar drafts",
			PeriodKey: periodKey,
			CircleID:  string(circleID),
		}
	}

	// Deterministically select the candidate with lowest hash
	selected := e.selectLowestHash(candidates)

	return &undoableexec.ActionEligibility{
		Eligible:   true,
		Reason:     "eligible",
		DraftID:    string(selected.DraftID),
		ActionKind: undoableexec.ActionKindCalendarRespond,
		PeriodKey:  periodKey,
		CircleID:   string(circleID),
	}
}

// findApprovedCalendarDrafts finds all approved calendar response drafts.
func (e *Engine) findApprovedCalendarDrafts(circleID identity.EntityID) []draft.Draft {
	if e.draftStore == nil {
		return nil
	}

	// Use filter to get approved calendar response drafts for this circle
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

// RunOnceResult contains the result of running an undoable action.
type RunOnceResult struct {
	// Success indicates the execution succeeded.
	Success bool

	// UndoRecord is the undo record created.
	UndoRecord *undoableexec.UndoRecord

	// ExecResult contains execution details.
	ExecResult *calexec.ExecuteResult

	// Error contains error details if failed.
	Error string
}

// RunOnce executes a single undoable action.
//
// CRITICAL: Single-shot, no retries, creates undo record.
func (e *Engine) RunOnce(ctx context.Context, circleID identity.EntityID, draftID string) *RunOnceResult {
	now := e.clock()
	periodKey := undoableexec.PeriodKeyFromTime(now)

	// Verify eligibility
	eligibility := e.EligibleAction(ctx, circleID)
	if !eligibility.Eligible {
		return &RunOnceResult{
			Success: false,
			Error:   eligibility.Reason,
		}
	}

	// Verify draft matches
	if eligibility.DraftID != draftID {
		return &RunOnceResult{
			Success: false,
			Error:   "draft mismatch",
		}
	}

	// Get the draft
	d, found := e.draftStore.Get(draft.DraftID(draftID))
	if !found {
		return &RunOnceResult{
			Success: false,
			Error:   "draft not found",
		}
	}

	// Get calendar content
	calContent, ok := d.CalendarContent()
	if !ok {
		return &RunOnceResult{
			Success: false,
			Error:   "invalid calendar content",
		}
	}

	// Get before/after status
	beforeStatus := calendarResponseToStatus(calContent.GetPreviousResponseStatus())
	afterStatus := calendarResponseToStatus(calContent.Response)

	// Execute via calendar execution boundary
	// CRITICAL: This is the ONLY external write path
	execResult := e.executeCalendarDraft(ctx, d)
	if !execResult.Success {
		return &RunOnceResult{
			Success:    false,
			ExecResult: &execResult,
			Error:      execResult.Error,
		}
	}

	// Create undo record
	undoRecord := undoableexec.NewUndoRecord(
		periodKey,
		string(circleID),
		undoableexec.ActionKindCalendarRespond,
		draftID,
		execResult.EnvelopeID,
		beforeStatus,
		afterStatus,
		now,
	)

	// Store undo record
	if e.undoStore != nil {
		_ = e.undoStore.AppendRecord(undoRecord)
	}

	return &RunOnceResult{
		Success:    true,
		UndoRecord: undoRecord,
		ExecResult: &execResult,
	}
}

// executeCalendarDraft executes a calendar draft via the calendar boundary.
func (e *Engine) executeCalendarDraft(ctx context.Context, d draft.Draft) calexec.ExecuteResult {
	if e.calendarExecutor == nil {
		return calexec.ExecuteResult{
			Success: false,
			Error:   "no calendar executor",
		}
	}

	// Create envelope from draft
	// Use empty policy/view snapshots for Phase 25 (simplified)
	policySnapshot := calexec.PolicySnapshot{
		PolicyHash: "phase25-undoable",
		CircleID:   d.CircleID,
	}
	viewSnapshot := calexec.ViewSnapshot{
		ViewHash:   "phase25-undoable",
		CapturedAt: e.clock(),
	}
	traceID := fmt.Sprintf("phase25-undoable-%s", d.DraftID)

	return e.calendarExecutor.ExecuteFromDraft(ctx, d, policySnapshot, viewSnapshot, traceID)
}

// UndoResult contains the result of an undo operation.
type UndoResult struct {
	// Success indicates the undo succeeded.
	Success bool

	// Ack is the acknowledgement record.
	Ack *undoableexec.UndoAck

	// ExecResult contains execution details.
	ExecResult *calexec.ExecuteResult

	// Error contains error details if failed.
	Error string
}

// Undo reverses a previous execution.
//
// CRITICAL: Must be within undo window. No double-undo allowed.
func (e *Engine) Undo(ctx context.Context, undoRecordID string) *UndoResult {
	now := e.clock()

	// Get the undo record
	if e.undoStore == nil {
		return &UndoResult{
			Success: false,
			Error:   "no undo store",
		}
	}

	record, found := e.undoStore.GetByID(undoRecordID)
	if !found {
		return &UndoResult{
			Success: false,
			Error:   "undo record not found",
		}
	}

	// Check if undo is still available
	if !record.IsUndoAvailable(now) {
		return &UndoResult{
			Success: false,
			Error:   "undo window expired",
		}
	}

	// Get the original draft to build reversal
	originalDraft, found := e.draftStore.Get(draft.DraftID(record.DraftID))
	if !found {
		return &UndoResult{
			Success: false,
			Error:   "original draft not found",
		}
	}

	// Build reversal draft (swap before/after status)
	reversalDraft := e.buildReversalDraft(originalDraft, record.BeforeStatus)

	// Execute the reversal via calendar boundary
	execResult := e.executeCalendarDraft(ctx, reversalDraft)
	if !execResult.Success {
		return &UndoResult{
			Success:    false,
			ExecResult: &execResult,
			Error:      execResult.Error,
		}
	}

	// Create and store ack
	ack := undoableexec.NewUndoAck(undoRecordID, undoableexec.StateUndone, now, "undo requested")
	_ = e.undoStore.AppendAck(ack)

	return &UndoResult{
		Success:    true,
		Ack:        ack,
		ExecResult: &execResult,
	}
}

// buildReversalDraft creates a draft to reverse the original action.
func (e *Engine) buildReversalDraft(original draft.Draft, targetStatus undoableexec.ResponseStatus) draft.Draft {
	calContent, _ := original.CalendarContent()

	reversalContent := draft.CalendarDraftContent{
		EventID:                calContent.EventID,
		Response:               statusToCalendarResponse(targetStatus),
		PreviousResponseStatus: calContent.Response, // Previous is now the executed response
		Message:                "",                  // No message for undo
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

// GetUndoRecord retrieves an undo record by ID.
func (e *Engine) GetUndoRecord(id string) (*undoableexec.UndoRecord, bool) {
	if e.undoStore == nil {
		return nil, false
	}
	return e.undoStore.GetByID(id)
}

// GetLatestUndoable returns the latest undoable record for a circle.
func (e *Engine) GetLatestUndoable(circleID identity.EntityID) *undoableexec.UndoRecord {
	if e.undoStore == nil {
		return nil
	}
	return e.undoStore.GetLatestUndoable(circleID)
}

// HasExecutedThisPeriod checks if any execution occurred this period.
func (e *Engine) HasExecutedThisPeriod(circleID identity.EntityID) bool {
	now := e.clock()
	periodKey := undoableexec.PeriodKeyFromTime(now)
	if e.undoStore == nil {
		return false
	}
	return e.undoStore.HasExecutedThisPeriod(circleID, periodKey)
}

// calendarResponseToStatus converts CalendarResponse to ResponseStatus.
func calendarResponseToStatus(r draft.CalendarResponse) undoableexec.ResponseStatus {
	switch r {
	case draft.CalendarResponseAccept:
		return undoableexec.StatusAccepted
	case draft.CalendarResponseDecline:
		return undoableexec.StatusDeclined
	case draft.CalendarResponseTentative:
		return undoableexec.StatusTentative
	case "needs_action":
		return undoableexec.StatusNeedsAction
	default:
		return undoableexec.StatusNeedsAction
	}
}

// statusToCalendarResponse converts ResponseStatus to CalendarResponse.
func statusToCalendarResponse(s undoableexec.ResponseStatus) draft.CalendarResponse {
	switch s {
	case undoableexec.StatusAccepted:
		return draft.CalendarResponseAccept
	case undoableexec.StatusDeclined:
		return draft.CalendarResponseDecline
	case undoableexec.StatusTentative:
		return draft.CalendarResponseTentative
	case undoableexec.StatusNeedsAction:
		return "needs_action"
	default:
		return "needs_action"
	}
}

// CurrentPeriod returns the current period key.
func (e *Engine) CurrentPeriod() string {
	return undoableexec.PeriodKeyFromTime(e.clock())
}
