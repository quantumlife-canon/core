package policy

import (
	"time"
)

// DefaultPolicySet returns conservative default policies for standard circles.
// The defaults prioritize reducing noise while ensuring urgent items surface.
func DefaultPolicySet(capturedAt time.Time) PolicySet {
	ps := PolicySet{
		Version:    1,
		CapturedAt: capturedAt,
		Circles:    make(map[string]CirclePolicy),
		Triggers:   make(map[string]TriggerPolicy),
	}

	// Work circle: higher thresholds, limited notify quota.
	// Work tends to be noisy; we want to surface only important items.
	ps.Circles["work"] = CirclePolicy{
		CircleID:         "work",
		RegretThreshold:  40, // Higher baseline
		NotifyThreshold:  60, // Only notify for important items
		UrgentThreshold:  80, // Urgent only for critical
		DailyNotifyQuota: 5,  // Limited notifications
		DailyQueuedQuota: 20, // More queued items allowed
		Hours: &HoursPolicy{
			AllowedWeekdays: 0b0011111, // Mon-Fri (bits 0-4)
			StartMinute:     8 * 60,    // 8:00 AM
			EndMinute:       18 * 60,   // 6:00 PM
		},
	}

	// Family circle: moderate thresholds, more permissive.
	// Family events are generally wanted but shouldn't overwhelm.
	ps.Circles["family"] = CirclePolicy{
		CircleID:         "family",
		RegretThreshold:  25,  // Lower baseline
		NotifyThreshold:  45,  // More willing to notify
		UrgentThreshold:  70,  // Reasonable urgent threshold
		DailyNotifyQuota: 10,  // More notifications allowed
		DailyQueuedQuota: 30,  // More queued items
		Hours:            nil, // No time restrictions for family
	}

	// Personal circle: moderate defaults.
	ps.Circles["personal"] = CirclePolicy{
		CircleID:         "personal",
		RegretThreshold:  30,
		NotifyThreshold:  50,
		UrgentThreshold:  75,
		DailyNotifyQuota: 8,
		DailyQueuedQuota: 25,
		Hours:            nil, // No time restrictions
	}

	// Finance circle: low thresholds for urgent anomalies, small quota.
	// Financial issues should surface quickly but not overwhelm.
	ps.Circles["finance"] = CirclePolicy{
		CircleID:         "finance",
		RegretThreshold:  20,  // Low baseline - most finance matters
		NotifyThreshold:  35,  // Lower notify threshold
		UrgentThreshold:  60,  // Lower urgent for anomalies
		DailyNotifyQuota: 3,   // Limited - finance items are rare
		DailyQueuedQuota: 10,  // Smaller queue
		Hours:            nil, // No time restrictions
	}

	// Default trigger policies for common patterns
	ps.Triggers["obligation.due_soon"] = TriggerPolicy{
		Trigger:           "obligation.due_soon",
		MinLevel:          "",
		SuppressByDefault: false,
		RegretBias:        10, // Boost due-soon items
	}

	ps.Triggers["obligation.overdue"] = TriggerPolicy{
		Trigger:           "obligation.overdue",
		MinLevel:          "",
		SuppressByDefault: false,
		RegretBias:        20, // Strong boost for overdue
	}

	ps.Triggers["balance.low"] = TriggerPolicy{
		Trigger:           "balance.low",
		MinLevel:          "",
		SuppressByDefault: false,
		RegretBias:        15, // Boost low balance alerts
	}

	ps.Triggers["newsletter"] = TriggerPolicy{
		Trigger:           "newsletter",
		MinLevel:          "",
		SuppressByDefault: false,
		RegretBias:        -20, // Reduce newsletters
	}

	ps.Triggers["marketing"] = TriggerPolicy{
		Trigger:           "marketing",
		MinLevel:          "",
		SuppressByDefault: true, // Suppress marketing by default
		RegretBias:        -30,
	}

	ps.ComputeHash()
	return ps
}

// EmptyPolicySet returns an empty policy set for testing.
func EmptyPolicySet(capturedAt time.Time) PolicySet {
	ps := PolicySet{
		Version:    1,
		CapturedAt: capturedAt,
		Circles:    make(map[string]CirclePolicy),
		Triggers:   make(map[string]TriggerPolicy),
	}
	ps.ComputeHash()
	return ps
}

// MinimalCirclePolicy returns a minimal valid circle policy.
func MinimalCirclePolicy(circleID string) CirclePolicy {
	return CirclePolicy{
		CircleID:         circleID,
		RegretThreshold:  30,
		NotifyThreshold:  50,
		UrgentThreshold:  75,
		DailyNotifyQuota: 10,
		DailyQueuedQuota: 50,
		Hours:            nil,
	}
}
