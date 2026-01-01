package persist

import (
	"strings"
	"sync"
	"time"

	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/storelog"
	"quantumlife/pkg/domain/suppress"
)

// SuppressStore implements persistent storage for suppression rules.
type SuppressStore struct {
	mu  sync.RWMutex
	log storelog.AppendOnlyLog
	set *suppress.SuppressionSet
}

// NewSuppressStore creates a new file-backed suppression store.
func NewSuppressStore(log storelog.AppendOnlyLog) (*SuppressStore, error) {
	store := &SuppressStore{
		log: log,
		set: suppress.NewSuppressionSet(),
	}

	// Replay existing records
	if err := store.replay(); err != nil {
		return nil, err
	}

	return store, nil
}

// replay loads suppression rules from the log.
func (s *SuppressStore) replay() error {
	// Get all suppression records
	addRecords, err := s.log.ListByType(storelog.RecordTypeSuppressionAdd)
	if err != nil {
		return err
	}

	remRecords, err := s.log.ListByType(storelog.RecordTypeSuppressionRem)
	if err != nil {
		return err
	}

	// Build a map of removed rule IDs
	removed := make(map[string]bool)
	for _, record := range remRecords {
		ruleID := parseSuppressionRemPayload(record.Payload)
		if ruleID != "" {
			removed[ruleID] = true
		}
	}

	// Add non-removed rules
	for _, record := range addRecords {
		rule, err := parseSuppressionAddPayload(record.Payload)
		if err != nil {
			continue // Skip corrupted records
		}
		if !removed[rule.RuleID] {
			s.set.Rules = append(s.set.Rules, rule)
		}
	}

	// Sort and recompute hash
	s.set.ComputeHash()
	return nil
}

// Get returns the current suppression set.
func (s *SuppressStore) Get() *suppress.SuppressionSet {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.set
}

// ListRules returns all rules for a circle.
func (s *SuppressStore) ListRules(circleID string) []suppress.SuppressionRule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.set.ListByCircle(circleID)
}

// ListActive returns all active rules at the given time.
func (s *SuppressStore) ListActive(at time.Time) []suppress.SuppressionRule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.set.ListActive(at)
}

// AddRule adds a new suppression rule.
func (s *SuppressStore) AddRule(rule suppress.SuppressionRule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create log record
	payload := formatSuppressionAddPayload(rule)
	logRecord := storelog.NewRecord(
		storelog.RecordTypeSuppressionAdd,
		rule.CreatedAt,
		identity.EntityID(rule.CircleID),
		payload,
	)

	// Append to log
	if err := s.log.Append(logRecord); err != nil && err != storelog.ErrRecordExists {
		return err
	}

	s.set.AddRule(rule)
	return nil
}

// RemoveRule removes a suppression rule by ID.
func (s *SuppressStore) RemoveRule(ruleID string, removedAt time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if rule exists
	rule := s.set.GetRule(ruleID)
	if rule == nil {
		return false
	}

	// Create log record
	payload := formatSuppressionRemPayload(ruleID, removedAt)
	logRecord := storelog.NewRecord(
		storelog.RecordTypeSuppressionRem,
		removedAt,
		identity.EntityID(rule.CircleID),
		payload,
	)

	// Append to log
	if err := s.log.Append(logRecord); err != nil && err != storelog.ErrRecordExists {
		return false
	}

	return s.set.RemoveRule(ruleID)
}

// FindMatch returns the first active rule matching the criteria.
func (s *SuppressStore) FindMatch(at time.Time, circleID string, scope suppress.Scope, key string) *suppress.SuppressionRule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.set.FindMatch(at, circleID, scope, key)
}

// PruneExpired removes all expired rules and persists the removals.
func (s *SuppressStore) PruneExpired(at time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	pruned := 0
	for _, rule := range s.set.Rules {
		if !rule.IsActive(at) {
			// Persist removal
			payload := formatSuppressionRemPayload(rule.RuleID, at)
			logRecord := storelog.NewRecord(
				storelog.RecordTypeSuppressionRem,
				at,
				identity.EntityID(rule.CircleID),
				payload,
			)
			s.log.Append(logRecord)
			pruned++
		}
	}

	// Actually prune from set
	s.set.PruneExpired(at)
	return pruned
}

// Hash returns the current suppression set hash.
func (s *SuppressStore) Hash() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.set.Hash
}

// Stats returns suppression statistics.
func (s *SuppressStore) Stats(at time.Time) suppress.Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.set.GetStats(at)
}

// Count returns the total number of rules.
func (s *SuppressStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.set.Rules)
}

// Flush ensures all records are persisted.
func (s *SuppressStore) Flush() error {
	return s.log.Flush()
}

// formatSuppressionAddPayload creates a canonical payload for a suppression rule.
func formatSuppressionAddPayload(r suppress.SuppressionRule) string {
	var b strings.Builder
	b.WriteString("suppress_add")
	b.WriteString("|rule_id:")
	b.WriteString(r.RuleID)
	b.WriteString("|circle:")
	b.WriteString(r.CircleID)
	b.WriteString("|scope:")
	b.WriteString(string(r.Scope))
	b.WriteString("|key:")
	b.WriteString(escapePayload(r.Key))
	b.WriteString("|created:")
	b.WriteString(r.CreatedAt.UTC().Format(time.RFC3339))
	b.WriteString("|expires:")
	if r.ExpiresAt != nil {
		b.WriteString(r.ExpiresAt.UTC().Format(time.RFC3339))
	} else {
		b.WriteString("permanent")
	}
	b.WriteString("|reason:")
	b.WriteString(escapePayload(r.Reason))
	b.WriteString("|source:")
	b.WriteString(string(r.Source))
	return b.String()
}

// parseSuppressionAddPayload parses a canonical payload into a suppression rule.
func parseSuppressionAddPayload(payload string) (suppress.SuppressionRule, error) {
	var r suppress.SuppressionRule

	parts := strings.Split(payload, "|")
	for _, part := range parts {
		if strings.HasPrefix(part, "rule_id:") {
			r.RuleID = part[8:]
		} else if strings.HasPrefix(part, "circle:") {
			r.CircleID = part[7:]
		} else if strings.HasPrefix(part, "scope:") {
			r.Scope = suppress.Scope(part[6:])
		} else if strings.HasPrefix(part, "key:") {
			r.Key = unescapePayload(part[4:])
		} else if strings.HasPrefix(part, "created:") {
			t, _ := time.Parse(time.RFC3339, part[8:])
			r.CreatedAt = t
		} else if strings.HasPrefix(part, "expires:") {
			expiresStr := part[8:]
			if expiresStr != "permanent" {
				t, _ := time.Parse(time.RFC3339, expiresStr)
				r.ExpiresAt = &t
			}
		} else if strings.HasPrefix(part, "reason:") {
			r.Reason = unescapePayload(part[7:])
		} else if strings.HasPrefix(part, "source:") {
			r.Source = suppress.Source(part[7:])
		}
	}

	return r, nil
}

// formatSuppressionRemPayload creates a canonical payload for a rule removal.
func formatSuppressionRemPayload(ruleID string, removedAt time.Time) string {
	var b strings.Builder
	b.WriteString("suppress_rem")
	b.WriteString("|rule_id:")
	b.WriteString(ruleID)
	b.WriteString("|removed:")
	b.WriteString(removedAt.UTC().Format(time.RFC3339))
	return b.String()
}

// parseSuppressionRemPayload parses a canonical payload for a rule removal.
func parseSuppressionRemPayload(payload string) string {
	parts := strings.Split(payload, "|")
	for _, part := range parts {
		if strings.HasPrefix(part, "rule_id:") {
			return part[8:]
		}
	}
	return ""
}
