// Package impl_inmem provides in-memory trust tracking.
package impl_inmem

import (
	"context"
	"sync"
	"time"

	"quantumlife/internal/audit"
	"quantumlife/pkg/events"
)

// TrustLevel represents the trust level between parties.
type TrustLevel int

const (
	TrustLevelUnknown TrustLevel = 0
	TrustLevelLow     TrustLevel = 1
	TrustLevelMedium  TrustLevel = 2
	TrustLevelHigh    TrustLevel = 3
)

func (t TrustLevel) String() string {
	switch t {
	case TrustLevelLow:
		return "low"
	case TrustLevelMedium:
		return "medium"
	case TrustLevelHigh:
		return "high"
	default:
		return "unknown"
	}
}

// TrustRecord tracks trust for a party within an intersection.
type TrustRecord struct {
	CircleID       string
	IntersectionID string
	Level          TrustLevel
	Score          int // Raw score: positive for good behavior, negative for bad
	Acceptances    int
	Rejections     int
	Settlements    int
	LastUpdated    time.Time
}

// TrustUpdate represents a trust change event.
type TrustUpdate struct {
	CircleID       string
	IntersectionID string
	OldLevel       TrustLevel
	NewLevel       TrustLevel
	Reason         string
	Timestamp      time.Time
}

// TrustStore manages trust state for parties within intersections.
type TrustStore struct {
	mu          sync.RWMutex
	records     map[string]*TrustRecord // key: intersectionID:circleID
	updates     []TrustUpdate
	auditLogger audit.Logger
}

// NewTrustStore creates a new in-memory trust store.
func NewTrustStore(auditLogger audit.Logger) *TrustStore {
	return &TrustStore{
		records:     make(map[string]*TrustRecord),
		updates:     []TrustUpdate{},
		auditLogger: auditLogger,
	}
}

// getKey creates a key for the records map.
func getKey(intersectionID, circleID string) string {
	return intersectionID + ":" + circleID
}

// GetTrust retrieves trust record for a party.
func (ts *TrustStore) GetTrust(ctx context.Context, intersectionID, circleID string) *TrustRecord {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	key := getKey(intersectionID, circleID)
	if record, exists := ts.records[key]; exists {
		copy := *record
		return &copy
	}

	// Return default if not found
	return &TrustRecord{
		CircleID:       circleID,
		IntersectionID: intersectionID,
		Level:          TrustLevelMedium,
		Score:          0,
	}
}

// RecordAcceptance records a proposal acceptance, which increases trust.
func (ts *TrustStore) RecordAcceptance(ctx context.Context, intersectionID, circleID string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	key := getKey(intersectionID, circleID)
	record := ts.getOrCreate(intersectionID, circleID)

	oldLevel := record.Level
	record.Acceptances++
	record.Score++
	record.LastUpdated = time.Now()

	// Recalculate level
	record.Level = ts.calculateLevel(record.Score)

	if oldLevel != record.Level {
		ts.recordUpdate(ctx, record, oldLevel, "acceptance")
	}

	ts.records[key] = record
}

// RecordRejection records a proposal rejection, which decreases trust.
func (ts *TrustStore) RecordRejection(ctx context.Context, intersectionID, circleID string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	key := getKey(intersectionID, circleID)
	record := ts.getOrCreate(intersectionID, circleID)

	oldLevel := record.Level
	record.Rejections++
	record.Score--
	record.LastUpdated = time.Now()

	// Recalculate level
	record.Level = ts.calculateLevel(record.Score)

	if oldLevel != record.Level {
		ts.recordUpdate(ctx, record, oldLevel, "rejection")
	}

	ts.records[key] = record
}

// RecordSettlement records a successful settlement, which increases trust.
func (ts *TrustStore) RecordSettlement(ctx context.Context, intersectionID, circleID string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	key := getKey(intersectionID, circleID)
	record := ts.getOrCreate(intersectionID, circleID)

	oldLevel := record.Level
	record.Settlements++
	record.Score += 2 // Settlements worth more
	record.LastUpdated = time.Now()

	// Recalculate level
	record.Level = ts.calculateLevel(record.Score)

	if oldLevel != record.Level {
		ts.recordUpdate(ctx, record, oldLevel, "settlement")
	}

	ts.records[key] = record
}

// getOrCreate returns existing record or creates a new one.
func (ts *TrustStore) getOrCreate(intersectionID, circleID string) *TrustRecord {
	key := getKey(intersectionID, circleID)
	if record, exists := ts.records[key]; exists {
		return record
	}

	return &TrustRecord{
		CircleID:       circleID,
		IntersectionID: intersectionID,
		Level:          TrustLevelMedium,
		Score:          0,
	}
}

// calculateLevel determines trust level from score.
func (ts *TrustStore) calculateLevel(score int) TrustLevel {
	if score >= 3 {
		return TrustLevelHigh
	}
	if score >= 0 {
		return TrustLevelMedium
	}
	return TrustLevelLow
}

// recordUpdate logs a trust level change.
func (ts *TrustStore) recordUpdate(ctx context.Context, record *TrustRecord, oldLevel TrustLevel, reason string) {
	update := TrustUpdate{
		CircleID:       record.CircleID,
		IntersectionID: record.IntersectionID,
		OldLevel:       oldLevel,
		NewLevel:       record.Level,
		Reason:         reason,
		Timestamp:      time.Now(),
	}
	ts.updates = append(ts.updates, update)

	// Log audit event
	if ts.auditLogger != nil {
		ts.auditLogger.Log(ctx, audit.Entry{
			CircleID:       record.CircleID,
			IntersectionID: record.IntersectionID,
			EventType:      string(events.EventTrustUpdated),
			SubjectID:      record.CircleID,
			Action:         "trust_updated",
			Outcome:        record.Level.String(),
			Metadata: map[string]string{
				"old_level": oldLevel.String(),
				"new_level": record.Level.String(),
				"reason":    reason,
			},
		})
	}
}

// GetUpdates returns all trust updates (for audit purposes).
func (ts *TrustStore) GetUpdates() []TrustUpdate {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	result := make([]TrustUpdate, len(ts.updates))
	copy(result, ts.updates)
	return result
}

// GetAllRecords returns all trust records (for display purposes).
func (ts *TrustStore) GetAllRecords() []TrustRecord {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	var result []TrustRecord
	for _, record := range ts.records {
		result = append(result, *record)
	}
	return result
}
