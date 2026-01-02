// Package persist provides persistence for sync receipts.
//
// Phase 19.1: Real Gmail Connection (You-only)
//
// CRITICAL INVARIANTS:
//   - SyncReceipt stores ONLY: circle_id, magnitude_bucket, receipt_hash, time_bucket
//   - NO raw message content, NO subject lines, NO sender names, NO raw counts
//   - Magnitude buckets: "none" | "handful" | "several" | "many"
//   - Time buckets: floored to 5-minute intervals
//   - Receipts are deterministic: same inputs => same hash
//   - No goroutines. No time.Now() - clock injection only.
package persist

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"quantumlife/pkg/domain/identity"
)

// MagnitudeBucket represents an abstract count bucket.
// Never stores raw counts.
type MagnitudeBucket string

const (
	MagnitudeNone    MagnitudeBucket = "none"     // 0 items
	MagnitudeHandful MagnitudeBucket = "handful"  // 1-5 items
	MagnitudeSeveral MagnitudeBucket = "several"  // 6-20 items
	MagnitudeMany    MagnitudeBucket = "many"     // 21+ items
)

// ToMagnitudeBucket converts a raw count to a magnitude bucket.
// This is the ONLY place where raw counts are used.
func ToMagnitudeBucket(count int) MagnitudeBucket {
	switch {
	case count == 0:
		return MagnitudeNone
	case count <= 5:
		return MagnitudeHandful
	case count <= 20:
		return MagnitudeSeveral
	default:
		return MagnitudeMany
	}
}

// DisplayText returns human-readable text for the bucket.
func (m MagnitudeBucket) DisplayText() string {
	switch m {
	case MagnitudeNone:
		return "nothing"
	case MagnitudeHandful:
		return "a handful"
	case MagnitudeSeveral:
		return "several"
	case MagnitudeMany:
		return "many"
	default:
		return "unknown"
	}
}

// TimeBucket floors a timestamp to 5-minute intervals for privacy.
func TimeBucket(t time.Time) time.Time {
	return t.Truncate(5 * time.Minute)
}

// TimeBucketString returns a display string for the time bucket.
func TimeBucketString(t time.Time) string {
	bucket := TimeBucket(t)
	return bucket.Format("Jan 2 15:04")
}

// SyncReceipt represents a sync operation receipt.
//
// CRITICAL: Contains NO raw data, NO message content, NO identifiable info.
// Only: circle_id, magnitude_bucket, receipt_hash, time_bucket.
type SyncReceipt struct {
	// ReceiptID uniquely identifies this receipt (deterministic hash).
	ReceiptID string

	// CircleID identifies the circle this sync was for.
	CircleID identity.EntityID

	// Provider identifies the sync source (e.g., "gmail").
	Provider string

	// MagnitudeBucket is the abstract count bucket (never raw counts).
	MagnitudeBucket MagnitudeBucket

	// EventsStoredBucket is the abstract count of events stored.
	EventsStoredBucket MagnitudeBucket

	// TimeBucket is the floored sync time (5-minute granularity).
	TimeBucket time.Time

	// Success indicates if the sync completed successfully.
	Success bool

	// FailReason is set only if Success is false.
	// Contains generic reason, never raw error messages with PII.
	FailReason string

	// Hash is the deterministic hash of this receipt.
	Hash string
}

// NewSyncReceipt creates a new sync receipt.
// The receipt_id and hash are computed deterministically.
func NewSyncReceipt(
	circleID identity.EntityID,
	provider string,
	messageCount int,
	eventsStored int,
	syncTime time.Time,
	success bool,
	failReason string,
) *SyncReceipt {
	magnitudeBucket := ToMagnitudeBucket(messageCount)
	eventsStoredBucket := ToMagnitudeBucket(eventsStored)
	timeBucket := TimeBucket(syncTime)

	// Compute deterministic receipt ID
	receiptID := computeReceiptID(circleID, provider, magnitudeBucket, timeBucket)

	r := &SyncReceipt{
		ReceiptID:          receiptID,
		CircleID:           circleID,
		Provider:           provider,
		MagnitudeBucket:    magnitudeBucket,
		EventsStoredBucket: eventsStoredBucket,
		TimeBucket:         timeBucket,
		Success:            success,
		FailReason:         failReason,
	}

	r.Hash = r.computeHash()
	return r
}

// computeReceiptID generates a deterministic receipt ID.
func computeReceiptID(circleID identity.EntityID, provider string, magnitude MagnitudeBucket, timeBucket time.Time) string {
	canonical := fmt.Sprintf("SYNC_RECEIPT_ID|v1|%s|%s|%s|%d",
		circleID, provider, magnitude, timeBucket.Unix())
	h := sha256.Sum256([]byte(canonical))
	return fmt.Sprintf("%x", h[:8]) // 16 hex chars
}

// computeHash generates a deterministic hash for the receipt.
func (r *SyncReceipt) computeHash() string {
	successStr := "false"
	if r.Success {
		successStr = "true"
	}
	canonical := fmt.Sprintf("SYNC_RECEIPT|v1|%s|%s|%s|%s|%d|%s|%s",
		r.ReceiptID, r.CircleID, r.Provider, r.MagnitudeBucket,
		r.TimeBucket.Unix(), successStr, r.FailReason)
	h := sha256.Sum256([]byte(canonical))
	return fmt.Sprintf("%x", h)
}

// Validate checks the receipt is valid.
func (r *SyncReceipt) Validate() error {
	if r.ReceiptID == "" {
		return fmt.Errorf("missing receipt_id")
	}
	if r.CircleID == "" {
		return fmt.Errorf("missing circle_id")
	}
	if r.Provider == "" {
		return fmt.Errorf("missing provider")
	}
	if r.TimeBucket.IsZero() {
		return fmt.Errorf("missing time_bucket")
	}
	return nil
}

// SyncReceiptStore stores sync receipts.
// Thread-safe, in-memory implementation.
type SyncReceiptStore struct {
	mu       sync.RWMutex
	receipts map[string]*SyncReceipt            // receiptID -> receipt
	byCircle map[identity.EntityID][]*SyncReceipt // circleID -> receipts
	clock    func() time.Time
}

// NewSyncReceiptStore creates a new sync receipt store.
func NewSyncReceiptStore(clock func() time.Time) *SyncReceiptStore {
	return &SyncReceiptStore{
		receipts: make(map[string]*SyncReceipt),
		byCircle: make(map[identity.EntityID][]*SyncReceipt),
		clock:    clock,
	}
}

// Store stores a sync receipt.
func (s *SyncReceiptStore) Store(receipt *SyncReceipt) error {
	if err := receipt.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.receipts[receipt.ReceiptID] = receipt
	s.byCircle[receipt.CircleID] = append(s.byCircle[receipt.CircleID], receipt)

	return nil
}

// Get retrieves a receipt by ID.
func (s *SyncReceiptStore) Get(receiptID string) (*SyncReceipt, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	r, ok := s.receipts[receiptID]
	return r, ok
}

// GetByCircle retrieves all receipts for a circle.
func (s *SyncReceiptStore) GetByCircle(circleID identity.EntityID) []*SyncReceipt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.byCircle[circleID]
}

// GetLatestByCircle retrieves the most recent receipt for a circle.
func (s *SyncReceiptStore) GetLatestByCircle(circleID identity.EntityID) *SyncReceipt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	receipts := s.byCircle[circleID]
	if len(receipts) == 0 {
		return nil
	}

	// Find the latest by time bucket
	var latest *SyncReceipt
	for _, r := range receipts {
		if latest == nil || r.TimeBucket.After(latest.TimeBucket) {
			latest = r
		}
	}
	return latest
}

// Count returns the total number of receipts.
func (s *SyncReceiptStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.receipts)
}

// QuietCheckStatus represents the quiet baseline verification status.
type QuietCheckStatus struct {
	// GmailConnected indicates if Gmail is connected.
	GmailConnected bool

	// LastSyncTimeBucket is the last sync time bucket (5-minute granularity).
	LastSyncTimeBucket string

	// LastSyncMagnitude is the magnitude bucket of the last sync.
	LastSyncMagnitude MagnitudeBucket

	// ObligationsHeld indicates all obligations are held (not surfaced).
	ObligationsHeld bool

	// AutoSurface indicates if auto-surface is enabled (should always be false).
	AutoSurface bool

	// Hash is a deterministic hash of this status.
	Hash string
}

// NewQuietCheckStatus creates a new quiet check status.
func NewQuietCheckStatus(
	gmailConnected bool,
	lastSyncTime time.Time,
	lastSyncMagnitude MagnitudeBucket,
	obligationsHeld bool,
	autoSurface bool,
) *QuietCheckStatus {
	var timeBucket string
	if !lastSyncTime.IsZero() {
		timeBucket = TimeBucketString(lastSyncTime)
	} else {
		timeBucket = "never"
	}

	s := &QuietCheckStatus{
		GmailConnected:     gmailConnected,
		LastSyncTimeBucket: timeBucket,
		LastSyncMagnitude:  lastSyncMagnitude,
		ObligationsHeld:    obligationsHeld,
		AutoSurface:        autoSurface,
	}

	s.Hash = s.computeHash()
	return s
}

// computeHash computes a deterministic hash of the status.
func (s *QuietCheckStatus) computeHash() string {
	canonical := fmt.Sprintf("QUIET_CHECK|v1|%t|%s|%s|%t|%t",
		s.GmailConnected, s.LastSyncTimeBucket, s.LastSyncMagnitude,
		s.ObligationsHeld, s.AutoSurface)
	h := sha256.Sum256([]byte(canonical))
	return fmt.Sprintf("%x", h)
}

// IsQuiet returns true if the system is in quiet mode.
// Quiet mode means: obligations held, no auto-surface.
func (s *QuietCheckStatus) IsQuiet() bool {
	return s.ObligationsHeld && !s.AutoSurface
}
