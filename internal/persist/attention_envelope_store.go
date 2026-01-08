// Package persist provides the attention envelope store for Phase 39.
//
// CRITICAL INVARIANTS:
//   - Hash-only storage. No raw content.
//   - One active envelope per circle at a time.
//   - Bounded retention: 30 days, max 200 records.
//   - FIFO eviction when limits exceeded.
//   - Storelog integration required.
//   - No goroutines. Thread-safe with RWMutex.
//
// Reference: docs/ADR/ADR-0076-phase39-attention-envelopes.md
package persist

import (
	"fmt"
	"sort"
	"sync"
	"time"

	ae "quantumlife/pkg/domain/attentionenvelope"
	"quantumlife/pkg/domain/storelog"
)

// AttentionEnvelopeStore persists attention envelopes and receipts.
// CRITICAL: Hash-only. One active per circle. Bounded retention.
type AttentionEnvelopeStore struct {
	mu sync.RWMutex

	// activeEnvelopes stores the current active envelope per circle.
	// Key: circleIDHash, Value: envelope
	activeEnvelopes map[string]*ae.AttentionEnvelope

	// receipts stores all receipts by ID.
	receipts map[string]*ae.EnvelopeReceipt

	// receiptOrder tracks receipts in chronological order for FIFO eviction.
	receiptOrder []string

	// periodOrder tracks periods for retention eviction.
	periodOrder []string

	// storelogRef for replay.
	storelogRef storelog.AppendOnlyLog

	// configuration
	maxRetentionDays int
	maxRecords       int
}

// AttentionEnvelopeStoreConfig configures the store.
type AttentionEnvelopeStoreConfig struct {
	Storelog         storelog.AppendOnlyLog
	MaxRetentionDays int
	MaxRecords       int
}

// DefaultAttentionEnvelopeStoreConfig returns default configuration.
func DefaultAttentionEnvelopeStoreConfig() AttentionEnvelopeStoreConfig {
	return AttentionEnvelopeStoreConfig{
		MaxRetentionDays: ae.MaxRetentionDays,
		MaxRecords:       ae.MaxEnvelopeRecords,
	}
}

// NewAttentionEnvelopeStore creates a new attention envelope store.
func NewAttentionEnvelopeStore(cfg AttentionEnvelopeStoreConfig) *AttentionEnvelopeStore {
	if cfg.MaxRetentionDays <= 0 {
		cfg.MaxRetentionDays = ae.MaxRetentionDays
	}
	if cfg.MaxRecords <= 0 {
		cfg.MaxRecords = ae.MaxEnvelopeRecords
	}

	return &AttentionEnvelopeStore{
		activeEnvelopes:  make(map[string]*ae.AttentionEnvelope),
		receipts:         make(map[string]*ae.EnvelopeReceipt),
		receiptOrder:     make([]string, 0),
		periodOrder:      make([]string, 0),
		storelogRef:      cfg.Storelog,
		maxRetentionDays: cfg.MaxRetentionDays,
		maxRecords:       cfg.MaxRecords,
	}
}

// StartEnvelope stores a new active envelope for a circle.
// CRITICAL: One active envelope per circle. Replaces any existing.
func (s *AttentionEnvelopeStore) StartEnvelope(envelope *ae.AttentionEnvelope) error {
	if envelope == nil {
		return fmt.Errorf("nil envelope")
	}

	if err := envelope.Validate(); err != nil {
		return fmt.Errorf("invalid envelope: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Store as active for this circle (replaces any existing)
	s.activeEnvelopes[envelope.CircleIDHash] = envelope

	// Write to storelog if available
	if s.storelogRef != nil {
		logRecord := storelog.NewRecord(
			storelog.RecordTypeEnvelopeStart,
			time.Now(), // Timestamp for storelog only (not business logic)
			"",         // No specific circle ID in storelog
			envelope.CanonicalString(),
		)
		if err := s.storelogRef.Append(logRecord); err != nil {
			// Log error but do not fail — in-memory state is authoritative
			_ = err
		}
	}

	return nil
}

// StopEnvelope stops the active envelope for a circle.
// Returns the stopped envelope or nil if no active envelope.
func (s *AttentionEnvelopeStore) StopEnvelope(circleIDHash string) *ae.AttentionEnvelope {
	s.mu.Lock()
	defer s.mu.Unlock()

	envelope, exists := s.activeEnvelopes[circleIDHash]
	if !exists {
		return nil
	}

	// Update state to stopped
	envelope.State = ae.StateStopped
	envelope.StatusHash = envelope.ComputeStatusHash()

	// Remove from active
	delete(s.activeEnvelopes, circleIDHash)

	// Write to storelog if available
	if s.storelogRef != nil {
		logRecord := storelog.NewRecord(
			storelog.RecordTypeEnvelopeStop,
			time.Now(),
			"",
			envelope.CanonicalString(),
		)
		if err := s.storelogRef.Append(logRecord); err != nil {
			_ = err
		}
	}

	return envelope
}

// ExpireEnvelope expires the active envelope for a circle if it has expired.
// Returns the expired envelope or nil if no expiry occurred.
func (s *AttentionEnvelopeStore) ExpireEnvelope(circleIDHash string, clock time.Time) *ae.AttentionEnvelope {
	s.mu.Lock()
	defer s.mu.Unlock()

	envelope, exists := s.activeEnvelopes[circleIDHash]
	if !exists {
		return nil
	}

	// Check if expired
	expired, err := ae.IsExpired(envelope.ExpiresAtPeriod, clock)
	if err != nil || !expired {
		return nil
	}

	// Update state to expired
	envelope.State = ae.StateExpired
	envelope.StatusHash = envelope.ComputeStatusHash()

	// Remove from active
	delete(s.activeEnvelopes, circleIDHash)

	// Write to storelog if available
	if s.storelogRef != nil {
		logRecord := storelog.NewRecord(
			storelog.RecordTypeEnvelopeExpire,
			time.Now(),
			"",
			envelope.CanonicalString(),
		)
		if err := s.storelogRef.Append(logRecord); err != nil {
			_ = err
		}
	}

	return envelope
}

// GetActiveEnvelope returns the active envelope for a circle.
// CRITICAL: Also checks expiry against clock and expires if needed.
func (s *AttentionEnvelopeStore) GetActiveEnvelope(circleIDHash string, clock time.Time) *ae.AttentionEnvelope {
	s.mu.Lock()
	defer s.mu.Unlock()

	envelope, exists := s.activeEnvelopes[circleIDHash]
	if !exists {
		return nil
	}

	// Check if expired
	expired, err := ae.IsExpired(envelope.ExpiresAtPeriod, clock)
	if err != nil {
		return nil
	}

	if expired {
		// Expire the envelope
		envelope.State = ae.StateExpired
		envelope.StatusHash = envelope.ComputeStatusHash()
		delete(s.activeEnvelopes, circleIDHash)

		// Write to storelog
		if s.storelogRef != nil {
			logRecord := storelog.NewRecord(
				storelog.RecordTypeEnvelopeExpire,
				time.Now(),
				"",
				envelope.CanonicalString(),
			)
			_ = s.storelogRef.Append(logRecord)
		}

		return nil
	}

	return envelope
}

// HasActiveEnvelope returns whether a circle has an active envelope.
func (s *AttentionEnvelopeStore) HasActiveEnvelope(circleIDHash string, clock time.Time) bool {
	return s.GetActiveEnvelope(circleIDHash, clock) != nil
}

// PersistReceipt stores a receipt.
// CRITICAL: Bounded retention with FIFO eviction.
func (s *AttentionEnvelopeStore) PersistReceipt(receipt *ae.EnvelopeReceipt) error {
	if receipt == nil {
		return fmt.Errorf("nil receipt")
	}

	if err := receipt.Validate(); err != nil {
		return fmt.Errorf("invalid receipt: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicate
	if _, exists := s.receipts[receipt.ReceiptID]; exists {
		return nil // Idempotent
	}

	// Store receipt
	s.receipts[receipt.ReceiptID] = receipt
	s.receiptOrder = append(s.receiptOrder, receipt.ReceiptID)

	// Track period for retention
	periodExists := false
	for _, p := range s.periodOrder {
		if p == receipt.PeriodKey {
			periodExists = true
			break
		}
	}
	if !periodExists {
		s.periodOrder = append(s.periodOrder, receipt.PeriodKey)
		sort.Strings(s.periodOrder)
	}

	// Write to storelog if available
	if s.storelogRef != nil {
		logRecord := storelog.NewRecord(
			storelog.RecordTypeEnvelopeApply, // Use apply for receipt persistence
			time.Now(),
			"",
			receipt.CanonicalString(),
		)
		if err := s.storelogRef.Append(logRecord); err != nil {
			_ = err
		}
	}

	// FIFO eviction if over max records
	s.evictExcessRecordsLocked()

	return nil
}

// GetReceipts returns all receipts for a circle.
func (s *AttentionEnvelopeStore) GetReceipts(circleIDHash string) []*ae.EnvelopeReceipt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*ae.EnvelopeReceipt
	for _, receipt := range s.receipts {
		if receipt.CircleIDHash == circleIDHash {
			result = append(result, receipt)
		}
	}

	// Sort by period then receipt ID for determinism
	sort.Slice(result, func(i, j int) bool {
		if result[i].PeriodKey != result[j].PeriodKey {
			return result[i].PeriodKey < result[j].PeriodKey
		}
		return result[i].ReceiptID < result[j].ReceiptID
	})

	return result
}

// GetRecentReceiptCount returns the count of recent receipts for a circle.
func (s *AttentionEnvelopeStore) GetRecentReceiptCount(circleIDHash string, days int) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cutoff := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

	count := 0
	for _, receipt := range s.receipts {
		if receipt.CircleIDHash == circleIDHash && receipt.PeriodKey >= cutoff {
			count++
		}
	}

	return count
}

// evictExcessRecordsLocked removes oldest records when over max.
// MUST be called with lock held.
func (s *AttentionEnvelopeStore) evictExcessRecordsLocked() {
	for len(s.receiptOrder) > s.maxRecords {
		// Remove oldest (first in order)
		oldestID := s.receiptOrder[0]
		s.receiptOrder = s.receiptOrder[1:]
		delete(s.receipts, oldestID)
	}
}

// EvictOldPeriods explicitly evicts old periods.
// Called with explicit clock for determinism in tests.
func (s *AttentionEnvelopeStore) EvictOldPeriods(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.periodOrder) == 0 {
		return
	}

	cutoff := now.AddDate(0, 0, -s.maxRetentionDays).Format("2006-01-02")

	// Find periods to evict and remove associated receipts
	var newOrder []string
	for _, period := range s.periodOrder {
		if period < cutoff {
			// Remove all receipts for this period
			for id, receipt := range s.receipts {
				if receipt.PeriodKey == period {
					delete(s.receipts, id)
				}
			}
		} else {
			newOrder = append(newOrder, period)
		}
	}

	s.periodOrder = newOrder

	// Rebuild receipt order
	var newReceiptOrder []string
	for _, id := range s.receiptOrder {
		if _, exists := s.receipts[id]; exists {
			newReceiptOrder = append(newReceiptOrder, id)
		}
	}
	s.receiptOrder = newReceiptOrder
}

// TotalReceipts returns the total number of receipts.
func (s *AttentionEnvelopeStore) TotalReceipts() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.receipts)
}

// TotalActiveEnvelopes returns the count of active envelopes.
func (s *AttentionEnvelopeStore) TotalActiveEnvelopes() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.activeEnvelopes)
}

// GetAllActiveCircles returns all circle IDs with active envelopes.
func (s *AttentionEnvelopeStore) GetAllActiveCircles() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]string, 0, len(s.activeEnvelopes))
	for circleID := range s.activeEnvelopes {
		result = append(result, circleID)
	}

	sort.Strings(result)
	return result
}

// SetStorelog sets the storelog reference.
func (s *AttentionEnvelopeStore) SetStorelog(log storelog.AppendOnlyLog) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.storelogRef = log
}

// Clear removes all data from the store.
// Used for testing only.
func (s *AttentionEnvelopeStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activeEnvelopes = make(map[string]*ae.AttentionEnvelope)
	s.receipts = make(map[string]*ae.EnvelopeReceipt)
	s.receiptOrder = make([]string, 0)
	s.periodOrder = make([]string, 0)
}
