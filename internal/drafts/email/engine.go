// Package email implements the email draft generation engine.
//
// CRITICAL: This engine generates DRAFTS ONLY. NO email sending.
// CRITICAL: Deterministic. Same inputs + clock = same draft.
package email

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

// Engine generates email reply drafts from obligations.
type Engine struct {
	rules draft.ReplyRules
}

// NewEngine creates a new email draft engine.
func NewEngine(rules draft.ReplyRules) *Engine {
	return &Engine{
		rules: rules,
	}
}

// NewDefaultEngine creates an engine with default rules.
func NewDefaultEngine() *Engine {
	return NewEngine(draft.DefaultReplyRules())
}

// CanHandle returns true if this engine handles the obligation.
func (e *Engine) CanHandle(obl *obligation.Obligation) bool {
	if obl == nil {
		return false
	}
	// Handle email reply obligations
	return obl.Type == obligation.ObligationReply && obl.SourceType == "email"
}

// Generate creates an email reply draft from an obligation.
func (e *Engine) Generate(ctx draft.GenerationContext) draft.GenerationResult {
	if !e.CanHandle(ctx.Obligation) {
		return draft.GenerationResult{
			Skipped:    true,
			SkipReason: "obligation type not handled by email engine",
		}
	}

	// Extract email context from obligation evidence
	emailCtx, err := e.extractEmailContext(ctx.Obligation)
	if err != nil {
		return draft.GenerationResult{
			Error: fmt.Errorf("failed to extract email context: %w", err),
		}
	}

	// Generate the reply body (rule-based, deterministic)
	replyBody := e.generateReplyBody(ctx.Obligation, emailCtx)
	if len(replyBody) < e.rules.MinimumBodyLength {
		return draft.GenerationResult{
			Skipped:    true,
			SkipReason: fmt.Sprintf("reply body too short (%d < %d)", len(replyBody), e.rules.MinimumBodyLength),
		}
	}

	// Build subject line
	subject := e.buildSubject(emailCtx.OriginalSubject)

	// Create the draft content
	content := draft.EmailDraftContent{
		To:                 emailCtx.OriginalFrom,
		Cc:                 []string{}, // Optionally derive from original
		Subject:            subject,
		Body:               replyBody,
		ThreadID:           emailCtx.ThreadID,
		ProviderHint:       emailCtx.ProviderHint,
		InReplyToMessageID: emailCtx.InReplyToMessageID,
	}

	// Compute content hash for draft ID
	contentHash := hashContent(content.CanonicalString())

	// Compute draft ID
	draftID := draft.ComputeDraftID(
		draft.DraftTypeEmailReply,
		ctx.CircleID,
		ctx.Obligation.ID,
		contentHash,
	)

	// Compute expiry
	expiresAt := ctx.Policy.ComputeExpiresAt(draft.DraftTypeEmailReply, ctx.Now)

	// Build safety notes
	safetyNotes := e.buildSafetyNotes(content, ctx.Obligation)

	// Assemble the draft
	d := draft.Draft{
		DraftID:            draftID,
		DraftType:          draft.DraftTypeEmailReply,
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
		GenerationRuleID:   "email-reply-basic",
	}

	return draft.GenerationResult{
		Draft: &d,
	}
}

// extractEmailContext extracts email-specific context from obligation evidence.
func (e *Engine) extractEmailContext(obl *obligation.Obligation) (draft.EmailContext, error) {
	ctx := draft.EmailContext{}

	// Thread ID from source event ID (email thread)
	ctx.ThreadID = obl.SourceEventID

	// Message ID for reply
	if v, ok := obl.Evidence["message_id"]; ok {
		ctx.InReplyToMessageID = v
	}

	// Original sender (we reply to this person) - use the sender evidence
	if v, ok := obl.Evidence[obligation.EvidenceKeySender]; ok {
		ctx.OriginalFrom = v
	}
	if ctx.OriginalFrom == "" {
		return ctx, fmt.Errorf("missing sender in obligation evidence")
	}

	// Subject from evidence
	if v, ok := obl.Evidence[obligation.EvidenceKeySubject]; ok {
		ctx.OriginalSubject = v
	}

	// Provider hint from source type or evidence
	if v, ok := obl.Evidence["provider"]; ok {
		ctx.ProviderHint = v
	}

	return ctx, nil
}

// generateReplyBody generates a deterministic reply body.
// This is rule-based. LLM integration would be a hook here.
func (e *Engine) generateReplyBody(obl *obligation.Obligation, emailCtx draft.EmailContext) string {
	// Rule-based response generation
	// In production, this could invoke an LLM with deterministic sampling

	var body strings.Builder

	// Acknowledge the email
	body.WriteString("Thank you for your email")

	// If there's a subject, reference it
	if emailCtx.OriginalSubject != "" {
		body.WriteString(fmt.Sprintf(" regarding \"%s\"", emailCtx.OriginalSubject))
	}
	body.WriteString(".\n\n")

	// Add content based on obligation reason
	if obl.Reason != "" {
		body.WriteString(obl.Reason)
		body.WriteString("\n\n")
	}

	// Add signature if configured
	if e.rules.DefaultSignature != "" {
		body.WriteString(e.rules.DefaultSignature)
	}

	return body.String()
}

// buildSubject builds the reply subject line.
func (e *Engine) buildSubject(originalSubject string) string {
	if !e.rules.IncludeSubjectPrefix {
		return originalSubject
	}

	// Already has Re: prefix
	if strings.HasPrefix(strings.ToLower(originalSubject), "re:") {
		return originalSubject
	}

	return "Re: " + originalSubject
}

// buildSafetyNotes generates safety notes for the draft.
func (e *Engine) buildSafetyNotes(content draft.EmailDraftContent, obl *obligation.Obligation) []string {
	var notes []string

	// Check for external recipients
	if !isInternalEmail(content.To) {
		notes = append(notes, "Recipient is external - verify before sending")
	}

	// Check for attachments mentioned
	if strings.Contains(strings.ToLower(content.Body), "attach") {
		notes = append(notes, "Email mentions attachments - ensure files are included")
	}

	// Check for urgency signals based on severity
	if obl.Severity == obligation.SeverityCritical || obl.Severity == obligation.SeverityHigh {
		notes = append(notes, "Flagged as urgent - review carefully")
	}

	// Sort for determinism
	sort.Strings(notes)

	return notes
}

// isInternalEmail checks if an email is internal (simplified).
func isInternalEmail(email string) bool {
	// This is a placeholder. In production, check against known domains.
	return strings.HasSuffix(email, "@internal.example.com")
}

// hashContent returns a SHA256 hash of content.
func hashContent(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// GenerateFromContext creates a draft from explicit email context.
// Useful for testing or when obligation metadata is not used.
func (e *Engine) GenerateFromContext(
	circleID identity.EntityID,
	intersectionID identity.EntityID,
	obligationID string,
	sourceEventIDs []string,
	emailCtx draft.EmailContext,
	policy draft.DraftPolicy,
	now time.Time,
) draft.GenerationResult {

	// Generate the reply body
	replyBody := fmt.Sprintf("Thank you for your email regarding \"%s\".\n\n", emailCtx.OriginalSubject)
	if len(replyBody) < e.rules.MinimumBodyLength {
		return draft.GenerationResult{
			Skipped:    true,
			SkipReason: "reply body too short",
		}
	}

	// Build subject line
	subject := e.buildSubject(emailCtx.OriginalSubject)

	// Create the draft content
	content := draft.EmailDraftContent{
		To:                 emailCtx.OriginalFrom,
		Cc:                 []string{},
		Subject:            subject,
		Body:               replyBody,
		ThreadID:           emailCtx.ThreadID,
		ProviderHint:       emailCtx.ProviderHint,
		InReplyToMessageID: emailCtx.InReplyToMessageID,
	}

	// Compute content hash for draft ID
	contentHash := hashContent(content.CanonicalString())

	// Compute draft ID
	draftID := draft.ComputeDraftID(
		draft.DraftTypeEmailReply,
		circleID,
		obligationID,
		contentHash,
	)

	// Compute expiry
	expiresAt := policy.ComputeExpiresAt(draft.DraftTypeEmailReply, now)

	// Build safety notes
	var safetyNotes []string
	if !isInternalEmail(content.To) {
		safetyNotes = append(safetyNotes, "Recipient is external - verify before sending")
	}

	// Assemble the draft
	d := draft.Draft{
		DraftID:            draftID,
		DraftType:          draft.DraftTypeEmailReply,
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
		GenerationRuleID:   "email-reply-basic",
	}

	return draft.GenerationResult{
		Draft: &d,
	}
}

// Verify interface compliance.
var _ draft.DraftGenerator = (*Engine)(nil)
