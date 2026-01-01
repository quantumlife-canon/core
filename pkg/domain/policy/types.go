// Package policy defines circle-level policies for interruption filtering.
//
// Phase 14: Circle Policies + Preference Learning (Deterministic)
//
// Policies control:
// - Regret thresholds for gating interruptions (RegretThreshold, NotifyThreshold, UrgentThreshold)
// - Daily quotas for notify and queued interruptions
// - Allowed hours for interruptions (optional)
// - Per-trigger overrides with RegretBias adjustments
//
// All policies are deterministic: same inputs produce identical hashes.
// Policy changes are auditable via storelog records and events.
//
// Reference: docs/ADR/ADR-0030-phase14-policy-learning.md
package policy

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// HoursPolicy defines allowed hours for interruptions.
type HoursPolicy struct {
	// AllowedWeekdays is a bitmask for Mon(1)..Sun(7).
	// Bit 0 = Monday, Bit 6 = Sunday.
	AllowedWeekdays uint8

	// StartMinute is the start time in minutes from midnight (0-1439).
	StartMinute int

	// EndMinute is the end time in minutes from midnight (0-1439).
	EndMinute int
}

// CanonicalString returns a deterministic string representation.
func (h HoursPolicy) CanonicalString() string {
	return fmt.Sprintf("weekdays:%d|start:%d|end:%d",
		h.AllowedWeekdays, h.StartMinute, h.EndMinute)
}

// IsAllowed checks if the given time is within allowed hours.
func (h HoursPolicy) IsAllowed(t time.Time) bool {
	// Check weekday (time.Weekday: Sunday=0, Monday=1, ..., Saturday=6)
	// Our bitmask: Bit 0 = Monday, Bit 6 = Sunday
	weekday := t.Weekday()
	var bit uint8
	if weekday == time.Sunday {
		bit = 6 // Sunday is bit 6
	} else {
		bit = uint8(weekday - 1) // Monday=0, Tuesday=1, etc.
	}

	if h.AllowedWeekdays&(1<<bit) == 0 {
		return false
	}

	// Check time of day
	minute := t.Hour()*60 + t.Minute()
	if h.StartMinute <= h.EndMinute {
		return minute >= h.StartMinute && minute <= h.EndMinute
	}
	// Wrap around midnight
	return minute >= h.StartMinute || minute <= h.EndMinute
}

// CirclePolicy defines policy for a single circle.
type CirclePolicy struct {
	// CircleID is the unique identifier for this circle.
	CircleID string

	// RegretThreshold is the baseline gating (0-100).
	// Interruptions below this score are queued.
	RegretThreshold int

	// NotifyThreshold is the minimum regret to Notify (0-100).
	NotifyThreshold int

	// UrgentThreshold is the minimum regret to Urgent (0-100).
	UrgentThreshold int

	// DailyNotifyQuota limits notify interruptions per day.
	DailyNotifyQuota int

	// DailyQueuedQuota limits queued interruptions per day.
	DailyQueuedQuota int

	// Hours is optional time-of-day restrictions.
	Hours *HoursPolicy
}

// CanonicalString returns a deterministic string representation.
func (c CirclePolicy) CanonicalString() string {
	var sb strings.Builder
	sb.WriteString("circle:")
	sb.WriteString(c.CircleID)
	sb.WriteString("|regret:")
	sb.WriteString(strconv.Itoa(c.RegretThreshold))
	sb.WriteString("|notify:")
	sb.WriteString(strconv.Itoa(c.NotifyThreshold))
	sb.WriteString("|urgent:")
	sb.WriteString(strconv.Itoa(c.UrgentThreshold))
	sb.WriteString("|daily_notify:")
	sb.WriteString(strconv.Itoa(c.DailyNotifyQuota))
	sb.WriteString("|daily_queued:")
	sb.WriteString(strconv.Itoa(c.DailyQueuedQuota))
	if c.Hours != nil {
		sb.WriteString("|hours:")
		sb.WriteString(c.Hours.CanonicalString())
	}
	return sb.String()
}

// Validate checks that the policy is valid.
func (c CirclePolicy) Validate() error {
	if c.CircleID == "" {
		return errors.New("circle_id is required")
	}

	// Thresholds must be 0-100
	if c.RegretThreshold < 0 || c.RegretThreshold > 100 {
		return fmt.Errorf("regret_threshold must be 0-100, got %d", c.RegretThreshold)
	}
	if c.NotifyThreshold < 0 || c.NotifyThreshold > 100 {
		return fmt.Errorf("notify_threshold must be 0-100, got %d", c.NotifyThreshold)
	}
	if c.UrgentThreshold < 0 || c.UrgentThreshold > 100 {
		return fmt.Errorf("urgent_threshold must be 0-100, got %d", c.UrgentThreshold)
	}

	// Thresholds must be monotonic: Urgent >= Notify >= Regret
	if c.UrgentThreshold < c.NotifyThreshold {
		return fmt.Errorf("urgent_threshold (%d) must be >= notify_threshold (%d)",
			c.UrgentThreshold, c.NotifyThreshold)
	}
	if c.NotifyThreshold < c.RegretThreshold {
		return fmt.Errorf("notify_threshold (%d) must be >= regret_threshold (%d)",
			c.NotifyThreshold, c.RegretThreshold)
	}

	// Quotas must be non-negative
	if c.DailyNotifyQuota < 0 {
		return fmt.Errorf("daily_notify_quota must be >= 0, got %d", c.DailyNotifyQuota)
	}
	if c.DailyQueuedQuota < 0 {
		return fmt.Errorf("daily_queued_quota must be >= 0, got %d", c.DailyQueuedQuota)
	}

	// Validate hours if present
	if c.Hours != nil {
		if c.Hours.StartMinute < 0 || c.Hours.StartMinute > 1439 {
			return fmt.Errorf("start_minute must be 0-1439, got %d", c.Hours.StartMinute)
		}
		if c.Hours.EndMinute < 0 || c.Hours.EndMinute > 1439 {
			return fmt.Errorf("end_minute must be 0-1439, got %d", c.Hours.EndMinute)
		}
	}

	return nil
}

// TriggerPolicy defines per-trigger overrides.
type TriggerPolicy struct {
	// Trigger matches interrupt.Trigger string.
	Trigger string

	// MinLevel is an optional minimum level override.
	MinLevel string

	// SuppressByDefault if true, suppresses this trigger unless overridden.
	SuppressByDefault bool

	// RegretBias is applied to regret score (-50 to +50).
	RegretBias int
}

// CanonicalString returns a deterministic string representation.
func (t TriggerPolicy) CanonicalString() string {
	return fmt.Sprintf("trigger:%s|min_level:%s|suppress:%t|bias:%d",
		t.Trigger, t.MinLevel, t.SuppressByDefault, t.RegretBias)
}

// Validate checks that the trigger policy is valid.
func (t TriggerPolicy) Validate() error {
	if t.Trigger == "" {
		return errors.New("trigger is required")
	}
	if t.RegretBias < -50 || t.RegretBias > 50 {
		return fmt.Errorf("regret_bias must be -50 to +50, got %d", t.RegretBias)
	}
	return nil
}

// PolicySet contains all policies with versioning.
type PolicySet struct {
	// Version is incremented on each update.
	Version int

	// CapturedAt is when this policy set was created/updated.
	CapturedAt time.Time

	// Circles contains per-circle policies.
	Circles map[string]CirclePolicy

	// Triggers contains global trigger overrides.
	Triggers map[string]TriggerPolicy

	// Hash is the computed SHA256 hash of canonical string.
	Hash string
}

// CanonicalString returns a deterministic string representation.
// Maps are sorted by key for determinism.
func (p PolicySet) CanonicalString() string {
	var sb strings.Builder
	sb.WriteString("version:")
	sb.WriteString(strconv.Itoa(p.Version))
	sb.WriteString("|captured:")
	sb.WriteString(p.CapturedAt.UTC().Format(time.RFC3339))

	// Sort circle keys
	circleKeys := sortedKeys(p.Circles)
	sb.WriteString("|circles:[")
	for i, key := range circleKeys {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(p.Circles[key].CanonicalString())
	}
	sb.WriteString("]")

	// Sort trigger keys
	triggerKeys := sortedTriggerKeys(p.Triggers)
	sb.WriteString("|triggers:[")
	for i, key := range triggerKeys {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(p.Triggers[key].CanonicalString())
	}
	sb.WriteString("]")

	return sb.String()
}

// ComputeHash computes and sets the Hash field.
func (p *PolicySet) ComputeHash() string {
	canonical := p.CanonicalString()
	hash := sha256.Sum256([]byte(canonical))
	p.Hash = hex.EncodeToString(hash[:])
	return p.Hash
}

// Validate validates the entire policy set.
func (p PolicySet) Validate() error {
	for _, circle := range p.Circles {
		if err := circle.Validate(); err != nil {
			return fmt.Errorf("circle %s: %w", circle.CircleID, err)
		}
	}
	for _, trigger := range p.Triggers {
		if err := trigger.Validate(); err != nil {
			return fmt.Errorf("trigger %s: %w", trigger.Trigger, err)
		}
	}
	return nil
}

// GetCircle returns the policy for a circle, or nil if not found.
func (p PolicySet) GetCircle(circleID string) *CirclePolicy {
	if circle, ok := p.Circles[circleID]; ok {
		return &circle
	}
	return nil
}

// GetTrigger returns the policy for a trigger, or nil if not found.
func (p PolicySet) GetTrigger(trigger string) *TriggerPolicy {
	if t, ok := p.Triggers[trigger]; ok {
		return &t
	}
	return nil
}

// sortedKeys returns map keys sorted alphabetically (bubble sort, stdlib only).
func sortedKeys(m map[string]CirclePolicy) []string {
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

// sortedTriggerKeys returns map keys sorted alphabetically (bubble sort, stdlib only).
func sortedTriggerKeys(m map[string]TriggerPolicy) []string {
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
