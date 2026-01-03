// Package shadowview provides the shadow receipt viewer for Phase 21.
//
// Phase 21: Unified Onboarding + Shadow Receipt Viewer
//
// CRITICAL INVARIANTS:
//   - Stores ONLY hashes - never raw content
//   - No goroutines. No time.Now().
//   - Stdlib only.
//   - Append-only pattern with bounded size
//
// Reference: docs/ADR/ADR-0051-phase21-onboarding-modes-shadow-receipt-viewer.md
package shadowview

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// AckAction represents acknowledgement actions.
type AckAction string

const (
	// AckViewed means the receipt was viewed.
	AckViewed AckAction = "viewed"

	// AckDismissed means the receipt whisper was dismissed.
	AckDismissed AckAction = "dismissed"
)

// AckRecord represents a single acknowledgement record.
//
// CRITICAL: Contains ONLY hashes - never identifiable info.
type AckRecord struct {
	// Action is the acknowledgement action.
	Action AckAction

	// ReceiptHash is the SHA256 hash of the receipt.
	ReceiptHash string

	// TSHash is the SHA256 hash of the timestamp (never raw).
	TSHash string

	// PeriodBucket is the day bucket (YYYY-MM-DD).
	PeriodBucket string
}

// AckStore stores shadow receipt acknowledgements.
//
// CRITICAL: Bounded, in-memory, hash-only storage.
// Pattern matches internal/proof/store.go.
type AckStore struct {
	mu         sync.RWMutex
	records    []AckRecord
	maxRecords int

	// receiptIndex maps receipt hash to most recent record index.
	receiptIndex map[string]int
}

// DefaultMaxAckRecords is the default maximum acknowledgement records.
const DefaultMaxAckRecords = 128

// NewAckStore creates a new acknowledgement store.
func NewAckStore(maxRecords int) *AckStore {
	if maxRecords <= 0 {
		maxRecords = DefaultMaxAckRecords
	}
	return &AckStore{
		records:      make([]AckRecord, 0, maxRecords),
		maxRecords:   maxRecords,
		receiptIndex: make(map[string]int),
	}
}

// Record stores an acknowledgement hash.
//
// CRITICAL: Only stores hashes, never raw timestamps.
func (s *AckStore) Record(action AckAction, receiptHash string, periodBucket string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Hash the timestamp (never store raw)
	tsHash := hashTimestamp(now)

	record := AckRecord{
		Action:       action,
		ReceiptHash:  receiptHash,
		TSHash:       tsHash,
		PeriodBucket: periodBucket,
	}

	// Evict oldest if at capacity
	if len(s.records) >= s.maxRecords {
		// Remove oldest from index
		oldest := s.records[0]
		if idx, ok := s.receiptIndex[oldest.ReceiptHash]; ok && idx == 0 {
			delete(s.receiptIndex, oldest.ReceiptHash)
		}
		// Shift records and update indices
		s.records = s.records[1:]
		for hash, idx := range s.receiptIndex {
			if idx > 0 {
				s.receiptIndex[hash] = idx - 1
			}
		}
	}

	// Append new record
	s.records = append(s.records, record)
	s.receiptIndex[receiptHash] = len(s.records) - 1

	return nil
}

// HasRecentForPeriod checks if receipt was acknowledged for given period.
//
// Used to suppress whisper cues.
func (s *AckStore) HasRecentForPeriod(receiptHash, periodBucket string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	idx, ok := s.receiptIndex[receiptHash]
	if !ok {
		return false
	}

	// Check if the acknowledgement was for this period
	record := s.records[idx]
	return record.PeriodBucket == periodBucket
}

// HasDismissedForPeriod checks if receipt was dismissed for given period.
func (s *AckStore) HasDismissedForPeriod(receiptHash, periodBucket string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	idx, ok := s.receiptIndex[receiptHash]
	if !ok {
		return false
	}

	record := s.records[idx]
	return record.Action == AckDismissed && record.PeriodBucket == periodBucket
}

// Len returns the current record count.
func (s *AckStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}

// hashTimestamp creates a SHA256 hash of a timestamp.
// CRITICAL: Never store raw timestamps.
func hashTimestamp(t time.Time) string {
	h := sha256.New()
	h.Write([]byte("SHADOW_ACK_TS|"))
	h.Write([]byte(t.UTC().Format(time.RFC3339Nano)))
	return hex.EncodeToString(h.Sum(nil)[:16])
}
