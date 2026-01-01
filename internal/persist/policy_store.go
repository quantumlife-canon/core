package persist

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"quantumlife/pkg/domain/policy"
	"quantumlife/pkg/domain/storelog"
)

// PolicyStore implements persistent storage for PolicySet.
type PolicyStore struct {
	mu     sync.RWMutex
	log    storelog.AppendOnlyLog
	policy *policy.PolicySet
}

// NewPolicyStore creates a new file-backed policy store.
func NewPolicyStore(log storelog.AppendOnlyLog) (*PolicyStore, error) {
	store := &PolicyStore{
		log: log,
	}

	// Replay existing records
	if err := store.replay(); err != nil {
		return nil, err
	}

	return store, nil
}

// replay loads policy set from the log.
func (s *PolicyStore) replay() error {
	records, err := s.log.ListByType(storelog.RecordTypePolicySet)
	if err != nil {
		return err
	}

	// Find the latest version
	var latest *policy.PolicySet
	for _, record := range records {
		ps, err := parsePolicySetPayload(record.Payload)
		if err != nil {
			continue // Skip corrupted records
		}
		if latest == nil || ps.Version > latest.Version {
			latest = ps
		}
	}

	s.policy = latest
	return nil
}

// Get returns the current policy set.
func (s *PolicyStore) Get() *policy.PolicySet {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.policy
}

// Put stores a new policy set (version must be higher than current).
func (s *PolicyStore) Put(ps *policy.PolicySet) error {
	if err := ps.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check version
	if s.policy != nil && ps.Version <= s.policy.Version {
		return errors.New("policy version must be higher than current")
	}

	// Compute hash if not set
	if ps.Hash == "" {
		ps.ComputeHash()
	}

	// Create log record
	payload := formatPolicySetPayload(ps)
	logRecord := storelog.NewRecord(
		storelog.RecordTypePolicySet,
		ps.CapturedAt,
		"", // No circle ID for global policy set
		payload,
	)

	// Append to log
	if err := s.log.Append(logRecord); err != nil && err != storelog.ErrRecordExists {
		return err
	}

	s.policy = ps
	return nil
}

// UpdateCircle updates a single circle's policy.
func (s *PolicyStore) UpdateCircle(circleID string, mutator func(policy.CirclePolicy) policy.CirclePolicy, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.policy == nil {
		return errors.New("no policy set exists")
	}

	// Get existing or create new
	existing, exists := s.policy.Circles[circleID]
	if !exists {
		existing = policy.MinimalCirclePolicy(circleID)
	}

	// Apply mutation
	updated := mutator(existing)
	if err := updated.Validate(); err != nil {
		return err
	}

	// Create new policy set with incremented version
	newPS := &policy.PolicySet{
		Version:    s.policy.Version + 1,
		CapturedAt: now,
		Circles:    make(map[string]policy.CirclePolicy),
		Triggers:   make(map[string]policy.TriggerPolicy),
	}

	// Copy existing circles
	for k, v := range s.policy.Circles {
		newPS.Circles[k] = v
	}
	// Apply update
	newPS.Circles[circleID] = updated

	// Copy existing triggers
	for k, v := range s.policy.Triggers {
		newPS.Triggers[k] = v
	}

	newPS.ComputeHash()

	// Persist
	payload := formatPolicySetPayload(newPS)
	logRecord := storelog.NewRecord(
		storelog.RecordTypePolicySet,
		now,
		"",
		payload,
	)

	if err := s.log.Append(logRecord); err != nil && err != storelog.ErrRecordExists {
		return err
	}

	s.policy = newPS
	return nil
}

// UpdateTrigger updates a trigger policy.
func (s *PolicyStore) UpdateTrigger(trigger string, tp policy.TriggerPolicy, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.policy == nil {
		return errors.New("no policy set exists")
	}

	if err := tp.Validate(); err != nil {
		return err
	}

	// Create new policy set with incremented version
	newPS := &policy.PolicySet{
		Version:    s.policy.Version + 1,
		CapturedAt: now,
		Circles:    make(map[string]policy.CirclePolicy),
		Triggers:   make(map[string]policy.TriggerPolicy),
	}

	// Copy existing
	for k, v := range s.policy.Circles {
		newPS.Circles[k] = v
	}
	for k, v := range s.policy.Triggers {
		newPS.Triggers[k] = v
	}

	// Apply update
	newPS.Triggers[trigger] = tp
	newPS.ComputeHash()

	// Persist
	payload := formatPolicySetPayload(newPS)
	logRecord := storelog.NewRecord(
		storelog.RecordTypePolicySet,
		now,
		"",
		payload,
	)

	if err := s.log.Append(logRecord); err != nil && err != storelog.ErrRecordExists {
		return err
	}

	s.policy = newPS
	return nil
}

// Hash returns the current policy hash.
func (s *PolicyStore) Hash() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.policy == nil {
		return ""
	}
	return s.policy.Hash
}

// Stats returns policy statistics.
func (s *PolicyStore) Stats() PolicyStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := PolicyStats{}
	if s.policy != nil {
		stats.Version = s.policy.Version
		stats.CircleCount = len(s.policy.Circles)
		stats.TriggerCount = len(s.policy.Triggers)
		stats.Hash = s.policy.Hash
	}
	return stats
}

// PolicyStats contains policy statistics.
type PolicyStats struct {
	Version      int
	CircleCount  int
	TriggerCount int
	Hash         string
}

// Flush ensures all records are persisted.
func (s *PolicyStore) Flush() error {
	return s.log.Flush()
}

// formatPolicySetPayload creates a canonical payload for a policy set.
func formatPolicySetPayload(ps *policy.PolicySet) string {
	var b strings.Builder
	b.WriteString("policy_set")
	b.WriteString("|version:")
	b.WriteString(strconv.Itoa(ps.Version))
	b.WriteString("|captured:")
	b.WriteString(ps.CapturedAt.UTC().Format(time.RFC3339))
	b.WriteString("|hash:")
	b.WriteString(ps.Hash)

	// Serialize circles
	b.WriteString("|circles:")
	circleKeys := sortedStringKeys(ps.Circles)
	for i, key := range circleKeys {
		if i > 0 {
			b.WriteString(";")
		}
		c := ps.Circles[key]
		b.WriteString(fmt.Sprintf("%s:%d:%d:%d:%d:%d",
			c.CircleID, c.RegretThreshold, c.NotifyThreshold,
			c.UrgentThreshold, c.DailyNotifyQuota, c.DailyQueuedQuota))
		if c.Hours != nil {
			b.WriteString(fmt.Sprintf(":%d:%d:%d",
				c.Hours.AllowedWeekdays, c.Hours.StartMinute, c.Hours.EndMinute))
		}
	}

	// Serialize triggers
	b.WriteString("|triggers:")
	triggerKeys := sortedStringKeys(ps.Triggers)
	for i, key := range triggerKeys {
		if i > 0 {
			b.WriteString(";")
		}
		t := ps.Triggers[key]
		suppress := "0"
		if t.SuppressByDefault {
			suppress = "1"
		}
		b.WriteString(fmt.Sprintf("%s:%s:%s:%d",
			t.Trigger, t.MinLevel, suppress, t.RegretBias))
	}

	return b.String()
}

// parsePolicySetPayload parses a canonical payload into a PolicySet.
func parsePolicySetPayload(payload string) (*policy.PolicySet, error) {
	ps := &policy.PolicySet{
		Circles:  make(map[string]policy.CirclePolicy),
		Triggers: make(map[string]policy.TriggerPolicy),
	}

	parts := strings.Split(payload, "|")
	for _, part := range parts {
		if strings.HasPrefix(part, "version:") {
			v, _ := strconv.Atoi(part[8:])
			ps.Version = v
		} else if strings.HasPrefix(part, "captured:") {
			t, _ := time.Parse(time.RFC3339, part[9:])
			ps.CapturedAt = t
		} else if strings.HasPrefix(part, "hash:") {
			ps.Hash = part[5:]
		} else if strings.HasPrefix(part, "circles:") {
			circlesStr := part[8:]
			if circlesStr != "" {
				for _, circleStr := range strings.Split(circlesStr, ";") {
					c := parseCirclePolicy(circleStr)
					if c.CircleID != "" {
						ps.Circles[c.CircleID] = c
					}
				}
			}
		} else if strings.HasPrefix(part, "triggers:") {
			triggersStr := part[9:]
			if triggersStr != "" {
				for _, triggerStr := range strings.Split(triggersStr, ";") {
					t := parseTriggerPolicy(triggerStr)
					if t.Trigger != "" {
						ps.Triggers[t.Trigger] = t
					}
				}
			}
		}
	}

	return ps, nil
}

// parseCirclePolicy parses a circle policy from a colon-separated string.
func parseCirclePolicy(s string) policy.CirclePolicy {
	parts := strings.Split(s, ":")
	if len(parts) < 6 {
		return policy.CirclePolicy{}
	}

	regret, _ := strconv.Atoi(parts[1])
	notify, _ := strconv.Atoi(parts[2])
	urgent, _ := strconv.Atoi(parts[3])
	dailyNotify, _ := strconv.Atoi(parts[4])
	dailyQueued, _ := strconv.Atoi(parts[5])

	cp := policy.CirclePolicy{
		CircleID:         parts[0],
		RegretThreshold:  regret,
		NotifyThreshold:  notify,
		UrgentThreshold:  urgent,
		DailyNotifyQuota: dailyNotify,
		DailyQueuedQuota: dailyQueued,
	}

	// Parse hours if present
	if len(parts) >= 9 {
		weekdays, _ := strconv.Atoi(parts[6])
		start, _ := strconv.Atoi(parts[7])
		end, _ := strconv.Atoi(parts[8])
		cp.Hours = &policy.HoursPolicy{
			AllowedWeekdays: uint8(weekdays),
			StartMinute:     start,
			EndMinute:       end,
		}
	}

	return cp
}

// parseTriggerPolicy parses a trigger policy from a colon-separated string.
func parseTriggerPolicy(s string) policy.TriggerPolicy {
	parts := strings.Split(s, ":")
	if len(parts) < 4 {
		return policy.TriggerPolicy{}
	}

	bias, _ := strconv.Atoi(parts[3])

	return policy.TriggerPolicy{
		Trigger:           parts[0],
		MinLevel:          parts[1],
		SuppressByDefault: parts[2] == "1",
		RegretBias:        bias,
	}
}

// sortedStringKeys returns map keys sorted alphabetically.
func sortedStringKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Bubble sort
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}
