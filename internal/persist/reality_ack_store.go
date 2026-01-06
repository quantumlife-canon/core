// Package persist provides persistence for reality page acknowledgements.
//
// Phase 26C: Connected Reality Check
//
// CRITICAL INVARIANTS:
//   - Stores ONLY: period (day bucket) + status_hash
//   - NO identifiers, NO timestamps beyond period key
//   - Bounded retention: 30 days max
//   - Append-only with storelog integration
//   - No goroutines. No time.Now() - clock injection only.
//
// Reference: docs/ADR/ADR-0057-phase26C-connected-reality-check.md
package persist

import (
	"sync"
	"time"

	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/reality"
	"quantumlife/pkg/domain/storelog"
)

// RealityAckStore provides storage for reality page acknowledgements.
//
// CRITICAL: This store does NOT spawn goroutines.
// CRITICAL: All operations are synchronous.
// CRITICAL: Stores ONLY hashes - never identifiers.
type RealityAckStore struct {
	mu         sync.RWMutex
	acks       map[string]*reality.RealityAck // keyed by period
	maxPeriods int
	clock      func() time.Time
	log        storelog.AppendOnlyLog
}

// NewRealityAckStore creates a new reality ack store.
func NewRealityAckStore(clock func() time.Time) *RealityAckStore {
	return &RealityAckStore{
		acks:       make(map[string]*reality.RealityAck),
		maxPeriods: 30, // 30 days bounded retention
		clock:      clock,
	}
}

// NewRealityAckStoreWithLog creates a store backed by an append-only log.
func NewRealityAckStoreWithLog(clock func() time.Time, log storelog.AppendOnlyLog) *RealityAckStore {
	return &RealityAckStore{
		acks:       make(map[string]*reality.RealityAck),
		maxPeriods: 30,
		clock:      clock,
		log:        log,
	}
}

// RecordAck records an acknowledgement for the given period and status hash.
// Idempotent: recording the same ack twice is a no-op.
func (s *RealityAckStore) RecordAck(period string, statusHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if already acked with same hash
	if existing, ok := s.acks[period]; ok {
		if existing.StatusHash == statusHash {
			return nil // Already acked with same hash
		}
		// Different hash - update (material change)
	}

	ack := &reality.RealityAck{
		Period:     period,
		StatusHash: statusHash,
	}

	// Store in memory
	s.acks[period] = ack

	// Enforce bounded retention
	s.enforceBoundedRetention()

	// Append to log if available
	if s.log != nil {
		record := storelog.NewRecord(
			storelog.RecordTypeRealityAck,
			s.clock(),
			identity.EntityID(""), // No circle ID for acks
			ack.CanonicalString(),
		)
		if err := s.log.Append(record); err != nil && err != storelog.ErrRecordExists {
			// Log error but don't fail - memory store is authoritative
			_ = err
		}
	}

	return nil
}

// IsAcked returns true if the given period+hash has been acknowledged.
func (s *RealityAckStore) IsAcked(period string, statusHash string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ack, ok := s.acks[period]
	if !ok {
		return false
	}
	return ack.StatusHash == statusHash
}

// GetAckForPeriod returns the ack for a period, if any.
func (s *RealityAckStore) GetAckForPeriod(period string) *reality.RealityAck {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.acks[period]
}

// Count returns the number of stored acks.
func (s *RealityAckStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.acks)
}

// enforceBoundedRetention ensures we don't exceed maxPeriods.
// Must be called with lock held.
func (s *RealityAckStore) enforceBoundedRetention() {
	if len(s.acks) <= s.maxPeriods {
		return
	}

	// Find oldest periods to remove
	// Since period format is YYYY-MM-DD, lexical sort works
	periods := make([]string, 0, len(s.acks))
	for p := range s.acks {
		periods = append(periods, p)
	}

	// Sort periods (lexical sort works for YYYY-MM-DD)
	for i := 0; i < len(periods); i++ {
		for j := i + 1; j < len(periods); j++ {
			if periods[j] < periods[i] {
				periods[i], periods[j] = periods[j], periods[i]
			}
		}
	}

	// Remove oldest until we're at max
	toRemove := len(periods) - s.maxPeriods
	for i := 0; i < toRemove; i++ {
		delete(s.acks, periods[i])
	}
}

// ReplayFromStorelog replays records from the storelog.
func (s *RealityAckStore) ReplayFromStorelog(entries []*storelog.LogRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, entry := range entries {
		if entry.Type != storelog.RecordTypeRealityAck {
			continue
		}

		ack, err := parseRealityAck(entry.Payload)
		if err != nil {
			continue // Skip corrupted records
		}

		s.acks[ack.Period] = ack
	}

	// Enforce bounded retention after replay
	s.enforceBoundedRetention()

	return nil
}

// parseRealityAck parses a canonical ack string.
// Format: REALITY_ACK|v1|period|status_hash
func parseRealityAck(canonical string) (*reality.RealityAck, error) {
	// Simple pipe-split parsing
	parts := splitPipe(canonical, 4)
	if len(parts) < 4 {
		return nil, storelog.ErrInvalidRecord
	}

	if parts[0] != "REALITY_ACK" || parts[1] != "v1" {
		return nil, storelog.ErrInvalidRecord
	}

	return &reality.RealityAck{
		Period:     parts[2],
		StatusHash: parts[3],
	}, nil
}

// splitPipe splits a string by pipe, returning at most n parts.
func splitPipe(s string, n int) []string {
	if n <= 0 {
		return nil
	}

	result := make([]string, 0, n)
	start := 0

	for i := 0; i < len(s) && len(result) < n-1; i++ {
		if s[i] == '|' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}

	// Last part gets the remainder
	if start <= len(s) {
		result = append(result, s[start:])
	}

	return result
}
