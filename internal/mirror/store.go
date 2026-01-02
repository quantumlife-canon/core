package mirror

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"quantumlife/pkg/domain/mirror"
)

// AckStore is an append-only bounded store for mirror acknowledgement records.
// Only hashes are stored, never raw content.
//
// Phase 18.7: Mirror Proof
//
// CRITICAL: No raw content stored - hash-only storage.
// CRITICAL: No time.Now() - clock is injected.
// CRITICAL: No goroutines.
type AckStore struct {
	mu          sync.RWMutex
	records     []string          // Only record hashes
	mirrorIndex map[string]bool   // Index of acknowledged mirror hashes
	maxRecords  int
}

// NewAckStore creates a new bounded mirror ack store.
func NewAckStore(maxRecords int) *AckStore {
	if maxRecords <= 0 {
		maxRecords = 128
	}
	return &AckStore{
		records:     make([]string, 0),
		mirrorIndex: make(map[string]bool),
		maxRecords:  maxRecords,
	}
}

// Record stores an acknowledgement action.
// Only the hash of the record is stored, never raw content.
// The now parameter is injected for determinism (no time.Now()).
func (s *AckStore) Record(action mirror.AckAction, mirrorHash string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Hash the timestamp (never store raw)
	tsHash := hashTimestamp(now)

	ack := mirror.MirrorAck{
		PageHash: mirrorHash,
		Action:   action,
		At:       now,
	}

	recordHash := computeRecordHash(ack, tsHash)

	// Evict oldest if at capacity
	if len(s.records) >= s.maxRecords {
		s.evictOldest()
	}

	// Append record hash
	s.records = append(s.records, recordHash)

	// Index the mirror hash for quick lookup
	s.mirrorIndex[mirrorHash] = true

	return nil
}

// HasRecent checks if a mirror hash has been acknowledged.
// Returns true if the mirror hash exists in the store.
func (s *AckStore) HasRecent(mirrorHash string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mirrorIndex[mirrorHash]
}

// evictOldest removes the oldest record to maintain bounds.
func (s *AckStore) evictOldest() {
	if len(s.records) > 0 {
		s.records = s.records[1:]
	}
}

// Len returns the current number of stored record hashes.
func (s *AckStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}

// hashTimestamp creates a SHA256 hash of the timestamp.
// We never store raw timestamps.
func hashTimestamp(t time.Time) string {
	h := sha256.Sum256([]byte(t.Format(time.RFC3339Nano)))
	return fmt.Sprintf("%x", h)
}

// computeRecordHash computes the hash of an ack record.
func computeRecordHash(ack mirror.MirrorAck, tsHash string) string {
	canonical := fmt.Sprintf("MIRROR_ACK_REC|v1|%s|%s|%s", ack.Action, ack.PageHash, tsHash)
	h := sha256.Sum256([]byte(canonical))
	return fmt.Sprintf("%x", h)
}
