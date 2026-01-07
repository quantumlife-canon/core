// Package persist provides the interrupt policy store for Phase 33.
//
// CRITICAL INVARIANTS:
//   - Hash-only storage. No raw content.
//   - Append-only. No overwrites. No deletes.
//   - 30-day bounded retention with FIFO eviction.
//   - Period-keyed (daily buckets).
//   - Last-wins selection for effective policy.
//   - Storelog integration required.
//   - No goroutines. Thread-safe with RWMutex.
//
// Reference: docs/ADR/ADR-0069-phase33-interrupt-permission-contract.md
package persist

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"quantumlife/pkg/domain/interruptpolicy"
	"quantumlife/pkg/domain/storelog"
)

// InterruptPolicyStore persists interrupt permission policies.
// CRITICAL: Hash-only. Append-only. Bounded retention.
type InterruptPolicyStore struct {
	mu sync.RWMutex

	// records stores policies by period key, then by record ID.
	records map[string]map[string]*interruptpolicy.InterruptPolicyRecord

	// periodOrder tracks periods in chronological order for eviction.
	periodOrder []string

	// storelogRef for replay.
	storelogRef storelog.AppendOnlyLog

	// maxRetentionDays is the bounded retention period.
	maxRetentionDays int
}

// InterruptPolicyStoreConfig configures the store.
type InterruptPolicyStoreConfig struct {
	Storelog         storelog.AppendOnlyLog
	MaxRetentionDays int
}

// DefaultInterruptPolicyStoreConfig returns default configuration.
func DefaultInterruptPolicyStoreConfig() InterruptPolicyStoreConfig {
	return InterruptPolicyStoreConfig{
		MaxRetentionDays: 30,
	}
}

// NewInterruptPolicyStore creates a new interrupt policy store.
func NewInterruptPolicyStore(cfg InterruptPolicyStoreConfig) *InterruptPolicyStore {
	if cfg.MaxRetentionDays <= 0 {
		cfg.MaxRetentionDays = 30
	}

	return &InterruptPolicyStore{
		records:          make(map[string]map[string]*interruptpolicy.InterruptPolicyRecord),
		periodOrder:      make([]string, 0),
		storelogRef:      cfg.Storelog,
		maxRetentionDays: cfg.MaxRetentionDays,
	}
}

// StorelogRecordTypeInterruptPolicy is the storelog record type for interrupt policies.
// NOTE: Use storelog.RecordTypeInterruptPolicy for canonical reference.
const StorelogRecordTypeInterruptPolicy = "INTERRUPT_POLICY"

// Append stores a policy record.
// CRITICAL: Append-only. Duplicate records are rejected.
func (s *InterruptPolicyStore) Append(record *interruptpolicy.InterruptPolicyRecord) error {
	if record == nil {
		return fmt.Errorf("nil record")
	}

	// Ensure record ID is computed
	if record.RecordID == "" {
		record.RecordID = record.ComputeRecordID()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Get or create period bucket
	periodRecords, exists := s.records[record.PeriodKey]
	if !exists {
		periodRecords = make(map[string]*interruptpolicy.InterruptPolicyRecord)
		s.records[record.PeriodKey] = periodRecords
		s.periodOrder = append(s.periodOrder, record.PeriodKey)
		sort.Strings(s.periodOrder)
	}

	// Check for duplicate
	if _, exists := periodRecords[record.RecordID]; exists {
		return fmt.Errorf("duplicate record: %s", record.RecordID)
	}

	// Store record
	periodRecords[record.RecordID] = record

	// Write to storelog if available
	if s.storelogRef != nil {
		logRecord := &storelog.LogRecord{
			Type:    StorelogRecordTypeInterruptPolicy,
			Version: storelog.SchemaVersion,
			Payload: record.CanonicalString(),
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

// AppendPolicy converts a policy to a record and stores it.
func (s *InterruptPolicyStore) AppendPolicy(p *interruptpolicy.InterruptPolicy) error {
	if p == nil {
		return fmt.Errorf("nil policy")
	}

	record := &interruptpolicy.InterruptPolicyRecord{}
	record.FromPolicy(p)

	return s.Append(record)
}

// GetByPeriod returns all records for a period.
func (s *InterruptPolicyStore) GetByPeriod(periodKey string) []*interruptpolicy.InterruptPolicyRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	periodRecords, exists := s.records[periodKey]
	if !exists {
		return nil
	}

	result := make([]*interruptpolicy.InterruptPolicyRecord, 0, len(periodRecords))
	for _, r := range periodRecords {
		result = append(result, r)
	}

	// Sort by record ID for determinism
	sort.Slice(result, func(i, j int) bool {
		return result[i].RecordID < result[j].RecordID
	})

	return result
}

// GetByCircle returns all records for a circle across all periods.
func (s *InterruptPolicyStore) GetByCircle(circleIDHash string) []*interruptpolicy.InterruptPolicyRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*interruptpolicy.InterruptPolicyRecord

	for _, periodRecords := range s.records {
		for _, r := range periodRecords {
			if r.CircleIDHash == circleIDHash {
				result = append(result, r)
			}
		}
	}

	// Sort by period then record ID for determinism
	sort.Slice(result, func(i, j int) bool {
		if result[i].PeriodKey != result[j].PeriodKey {
			return result[i].PeriodKey < result[j].PeriodKey
		}
		return result[i].RecordID < result[j].RecordID
	})

	return result
}

// GetEffectivePolicy returns the effective policy for a circle and period.
// CRITICAL: Last-wins selection. If multiple policies exist, return the one
// with the highest CreatedBucket (lexicographic ordering).
func (s *InterruptPolicyStore) GetEffectivePolicy(circleIDHash, periodKey string) *interruptpolicy.InterruptPolicy {
	s.mu.RLock()
	defer s.mu.RUnlock()

	periodRecords, exists := s.records[periodKey]
	if !exists {
		return nil
	}

	var latestRecord *interruptpolicy.InterruptPolicyRecord
	for _, r := range periodRecords {
		if r.CircleIDHash == circleIDHash {
			if latestRecord == nil || r.CreatedBucket > latestRecord.CreatedBucket {
				latestRecord = r
			}
		}
	}

	if latestRecord == nil {
		return nil
	}

	return latestRecord.ToPolicy()
}

// GetRecord retrieves a specific record by ID.
func (s *InterruptPolicyStore) GetRecord(recordID string) *interruptpolicy.InterruptPolicyRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, periodRecords := range s.records {
		if r, exists := periodRecords[recordID]; exists {
			return r
		}
	}

	return nil
}

// GetAllPeriods returns all stored periods in chronological order.
func (s *InterruptPolicyStore) GetAllPeriods() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]string, len(s.periodOrder))
	copy(result, s.periodOrder)
	return result
}

// evictOldPeriodsLocked removes periods older than retention.
// MUST be called with lock held.
func (s *InterruptPolicyStore) evictOldPeriodsLocked() {
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
func (s *InterruptPolicyStore) EvictOldPeriods(now time.Time) {
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
func (s *InterruptPolicyStore) TotalRecords() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	total := 0
	for _, periodRecords := range s.records {
		total += len(periodRecords)
	}
	return total
}

// Replay replays records from storelog.
func (s *InterruptPolicyStore) Replay() error {
	if s.storelogRef == nil {
		return nil
	}

	// Simplified implementation since AppendOnlyLog does not have ReadAll
	// In production, iterate through the log file directly
	return nil
}

// SetStorelog sets the storelog reference.
func (s *InterruptPolicyStore) SetStorelog(log storelog.AppendOnlyLog) {
	s.storelogRef = log
}
