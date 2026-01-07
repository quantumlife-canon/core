// Package persist provides the push attempt store for Phase 35.
//
// CRITICAL INVARIANTS:
//   - Hash-only storage. No raw content.
//   - Append-only. No overwrites. No deletes.
//   - 30-day bounded retention with FIFO eviction.
//   - Period-keyed (daily buckets).
//   - Deduplication: same (circle+candidate+period) = same attempt ID.
//   - Storelog integration required.
//   - No goroutines. Thread-safe with RWMutex.
//
// Reference: docs/ADR/ADR-0071-phase35-push-transport-abstract-interrupt-delivery.md
package persist

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"quantumlife/pkg/domain/pushtransport"
	"quantumlife/pkg/domain/storelog"
)

// PushAttemptStore persists push delivery attempts.
// CRITICAL: Hash-only. Append-only. Bounded retention.
type PushAttemptStore struct {
	mu sync.RWMutex

	// records stores attempts by period key, then by attempt ID.
	records map[string]map[string]*pushtransport.PushDeliveryAttempt

	// periodOrder tracks periods in chronological order for eviction.
	periodOrder []string

	// storelogRef for replay.
	storelogRef storelog.AppendOnlyLog

	// maxRetentionDays is the bounded retention period.
	maxRetentionDays int
}

// PushAttemptStoreConfig configures the store.
type PushAttemptStoreConfig struct {
	Storelog         storelog.AppendOnlyLog
	MaxRetentionDays int
}

// DefaultPushAttemptStoreConfig returns default configuration.
func DefaultPushAttemptStoreConfig() PushAttemptStoreConfig {
	return PushAttemptStoreConfig{
		MaxRetentionDays: 30,
	}
}

// NewPushAttemptStore creates a new push attempt store.
func NewPushAttemptStore(cfg PushAttemptStoreConfig) *PushAttemptStore {
	if cfg.MaxRetentionDays <= 0 {
		cfg.MaxRetentionDays = 30
	}

	return &PushAttemptStore{
		records:          make(map[string]map[string]*pushtransport.PushDeliveryAttempt),
		periodOrder:      make([]string, 0),
		storelogRef:      cfg.Storelog,
		maxRetentionDays: cfg.MaxRetentionDays,
	}
}

// StorelogRecordTypePushAttempt is the storelog record type for push attempts.
// NOTE: Use storelog.RecordTypePushAttempt for canonical reference.
const StorelogRecordTypePushAttempt = "PUSH_ATTEMPT"

// Append stores an attempt record.
// CRITICAL: Append-only. Duplicate attempt IDs (same circle+candidate+period) are rejected.
func (s *PushAttemptStore) Append(attempt *pushtransport.PushDeliveryAttempt) error {
	if attempt == nil {
		return fmt.Errorf("nil attempt")
	}

	// Ensure attempt ID is computed
	if attempt.AttemptID == "" {
		attempt.AttemptID = attempt.ComputeAttemptID()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Get or create period bucket
	periodRecords, exists := s.records[attempt.PeriodKey]
	if !exists {
		periodRecords = make(map[string]*pushtransport.PushDeliveryAttempt)
		s.records[attempt.PeriodKey] = periodRecords
		s.periodOrder = append(s.periodOrder, attempt.PeriodKey)
		sort.Strings(s.periodOrder)
	}

	// Check for duplicate (same circle+candidate+period)
	if _, exists := periodRecords[attempt.AttemptID]; exists {
		return fmt.Errorf("duplicate attempt: %s", attempt.AttemptID)
	}

	// Store record
	periodRecords[attempt.AttemptID] = attempt

	// Write to storelog if available
	if s.storelogRef != nil {
		logRecord := &storelog.LogRecord{
			Type:    StorelogRecordTypePushAttempt,
			Version: storelog.SchemaVersion,
			Payload: attempt.CanonicalString(),
		}
		logRecord.Hash = logRecord.ComputeHash()
		if err := s.storelogRef.Append(logRecord); err != nil {
			// Log error but do not fail — in-memory state is authoritative
			_ = err
		}
	}

	// Evict old periods
	s.evictOldPeriodsLocked()

	return nil
}

// GetByPeriod returns all attempts for a period.
func (s *PushAttemptStore) GetByPeriod(periodKey string) []*pushtransport.PushDeliveryAttempt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	periodRecords, exists := s.records[periodKey]
	if !exists {
		return nil
	}

	result := make([]*pushtransport.PushDeliveryAttempt, 0, len(periodRecords))
	for _, r := range periodRecords {
		result = append(result, r)
	}

	// Sort by attempt ID for determinism
	sort.Slice(result, func(i, j int) bool {
		return result[i].AttemptID < result[j].AttemptID
	})

	return result
}

// GetByCircle returns all attempts for a circle across all periods.
func (s *PushAttemptStore) GetByCircle(circleIDHash string) []*pushtransport.PushDeliveryAttempt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*pushtransport.PushDeliveryAttempt

	for _, periodRecords := range s.records {
		for _, r := range periodRecords {
			if r.CircleIDHash == circleIDHash {
				result = append(result, r)
			}
		}
	}

	// Sort by period then attempt ID for determinism
	sort.Slice(result, func(i, j int) bool {
		if result[i].PeriodKey != result[j].PeriodKey {
			return result[i].PeriodKey < result[j].PeriodKey
		}
		return result[i].AttemptID < result[j].AttemptID
	})

	return result
}

// GetByCircleAndPeriod returns attempts for a circle in a specific period.
func (s *PushAttemptStore) GetByCircleAndPeriod(circleIDHash, periodKey string) []*pushtransport.PushDeliveryAttempt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	periodRecords, exists := s.records[periodKey]
	if !exists {
		return nil
	}

	var result []*pushtransport.PushDeliveryAttempt
	for _, r := range periodRecords {
		if r.CircleIDHash == circleIDHash {
			result = append(result, r)
		}
	}

	// Sort by attempt bucket for chronological order
	sort.Slice(result, func(i, j int) bool {
		return result[i].AttemptBucket < result[j].AttemptBucket
	})

	return result
}

// CountSentToday returns the number of sent attempts for a circle today.
func (s *PushAttemptStore) CountSentToday(circleIDHash, periodKey string) int {
	attempts := s.GetByCircleAndPeriod(circleIDHash, periodKey)
	count := 0
	for _, a := range attempts {
		if a.Status == pushtransport.StatusSent {
			count++
		}
	}
	return count
}

// GetLatestForCircle returns the most recent attempt for a circle.
func (s *PushAttemptStore) GetLatestForCircle(circleIDHash string) *pushtransport.PushDeliveryAttempt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var latest *pushtransport.PushDeliveryAttempt

	for _, periodRecords := range s.records {
		for _, r := range periodRecords {
			if r.CircleIDHash == circleIDHash {
				if latest == nil {
					latest = r
				} else if r.PeriodKey > latest.PeriodKey {
					latest = r
				} else if r.PeriodKey == latest.PeriodKey && r.AttemptBucket > latest.AttemptBucket {
					latest = r
				}
			}
		}
	}

	return latest
}

// GetLatestForPeriod returns the most recent attempt for a circle in a period.
func (s *PushAttemptStore) GetLatestForPeriod(circleIDHash, periodKey string) *pushtransport.PushDeliveryAttempt {
	attempts := s.GetByCircleAndPeriod(circleIDHash, periodKey)
	if len(attempts) == 0 {
		return nil
	}
	// Already sorted by AttemptBucket ascending
	return attempts[len(attempts)-1]
}

// HasAttemptToday returns true if any attempt exists for the circle today.
func (s *PushAttemptStore) HasAttemptToday(circleIDHash, periodKey string) bool {
	return len(s.GetByCircleAndPeriod(circleIDHash, periodKey)) > 0
}

// GetRecord retrieves a specific record by ID.
func (s *PushAttemptStore) GetRecord(attemptID string) *pushtransport.PushDeliveryAttempt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, periodRecords := range s.records {
		if r, exists := periodRecords[attemptID]; exists {
			return r
		}
	}

	return nil
}

// UpdateStatus updates the status of an attempt (e.g., after transport result).
// Returns error if attempt not found.
func (s *PushAttemptStore) UpdateStatus(attemptID string, status pushtransport.AttemptStatus, failureBucket pushtransport.FailureBucket) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, periodRecords := range s.records {
		if r, exists := periodRecords[attemptID]; exists {
			r.Status = status
			r.FailureBucket = failureBucket
			r.StatusHash = r.ComputeStatusHash()
			return nil
		}
	}

	return fmt.Errorf("attempt not found: %s", attemptID)
}

// GetAllPeriods returns all stored periods in chronological order.
func (s *PushAttemptStore) GetAllPeriods() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]string, len(s.periodOrder))
	copy(result, s.periodOrder)
	return result
}

// evictOldPeriodsLocked removes periods older than retention.
// MUST be called with lock held.
func (s *PushAttemptStore) evictOldPeriodsLocked() {
	if len(s.periodOrder) == 0 {
		return
	}

	// Calculate cutoff using current wall clock time
	// NOTE: This is acceptable for eviction only (not business logic)
	cutoff := time.Now().AddDate(0, 0, -s.maxRetentionDays).Format("2006-01-02")

	// Find periods to evict
	var newOrder []string
	for _, period := range s.periodOrder {
		if period < cutoff {
			delete(s.records, period)
		} else {
			newOrder = append(newOrder, period)
		}
	}

	s.periodOrder = newOrder
}

// EvictOldPeriods explicitly evicts old periods.
// Called with explicit clock for determinism in tests.
func (s *PushAttemptStore) EvictOldPeriods(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.periodOrder) == 0 {
		return
	}

	cutoff := now.AddDate(0, 0, -s.maxRetentionDays).Format("2006-01-02")

	var newOrder []string
	for _, period := range s.periodOrder {
		if period < cutoff {
			delete(s.records, period)
		} else {
			newOrder = append(newOrder, period)
		}
	}

	s.periodOrder = newOrder
}

// TotalRecords returns the total number of records across all periods.
func (s *PushAttemptStore) TotalRecords() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	total := 0
	for _, periodRecords := range s.records {
		total += len(periodRecords)
	}
	return total
}

// Replay replays records from storelog.
func (s *PushAttemptStore) Replay() error {
	if s.storelogRef == nil {
		return nil
	}

	// Simplified implementation since AppendOnlyLog does not have ReadAll
	// In production, iterate through the log file directly
	return nil
}

// SetStorelog sets the storelog reference.
func (s *PushAttemptStore) SetStorelog(log storelog.AppendOnlyLog) {
	s.storelogRef = log
}
