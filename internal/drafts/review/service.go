// Package review implements the draft review and approval workflow.
//
// CRITICAL: Approval does NOT trigger execution. It only marks status.
// CRITICAL: External execution is a separate, future concern.
// CRITICAL: All status changes are audit logged via events.
package review

import (
	"fmt"
	"time"

	"quantumlife/pkg/domain/draft"
	"quantumlife/pkg/domain/identity"
)

// Service handles draft review and approval workflow.
type Service struct {
	store        draft.Store
	safetyChecks []SafetyCheck
}

// SafetyCheck validates a draft before approval.
type SafetyCheck interface {
	// Name returns the check name.
	Name() string

	// Check validates the draft. Returns error if unsafe.
	Check(d draft.Draft) error
}

// NewService creates a new review service.
func NewService(store draft.Store, checks ...SafetyCheck) *Service {
	return &Service{
		store:        store,
		safetyChecks: checks,
	}
}

// ReviewRequest represents a request to review a draft.
type ReviewRequest struct {
	// DraftID is the draft to review.
	DraftID draft.DraftID

	// CircleID must match the draft's circle.
	CircleID identity.EntityID

	// ReviewerID identifies who is reviewing.
	ReviewerID string

	// Now is the current time (injected clock).
	Now time.Time
}

// ApprovalRequest represents a request to approve a draft.
type ApprovalRequest struct {
	ReviewRequest

	// Reason is an optional approval reason.
	Reason string
}

// RejectionRequest represents a request to reject a draft.
type RejectionRequest struct {
	ReviewRequest

	// Reason is required for rejection.
	Reason string
}

// ReviewResult contains the outcome of a review action.
type ReviewResult struct {
	// Draft is the updated draft (if successful).
	Draft *draft.Draft

	// SafetyWarnings are non-blocking safety concerns.
	SafetyWarnings []string

	// Error indicates a failure.
	Error error
}

// GetForReview retrieves a draft for review.
func (s *Service) GetForReview(req ReviewRequest) ReviewResult {
	d, found := s.store.Get(req.DraftID)
	if !found {
		return ReviewResult{
			Error: fmt.Errorf("draft not found: %s", req.DraftID),
		}
	}

	// Verify circle ownership
	if d.CircleID != req.CircleID {
		return ReviewResult{
			Error: fmt.Errorf("draft belongs to different circle"),
		}
	}

	// Check if still reviewable
	if d.IsTerminal() {
		return ReviewResult{
			Error: fmt.Errorf("draft is in terminal status: %s", d.Status),
		}
	}

	// Check expiry
	if d.IsExpired(req.Now) {
		// Mark as expired
		_ = s.store.UpdateStatus(d.DraftID, draft.StatusExpired, "TTL expired", "system", req.Now)
		d.Status = draft.StatusExpired
		return ReviewResult{
			Draft: &d,
			Error: fmt.Errorf("draft has expired"),
		}
	}

	// Collect safety warnings (non-blocking)
	var warnings []string
	for _, check := range s.safetyChecks {
		if err := check.Check(d); err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %s", check.Name(), err.Error()))
		}
	}

	// Include draft's own safety notes
	warnings = append(warnings, d.SafetyNotes...)

	return ReviewResult{
		Draft:          &d,
		SafetyWarnings: warnings,
	}
}

// Approve approves a draft.
func (s *Service) Approve(req ApprovalRequest) ReviewResult {
	// First get for review to validate
	result := s.GetForReview(req.ReviewRequest)
	if result.Error != nil {
		return result
	}

	// Run safety checks (these are blocking for approval)
	for _, check := range s.safetyChecks {
		if err := check.Check(*result.Draft); err != nil {
			return ReviewResult{
				Draft: result.Draft,
				Error: fmt.Errorf("safety check '%s' failed: %w", check.Name(), err),
			}
		}
	}

	// Update status
	reason := req.Reason
	if reason == "" {
		reason = "Approved by reviewer"
	}

	err := s.store.UpdateStatus(
		req.DraftID,
		draft.StatusApproved,
		reason,
		req.ReviewerID,
		req.Now,
	)
	if err != nil {
		return ReviewResult{
			Draft: result.Draft,
			Error: fmt.Errorf("failed to update status: %w", err),
		}
	}

	// Get updated draft
	updated, _ := s.store.Get(req.DraftID)
	return ReviewResult{
		Draft:          &updated,
		SafetyWarnings: result.SafetyWarnings,
	}
}

// Reject rejects a draft.
func (s *Service) Reject(req RejectionRequest) ReviewResult {
	// First get for review to validate
	result := s.GetForReview(req.ReviewRequest)
	if result.Error != nil {
		return result
	}

	// Reason is required for rejection
	if req.Reason == "" {
		return ReviewResult{
			Draft: result.Draft,
			Error: fmt.Errorf("rejection reason is required"),
		}
	}

	// Update status
	err := s.store.UpdateStatus(
		req.DraftID,
		draft.StatusRejected,
		req.Reason,
		req.ReviewerID,
		req.Now,
	)
	if err != nil {
		return ReviewResult{
			Draft: result.Draft,
			Error: fmt.Errorf("failed to update status: %w", err),
		}
	}

	// Get updated draft
	updated, _ := s.store.Get(req.DraftID)
	return ReviewResult{
		Draft: &updated,
	}
}

// ListPending returns all pending drafts for a circle.
func (s *Service) ListPending(circleID identity.EntityID) []draft.Draft {
	return s.store.List(draft.ListFilter{
		CircleID: circleID,
		Status:   draft.StatusProposed,
	})
}

// ListAll returns all drafts for a circle (including expired).
func (s *Service) ListAll(circleID identity.EntityID, includeExpired bool) []draft.Draft {
	return s.store.List(draft.ListFilter{
		CircleID:       circleID,
		IncludeExpired: includeExpired,
	})
}

// ExpireStale marks all expired drafts as expired.
func (s *Service) ExpireStale(now time.Time) int {
	return s.store.MarkExpired(now)
}

// GetStats returns review statistics for a circle.
type ReviewStats struct {
	PendingCount  int
	ApprovedCount int
	RejectedCount int
	ExpiredCount  int
}

// GetStats returns review statistics for a circle.
func (s *Service) GetStats(circleID identity.EntityID) ReviewStats {
	allDrafts := s.store.List(draft.ListFilter{
		CircleID:       circleID,
		IncludeExpired: true,
	})

	stats := ReviewStats{}
	for _, d := range allDrafts {
		switch d.Status {
		case draft.StatusProposed:
			stats.PendingCount++
		case draft.StatusApproved:
			stats.ApprovedCount++
		case draft.StatusRejected:
			stats.RejectedCount++
		case draft.StatusExpired:
			stats.ExpiredCount++
		}
	}

	return stats
}

// ExternalRecipientCheck is a safety check for external recipients.
type ExternalRecipientCheck struct {
	InternalDomains []string
}

// Name returns the check name.
func (c *ExternalRecipientCheck) Name() string {
	return "external_recipient"
}

// Check validates the draft.
func (c *ExternalRecipientCheck) Check(d draft.Draft) error {
	emailContent, ok := d.EmailContent()
	if !ok {
		return nil // Not an email draft
	}

	// Check if recipient is external
	for _, domain := range c.InternalDomains {
		if containsDomain(emailContent.To, domain) {
			return nil // Internal recipient
		}
	}

	return fmt.Errorf("recipient %s is external", emailContent.To)
}

// containsDomain checks if an email contains a domain.
func containsDomain(email, domain string) bool {
	return len(email) > len(domain) && email[len(email)-len(domain):] == domain
}

// Verify interface compliance.
var _ SafetyCheck = (*ExternalRecipientCheck)(nil)
