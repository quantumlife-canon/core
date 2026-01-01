// Package loop provides the daily loop orchestrator for QuantumLife.
//
// CRITICAL: The loop runs SYNCHRONOUSLY per request.
// CRITICAL: No background workers, no auto-retries.
// CRITICAL: Deterministic given same inputs + clock.
//
// Reference: docs/ADR/ADR-0023-phase6-quiet-loop-web.md
package loop

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"

	calexec "quantumlife/internal/calendar/execution"
	"quantumlife/internal/commerce/extract"
	"quantumlife/internal/drafts"
	"quantumlife/internal/drafts/review"
	emailexec "quantumlife/internal/email/execution"
	"quantumlife/internal/interruptions"
	"quantumlife/internal/obligations"
	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/commerce"
	"quantumlife/pkg/domain/draft"
	domainevents "quantumlife/pkg/domain/events"
	"quantumlife/pkg/domain/feedback"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/interrupt"
	"quantumlife/pkg/domain/obligation"
	"quantumlife/pkg/domain/view"
	"quantumlife/pkg/events"
)

// Engine orchestrates the daily loop.
type Engine struct {
	// Clock for deterministic time.
	Clock clock.Clock

	// IdentityRepo provides circle/identity access.
	IdentityRepo identity.Repository

	// EventStore provides domain events for obligation extraction.
	EventStore domainevents.EventStore

	// ObligationEngine extracts obligations from events.
	ObligationEngine *obligations.Engine

	// InterruptionEngine computes interruptions.
	InterruptionEngine *interruptions.Engine

	// DraftEngine generates drafts from obligations.
	DraftEngine *drafts.Engine

	// DraftStore stores generated drafts.
	DraftStore draft.Store

	// ReviewService handles draft approval/rejection.
	ReviewService *review.Service

	// CalendarExecutor executes approved calendar drafts.
	CalendarExecutor *calexec.Executor

	// EmailExecutor executes approved email drafts.
	EmailExecutor *emailexec.Executor

	// FeedbackStore stores feedback signals.
	FeedbackStore feedback.Store

	// EventEmitter emits audit events.
	EventEmitter events.Emitter

	// CommerceExtractor extracts commerce events from emails (Phase 8).
	CommerceExtractor *extract.Engine

	// CommerceObligationExtractor extracts obligations from commerce events (Phase 8).
	CommerceObligationExtractor *obligations.CommerceObligationExtractor
}

// RunOptions configures a loop run.
type RunOptions struct {
	// CircleID limits the run to a specific circle.
	CircleID identity.EntityID

	// IncludeMockData uses mock data if true.
	IncludeMockData bool

	// ExecuteApprovedDrafts executes approved calendar drafts if true.
	ExecuteApprovedDrafts bool
}

// RunResult contains the result of a loop run.
type RunResult struct {
	// RunID is the deterministic run ID.
	RunID string

	// StartedAt is when the run started.
	StartedAt time.Time

	// CompletedAt is when the run completed.
	CompletedAt time.Time

	// Circles contains results per circle.
	Circles []CircleResult

	// NeedsYou contains all items requiring attention.
	NeedsYou NeedsYouSummary

	// Errors contains any errors that occurred.
	Errors []string
}

// CircleResult contains results for a single circle.
type CircleResult struct {
	CircleID            identity.EntityID
	CircleName          string
	DailyView           *view.DailyView
	Obligations         []*obligation.Obligation
	Interruptions       []*interrupt.Interruption
	DraftsGenerated     []draft.Draft
	DraftsPending       []draft.Draft
	CalendarExecResults []calexec.ExecuteResult
	EmailExecResults    []EmailExecuteResult
	ObligationCount     int
	InterruptionCount   int
	DraftCount          int

	// Commerce (Phase 8)
	CommerceEvents            []*commerce.CommerceEvent
	CommerceObligations       []*obligation.Obligation
	CommerceEventCount        int
	CommerceObligationCount   int
	CommerceExtractionMetrics commerce.ExtractionMetrics
}

// EmailExecuteResult wraps the result of an email execution.
type EmailExecuteResult struct {
	EnvelopeID string
	Success    bool
	MessageID  string
	Error      string
}

// NeedsYouSummary contains the "needs you" state.
type NeedsYouSummary struct {
	// TotalItems is the total count of items needing attention.
	TotalItems int

	// PendingDrafts are drafts awaiting approval.
	PendingDrafts []draft.Draft

	// ActiveInterruptions are interruptions that should surface.
	ActiveInterruptions []*interrupt.Interruption

	// Hash is the deterministic hash of the needs-you state.
	Hash string

	// IsQuiet is true when nothing needs attention.
	IsQuiet bool
}

// Run executes one iteration of the daily loop.
func (e *Engine) Run(ctx context.Context, opts RunOptions) RunResult {
	now := e.Clock.Now()
	result := RunResult{
		StartedAt: now,
	}

	// Compute run ID
	result.RunID = computeRunID(now, opts)

	// Emit start event
	e.emitEvent(events.Phase6DailyRunStarted, map[string]string{
		"run_id": result.RunID,
	})

	// Get circles to process
	circles := e.getCircles(opts)

	// Process each circle
	for _, circle := range circles {
		circleResult := e.processCircle(ctx, circle, now, opts)
		result.Circles = append(result.Circles, circleResult)
	}

	// Compute needs-you summary
	result.NeedsYou = e.computeNeedsYou(result.Circles)

	// Emit needs-you computed event
	e.emitEvent(events.Phase6NeedsYouComputed, map[string]string{
		"run_id":      result.RunID,
		"total_items": fmt.Sprintf("%d", result.NeedsYou.TotalItems),
		"is_quiet":    fmt.Sprintf("%t", result.NeedsYou.IsQuiet),
		"hash":        result.NeedsYou.Hash,
	})

	result.CompletedAt = e.Clock.Now()

	// Emit completion event
	e.emitEvent(events.Phase6DailyRunCompleted, map[string]string{
		"run_id":      result.RunID,
		"duration_ms": fmt.Sprintf("%d", result.CompletedAt.Sub(result.StartedAt).Milliseconds()),
	})

	return result
}

// CircleInfo is a simplified circle representation for the loop engine.
type CircleInfo struct {
	ID   identity.EntityID
	Name string
}

// processCircle processes a single circle.
func (e *Engine) processCircle(ctx context.Context, circle CircleInfo, now time.Time, opts RunOptions) CircleResult {
	result := CircleResult{
		CircleID:   circle.ID,
		CircleName: circle.Name,
	}

	// Extract obligations for this circle
	if e.ObligationEngine != nil && e.EventStore != nil {
		extractResult := e.ObligationEngine.Extract(e.EventStore, []identity.EntityID{circle.ID})
		result.Obligations = extractResult.Obligations
		result.ObligationCount = len(extractResult.Obligations)
	}

	// Extract commerce events from emails (Phase 8)
	if e.CommerceExtractor != nil && e.EventStore != nil {
		result.CommerceEvents, result.CommerceExtractionMetrics = e.extractCommerceEvents(circle, now)
		result.CommerceEventCount = len(result.CommerceEvents)

		// Extract commerce-specific obligations
		if e.CommerceObligationExtractor != nil && len(result.CommerceEvents) > 0 {
			result.CommerceObligations = e.CommerceObligationExtractor.ExtractFromCommerceEvents(result.CommerceEvents, now)
			result.CommerceObligationCount = len(result.CommerceObligations)

			// Merge commerce obligations into main obligations list
			result.Obligations = append(result.Obligations, result.CommerceObligations...)
			result.ObligationCount = len(result.Obligations)
		}
	}

	// Build daily view using obligations
	dailyView := e.buildDailyView(circle, now, result.Obligations)
	result.DailyView = dailyView

	// Emit view computed event
	e.emitEvent(events.Phase6ViewComputed, map[string]string{
		"circle_id": string(circle.ID),
	})

	// Compute interruptions
	if e.InterruptionEngine != nil && len(result.Obligations) > 0 {
		intResult := e.InterruptionEngine.Process(dailyView, result.Obligations)
		result.Interruptions = intResult.Interruptions
		result.InterruptionCount = len(result.Interruptions)
	}

	// Generate drafts from obligations
	if e.DraftEngine != nil {
		for _, obl := range result.Obligations {
			draftResult := e.DraftEngine.Process(circle.ID, "", obl, now)
			if draftResult.Generated {
				if d, found := e.DraftStore.Get(draftResult.DraftID); found {
					result.DraftsGenerated = append(result.DraftsGenerated, d)
				}
			}
		}
		result.DraftCount = len(result.DraftsGenerated)
	}

	// Get pending drafts
	if e.DraftStore != nil {
		pending := e.DraftStore.List(draft.ListFilter{
			CircleID: circle.ID,
			Status:   draft.StatusProposed,
		})
		result.DraftsPending = pending
	}

	// Execute approved drafts if requested
	if opts.ExecuteApprovedDrafts {
		approvedDrafts := e.DraftStore.List(draft.ListFilter{
			CircleID: circle.ID,
			Status:   draft.StatusApproved,
		})

		for _, d := range approvedDrafts {
			switch d.DraftType {
			case draft.DraftTypeCalendarResponse:
				if e.CalendarExecutor != nil {
					execResult := e.executeCalendarDraft(ctx, d, now)
					result.CalendarExecResults = append(result.CalendarExecResults, execResult)
				}
			case draft.DraftTypeEmailReply:
				if e.EmailExecutor != nil {
					execResult := e.executeEmailDraft(ctx, d, now)
					result.EmailExecResults = append(result.EmailExecResults, execResult)
				}
			}
		}
	}

	return result
}

// buildDailyView builds a daily view for a circle.
func (e *Engine) buildDailyView(circle CircleInfo, now time.Time, obligs []*obligation.Obligation) *view.DailyView {
	builder := view.NewDailyViewBuilder(now, view.DefaultNeedsYouConfig())
	builder.AddCircle(circle.ID, circle.Name)
	builder.SetObligations(obligs)
	return builder.Build()
}

// extractCommerceEvents extracts commerce events from emails for a circle.
func (e *Engine) extractCommerceEvents(circle CircleInfo, now time.Time) ([]*commerce.CommerceEvent, commerce.ExtractionMetrics) {
	if e.CommerceExtractor == nil || e.EventStore == nil {
		return nil, commerce.ExtractionMetrics{}
	}

	// Get email events for this circle
	emailType := domainevents.EventTypeEmailMessage
	emailEvents, _ := e.EventStore.GetByCircle(circle.ID, &emailType, 0)

	// Convert to EmailMessageEvent pointers
	var emails []*domainevents.EmailMessageEvent
	for _, evt := range emailEvents {
		if email, ok := evt.(*domainevents.EmailMessageEvent); ok {
			emails = append(emails, email)
		}
	}

	if len(emails) == 0 {
		return nil, commerce.ExtractionMetrics{}
	}

	// Extract commerce events
	commerceEvents, metrics := e.CommerceExtractor.ExtractFromEmails(emails)

	// Set circle ID on extracted events
	for _, evt := range commerceEvents {
		evt.WithCircle(circle.ID)
	}

	return commerceEvents, metrics
}

// GetCommerceEvents returns commerce events for a circle.
func (e *Engine) GetCommerceEvents(circleID identity.EntityID) []*commerce.CommerceEvent {
	if e.CommerceExtractor == nil || e.EventStore == nil {
		return nil
	}

	now := e.Clock.Now()
	circle := CircleInfo{ID: circleID, Name: string(circleID)}
	events, _ := e.extractCommerceEvents(circle, now)
	return events
}

// executeCalendarDraft executes an approved calendar draft.
func (e *Engine) executeCalendarDraft(ctx context.Context, d draft.Draft, now time.Time) calexec.ExecuteResult {
	// Create policy and view snapshots
	policySnapshot := calexec.NewPolicySnapshot(calexec.PolicySnapshotParams{
		CircleID:             d.CircleID,
		IntersectionID:       d.IntersectionID,
		CalendarWriteEnabled: true,
		AllowedProviders:     []string{"mock", "google"},
		MaxStalenessMinutes:  15,
	}, now)

	calContent, ok := d.Content.(draft.CalendarDraftContent)
	if !ok {
		return calexec.ExecuteResult{
			Success: false,
			Error:   "invalid draft content type",
		}
	}

	viewSnapshot := calexec.NewViewSnapshot(calexec.ViewSnapshotParams{
		CircleID:               d.CircleID,
		Provider:               calContent.ProviderHint,
		CalendarID:             calContent.CalendarID,
		EventID:                calContent.EventID,
		EventETag:              "mock-etag",
		EventUpdatedAt:         now.Add(-1 * time.Hour),
		AttendeeResponseStatus: "needsAction",
	}, now)

	// Create envelope and execute
	envelope, err := calexec.NewEnvelopeFromDraft(
		d,
		policySnapshot.PolicyHash,
		viewSnapshot.ViewHash,
		viewSnapshot.CapturedAt,
		fmt.Sprintf("trace-loop-%s", d.DraftID),
		now,
	)
	if err != nil {
		return calexec.ExecuteResult{
			Success: false,
			Error:   fmt.Sprintf("failed to create envelope: %v", err),
		}
	}

	return e.CalendarExecutor.Execute(ctx, envelope)
}

// executeEmailDraft executes an approved email draft.
func (e *Engine) executeEmailDraft(ctx context.Context, d draft.Draft, now time.Time) EmailExecuteResult {
	emailContent, ok := d.Content.(draft.EmailDraftContent)
	if !ok {
		return EmailExecuteResult{
			Success: false,
			Error:   "invalid draft content type",
		}
	}

	// Create policy snapshot
	policySnapshot := emailexec.NewPolicySnapshot(emailexec.PolicySnapshotParams{
		CircleID:          d.CircleID,
		IntersectionID:    d.IntersectionID,
		EmailWriteEnabled: true,
		AllowedProviders:  []string{"mock", "google"},
		MaxSendsPerDay:    100,
		DryRunMode:        false,
	}, now)

	// Create view snapshot
	viewSnapshot := emailexec.NewViewSnapshot(emailexec.ViewSnapshotParams{
		Provider:           emailContent.ProviderHint,
		AccountID:          "mock-account",
		CircleID:           d.CircleID,
		IntersectionID:     d.IntersectionID,
		ThreadID:           emailContent.ThreadID,
		InReplyToMessageID: emailContent.InReplyToMessageID,
		MessageCount:       1,
		LastMessageAt:      now.Add(-1 * time.Hour),
	}, now)

	// Create envelope
	envelope, err := emailexec.NewEnvelopeFromDraft(
		d,
		policySnapshot.PolicyHash,
		viewSnapshot.SnapshotHash,
		viewSnapshot.CapturedAt,
		fmt.Sprintf("trace-email-%s", d.DraftID),
		now,
	)
	if err != nil {
		return EmailExecuteResult{
			Success: false,
			Error:   fmt.Sprintf("failed to create envelope: %v", err),
		}
	}

	// Execute
	result, err := e.EmailExecutor.Execute(ctx, *envelope)
	if err != nil {
		return EmailExecuteResult{
			EnvelopeID: envelope.EnvelopeID,
			Success:    false,
			Error:      err.Error(),
		}
	}

	if result.Status != emailexec.EnvelopeStatusExecuted {
		errorMsg := ""
		if result.ExecutionResult != nil {
			if result.ExecutionResult.Error != "" {
				errorMsg = result.ExecutionResult.Error
			} else if result.ExecutionResult.BlockedReason != "" {
				errorMsg = result.ExecutionResult.BlockedReason
			}
		}
		return EmailExecuteResult{
			EnvelopeID: result.EnvelopeID,
			Success:    false,
			Error:      errorMsg,
		}
	}

	messageID := ""
	if result.ExecutionResult != nil {
		messageID = result.ExecutionResult.MessageID
	}

	return EmailExecuteResult{
		EnvelopeID: result.EnvelopeID,
		Success:    true,
		MessageID:  messageID,
	}
}

// computeNeedsYou computes the needs-you summary.
func (e *Engine) computeNeedsYou(circles []CircleResult) NeedsYouSummary {
	summary := NeedsYouSummary{}

	for _, circle := range circles {
		summary.PendingDrafts = append(summary.PendingDrafts, circle.DraftsPending...)
		summary.ActiveInterruptions = append(summary.ActiveInterruptions, circle.Interruptions...)
	}

	// Sort for determinism
	sort.Slice(summary.PendingDrafts, func(i, j int) bool {
		return string(summary.PendingDrafts[i].DraftID) < string(summary.PendingDrafts[j].DraftID)
	})
	sort.Slice(summary.ActiveInterruptions, func(i, j int) bool {
		return summary.ActiveInterruptions[i].InterruptionID < summary.ActiveInterruptions[j].InterruptionID
	})

	summary.TotalItems = len(summary.PendingDrafts) + len(summary.ActiveInterruptions)
	summary.IsQuiet = summary.TotalItems == 0
	summary.Hash = computeNeedsYouHash(summary.PendingDrafts, summary.ActiveInterruptions)

	return summary
}

// getCircles returns circles to process.
func (e *Engine) getCircles(opts RunOptions) []CircleInfo {
	if e.IdentityRepo == nil {
		return nil
	}

	// Get all circle entities
	entities, err := e.IdentityRepo.GetByType(identity.EntityTypeCircle)
	if err != nil {
		return nil
	}

	var circles []CircleInfo
	for _, entity := range entities {
		circle, ok := entity.(*identity.Circle)
		if !ok {
			continue
		}

		info := CircleInfo{
			ID:   circle.ID(),
			Name: circle.Name,
		}

		// Filter by CircleID if specified
		if opts.CircleID != "" && info.ID != opts.CircleID {
			continue
		}

		circles = append(circles, info)
	}

	// Sort for determinism
	sort.Slice(circles, func(i, j int) bool {
		return circles[i].ID < circles[j].ID
	})

	return circles
}

// emitEvent emits an event.
func (e *Engine) emitEvent(eventType events.EventType, metadata map[string]string) {
	if e.EventEmitter == nil {
		return
	}
	e.EventEmitter.Emit(events.Event{
		Type:      eventType,
		Timestamp: e.Clock.Now(),
		Metadata:  metadata,
	})
}

// computeRunID computes a deterministic run ID.
func computeRunID(now time.Time, opts RunOptions) string {
	canonical := fmt.Sprintf("run|%s|%s|%t",
		now.UTC().Format(time.RFC3339Nano),
		opts.CircleID,
		opts.IncludeMockData,
	)
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:])[:16]
}

// computeNeedsYouHash computes a deterministic hash of the needs-you state.
func computeNeedsYouHash(drafts []draft.Draft, interrupts []*interrupt.Interruption) string {
	var ids []string
	for _, d := range drafts {
		ids = append(ids, string(d.DraftID))
	}
	for _, i := range interrupts {
		ids = append(ids, i.InterruptionID)
	}
	sort.Strings(ids)

	canonical := fmt.Sprintf("needsyou|%v", ids)
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:])[:16]
}

// RecordFeedback records feedback for an item.
func (e *Engine) RecordFeedback(
	targetType feedback.FeedbackTargetType,
	targetID string,
	circleID identity.EntityID,
	signal feedback.FeedbackSignal,
	reason string,
) (feedback.FeedbackRecord, error) {
	now := e.Clock.Now()

	record := feedback.NewFeedbackRecord(targetType, targetID, circleID, now, signal, reason)

	if err := e.FeedbackStore.Put(record); err != nil {
		return feedback.FeedbackRecord{}, err
	}

	// Emit event
	e.emitEvent(events.Phase6FeedbackRecorded, map[string]string{
		"feedback_id": record.FeedbackID,
		"target_type": string(targetType),
		"target_id":   targetID,
		"signal":      string(signal),
	})

	return record, nil
}

// ApproveDraft approves a draft.
func (e *Engine) ApproveDraft(draftID draft.DraftID, circleID identity.EntityID, reason string) error {
	if e.ReviewService == nil {
		return fmt.Errorf("review service not configured")
	}

	result := e.ReviewService.Approve(review.ApprovalRequest{
		ReviewRequest: review.ReviewRequest{
			DraftID:    draftID,
			CircleID:   circleID,
			ReviewerID: "loop-engine",
			Now:        e.Clock.Now(),
		},
		Reason: reason,
	})
	if result.Error != nil {
		return result.Error
	}
	return nil
}

// RejectDraft rejects a draft.
func (e *Engine) RejectDraft(draftID draft.DraftID, circleID identity.EntityID, reason string) error {
	if e.ReviewService == nil {
		return fmt.Errorf("review service not configured")
	}

	result := e.ReviewService.Reject(review.RejectionRequest{
		ReviewRequest: review.ReviewRequest{
			DraftID:    draftID,
			CircleID:   circleID,
			ReviewerID: "loop-engine",
			Now:        e.Clock.Now(),
		},
		Reason: reason,
	})
	if result.Error != nil {
		return result.Error
	}
	return nil
}

// GetDraft retrieves a draft by ID.
func (e *Engine) GetDraft(draftID draft.DraftID) (draft.Draft, bool) {
	if e.DraftStore == nil {
		return draft.Draft{}, false
	}
	return e.DraftStore.Get(draftID)
}

// GetPendingDrafts returns all pending drafts.
func (e *Engine) GetPendingDrafts() []draft.Draft {
	if e.DraftStore == nil {
		return nil
	}
	return e.DraftStore.List(draft.ListFilter{
		Status: draft.StatusProposed,
	})
}

// GetExecutionHistory returns calendar execution history.
func (e *Engine) GetExecutionHistory() []calexec.Envelope {
	if e.CalendarExecutor == nil {
		return nil
	}
	env, _ := e.CalendarExecutor.GetEnvelope("")
	// Return empty for now - would need to list all
	_ = env
	return nil
}

// GetEmailExecutionHistory returns email execution history.
func (e *Engine) GetEmailExecutionHistory() []emailexec.Envelope {
	if e.EmailExecutor == nil {
		return nil
	}
	return e.EmailExecutor.ListEnvelopes(emailexec.ListFilter{IncludeAll: true})
}
