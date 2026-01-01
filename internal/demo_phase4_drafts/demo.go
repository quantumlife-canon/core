// Package demo_phase4_drafts demonstrates Phase 4: Drafts-Only Assistance.
//
// This demo shows:
// 1. Draft generation from obligations (email + calendar)
// 2. Draft deduplication (same obligation = same draft)
// 3. Draft review and approval workflow
// 4. TTL-based expiration
// 5. Rate limiting per circle per day
//
// CRITICAL: No external writes occur. Drafts are internal proposals only.
package demo_phase4_drafts

import (
	"fmt"
	"strings"
	"time"

	"quantumlife/internal/drafts"
	"quantumlife/internal/drafts/calendar"
	"quantumlife/internal/drafts/commerce"
	"quantumlife/internal/drafts/email"
	"quantumlife/internal/drafts/review"
	"quantumlife/pkg/domain/draft"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/obligation"
)

// DemoResult contains the demo output.
type DemoResult struct {
	Output string
	Err    error
}

// RunDemo executes the Phase 4 drafts demo.
func RunDemo() DemoResult {
	var out strings.Builder

	out.WriteString("=== Phase 4: Drafts-Only Assistance Demo ===\n\n")

	// Create domain entities
	circleID := identity.EntityID("circle-alice-123")
	now := time.Date(2024, 3, 15, 10, 0, 0, 0, time.UTC)

	// Initialize components
	store := draft.NewInMemoryStore()
	policy := draft.DefaultDraftPolicy()
	emailEngine := email.NewDefaultEngine()
	calendarEngine := calendar.NewDefaultEngine()
	commerceEngine := commerce.NewDefaultEngine()
	engine := drafts.NewEngine(store, policy, emailEngine, calendarEngine, commerceEngine)
	reviewService := review.NewService(store)

	out.WriteString("1. DRAFT GENERATION FROM OBLIGATIONS\n")
	out.WriteString("------------------------------------\n\n")

	// Create an email reply obligation
	emailObl := obligation.NewObligation(
		circleID,
		"email-thread-001",
		"email",
		obligation.ObligationReply,
		now,
	).WithReason("Email from manager needs reply").
		WithEvidence(obligation.EvidenceKeySender, "manager@external.com").
		WithEvidence(obligation.EvidenceKeySubject, "Q1 Review Meeting").
		WithSeverity(obligation.SeverityHigh)

	// Process the email obligation
	result := engine.Process(circleID, "", emailObl, now)
	if result.Error != nil {
		return DemoResult{Err: fmt.Errorf("email draft generation failed: %w", result.Error)}
	}

	if result.Generated {
		out.WriteString(fmt.Sprintf("✓ Generated email draft: %s\n", result.DraftID))
		d, _ := engine.GetDraft(result.DraftID)
		if emailContent, ok := d.EmailContent(); ok {
			out.WriteString(fmt.Sprintf("  To: %s\n", emailContent.To))
			out.WriteString(fmt.Sprintf("  Subject: %s\n", emailContent.Subject))
			out.WriteString(fmt.Sprintf("  Expires: %s\n", d.ExpiresAt.Format(time.RFC3339)))
			out.WriteString(fmt.Sprintf("  Safety Notes: %v\n", d.SafetyNotes))
		}
	}
	out.WriteString("\n")

	// Create a calendar response obligation
	eventTime := now.Add(24 * time.Hour)
	calendarObl := obligation.NewObligation(
		circleID,
		"event-meeting-002",
		"calendar",
		obligation.ObligationDecide,
		now,
	).WithDueBy(eventTime, now).
		WithReason("Calendar invite needs response").
		WithEvidence(obligation.EvidenceKeyEventTitle, "Team Sync").
		WithEvidence(obligation.EvidenceKeySender, "team@external.com")

	result = engine.Process(circleID, "", calendarObl, now)
	if result.Error != nil {
		return DemoResult{Err: fmt.Errorf("calendar draft generation failed: %w", result.Error)}
	}

	if result.Generated {
		out.WriteString(fmt.Sprintf("✓ Generated calendar draft: %s\n", result.DraftID))
		d, _ := engine.GetDraft(result.DraftID)
		if calContent, ok := d.CalendarContent(); ok {
			out.WriteString(fmt.Sprintf("  Event: %s\n", calContent.EventID))
			out.WriteString(fmt.Sprintf("  Response: %s\n", calContent.Response))
			out.WriteString(fmt.Sprintf("  Message: %s\n", calContent.Message))
			out.WriteString(fmt.Sprintf("  Expires: %s\n", d.ExpiresAt.Format(time.RFC3339)))
		}
	}
	out.WriteString("\n")

	out.WriteString("2. DRAFT DEDUPLICATION\n")
	out.WriteString("----------------------\n\n")

	// Try to generate the same email draft again
	result = engine.Process(circleID, "", emailObl, now.Add(1*time.Minute))
	if result.Deduplicated {
		out.WriteString(fmt.Sprintf("✓ Deduplicated: existing draft %s returned\n", result.DraftID))
	}
	out.WriteString("\n")

	out.WriteString("3. DRAFT REVIEW WORKFLOW\n")
	out.WriteString("------------------------\n\n")

	// Get pending drafts
	pending := reviewService.ListPending(circleID)
	out.WriteString(fmt.Sprintf("Pending drafts: %d\n\n", len(pending)))

	for _, d := range pending {
		out.WriteString(fmt.Sprintf("Draft %s [%s]:\n", d.DraftID, d.DraftType))

		// Get for review
		reviewResult := reviewService.GetForReview(review.ReviewRequest{
			DraftID:    d.DraftID,
			CircleID:   circleID,
			ReviewerID: "alice",
			Now:        now,
		})
		if reviewResult.Error != nil {
			out.WriteString(fmt.Sprintf("  Error: %v\n", reviewResult.Error))
			continue
		}

		if len(reviewResult.SafetyWarnings) > 0 {
			out.WriteString("  Safety Warnings:\n")
			for _, w := range reviewResult.SafetyWarnings {
				out.WriteString(fmt.Sprintf("    - %s\n", w))
			}
		}
	}
	out.WriteString("\n")

	out.WriteString("4. DRAFT APPROVAL\n")
	out.WriteString("-----------------\n\n")

	// Approve the email draft
	emailDraftID := pending[0].DraftID
	approveResult := reviewService.Approve(review.ApprovalRequest{
		ReviewRequest: review.ReviewRequest{
			DraftID:    emailDraftID,
			CircleID:   circleID,
			ReviewerID: "alice",
			Now:        now.Add(5 * time.Minute),
		},
		Reason: "Content verified, ready to send",
	})

	if approveResult.Error != nil {
		out.WriteString(fmt.Sprintf("✗ Approval failed: %v\n", approveResult.Error))
	} else {
		out.WriteString(fmt.Sprintf("✓ Draft %s approved\n", emailDraftID))
		out.WriteString(fmt.Sprintf("  Status: %s\n", approveResult.Draft.Status))
		out.WriteString(fmt.Sprintf("  Changed by: %s\n", approveResult.Draft.StatusChangedBy))
		out.WriteString(fmt.Sprintf("  Reason: %s\n", approveResult.Draft.StatusReason))
	}
	out.WriteString("\n")

	out.WriteString("5. DRAFT REJECTION\n")
	out.WriteString("------------------\n\n")

	// Reject the calendar draft
	calendarDraftID := pending[1].DraftID
	rejectResult := reviewService.Reject(review.RejectionRequest{
		ReviewRequest: review.ReviewRequest{
			DraftID:    calendarDraftID,
			CircleID:   circleID,
			ReviewerID: "alice",
			Now:        now.Add(5 * time.Minute),
		},
		Reason: "I need to check my calendar first",
	})

	if rejectResult.Error != nil {
		out.WriteString(fmt.Sprintf("✗ Rejection failed: %v\n", rejectResult.Error))
	} else {
		out.WriteString(fmt.Sprintf("✓ Draft %s rejected\n", calendarDraftID))
		out.WriteString(fmt.Sprintf("  Status: %s\n", rejectResult.Draft.Status))
		out.WriteString(fmt.Sprintf("  Reason: %s\n", rejectResult.Draft.StatusReason))
	}
	out.WriteString("\n")

	out.WriteString("6. TTL-BASED EXPIRATION\n")
	out.WriteString("-----------------------\n\n")

	// Create another draft and let it expire
	expireObl := obligation.NewObligation(
		circleID,
		"email-thread-003",
		"email",
		obligation.ObligationReply,
		now,
	).WithReason("Low priority email").
		WithEvidence(obligation.EvidenceKeySender, "newsletter@external.com").
		WithEvidence(obligation.EvidenceKeySubject, "Weekly Newsletter")

	engine.Process(circleID, "", expireObl, now)

	// Fast forward past TTL (48 hours for email)
	futureTime := now.Add(49 * time.Hour)
	expiredCount := engine.MarkExpiredDrafts(futureTime)
	out.WriteString(fmt.Sprintf("✓ Marked %d draft(s) as expired at %s\n", expiredCount, futureTime.Format(time.RFC3339)))
	out.WriteString("\n")

	out.WriteString("7. FINAL STATISTICS\n")
	out.WriteString("-------------------\n\n")

	stats := reviewService.GetStats(circleID)
	out.WriteString(fmt.Sprintf("Pending:  %d\n", stats.PendingCount))
	out.WriteString(fmt.Sprintf("Approved: %d\n", stats.ApprovedCount))
	out.WriteString(fmt.Sprintf("Rejected: %d\n", stats.RejectedCount))
	out.WriteString(fmt.Sprintf("Expired:  %d\n", stats.ExpiredCount))
	out.WriteString("\n")

	out.WriteString("=== Demo Complete ===\n")
	out.WriteString("\nNOTE: No external writes occurred. Approved drafts are\n")
	out.WriteString("ready for execution in a future phase.\n")

	return DemoResult{Output: out.String()}
}
