// Package calendar implements the calendar draft generation engine.
//
// CRITICAL: This engine generates DRAFTS ONLY. NO calendar writes.
// CRITICAL: Deterministic. Same inputs + clock = same draft.
package calendar

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"quantumlife/pkg/domain/draft"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/obligation"
)

// Engine generates calendar response drafts from obligations.
type Engine struct {
	rules draft.CalendarRules
}

// NewEngine creates a new calendar draft engine.
func NewEngine(rules draft.CalendarRules) *Engine {
	return &Engine{
		rules: rules,
	}
}

// NewDefaultEngine creates an engine with default rules.
func NewDefaultEngine() *Engine {
	return NewEngine(draft.DefaultCalendarRules())
}

// CanHandle returns true if this engine handles the obligation.
func (e *Engine) CanHandle(obl *obligation.Obligation) bool {
	if obl == nil {
		return false
	}
	// Handle calendar decision obligations (invites) and attend obligations
	return (obl.Type == obligation.ObligationDecide || obl.Type == obligation.ObligationAttend) &&
		obl.SourceType == "calendar"
}

// Generate creates a calendar response draft from an obligation.
func (e *Engine) Generate(ctx draft.GenerationContext) draft.GenerationResult {
	if !e.CanHandle(ctx.Obligation) {
		return draft.GenerationResult{
			Skipped:    true,
			SkipReason: "obligation type not handled by calendar engine",
		}
	}

	// Extract calendar context from obligation evidence
	calCtx, err := e.extractCalendarContext(ctx.Obligation)
	if err != nil {
		return draft.GenerationResult{
			Error: fmt.Errorf("failed to extract calendar context: %w", err),
		}
	}

	// Determine the response (rule-based, deterministic)
	response, message := e.determineResponse(ctx.Obligation, calCtx)

	// Create the draft content
	content := draft.CalendarDraftContent{
		EventID:      calCtx.EventID,
		Response:     response,
		Message:      message,
		ProviderHint: calCtx.ProviderHint,
		CalendarID:   calCtx.CalendarID,
	}

	// Compute content hash for draft ID
	contentHash := hashContent(content.CanonicalString())

	// Compute draft ID
	draftID := draft.ComputeDraftID(
		draft.DraftTypeCalendarResponse,
		ctx.CircleID,
		ctx.Obligation.ID,
		contentHash,
	)

	// Compute expiry
	expiresAt := ctx.Policy.ComputeExpiresAt(draft.DraftTypeCalendarResponse, ctx.Now)

	// Build safety notes
	safetyNotes := e.buildSafetyNotes(content, calCtx, ctx.Obligation)

	// Assemble the draft
	d := draft.Draft{
		DraftID:            draftID,
		DraftType:          draft.DraftTypeCalendarResponse,
		CircleID:           ctx.CircleID,
		IntersectionID:     ctx.IntersectionID,
		SourceObligationID: ctx.Obligation.ID,
		SourceEventIDs:     []string{ctx.Obligation.SourceEventID},
		CreatedAt:          ctx.Now,
		ExpiresAt:          expiresAt,
		Status:             draft.StatusProposed,
		Content:            content,
		SafetyNotes:        safetyNotes,
		DeterministicHash:  hashContent(content.CanonicalString() + ctx.Now.UTC().Format(time.RFC3339)),
		GenerationRuleID:   "calendar-response-basic",
		PolicySnapshotHash: ctx.PolicySnapshotHash,
		ViewSnapshotHash:   ctx.ViewSnapshotHash,
	}

	return draft.GenerationResult{
		Draft: &d,
	}
}

// extractCalendarContext extracts calendar-specific context from obligation evidence.
func (e *Engine) extractCalendarContext(obl *obligation.Obligation) (draft.CalendarContext, error) {
	ctx := draft.CalendarContext{}

	// Event ID from source event ID
	ctx.EventID = obl.SourceEventID
	if ctx.EventID == "" {
		return ctx, fmt.Errorf("missing event ID in obligation")
	}

	// Event title from evidence
	if v, ok := obl.Evidence[obligation.EvidenceKeyEventTitle]; ok {
		ctx.EventTitle = v
	}

	// Organizer from evidence (using sender as proxy)
	if v, ok := obl.Evidence[obligation.EvidenceKeySender]; ok {
		ctx.OrganizerEmail = v
	}

	// Event times from due date (if available)
	if obl.DueBy != nil {
		ctx.EventStart = *obl.DueBy
		ctx.EventEnd = obl.DueBy.Add(time.Hour) // Default 1 hour
	}

	// Provider hint from evidence
	if v, ok := obl.Evidence["provider"]; ok {
		ctx.ProviderHint = v
	}

	// Calendar ID from evidence
	if v, ok := obl.Evidence["calendar_id"]; ok {
		ctx.CalendarID = v
	}

	return ctx, nil
}

// determineResponse determines the calendar response using rules.
func (e *Engine) determineResponse(obl *obligation.Obligation, calCtx draft.CalendarContext) (draft.CalendarResponse, string) {
	// Check for explicit response in obligation evidence
	if v, ok := obl.Evidence["suggested_response"]; ok {
		switch v {
		case "accept":
			return draft.CalendarResponseAccept, "I will attend."
		case "decline":
			return draft.CalendarResponseDecline, "I am unable to attend."
		case "tentative":
			return draft.CalendarResponseTentative, "I may be able to attend."
		}
	}

	// Check for conflicts (indicated by conflict_with evidence)
	if _, hasConflict := obl.Evidence[obligation.EvidenceKeyConflictWith]; hasConflict {
		if e.rules.AutoDeclineConflicts {
			return draft.CalendarResponseDecline, "This conflicts with another commitment."
		}
		return draft.CalendarResponseTentative, "This may conflict with another commitment. Please verify."
	}

	// Use default response
	switch e.rules.DefaultResponse {
	case draft.CalendarResponseAccept:
		return draft.CalendarResponseAccept, "I will attend."
	case draft.CalendarResponseDecline:
		return draft.CalendarResponseDecline, "I am unable to attend."
	default:
		return draft.CalendarResponseTentative, "I may be able to attend. Please confirm."
	}
}

// buildSafetyNotes generates safety notes for the draft.
func (e *Engine) buildSafetyNotes(content draft.CalendarDraftContent, calCtx draft.CalendarContext, obl *obligation.Obligation) []string {
	var notes []string

	// Check for external organizer
	if !isInternalOrganizer(calCtx.OrganizerEmail) {
		notes = append(notes, "Event from external organizer - verify authenticity")
	}

	// Check for urgency based on severity
	if obl.Severity == obligation.SeverityCritical || obl.Severity == obligation.SeverityHigh {
		notes = append(notes, "Flagged as urgent - review carefully")
	}

	// Check for all-day event
	if calCtx.EventEnd.Sub(calCtx.EventStart) >= 24*time.Hour {
		notes = append(notes, "All-day event - confirm availability for entire day")
	}

	// Check if declining
	if content.Response == draft.CalendarResponseDecline {
		notes = append(notes, "Declining invitation - organizer will be notified")
	}

	// Sort for determinism
	sort.Strings(notes)

	return notes
}

// isInternalOrganizer checks if organizer is internal (simplified).
func isInternalOrganizer(email string) bool {
	// This is a placeholder. In production, check against known domains.
	return strings.HasSuffix(email, "@internal.example.com")
}

// hashContent returns a SHA256 hash of content.
func hashContent(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// GenerateFromContext creates a draft from explicit calendar context.
// Useful for testing or when obligation metadata is not used.
func (e *Engine) GenerateFromContext(
	circleID identity.EntityID,
	intersectionID identity.EntityID,
	obligationID string,
	sourceEventIDs []string,
	calCtx draft.CalendarContext,
	response draft.CalendarResponse,
	message string,
	policy draft.DraftPolicy,
	now time.Time,
) draft.GenerationResult {

	// Create the draft content
	content := draft.CalendarDraftContent{
		EventID:      calCtx.EventID,
		Response:     response,
		Message:      message,
		ProviderHint: calCtx.ProviderHint,
		CalendarID:   calCtx.CalendarID,
	}

	// Compute content hash for draft ID
	contentHash := hashContent(content.CanonicalString())

	// Compute draft ID
	draftID := draft.ComputeDraftID(
		draft.DraftTypeCalendarResponse,
		circleID,
		obligationID,
		contentHash,
	)

	// Compute expiry
	expiresAt := policy.ComputeExpiresAt(draft.DraftTypeCalendarResponse, now)

	// Build safety notes
	var safetyNotes []string
	if !isInternalOrganizer(calCtx.OrganizerEmail) {
		safetyNotes = append(safetyNotes, "Event from external organizer - verify authenticity")
	}
	if response == draft.CalendarResponseDecline {
		safetyNotes = append(safetyNotes, "Declining invitation - organizer will be notified")
	}
	sort.Strings(safetyNotes)

	// Assemble the draft
	d := draft.Draft{
		DraftID:            draftID,
		DraftType:          draft.DraftTypeCalendarResponse,
		CircleID:           circleID,
		IntersectionID:     intersectionID,
		SourceObligationID: obligationID,
		SourceEventIDs:     sourceEventIDs,
		CreatedAt:          now,
		ExpiresAt:          expiresAt,
		Status:             draft.StatusProposed,
		Content:            content,
		SafetyNotes:        safetyNotes,
		DeterministicHash:  hashContent(content.CanonicalString() + now.UTC().Format(time.RFC3339)),
		GenerationRuleID:   "calendar-response-basic",
	}

	return draft.GenerationResult{
		Draft: &d,
	}
}

// Verify interface compliance.
var _ draft.DraftGenerator = (*Engine)(nil)
