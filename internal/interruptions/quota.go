// Package interruptions - quota/rate-limit logic.
//
// Quota enforces per-circle limits on Notify+Urgent interruptions per day.
// When quota is exceeded, Notify is downgraded to Queued.
// Urgent is NEVER downgraded.
//
// CRITICAL: Deterministic. Same inputs + same clock = same decisions.
// CRITICAL: Uses UTC day key for quota bucket.
package interruptions

import (
	"fmt"
	"time"

	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/interrupt"
)

// QuotaConfig defines per-circle quota limits.
type QuotaConfig struct {
	// MaxNotifyUrgentPerDay is the max Notify+Urgent per circle per day.
	// Urgent is never downgraded, so this mainly limits Notify.
	MaxNotifyUrgentPerDay map[string]int
}

// DefaultQuotaConfig returns default quota limits.
func DefaultQuotaConfig() QuotaConfig {
	return QuotaConfig{
		MaxNotifyUrgentPerDay: map[string]int{
			"finance": 3,
			"family":  3,
			"work":    2,
			"health":  1,
			"home":    1,
		},
	}
}

// QuotaStore tracks quota usage.
type QuotaStore interface {
	// GetUsage returns current usage for circle on day.
	GetUsage(circleKey string, dayKey string) int

	// IncrementUsage increments usage for circle on day.
	IncrementUsage(circleKey string, dayKey string)

	// Clear removes all entries.
	Clear()
}

// InMemoryQuotaStore implements QuotaStore with in-memory storage.
type InMemoryQuotaStore struct {
	usage map[string]int // key = "circle|day"
}

// NewInMemoryQuotaStore creates a new in-memory quota store.
func NewInMemoryQuotaStore() *InMemoryQuotaStore {
	return &InMemoryQuotaStore{
		usage: make(map[string]int),
	}
}

// GetUsage returns current usage.
func (s *InMemoryQuotaStore) GetUsage(circleKey string, dayKey string) int {
	key := fmt.Sprintf("%s|%s", circleKey, dayKey)
	return s.usage[key]
}

// IncrementUsage increments usage.
func (s *InMemoryQuotaStore) IncrementUsage(circleKey string, dayKey string) {
	key := fmt.Sprintf("%s|%s", circleKey, dayKey)
	s.usage[key]++
}

// Clear removes all entries.
func (s *InMemoryQuotaStore) Clear() {
	s.usage = make(map[string]int)
}

// QuotaEnforcer applies quota limits to interruptions.
type QuotaEnforcer struct {
	config QuotaConfig
	store  QuotaStore
}

// NewQuotaEnforcer creates a new quota enforcer.
func NewQuotaEnforcer(config QuotaConfig, store QuotaStore) *QuotaEnforcer {
	return &QuotaEnforcer{
		config: config,
		store:  store,
	}
}

// Apply applies quota limits to interruptions.
// Returns (result interruptions, downgrade count).
// Urgent is NEVER downgraded.
// Notify is downgraded to Queued if quota exceeded.
func (e *QuotaEnforcer) Apply(interruptions []*interrupt.Interruption, now time.Time) ([]*interrupt.Interruption, int) {
	dayKey := now.UTC().Format("2006-01-02")
	downgraded := 0

	result := make([]*interrupt.Interruption, len(interruptions))
	copy(result, interruptions)

	for i, intr := range result {
		// Only check Notify and Urgent levels
		if intr.Level != interrupt.LevelNotify && intr.Level != interrupt.LevelUrgent {
			continue
		}

		circleKey := circleTypeFromID(intr.CircleID)
		limit := e.getLimit(circleKey)
		currentUsage := e.store.GetUsage(circleKey, dayKey)

		if currentUsage >= limit {
			// Quota exceeded
			if intr.Level == interrupt.LevelNotify {
				// Downgrade Notify to Queued
				downgraded++
				result[i] = downgradeToQueued(intr)
			}
			// Urgent is NEVER downgraded, even if over quota
		} else {
			// Within quota, increment usage
			e.store.IncrementUsage(circleKey, dayKey)
		}
	}

	return result, downgraded
}

// getLimit returns the limit for a circle type.
func (e *QuotaEnforcer) getLimit(circleType string) int {
	if limit, ok := e.config.MaxNotifyUrgentPerDay[circleType]; ok {
		return limit
	}
	return 2 // Default limit
}

// circleTypeFromID extracts circle type from circle ID.
// Assumes format like "circle-work", "circle-family", etc.
func circleTypeFromID(circleID identity.EntityID) string {
	s := string(circleID)
	// Try to extract type from common patterns
	prefixes := []string{"circle-", "c-"}
	for _, p := range prefixes {
		if len(s) > len(p) && s[:len(p)] == p {
			return s[len(p):]
		}
	}
	// If no prefix match, check if it contains known types
	types := []string{"finance", "family", "work", "health", "home"}
	for _, t := range types {
		if containsType(s, t) {
			return t
		}
	}
	return "unknown"
}

// containsType checks if s contains the type string.
func containsType(s, t string) bool {
	for i := 0; i <= len(s)-len(t); i++ {
		if s[i:i+len(t)] == t {
			return true
		}
	}
	return false
}

// downgradeToQueued creates a new interruption with Queued level.
func downgradeToQueued(i *interrupt.Interruption) *interrupt.Interruption {
	return interrupt.NewInterruption(
		i.CircleID,
		i.Trigger,
		i.SourceEventID,
		i.ObligationID,
		i.RegretScore,
		i.Confidence,
		interrupt.LevelQueued, // Downgraded level
		i.ExpiresAt,
		i.CreatedAt,
		i.Summary+" (quota)",
	)
}

// Verify interface compliance.
var _ QuotaStore = (*InMemoryQuotaStore)(nil)
