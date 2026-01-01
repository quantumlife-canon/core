package execution

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"quantumlife/pkg/domain/identity"
)

// ViewSnapshot captures the email thread view at a point in time.
//
// CRITICAL: Must be verified before execution.
// CRITICAL: Stale snapshots block execution.
type ViewSnapshot struct {
	// SnapshotID is the deterministic ID of this snapshot.
	SnapshotID string

	// SnapshotHash is the deterministic hash of this snapshot.
	SnapshotHash string

	// CapturedAt is when this snapshot was taken.
	CapturedAt time.Time

	// Provider identifies the email provider.
	Provider string

	// AccountID identifies the email account.
	AccountID string

	// CircleID identifies the circle this snapshot applies to.
	CircleID identity.EntityID

	// IntersectionID is optional for shared contexts.
	IntersectionID identity.EntityID

	// ThreadID identifies the email thread.
	ThreadID string

	// InReplyToMessageID is the message being replied to.
	InReplyToMessageID string

	// MessageCount is the number of messages in the thread at snapshot time.
	MessageCount int

	// LastMessageAt is when the last message in the thread was received.
	LastMessageAt time.Time
}

// ViewSnapshotParams contains parameters for creating a view snapshot.
type ViewSnapshotParams struct {
	Provider           string
	AccountID          string
	CircleID           identity.EntityID
	IntersectionID     identity.EntityID
	ThreadID           string
	InReplyToMessageID string
	MessageCount       int
	LastMessageAt      time.Time
}

// NewViewSnapshot creates a new view snapshot with computed hash.
func NewViewSnapshot(params ViewSnapshotParams, now time.Time) ViewSnapshot {
	snapshot := ViewSnapshot{
		CapturedAt:         now,
		Provider:           params.Provider,
		AccountID:          params.AccountID,
		CircleID:           params.CircleID,
		IntersectionID:     params.IntersectionID,
		ThreadID:           params.ThreadID,
		InReplyToMessageID: params.InReplyToMessageID,
		MessageCount:       params.MessageCount,
		LastMessageAt:      params.LastMessageAt,
	}

	snapshot.SnapshotHash = snapshot.ComputeHash()
	snapshot.SnapshotID = snapshot.ComputeID()
	return snapshot
}

// ComputeHash computes a deterministic hash of the view.
//
// CRITICAL: Uses canonical string, not JSON, for determinism.
func (v *ViewSnapshot) ComputeHash() string {
	canonical := fmt.Sprintf("email-view|provider:%s|account:%s|circle:%s|intersection:%s|thread:%s|reply_to:%s|msg_count:%d|last_msg:%s",
		v.Provider,
		v.AccountID,
		v.CircleID,
		v.IntersectionID,
		v.ThreadID,
		v.InReplyToMessageID,
		v.MessageCount,
		v.LastMessageAt.UTC().Format(time.RFC3339),
	)

	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:16])
}

// ComputeID computes a deterministic ID for the snapshot.
func (v *ViewSnapshot) ComputeID() string {
	canonical := fmt.Sprintf("view-id|%s|%s",
		v.SnapshotHash,
		v.CapturedAt.UTC().Format(time.RFC3339Nano),
	)

	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:8])
}

// DefaultMaxStaleness is the default maximum age for a view snapshot.
const DefaultMaxStaleness = 5 * time.Minute

// ViewVerifier verifies view snapshots.
type ViewVerifier struct {
	// maxStaleness is the maximum age for a view snapshot.
	maxStaleness time.Duration

	// clock provides current time.
	clock func() time.Time

	// currentViewProvider provides the current view.
	currentViewProvider func(threadID string) (ViewSnapshot, error)
}

// ViewVerifierOption configures the view verifier.
type ViewVerifierOption func(*ViewVerifier)

// WithMaxStaleness sets the maximum staleness.
func WithMaxStaleness(d time.Duration) ViewVerifierOption {
	return func(v *ViewVerifier) {
		v.maxStaleness = d
	}
}

// WithViewClock sets the clock function.
func WithViewClock(clock func() time.Time) ViewVerifierOption {
	return func(v *ViewVerifier) {
		v.clock = clock
	}
}

// NewViewVerifier creates a new view verifier.
func NewViewVerifier(provider func(threadID string) (ViewSnapshot, error), opts ...ViewVerifierOption) *ViewVerifier {
	v := &ViewVerifier{
		maxStaleness:        DefaultMaxStaleness,
		clock:               time.Now,
		currentViewProvider: provider,
	}
	for _, opt := range opts {
		opt(v)
	}
	return v
}

// Verify verifies that the snapshot is fresh and matches current view.
//
// CRITICAL: Returns error if view is stale or has drifted.
func (v *ViewVerifier) Verify(snapshot ViewSnapshot) error {
	now := v.clock()

	// Check freshness
	age := now.Sub(snapshot.CapturedAt)
	if age > v.maxStaleness {
		return fmt.Errorf("view snapshot is stale: age=%v max=%v",
			age.Round(time.Second), v.maxStaleness)
	}

	// Check current view if provider available
	if v.currentViewProvider != nil {
		current, err := v.currentViewProvider(snapshot.ThreadID)
		if err != nil {
			return fmt.Errorf("failed to get current view: %w", err)
		}

		if current.SnapshotHash != snapshot.SnapshotHash {
			return fmt.Errorf("view drift detected: snapshot=%s current=%s",
				snapshot.SnapshotHash, current.SnapshotHash)
		}
	}

	return nil
}

// IsFresh checks if the snapshot is within staleness bounds.
func (v *ViewVerifier) IsFresh(snapshot ViewSnapshot) bool {
	now := v.clock()
	age := now.Sub(snapshot.CapturedAt)
	return age <= v.maxStaleness
}
