// Package persist provides file-backed persistent stores for QuantumLife.
//
// CRITICAL: All stores use append-only logging for durability.
// Changes are written to the log immediately and can be replayed.
//
// GUARDRAIL: No goroutines. All operations are synchronous.
// No time.Now() - clock must be injected.
//
// Reference: docs/ADR/ADR-0027-phase12-persistence-replay.md
package persist

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"quantumlife/pkg/domain/draft"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/storelog"
)

// DraftStore implements draft.Store with file-backed persistence.
type DraftStore struct {
	mu     sync.RWMutex
	log    storelog.AppendOnlyLog
	drafts map[draft.DraftID]draft.Draft
}

// NewDraftStore creates a new file-backed draft store.
func NewDraftStore(log storelog.AppendOnlyLog) (*DraftStore, error) {
	store := &DraftStore{
		log:    log,
		drafts: make(map[draft.DraftID]draft.Draft),
	}

	// Replay existing records
	if err := store.replay(); err != nil {
		return nil, err
	}

	return store, nil
}

// replay loads drafts from the log.
func (s *DraftStore) replay() error {
	records, err := s.log.ListByType(storelog.RecordTypeDraft)
	if err != nil {
		return err
	}

	for _, record := range records {
		d, err := parseDraftPayload(record.Payload)
		if err != nil {
			continue // Skip corrupted records
		}
		s.drafts[d.DraftID] = d
	}

	return nil
}

// Put stores a draft.
func (s *DraftStore) Put(d draft.Draft) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Compute hash if not set
	if d.DeterministicHash == "" {
		d.DeterministicHash = d.Hash()
	}

	// Create log record
	payload := formatDraftPayload(d)
	record := storelog.NewRecord(
		storelog.RecordTypeDraft,
		d.CreatedAt,
		d.CircleID,
		payload,
	)

	// Append to log (may return ErrRecordExists for duplicates)
	if err := s.log.Append(record); err != nil && err != storelog.ErrRecordExists {
		return err
	}

	s.drafts[d.DraftID] = d
	return nil
}

// Get retrieves a draft by ID.
func (s *DraftStore) Get(id draft.DraftID) (draft.Draft, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	d, ok := s.drafts[id]
	return d, ok
}

// GetByDedupKey finds a draft by its dedup key.
func (s *DraftStore) GetByDedupKey(dedupKey string) (draft.Draft, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, d := range s.drafts {
		if d.DedupKey() == dedupKey {
			return d, true
		}
	}
	return draft.Draft{}, false
}

// List returns drafts matching the filter, sorted deterministically.
func (s *DraftStore) List(filter draft.ListFilter) []draft.Draft {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []draft.Draft

	for _, d := range s.drafts {
		// Apply filters
		if filter.CircleID != "" && d.CircleID != filter.CircleID {
			continue
		}
		if filter.Status != "" && d.Status != filter.Status {
			continue
		}
		if filter.DraftType != "" && d.DraftType != filter.DraftType {
			continue
		}
		if !filter.IncludeExpired && d.Status == draft.StatusExpired {
			continue
		}

		result = append(result, d)
	}

	// Sort deterministically
	draft.SortDrafts(result)

	// Apply limit
	if filter.Limit > 0 && len(result) > filter.Limit {
		result = result[:filter.Limit]
	}

	return result
}

// UpdateStatus changes the status of a draft.
func (s *DraftStore) UpdateStatus(id draft.DraftID, status draft.DraftStatus, reason string, changedBy string, changedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	d, ok := s.drafts[id]
	if !ok {
		return fmt.Errorf("draft not found: %s", id)
	}

	if !d.CanTransitionTo(status) {
		return fmt.Errorf("cannot transition from %s to %s", d.Status, status)
	}

	d.Status = status
	d.StatusReason = reason
	d.StatusChangedBy = changedBy
	d.StatusChangedAt = changedAt

	// Create log record for status update
	payload := formatDraftPayload(d)
	record := storelog.NewRecord(
		storelog.RecordTypeDraft,
		changedAt,
		d.CircleID,
		payload,
	)

	// Append to log
	if err := s.log.Append(record); err != nil && err != storelog.ErrRecordExists {
		return err
	}

	s.drafts[id] = d
	return nil
}

// MarkExpired marks all expired drafts as expired based on the given time.
func (s *DraftStore) MarkExpired(now time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for id, d := range s.drafts {
		if d.Status == draft.StatusProposed && d.IsExpired(now) {
			d.Status = draft.StatusExpired
			d.StatusReason = "TTL expired"
			d.StatusChangedAt = now
			d.StatusChangedBy = "system"

			// Create log record for expiration
			payload := formatDraftPayload(d)
			record := storelog.NewRecord(
				storelog.RecordTypeDraft,
				now,
				d.CircleID,
				payload,
			)

			// Append to log (ignore errors for batch operation)
			s.log.Append(record)

			s.drafts[id] = d
			count++
		}
	}

	return count
}

// Count returns the total number of drafts.
func (s *DraftStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.drafts)
}

// CountByCircleAndDay returns the count of drafts for a circle on a day.
func (s *DraftStore) CountByCircleAndDay(circleID identity.EntityID, dayKey string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, d := range s.drafts {
		if d.CircleID == circleID && draft.DayKey(d.CreatedAt) == dayKey {
			count++
		}
	}
	return count
}

// Delete removes a draft (used for dedup replacement).
func (s *DraftStore) Delete(id draft.DraftID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.drafts, id)
}

// Clear removes all drafts from memory (log is preserved).
func (s *DraftStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.drafts = make(map[draft.DraftID]draft.Draft)
}

// Flush ensures all records are persisted.
func (s *DraftStore) Flush() error {
	return s.log.Flush()
}

// DraftIDs returns all draft IDs in sorted order.
func (s *DraftStore) DraftIDs() []draft.DraftID {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := make([]draft.DraftID, 0, len(s.drafts))
	for id := range s.drafts {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		return string(ids[i]) < string(ids[j])
	})
	return ids
}

// formatDraftPayload creates a canonical payload for a draft.
func formatDraftPayload(d draft.Draft) string {
	var b strings.Builder
	b.WriteString("draft")
	b.WriteString("|id:")
	b.WriteString(string(d.DraftID))
	b.WriteString("|type:")
	b.WriteString(string(d.DraftType))
	b.WriteString("|circle:")
	b.WriteString(string(d.CircleID))
	b.WriteString("|status:")
	b.WriteString(string(d.Status))
	b.WriteString("|created:")
	b.WriteString(d.CreatedAt.UTC().Format(time.RFC3339))
	b.WriteString("|expires:")
	b.WriteString(d.ExpiresAt.UTC().Format(time.RFC3339))
	b.WriteString("|content:")
	b.WriteString(d.CanonicalString())
	b.WriteString("|hash:")
	b.WriteString(d.Hash())
	return b.String()
}

// parseDraftPayload parses a canonical payload into a draft.
// Note: This is a simplified parser that extracts key fields.
func parseDraftPayload(payload string) (draft.Draft, error) {
	var d draft.Draft

	parts := strings.Split(payload, "|")
	for _, part := range parts {
		if strings.HasPrefix(part, "id:") {
			d.DraftID = draft.DraftID(part[3:])
		} else if strings.HasPrefix(part, "type:") {
			d.DraftType = draft.DraftType(part[5:])
		} else if strings.HasPrefix(part, "circle:") {
			d.CircleID = identity.EntityID(part[7:])
		} else if strings.HasPrefix(part, "status:") {
			d.Status = draft.DraftStatus(part[7:])
		} else if strings.HasPrefix(part, "created:") {
			t, _ := time.Parse(time.RFC3339, part[8:])
			d.CreatedAt = t
		} else if strings.HasPrefix(part, "expires:") {
			t, _ := time.Parse(time.RFC3339, part[8:])
			d.ExpiresAt = t
		} else if strings.HasPrefix(part, "hash:") {
			d.DeterministicHash = part[5:]
		}
	}

	return d, nil
}

// Verify interface compliance.
var _ draft.Store = (*DraftStore)(nil)
