// Package feedback provides feedback capture for interruptions and drafts.
//
// CRITICAL: Feedback is captured to improve future assistance.
// CRITICAL: All feedback IDs are deterministic via SHA256 hashing.
// CRITICAL: No ML/training in this phase - signals are stored for future use.
//
// Reference: docs/ADR/ADR-0023-phase6-quiet-loop-web.md
package feedback

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"quantumlife/pkg/domain/identity"
)

// FeedbackTargetType identifies what the feedback is about.
type FeedbackTargetType string

const (
	// TargetInterruption is feedback about an interruption.
	TargetInterruption FeedbackTargetType = "interruption"

	// TargetDraft is feedback about a draft.
	TargetDraft FeedbackTargetType = "draft"
)

// FeedbackSignal indicates the feedback sentiment.
type FeedbackSignal string

const (
	// SignalHelpful indicates the item was helpful.
	SignalHelpful FeedbackSignal = "helpful"

	// SignalUnnecessary indicates the item was unnecessary.
	SignalUnnecessary FeedbackSignal = "unnecessary"
)

// FeedbackRecord represents captured feedback about an item.
type FeedbackRecord struct {
	// FeedbackID is the deterministic ID (SHA256 of canonical string).
	FeedbackID string

	// TargetType identifies interrupt or draft.
	TargetType FeedbackTargetType

	// TargetID is the ID of the item receiving feedback.
	TargetID string

	// CircleID is the circle context.
	CircleID identity.EntityID

	// CapturedAt is when feedback was recorded.
	CapturedAt time.Time

	// Signal is the feedback sentiment.
	Signal FeedbackSignal

	// Reason is an optional explanation.
	Reason string

	// CanonicalHash is the hash of the canonical representation.
	CanonicalHash string
}

// ComputeFeedbackID computes a deterministic feedback ID.
func ComputeFeedbackID(
	targetType FeedbackTargetType,
	targetID string,
	circleID identity.EntityID,
	capturedAt time.Time,
	signal FeedbackSignal,
) string {
	canonical := fmt.Sprintf("feedback|%s|%s|%s|%s|%s",
		targetType,
		targetID,
		circleID,
		capturedAt.UTC().Format(time.RFC3339Nano),
		signal,
	)
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:])[:16] // Truncate to 16 chars
}

// NewFeedbackRecord creates a new feedback record with deterministic ID.
func NewFeedbackRecord(
	targetType FeedbackTargetType,
	targetID string,
	circleID identity.EntityID,
	capturedAt time.Time,
	signal FeedbackSignal,
	reason string,
) FeedbackRecord {
	feedbackID := ComputeFeedbackID(targetType, targetID, circleID, capturedAt, signal)

	canonical := fmt.Sprintf("feedback|%s|%s|%s|%s|%s|%s",
		targetType,
		targetID,
		circleID,
		capturedAt.UTC().Format(time.RFC3339Nano),
		signal,
		reason,
	)
	canonicalHash := sha256.Sum256([]byte(canonical))

	return FeedbackRecord{
		FeedbackID:    feedbackID,
		TargetType:    targetType,
		TargetID:      targetID,
		CircleID:      circleID,
		CapturedAt:    capturedAt,
		Signal:        signal,
		Reason:        reason,
		CanonicalHash: hex.EncodeToString(canonicalHash[:]),
	}
}

// Validate checks that the feedback record is valid.
func (f FeedbackRecord) Validate() error {
	if f.FeedbackID == "" {
		return ErrMissingFeedbackID
	}
	if f.TargetType == "" {
		return ErrMissingTargetType
	}
	if f.TargetType != TargetInterruption && f.TargetType != TargetDraft {
		return ErrInvalidTargetType
	}
	if f.TargetID == "" {
		return ErrMissingTargetID
	}
	if f.CircleID == "" {
		return ErrMissingCircleID
	}
	if f.Signal == "" {
		return ErrMissingSignal
	}
	if f.Signal != SignalHelpful && f.Signal != SignalUnnecessary {
		return ErrInvalidSignal
	}
	return nil
}

// Validation errors.
var (
	ErrMissingFeedbackID = feedbackError("missing feedback_id")
	ErrMissingTargetType = feedbackError("missing target_type")
	ErrInvalidTargetType = feedbackError("invalid target_type")
	ErrMissingTargetID   = feedbackError("missing target_id")
	ErrMissingCircleID   = feedbackError("missing circle_id")
	ErrMissingSignal     = feedbackError("missing signal")
	ErrInvalidSignal     = feedbackError("invalid signal")
)

type feedbackError string

func (e feedbackError) Error() string { return string(e) }
