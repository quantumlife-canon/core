// Package draft - policy configuration.
package draft

import (
	"time"

	"quantumlife/pkg/domain/identity"
)

// DraftPolicy configures draft generation and lifecycle.
type DraftPolicy struct {
	// EmailTTLHours is the default TTL for email drafts in hours.
	EmailTTLHours int

	// CalendarTTLHours is the default TTL for calendar drafts in hours.
	CalendarTTLHours int

	// MaxDraftsPerCirclePerDay is the rate limit for proposal spam.
	MaxDraftsPerCirclePerDay int

	// AutoExpireOnApproval determines if other drafts for same source expire on approval.
	AutoExpireOnApproval bool
}

// DefaultDraftPolicy returns sensible defaults.
func DefaultDraftPolicy() DraftPolicy {
	return DraftPolicy{
		EmailTTLHours:            48,
		CalendarTTLHours:         72,
		MaxDraftsPerCirclePerDay: 20,
		AutoExpireOnApproval:     true,
	}
}

// EmailTTL returns the email TTL as a duration.
func (p DraftPolicy) EmailTTL() time.Duration {
	return time.Duration(p.EmailTTLHours) * time.Hour
}

// CalendarTTL returns the calendar TTL as a duration.
func (p DraftPolicy) CalendarTTL() time.Duration {
	return time.Duration(p.CalendarTTLHours) * time.Hour
}

// TTLFor returns the TTL for a given draft type.
func (p DraftPolicy) TTLFor(draftType DraftType) time.Duration {
	switch draftType {
	case DraftTypeEmailReply:
		return p.EmailTTL()
	case DraftTypeCalendarResponse:
		return p.CalendarTTL()
	default:
		return p.EmailTTL() // Default to email TTL
	}
}

// ComputeExpiresAt computes the expiry time for a draft.
func (p DraftPolicy) ComputeExpiresAt(draftType DraftType, createdAt time.Time) time.Time {
	return createdAt.Add(p.TTLFor(draftType))
}

// DayKey returns a deterministic day key for rate limiting (UTC).
func DayKey(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

// RateLimitKey returns a key for rate limiting drafts per circle per day.
func RateLimitKey(circleID identity.EntityID, dayKey string) string {
	return string(circleID) + "|" + dayKey
}

// DraftQuotaTracker tracks draft counts for rate limiting.
type DraftQuotaTracker struct {
	counts map[string]int // key -> count
	policy DraftPolicy
}

// NewDraftQuotaTracker creates a new quota tracker.
func NewDraftQuotaTracker(policy DraftPolicy) *DraftQuotaTracker {
	return &DraftQuotaTracker{
		counts: make(map[string]int),
		policy: policy,
	}
}

// CanCreate returns true if another draft can be created for this circle today.
func (t *DraftQuotaTracker) CanCreate(circleID identity.EntityID, now time.Time) bool {
	key := RateLimitKey(circleID, DayKey(now))
	return t.counts[key] < t.policy.MaxDraftsPerCirclePerDay
}

// Increment records a draft creation.
func (t *DraftQuotaTracker) Increment(circleID identity.EntityID, now time.Time) {
	key := RateLimitKey(circleID, DayKey(now))
	t.counts[key]++
}

// GetCount returns the current count for a circle on a day.
func (t *DraftQuotaTracker) GetCount(circleID identity.EntityID, now time.Time) int {
	key := RateLimitKey(circleID, DayKey(now))
	return t.counts[key]
}

// Reset clears all counts.
func (t *DraftQuotaTracker) Reset() {
	t.counts = make(map[string]int)
}
