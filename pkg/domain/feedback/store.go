package feedback

import (
	"sort"
	"sync"

	"quantumlife/pkg/domain/identity"
)

// Store provides feedback storage operations.
type Store interface {
	// Put stores a feedback record.
	Put(record FeedbackRecord) error

	// Get retrieves a feedback record by ID.
	Get(id string) (FeedbackRecord, bool)

	// GetByTarget retrieves all feedback for a target.
	GetByTarget(targetType FeedbackTargetType, targetID string) []FeedbackRecord

	// GetByCircle retrieves all feedback for a circle.
	GetByCircle(circleID identity.EntityID) []FeedbackRecord

	// List lists all feedback records in deterministic order.
	List() []FeedbackRecord

	// Stats returns feedback statistics.
	Stats() FeedbackStats
}

// FeedbackStats contains feedback statistics.
type FeedbackStats struct {
	TotalRecords      int
	InterruptFeedback int
	DraftFeedback     int
	HelpfulCount      int
	UnnecessaryCount  int
}

// MemoryStore is an in-memory feedback store.
type MemoryStore struct {
	mu      sync.RWMutex
	records map[string]FeedbackRecord
}

// NewMemoryStore creates a new in-memory feedback store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		records: make(map[string]FeedbackRecord),
	}
}

// Put stores a feedback record.
func (s *MemoryStore) Put(record FeedbackRecord) error {
	if err := record.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.records[record.FeedbackID] = record
	return nil
}

// Get retrieves a feedback record by ID.
func (s *MemoryStore) Get(id string) (FeedbackRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, exists := s.records[id]
	return record, exists
}

// GetByTarget retrieves all feedback for a target.
func (s *MemoryStore) GetByTarget(targetType FeedbackTargetType, targetID string) []FeedbackRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []FeedbackRecord
	for _, record := range s.records {
		if record.TargetType == targetType && record.TargetID == targetID {
			result = append(result, record)
		}
	}

	// Sort for determinism
	sortFeedbackRecords(result)
	return result
}

// GetByCircle retrieves all feedback for a circle.
func (s *MemoryStore) GetByCircle(circleID identity.EntityID) []FeedbackRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []FeedbackRecord
	for _, record := range s.records {
		if record.CircleID == circleID {
			result = append(result, record)
		}
	}

	// Sort for determinism
	sortFeedbackRecords(result)
	return result
}

// List lists all feedback records in deterministic order.
func (s *MemoryStore) List() []FeedbackRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]FeedbackRecord, 0, len(s.records))
	for _, record := range s.records {
		result = append(result, record)
	}

	// Sort for determinism
	sortFeedbackRecords(result)
	return result
}

// Stats returns feedback statistics.
func (s *MemoryStore) Stats() FeedbackStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := FeedbackStats{
		TotalRecords: len(s.records),
	}

	for _, record := range s.records {
		switch record.TargetType {
		case TargetInterruption:
			stats.InterruptFeedback++
		case TargetDraft:
			stats.DraftFeedback++
		}

		switch record.Signal {
		case SignalHelpful:
			stats.HelpfulCount++
		case SignalUnnecessary:
			stats.UnnecessaryCount++
		}
	}

	return stats
}

// sortFeedbackRecords sorts feedback records by CapturedAt, then FeedbackID.
func sortFeedbackRecords(records []FeedbackRecord) {
	sort.Slice(records, func(i, j int) bool {
		if records[i].CapturedAt.Equal(records[j].CapturedAt) {
			return records[i].FeedbackID < records[j].FeedbackID
		}
		return records[i].CapturedAt.Before(records[j].CapturedAt)
	})
}

// Verify interface compliance.
var _ Store = (*MemoryStore)(nil)
