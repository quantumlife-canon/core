// Package persist provides persistence for shadow receipts.
//
// Phase 19.2: LLM Shadow Mode Contract
//
// CRITICAL: Stores ONLY abstract data (buckets, hashes) - never raw content.
// CRITICAL: Append-only storage - records are never modified or deleted.
// CRITICAL: Supports replay for determinism verification.
// CRITICAL: No goroutines. No time.Now() - clock injection only.
//
// Reference: docs/ADR/ADR-0043-phase19-2-shadow-mode-contract.md
package persist

import (
	"sync"
	"time"

	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/shadowllm"
	"quantumlife/pkg/domain/storelog"
)

// ShadowReceiptStore provides append-only storage for shadow receipts.
//
// CRITICAL: This store does NOT spawn goroutines.
// CRITICAL: All operations are synchronous.
type ShadowReceiptStore struct {
	mu       sync.RWMutex
	receipts map[string]*shadowllm.ShadowReceipt // keyed by receipt ID
	log      storelog.AppendOnlyLog              // optional backing log
	clock    func() time.Time
}

// NewShadowReceiptStore creates a new shadow receipt store.
func NewShadowReceiptStore(clk func() time.Time) *ShadowReceiptStore {
	return &ShadowReceiptStore{
		receipts: make(map[string]*shadowllm.ShadowReceipt),
		clock:    clk,
	}
}

// NewShadowReceiptStoreWithLog creates a store backed by an append-only log.
func NewShadowReceiptStoreWithLog(clk func() time.Time, log storelog.AppendOnlyLog) *ShadowReceiptStore {
	return &ShadowReceiptStore{
		receipts: make(map[string]*shadowllm.ShadowReceipt),
		log:      log,
		clock:    clk,
	}
}

// Append stores a shadow receipt.
//
// CRITICAL: This is idempotent - storing the same receipt twice is a no-op.
// CRITICAL: Does NOT modify the receipt.
func (s *ShadowReceiptStore) Append(receipt *shadowllm.ShadowReceipt) error {
	if err := receipt.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicate
	if _, exists := s.receipts[receipt.ReceiptID]; exists {
		return nil // Idempotent
	}

	// Store in memory
	s.receipts[receipt.ReceiptID] = receipt

	// Append to log if available
	if s.log != nil {
		record := storelog.NewRecord(
			storelog.RecordTypeShadowLLMReceipt,
			s.clock(),
			receipt.CircleID,
			receipt.CanonicalString(),
		)
		if err := s.log.Append(record); err != nil {
			// Log error but don't fail - memory store is authoritative
			// In production, would need better error handling
			_ = err
		}
	}

	return nil
}

// GetByID retrieves a receipt by its ID.
func (s *ShadowReceiptStore) GetByID(receiptID string) (*shadowllm.ShadowReceipt, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	receipt, ok := s.receipts[receiptID]
	return receipt, ok
}

// GetLatestForCircle retrieves the most recent receipt for a circle.
func (s *ShadowReceiptStore) GetLatestForCircle(circleID identity.EntityID) (*shadowllm.ShadowReceipt, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var latest *shadowllm.ShadowReceipt
	for _, receipt := range s.receipts {
		if receipt.CircleID != circleID {
			continue
		}
		if latest == nil || receipt.CreatedAt.After(latest.CreatedAt) {
			latest = receipt
		}
	}

	if latest == nil {
		return nil, false
	}
	return latest, true
}

// ListForCircle returns all receipts for a circle, sorted by creation time.
func (s *ShadowReceiptStore) ListForCircle(circleID identity.EntityID) []*shadowllm.ShadowReceipt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*shadowllm.ShadowReceipt
	for _, receipt := range s.receipts {
		if receipt.CircleID == circleID {
			result = append(result, receipt)
		}
	}

	// Sort by creation time (oldest first)
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].CreatedAt.Before(result[i].CreatedAt) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}

// Count returns the total number of stored receipts.
func (s *ShadowReceiptStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.receipts)
}

// Stats returns summary statistics about stored receipts.
func (s *ShadowReceiptStore) Stats() ShadowReceiptStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := ShadowReceiptStats{
		TotalReceipts:      len(s.receipts),
		SuggestionsByType:  make(map[shadowllm.SuggestionType]int),
		ReceiptsByCategory: make(map[shadowllm.AbstractCategory]int),
	}

	for _, receipt := range s.receipts {
		for _, sug := range receipt.Suggestions {
			stats.SuggestionsByType[sug.SuggestionType]++
			stats.ReceiptsByCategory[sug.Category]++
			stats.TotalSuggestions++
		}
	}

	return stats
}

// ShadowReceiptStats contains summary statistics.
//
// CRITICAL: Contains ONLY aggregate counts - no identifiable info.
type ShadowReceiptStats struct {
	// TotalReceipts is the total number of stored receipts.
	TotalReceipts int

	// TotalSuggestions is the total number of suggestions across all receipts.
	TotalSuggestions int

	// SuggestionsByType maps suggestion type => count.
	SuggestionsByType map[shadowllm.SuggestionType]int

	// ReceiptsByCategory maps category => count of receipts mentioning it.
	ReceiptsByCategory map[shadowllm.AbstractCategory]int
}

// VerifyHash checks if a receipt's hash matches the expected value.
// Used for replay verification.
func (s *ShadowReceiptStore) VerifyHash(receiptID string, expectedHash string) bool {
	receipt, ok := s.GetByID(receiptID)
	if !ok {
		return false
	}
	return receipt.Hash() == expectedHash
}
