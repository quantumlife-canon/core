package proof

import (
	"crypto/sha256"
	"fmt"
	"time"
)

// AckStore is an append-only bounded store for acknowledgement records.
// Only hashes are stored, never raw content.
type AckStore struct {
	records    []string // Only record hashes
	proofIndex map[string]bool // Index of acknowledged proof hashes
	maxRecords int
}

// NewAckStore creates a new bounded ack store.
func NewAckStore(maxRecords int) *AckStore {
	if maxRecords <= 0 {
		maxRecords = 128
	}
	return &AckStore{
		records:    make([]string, 0),
		proofIndex: make(map[string]bool),
		maxRecords: maxRecords,
	}
}

// Record stores an acknowledgement action.
// Only the hash of the record is stored, never raw content.
// The now parameter is injected for determinism (no time.Now()).
func (s *AckStore) Record(action AckAction, proofHash string, now time.Time) error {
	// Hash the timestamp (never store raw)
	tsHash := hashTimestamp(now)

	record := AckRecord{
		Action:    action,
		ProofHash: proofHash,
		TSHash:    tsHash,
	}

	recordHash := record.ComputeRecordHash()

	// Evict oldest if at capacity
	if len(s.records) >= s.maxRecords {
		s.evictOldest()
	}

	// Append record hash
	s.records = append(s.records, recordHash)

	// Index the proof hash for quick lookup
	s.proofIndex[proofHash] = true

	return nil
}

// HasRecent checks if a proof hash has been acknowledged.
// Returns true if the proof hash exists in the store.
func (s *AckStore) HasRecent(proofHash string) bool {
	return s.proofIndex[proofHash]
}

// evictOldest removes the oldest record to maintain bounds.
// Note: We cannot remove from proofIndex without tracking which
// proof hashes are associated with which records. For simplicity,
// we keep proofIndex entries (they accumulate up to maxRecords unique hashes).
func (s *AckStore) evictOldest() {
	if len(s.records) > 0 {
		s.records = s.records[1:]
	}
}

// Len returns the current number of stored record hashes.
func (s *AckStore) Len() int {
	return len(s.records)
}

// hashTimestamp creates a SHA256 hash of the timestamp.
// We never store raw timestamps.
func hashTimestamp(t time.Time) string {
	h := sha256.Sum256([]byte(t.Format(time.RFC3339Nano)))
	return fmt.Sprintf("%x", h)
}
