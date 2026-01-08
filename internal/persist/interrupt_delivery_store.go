// Package persist provides the interrupt delivery store for Phase 36.
//
// CRITICAL INVARIANTS:
//   - Hash-only storage. No raw content.
//   - Append-only. No overwrites. No deletes (except eviction).
//   - 30-day bounded retention with FIFO eviction.
//   - Period-keyed (daily buckets).
//   - Deduplication by (candidate_hash, period).
//   - Storelog integration required.
//   - No goroutines. Thread-safe with RWMutex.
//
// Reference: docs/ADR/ADR-0073-phase36-interrupt-delivery-orchestrator.md
package persist

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"quantumlife/pkg/domain/interruptdelivery"
	"quantumlife/pkg/domain/storelog"
)

// InterruptDeliveryStore persists delivery attempts and receipts.
// CRITICAL: Hash-only. Append-only. Bounded retention.
type InterruptDeliveryStore struct {
	mu sync.RWMutex

	// attempts stores attempts by period key, then by attempt ID.
	attempts map[string]map[string]*interruptdelivery.DeliveryAttempt

	// receipts stores receipts by period key, then by receipt ID.
	receipts map[string]map[string]*interruptdelivery.DeliveryReceipt

	// acks stores dismissal acknowledgments by period key, then by ack ID.
	acks map[string]map[string]*interruptdelivery.DeliveryAck

	// dedupIndex stores which candidates have been sent per period.
	// Key: period|candidate_hash, Value: attempt_id
	dedupIndex map[string]string

	// periodOrder tracks periods in chronological order for eviction.
	periodOrder []string

	// storelogRef for replay.
	storelogRef storelog.AppendOnlyLog

	// maxRetentionDays is the bounded retention period.
	maxRetentionDays int
}

// InterruptDeliveryStoreConfig configures the store.
type InterruptDeliveryStoreConfig struct {
	Storelog         storelog.AppendOnlyLog
	MaxRetentionDays int
}

// DefaultInterruptDeliveryStoreConfig returns default configuration.
func DefaultInterruptDeliveryStoreConfig() InterruptDeliveryStoreConfig {
	return InterruptDeliveryStoreConfig{
		MaxRetentionDays: 30,
	}
}

// NewInterruptDeliveryStore creates a new interrupt delivery store.
func NewInterruptDeliveryStore(cfg InterruptDeliveryStoreConfig) *InterruptDeliveryStore {
	if cfg.MaxRetentionDays <= 0 {
		cfg.MaxRetentionDays = 30
	}

	return &InterruptDeliveryStore{
		attempts:         make(map[string]map[string]*interruptdelivery.DeliveryAttempt),
		receipts:         make(map[string]map[string]*interruptdelivery.DeliveryReceipt),
		acks:             make(map[string]map[string]*interruptdelivery.DeliveryAck),
		dedupIndex:       make(map[string]string),
		periodOrder:      make([]string, 0),
		storelogRef:      cfg.Storelog,
		maxRetentionDays: cfg.MaxRetentionDays,
	}
}

// Storelog record types for Phase 36.
const (
	StorelogRecordTypeDeliveryAttempt = "DELIVERY_ATTEMPT"
	StorelogRecordTypeDeliveryReceipt = "DELIVERY_RECEIPT"
	StorelogRecordTypeDeliveryAck     = "DELIVERY_ACK"
)

// AppendAttempt stores a delivery attempt.
// CRITICAL: Append-only. Duplicate attempts are rejected.
func (s *InterruptDeliveryStore) AppendAttempt(attempt *interruptdelivery.DeliveryAttempt) error {
	if attempt == nil {
		return fmt.Errorf("nil attempt")
	}

	// Ensure IDs are computed
	if attempt.AttemptID == "" {
		attempt.AttemptID = attempt.ComputeAttemptID()
	}
	if attempt.StatusHash == "" {
		attempt.StatusHash = attempt.ComputeStatusHash()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Get or create period bucket
	periodAttempts, exists := s.attempts[attempt.PeriodKey]
	if !exists {
		periodAttempts = make(map[string]*interruptdelivery.DeliveryAttempt)
		s.attempts[attempt.PeriodKey] = periodAttempts
		s.addPeriodLocked(attempt.PeriodKey)
	}

	// Check for duplicate
	if _, exists := periodAttempts[attempt.AttemptID]; exists {
		return fmt.Errorf("duplicate attempt: %s", attempt.AttemptID)
	}

	// Store attempt
	periodAttempts[attempt.AttemptID] = attempt

	// Update dedup index for sent attempts
	if attempt.ResultBucket == interruptdelivery.ResultSent {
		dedupKey := fmt.Sprintf("%s|%s", attempt.PeriodKey, attempt.CandidateHash)
		s.dedupIndex[dedupKey] = attempt.AttemptID
	}

	// Write to storelog
	s.writeToStorelog(StorelogRecordTypeDeliveryAttempt, attempt.CanonicalString())

	// Evict old periods
	s.evictOldPeriodsLocked()

	return nil
}

// AppendReceipt stores a delivery receipt.
func (s *InterruptDeliveryStore) AppendReceipt(receipt *interruptdelivery.DeliveryReceipt) error {
	if receipt == nil {
		return fmt.Errorf("nil receipt")
	}

	// Ensure IDs are computed
	if receipt.ReceiptID == "" {
		receipt.ReceiptID = receipt.ComputeReceiptID()
	}
	if receipt.StatusHash == "" {
		receipt.StatusHash = receipt.ComputeStatusHash()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Get or create period bucket
	periodReceipts, exists := s.receipts[receipt.PeriodKey]
	if !exists {
		periodReceipts = make(map[string]*interruptdelivery.DeliveryReceipt)
		s.receipts[receipt.PeriodKey] = periodReceipts
		s.addPeriodLocked(receipt.PeriodKey)
	}

	// Check for duplicate
	if _, exists := periodReceipts[receipt.ReceiptID]; exists {
		return fmt.Errorf("duplicate receipt: %s", receipt.ReceiptID)
	}

	// Store receipt
	periodReceipts[receipt.ReceiptID] = receipt

	// Write to storelog
	s.writeToStorelog(StorelogRecordTypeDeliveryReceipt, receipt.CanonicalString())

	return nil
}

// AppendAck stores a delivery acknowledgment.
func (s *InterruptDeliveryStore) AppendAck(ack *interruptdelivery.DeliveryAck) error {
	if ack == nil {
		return fmt.Errorf("nil ack")
	}

	// Ensure ID is computed
	if ack.AckID == "" {
		ack.AckID = ack.ComputeAckID()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Get or create period bucket
	periodAcks, exists := s.acks[ack.PeriodKey]
	if !exists {
		periodAcks = make(map[string]*interruptdelivery.DeliveryAck)
		s.acks[ack.PeriodKey] = periodAcks
		s.addPeriodLocked(ack.PeriodKey)
	}

	// Allow duplicate acks (idempotent)
	periodAcks[ack.AckID] = ack

	// Write to storelog
	s.writeToStorelog(StorelogRecordTypeDeliveryAck, ack.CanonicalString())

	return nil
}

// GetAttemptsByPeriod returns all attempts for a period.
func (s *InterruptDeliveryStore) GetAttemptsByPeriod(periodKey string) []*interruptdelivery.DeliveryAttempt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	periodAttempts, exists := s.attempts[periodKey]
	if !exists {
		return nil
	}

	result := make([]*interruptdelivery.DeliveryAttempt, 0, len(periodAttempts))
	for _, a := range periodAttempts {
		result = append(result, a)
	}

	// Sort by attempt ID for determinism
	sort.Slice(result, func(i, j int) bool {
		return result[i].AttemptID < result[j].AttemptID
	})

	return result
}

// GetReceiptsByPeriod returns all receipts for a period.
func (s *InterruptDeliveryStore) GetReceiptsByPeriod(periodKey string) []*interruptdelivery.DeliveryReceipt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	periodReceipts, exists := s.receipts[periodKey]
	if !exists {
		return nil
	}

	result := make([]*interruptdelivery.DeliveryReceipt, 0, len(periodReceipts))
	for _, r := range periodReceipts {
		result = append(result, r)
	}

	// Sort by receipt ID for determinism
	sort.Slice(result, func(i, j int) bool {
		return result[i].ReceiptID < result[j].ReceiptID
	})

	return result
}

// GetLatestReceipt returns the most recent receipt for a period and circle.
func (s *InterruptDeliveryStore) GetLatestReceipt(periodKey, circleIDHash string) *interruptdelivery.DeliveryReceipt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	periodReceipts, exists := s.receipts[periodKey]
	if !exists {
		return nil
	}

	var latest *interruptdelivery.DeliveryReceipt
	for _, r := range periodReceipts {
		if r.CircleIDHash == circleIDHash {
			if latest == nil || r.ReceiptID > latest.ReceiptID {
				latest = r
			}
		}
	}

	return latest
}

// HasSentCandidate checks if a candidate has already been sent this period.
func (s *InterruptDeliveryStore) HasSentCandidate(periodKey, candidateHash string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dedupKey := fmt.Sprintf("%s|%s", periodKey, candidateHash)
	_, exists := s.dedupIndex[dedupKey]
	return exists
}

// GetSentCandidates returns all sent candidates for a period.
func (s *InterruptDeliveryStore) GetSentCandidates(periodKey string) map[string]bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]bool)
	prefix := periodKey + "|"

	for key := range s.dedupIndex {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			candidateHash := key[len(prefix):]
			result[candidateHash] = true
		}
	}

	return result
}

// CountSentToday counts sent attempts for a period.
func (s *InterruptDeliveryStore) CountSentToday(periodKey string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	periodAttempts, exists := s.attempts[periodKey]
	if !exists {
		return 0
	}

	count := 0
	for _, a := range periodAttempts {
		if a.ResultBucket == interruptdelivery.ResultSent {
			count++
		}
	}

	return count
}

// HasAck checks if a dismissal ack exists for a period and circle.
func (s *InterruptDeliveryStore) HasAck(periodKey, circleIDHash string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	periodAcks, exists := s.acks[periodKey]
	if !exists {
		return false
	}

	for _, a := range periodAcks {
		if a.CircleIDHash == circleIDHash {
			return true
		}
	}

	return false
}

// GetAllPeriods returns all stored periods in chronological order.
func (s *InterruptDeliveryStore) GetAllPeriods() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]string, len(s.periodOrder))
	copy(result, s.periodOrder)
	return result
}

// addPeriodLocked adds a period to the order tracking.
// MUST be called with lock held.
func (s *InterruptDeliveryStore) addPeriodLocked(periodKey string) {
	// Check if already tracked
	for _, p := range s.periodOrder {
		if p == periodKey {
			return
		}
	}

	s.periodOrder = append(s.periodOrder, periodKey)
	sort.Strings(s.periodOrder)
}

// evictOldPeriodsLocked removes periods older than retention.
// MUST be called with lock held.
func (s *InterruptDeliveryStore) evictOldPeriodsLocked() {
	if len(s.periodOrder) == 0 {
		return
	}

	// Calculate cutoff
	cutoff := time.Now().AddDate(0, 0, -s.maxRetentionDays).Format("2006-01-02")

	// Find periods to evict
	var newOrder []string
	for _, period := range s.periodOrder {
		if period < cutoff {
			delete(s.attempts, period)
			delete(s.receipts, period)
			delete(s.acks, period)

			// Clean dedup index
			prefix := period + "|"
			for key := range s.dedupIndex {
				if len(key) > len(prefix) && key[:len(prefix)] == prefix {
					delete(s.dedupIndex, key)
				}
			}
		} else {
			newOrder = append(newOrder, period)
		}
	}

	s.periodOrder = newOrder
}

// EvictOldPeriods explicitly evicts old periods with injected clock.
func (s *InterruptDeliveryStore) EvictOldPeriods(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.periodOrder) == 0 {
		return
	}

	cutoff := now.AddDate(0, 0, -s.maxRetentionDays).Format("2006-01-02")

	var newOrder []string
	for _, period := range s.periodOrder {
		if period < cutoff {
			delete(s.attempts, period)
			delete(s.receipts, period)
			delete(s.acks, period)

			prefix := period + "|"
			for key := range s.dedupIndex {
				if len(key) > len(prefix) && key[:len(prefix)] == prefix {
					delete(s.dedupIndex, key)
				}
			}
		} else {
			newOrder = append(newOrder, period)
		}
	}

	s.periodOrder = newOrder
}

// writeToStorelog writes a record to the storelog.
func (s *InterruptDeliveryStore) writeToStorelog(recordType, content string) {
	if s.storelogRef == nil {
		return
	}

	logRecord := &storelog.LogRecord{
		Type:    recordType,
		Version: storelog.SchemaVersion,
		Payload: content,
	}
	logRecord.Hash = logRecord.ComputeHash()
	_ = s.storelogRef.Append(logRecord)
}

// SetStorelog sets the storelog reference.
func (s *InterruptDeliveryStore) SetStorelog(log storelog.AppendOnlyLog) {
	s.storelogRef = log
}

// TotalAttempts returns the total number of attempts across all periods.
func (s *InterruptDeliveryStore) TotalAttempts() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	total := 0
	for _, periodAttempts := range s.attempts {
		total += len(periodAttempts)
	}
	return total
}

// TotalReceipts returns the total number of receipts across all periods.
func (s *InterruptDeliveryStore) TotalReceipts() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	total := 0
	for _, periodReceipts := range s.receipts {
		total += len(periodReceipts)
	}
	return total
}
