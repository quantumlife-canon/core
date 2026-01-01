package execution

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"

	"quantumlife/pkg/domain/identity"
)

const (
	// DefaultMaxStalenessMinutes is the default max staleness (15 minutes).
	DefaultMaxStalenessMinutes = 15
)

// ViewSnapshot captures the calendar view state at a point in time.
// CRITICAL: Execution MUST fail if view has changed beyond staleness threshold.
type ViewSnapshot struct {
	// SnapshotID is the deterministic ID for this snapshot.
	SnapshotID string

	// CircleID is the circle this view belongs to.
	CircleID identity.EntityID

	// Provider is the calendar provider.
	Provider string

	// CalendarID is the calendar this view is for.
	CalendarID string

	// EventID is the specific event this view is for.
	EventID string

	// CapturedAt is when this snapshot was taken.
	CapturedAt time.Time

	// ViewHash is the hash of the view content.
	ViewHash string

	// EventETag is the event's etag at snapshot time.
	EventETag string

	// EventUpdatedAt is when the event was last updated.
	EventUpdatedAt time.Time

	// AttendeeResponseStatus is the current user's response status.
	AttendeeResponseStatus string

	// EventSummary is the event title (for display/audit).
	EventSummary string

	// EventStart is the event start time.
	EventStart time.Time

	// EventEnd is the event end time.
	EventEnd time.Time
}

// ViewSnapshotParams contains parameters for creating a view snapshot.
type ViewSnapshotParams struct {
	CircleID               identity.EntityID
	Provider               string
	CalendarID             string
	EventID                string
	EventETag              string
	EventUpdatedAt         time.Time
	AttendeeResponseStatus string
	EventSummary           string
	EventStart             time.Time
	EventEnd               time.Time
}

// NewViewSnapshot creates a new view snapshot.
func NewViewSnapshot(params ViewSnapshotParams, now time.Time) ViewSnapshot {
	// Compute view hash
	viewHash := computeViewHash(
		params.EventID,
		params.EventETag,
		params.EventUpdatedAt,
		params.AttendeeResponseStatus,
	)

	// Compute snapshot ID
	snapshotID := computeViewSnapshotID(
		params.CircleID,
		params.Provider,
		params.CalendarID,
		params.EventID,
		viewHash,
		now,
	)

	return ViewSnapshot{
		SnapshotID:             snapshotID,
		CircleID:               params.CircleID,
		Provider:               params.Provider,
		CalendarID:             params.CalendarID,
		EventID:                params.EventID,
		CapturedAt:             now,
		ViewHash:               viewHash,
		EventETag:              params.EventETag,
		EventUpdatedAt:         params.EventUpdatedAt,
		AttendeeResponseStatus: params.AttendeeResponseStatus,
		EventSummary:           params.EventSummary,
		EventStart:             params.EventStart,
		EventEnd:               params.EventEnd,
	}
}

// computeViewHash computes a deterministic hash of view content.
func computeViewHash(
	eventID string,
	eventETag string,
	eventUpdatedAt time.Time,
	attendeeResponseStatus string,
) string {
	canonical := fmt.Sprintf("view|%s|%s|%s|%s",
		eventID,
		eventETag,
		eventUpdatedAt.UTC().Format(time.RFC3339),
		attendeeResponseStatus,
	)
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:])
}

// computeViewSnapshotID computes a deterministic snapshot ID.
func computeViewSnapshotID(
	circleID identity.EntityID,
	provider string,
	calendarID string,
	eventID string,
	viewHash string,
	capturedAt time.Time,
) string {
	canonical := fmt.Sprintf("viewsnapshot|%s|%s|%s|%s|%s|%s",
		circleID,
		provider,
		calendarID,
		eventID,
		viewHash,
		capturedAt.UTC().Format(time.RFC3339),
	)
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:])
}

// IsFresh checks if this snapshot is fresh enough for execution.
func (v ViewSnapshot) IsFresh(maxStaleness time.Duration, now time.Time) bool {
	age := now.Sub(v.CapturedAt)
	return age <= maxStaleness
}

// Age returns how old this snapshot is.
func (v ViewSnapshot) Age(now time.Time) time.Duration {
	return now.Sub(v.CapturedAt)
}

// ViewVerifier verifies view snapshots against current state.
type ViewVerifier struct {
	// getCurrentView returns the current view hash for an event.
	getCurrentView func(provider, calendarID, eventID string) (currentHash string, currentETag string, err error)
}

// NewViewVerifier creates a new view verifier.
func NewViewVerifier(getViewFn func(provider, calendarID, eventID string) (string, string, error)) *ViewVerifier {
	return &ViewVerifier{
		getCurrentView: getViewFn,
	}
}

// VerifyResult contains the result of view verification.
type VerifyResult struct {
	// Fresh indicates the snapshot is within staleness threshold.
	Fresh bool

	// Unchanged indicates the view hash hasn't changed.
	Unchanged bool

	// ETagMatch indicates the ETag matches.
	ETagMatch bool

	// CurrentHash is the current view hash.
	CurrentHash string

	// CurrentETag is the current ETag.
	CurrentETag string

	// Age is the snapshot age.
	Age time.Duration
}

// Verify checks if a view snapshot is fresh and unchanged.
// CRITICAL: Returns error if view is stale or has changed.
func (v *ViewVerifier) Verify(snapshot ViewSnapshot, maxStaleness time.Duration, now time.Time) (VerifyResult, error) {
	result := VerifyResult{
		Age: snapshot.Age(now),
	}

	// Check freshness
	result.Fresh = snapshot.IsFresh(maxStaleness, now)
	if !result.Fresh {
		return result, ErrViewStale
	}

	// Get current view
	currentHash, currentETag, err := v.getCurrentView(
		snapshot.Provider,
		snapshot.CalendarID,
		snapshot.EventID,
	)
	if err != nil {
		return result, fmt.Errorf("failed to get current view: %w", err)
	}

	result.CurrentHash = currentHash
	result.CurrentETag = currentETag
	result.Unchanged = (currentHash == snapshot.ViewHash)
	result.ETagMatch = (currentETag == snapshot.EventETag)

	if !result.Unchanged {
		return result, ErrViewChanged
	}

	return result, nil
}

// View verification errors.
var (
	ErrViewStale   = viewError("view snapshot is stale")
	ErrViewChanged = viewError("view has changed since snapshot")
)

type viewError string

func (e viewError) Error() string { return string(e) }

// FreshnessPolicy defines staleness thresholds for different operations.
type FreshnessPolicy struct {
	// DefaultMaxStaleness is the default max staleness.
	DefaultMaxStaleness time.Duration

	// PerOperationStaleness allows different staleness per operation.
	PerOperationStaleness map[string]time.Duration
}

// NewDefaultFreshnessPolicy creates a freshness policy with defaults.
func NewDefaultFreshnessPolicy() FreshnessPolicy {
	return FreshnessPolicy{
		DefaultMaxStaleness: DefaultMaxStalenessMinutes * time.Minute,
		PerOperationStaleness: map[string]time.Duration{
			"accept":    15 * time.Minute,
			"decline":   15 * time.Minute,
			"tentative": 15 * time.Minute,
			"propose":   5 * time.Minute, // Tighter for proposals
		},
	}
}

// GetMaxStaleness returns the max staleness for an operation.
func (f FreshnessPolicy) GetMaxStaleness(operation string) time.Duration {
	if staleness, ok := f.PerOperationStaleness[operation]; ok {
		return staleness
	}
	return f.DefaultMaxStaleness
}

// ViewSnapshotStore stores view snapshots.
type ViewSnapshotStore struct {
	snapshots map[string]ViewSnapshot
}

// NewViewSnapshotStore creates a new view snapshot store.
func NewViewSnapshotStore() *ViewSnapshotStore {
	return &ViewSnapshotStore{
		snapshots: make(map[string]ViewSnapshot),
	}
}

// Put stores a view snapshot.
func (s *ViewSnapshotStore) Put(snapshot ViewSnapshot) {
	s.snapshots[snapshot.SnapshotID] = snapshot
}

// Get retrieves a view snapshot by ID.
func (s *ViewSnapshotStore) Get(id string) (ViewSnapshot, bool) {
	snapshot, exists := s.snapshots[id]
	return snapshot, exists
}

// GetByEvent retrieves the latest snapshot for an event.
func (s *ViewSnapshotStore) GetByEvent(provider, calendarID, eventID string) (ViewSnapshot, bool) {
	var latest ViewSnapshot
	found := false

	for _, snapshot := range s.snapshots {
		if snapshot.Provider == provider &&
			snapshot.CalendarID == calendarID &&
			snapshot.EventID == eventID {
			if !found || snapshot.CapturedAt.After(latest.CapturedAt) {
				latest = snapshot
				found = true
			}
		}
	}

	return latest, found
}

// CleanupStale removes snapshots older than the given age.
func (s *ViewSnapshotStore) CleanupStale(maxAge time.Duration, now time.Time) int {
	var toDelete []string

	for id, snapshot := range s.snapshots {
		if snapshot.Age(now) > maxAge {
			toDelete = append(toDelete, id)
		}
	}

	// Sort for deterministic cleanup
	sort.Strings(toDelete)

	for _, id := range toDelete {
		delete(s.snapshots, id)
	}

	return len(toDelete)
}
