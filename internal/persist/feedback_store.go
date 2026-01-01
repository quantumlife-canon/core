package persist

import (
	"sort"
	"strings"
	"sync"
	"time"

	"quantumlife/pkg/domain/feedback"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/storelog"
)

// FeedbackStore implements feedback.Store with file-backed persistence.
type FeedbackStore struct {
	mu      sync.RWMutex
	log     storelog.AppendOnlyLog
	records map[string]feedback.FeedbackRecord
}

// NewFeedbackStore creates a new file-backed feedback store.
func NewFeedbackStore(log storelog.AppendOnlyLog) (*FeedbackStore, error) {
	store := &FeedbackStore{
		log:     log,
		records: make(map[string]feedback.FeedbackRecord),
	}

	// Replay existing records
	if err := store.replay(); err != nil {
		return nil, err
	}

	return store, nil
}

// replay loads feedback from the log.
func (s *FeedbackStore) replay() error {
	records, err := s.log.ListByType(storelog.RecordTypeFeedback)
	if err != nil {
		return err
	}

	for _, record := range records {
		fr, err := parseFeedbackPayload(record.Payload)
		if err != nil {
			continue // Skip corrupted records
		}
		s.records[fr.FeedbackID] = fr
	}

	return nil
}

// Put stores a feedback record.
func (s *FeedbackStore) Put(record feedback.FeedbackRecord) error {
	if err := record.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Create log record
	payload := formatFeedbackPayload(record)
	logRecord := storelog.NewRecord(
		storelog.RecordTypeFeedback,
		record.CapturedAt,
		record.CircleID,
		payload,
	)

	// Append to log
	if err := s.log.Append(logRecord); err != nil && err != storelog.ErrRecordExists {
		return err
	}

	s.records[record.FeedbackID] = record
	return nil
}

// Get retrieves a feedback record by ID.
func (s *FeedbackStore) Get(id string) (feedback.FeedbackRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, exists := s.records[id]
	return record, exists
}

// GetByTarget retrieves all feedback for a target.
func (s *FeedbackStore) GetByTarget(targetType feedback.FeedbackTargetType, targetID string) []feedback.FeedbackRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []feedback.FeedbackRecord
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
func (s *FeedbackStore) GetByCircle(circleID identity.EntityID) []feedback.FeedbackRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []feedback.FeedbackRecord
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
func (s *FeedbackStore) List() []feedback.FeedbackRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]feedback.FeedbackRecord, 0, len(s.records))
	for _, record := range s.records {
		result = append(result, record)
	}

	// Sort for determinism
	sortFeedbackRecords(result)
	return result
}

// Stats returns feedback statistics.
func (s *FeedbackStore) Stats() feedback.FeedbackStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := feedback.FeedbackStats{
		TotalRecords: len(s.records),
	}

	for _, record := range s.records {
		switch record.TargetType {
		case feedback.TargetInterruption:
			stats.InterruptFeedback++
		case feedback.TargetDraft:
			stats.DraftFeedback++
		}

		switch record.Signal {
		case feedback.SignalHelpful:
			stats.HelpfulCount++
		case feedback.SignalUnnecessary:
			stats.UnnecessaryCount++
		}
	}

	return stats
}

// Flush ensures all records are persisted.
func (s *FeedbackStore) Flush() error {
	return s.log.Flush()
}

// Count returns the total number of records.
func (s *FeedbackStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}

// Clear removes all records from memory.
func (s *FeedbackStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = make(map[string]feedback.FeedbackRecord)
}

// formatFeedbackPayload creates a canonical payload for a feedback record.
func formatFeedbackPayload(r feedback.FeedbackRecord) string {
	var b strings.Builder
	b.WriteString("feedback")
	b.WriteString("|id:")
	b.WriteString(r.FeedbackID)
	b.WriteString("|circle:")
	b.WriteString(string(r.CircleID))
	b.WriteString("|target_type:")
	b.WriteString(string(r.TargetType))
	b.WriteString("|target_id:")
	b.WriteString(r.TargetID)
	b.WriteString("|signal:")
	b.WriteString(string(r.Signal))
	b.WriteString("|captured:")
	b.WriteString(r.CapturedAt.UTC().Format(time.RFC3339))
	if r.Reason != "" {
		b.WriteString("|reason:")
		b.WriteString(escapePayload(r.Reason))
	}
	return b.String()
}

// parseFeedbackPayload parses a canonical payload into a feedback record.
func parseFeedbackPayload(payload string) (feedback.FeedbackRecord, error) {
	var r feedback.FeedbackRecord

	parts := strings.Split(payload, "|")
	for _, part := range parts {
		if strings.HasPrefix(part, "id:") {
			r.FeedbackID = part[3:]
		} else if strings.HasPrefix(part, "circle:") {
			r.CircleID = identity.EntityID(part[7:])
		} else if strings.HasPrefix(part, "target_type:") {
			r.TargetType = feedback.FeedbackTargetType(part[12:])
		} else if strings.HasPrefix(part, "target_id:") {
			r.TargetID = part[10:]
		} else if strings.HasPrefix(part, "signal:") {
			r.Signal = feedback.FeedbackSignal(part[7:])
		} else if strings.HasPrefix(part, "captured:") {
			t, _ := time.Parse(time.RFC3339, part[9:])
			r.CapturedAt = t
		} else if strings.HasPrefix(part, "reason:") {
			r.Reason = unescapePayload(part[7:])
		}
	}

	return r, nil
}

// sortFeedbackRecords sorts feedback records by CapturedAt, then FeedbackID.
func sortFeedbackRecords(records []feedback.FeedbackRecord) {
	sort.Slice(records, func(i, j int) bool {
		if records[i].CapturedAt.Equal(records[j].CapturedAt) {
			return records[i].FeedbackID < records[j].FeedbackID
		}
		return records[i].CapturedAt.Before(records[j].CapturedAt)
	})
}

// escapePayload escapes pipe characters in payload values.
func escapePayload(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "|", "\\|")
	return s
}

// unescapePayload unescapes pipe characters in payload values.
func unescapePayload(s string) string {
	s = strings.ReplaceAll(s, "\\|", "|")
	s = strings.ReplaceAll(s, "\\\\", "\\")
	return s
}

// Verify interface compliance.
var _ feedback.Store = (*FeedbackStore)(nil)
