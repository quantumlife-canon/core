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

// =============================================================================
// Phase 16: Notification Policy Extension
// =============================================================================

// NotificationChannel represents a delivery channel for notifications.
type NotificationChannel string

const (
	ChannelWebBadge    NotificationChannel = "web_badge"
	ChannelEmailDigest NotificationChannel = "email_digest"
	ChannelEmailAlert  NotificationChannel = "email_alert"
	ChannelPush        NotificationChannel = "push"
	ChannelSMS         NotificationChannel = "sms"
)

// QuietHoursPolicy defines when notifications should be suppressed or downgraded.
// All times are in UTC to ensure determinism.
type QuietHoursPolicy struct {
	// Enabled turns quiet hours on/off.
	Enabled bool

	// StartMinute is the start time in minutes from midnight UTC (0-1439).
	StartMinute int

	// EndMinute is the end time in minutes from midnight UTC (0-1439).
	EndMinute int

	// AllowUrgent allows urgent notifications even during quiet hours.
	AllowUrgent bool

	// DowngradeTo is the channel to use during quiet hours (web_badge typically).
	DowngradeTo NotificationChannel
}

// CanonicalString returns a deterministic string representation.
func (q QuietHoursPolicy) CanonicalString() string {
	return fmt.Sprintf("quiet|enabled:%t|start:%d|end:%d|allow_urgent:%t|downgrade:%s",
		q.Enabled, q.StartMinute, q.EndMinute, q.AllowUrgent, q.DowngradeTo)
}

// IsQuietTime checks if the given time is within quiet hours.
func (q QuietHoursPolicy) IsQuietTime(t time.Time) bool {
	if !q.Enabled {
		return false
	}

	minute := t.UTC().Hour()*60 + t.UTC().Minute()

	if q.StartMinute <= q.EndMinute {
		// Simple case: quiet hours don't span midnight
		return minute >= q.StartMinute && minute < q.EndMinute
	}
	// Quiet hours span midnight (e.g., 22:00 - 07:00)
	return minute >= q.StartMinute || minute < q.EndMinute
}

// Validate checks that the quiet hours policy is valid.
func (q QuietHoursPolicy) Validate() error {
	if q.StartMinute < 0 || q.StartMinute > 1439 {
		return fmt.Errorf("start_minute must be 0-1439, got %d", q.StartMinute)
	}
	if q.EndMinute < 0 || q.EndMinute > 1439 {
		return fmt.Errorf("end_minute must be 0-1439, got %d", q.EndMinute)
	}
	return nil
}

// DailyLimits defines per-channel daily notification limits.
type DailyLimits struct {
	// Push is the max push notifications per day.
	Push int

	// SMS is the max SMS notifications per day.
	SMS int

	// EmailAlert is the max email alerts per day.
	EmailAlert int

	// EmailDigest is the max digest emails per day (typically 1).
	EmailDigest int
}

// CanonicalString returns a deterministic string representation.
func (d DailyLimits) CanonicalString() string {
	return fmt.Sprintf("limits|push:%d|sms:%d|alert:%d|digest:%d",
		d.Push, d.SMS, d.EmailAlert, d.EmailDigest)
}

// GetLimit returns the limit for a channel.
func (d DailyLimits) GetLimit(channel NotificationChannel) int {
	switch channel {
	case ChannelPush:
		return d.Push
	case ChannelSMS:
		return d.SMS
	case ChannelEmailAlert:
		return d.EmailAlert
	case ChannelEmailDigest:
		return d.EmailDigest
	case ChannelWebBadge:
		return -1 // Unlimited
	default:
		return 0
	}
}

// Validate checks that limits are non-negative.
func (d DailyLimits) Validate() error {
	if d.Push < 0 {
		return fmt.Errorf("push limit must be >= 0, got %d", d.Push)
	}
	if d.SMS < 0 {
		return fmt.Errorf("sms limit must be >= 0, got %d", d.SMS)
	}
	if d.EmailAlert < 0 {
		return fmt.Errorf("email_alert limit must be >= 0, got %d", d.EmailAlert)
	}
	if d.EmailDigest < 0 {
		return fmt.Errorf("email_digest limit must be >= 0, got %d", d.EmailDigest)
	}
	return nil
}

// LevelChannels maps interruption levels to allowed channels.
type LevelChannels struct {
	// Ambient level channels (typically just web_badge).
	Ambient []NotificationChannel

	// Queued level channels (web_badge, email_digest).
	Queued []NotificationChannel

	// Notify level channels (web_badge, email_alert, push).
	Notify []NotificationChannel

	// Urgent level channels (all channels).
	Urgent []NotificationChannel
}

// CanonicalString returns a deterministic string representation.
func (l LevelChannels) CanonicalString() string {
	var sb strings.Builder
	sb.WriteString("levels|ambient:[")
	sb.WriteString(joinChannels(l.Ambient))
	sb.WriteString("]|queued:[")
	sb.WriteString(joinChannels(l.Queued))
	sb.WriteString("]|notify:[")
	sb.WriteString(joinChannels(l.Notify))
	sb.WriteString("]|urgent:[")
	sb.WriteString(joinChannels(l.Urgent))
	sb.WriteString("]")
	return sb.String()
}

// GetChannels returns allowed channels for a level.
func (l LevelChannels) GetChannels(level string) []NotificationChannel {
	switch level {
	case "ambient":
		return l.Ambient
	case "queued":
		return l.Queued
	case "notify":
		return l.Notify
	case "urgent":
		return l.Urgent
	default:
		return nil
	}
}

// IsChannelAllowed checks if a channel is allowed for a level.
func (l LevelChannels) IsChannelAllowed(level string, channel NotificationChannel) bool {
	channels := l.GetChannels(level)
	for _, c := range channels {
		if c == channel {
			return true
		}
	}
	return false
}

// joinChannels joins channels with commas (sorted for determinism).
func joinChannels(channels []NotificationChannel) string {
	if len(channels) == 0 {
		return ""
	}
	sorted := make([]string, len(channels))
	for i, c := range channels {
		sorted[i] = string(c)
	}
	// Bubble sort
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	return strings.Join(sorted, ",")
}

// DigestSchedule defines when digest emails can be sent.
type DigestSchedule struct {
	// Enabled allows digest emails.
	Enabled bool

	// PreferredDay is the preferred day of week (0=Sunday, 6=Saturday).
	PreferredDay int

	// PreferredHour is the preferred hour (0-23) in UTC.
	PreferredHour int

	// AutoSend allows automatic sending (false in Phase 16 - manual only).
	AutoSend bool
}

// CanonicalString returns a deterministic string representation.
func (d DigestSchedule) CanonicalString() string {
	return fmt.Sprintf("digest|enabled:%t|day:%d|hour:%d|auto:%t",
		d.Enabled, d.PreferredDay, d.PreferredHour, d.AutoSend)
}

// Validate checks that the digest schedule is valid.
func (d DigestSchedule) Validate() error {
	if d.PreferredDay < 0 || d.PreferredDay > 6 {
		return fmt.Errorf("preferred_day must be 0-6, got %d", d.PreferredDay)
	}
	if d.PreferredHour < 0 || d.PreferredHour > 23 {
		return fmt.Errorf("preferred_hour must be 0-23, got %d", d.PreferredHour)
	}
	return nil
}

// NotificationPolicy combines all notification settings for a circle.
type NotificationPolicy struct {
	// CircleID this policy applies to.
	CircleID string

	// QuietHours defines when to suppress/downgrade notifications.
	QuietHours QuietHoursPolicy

	// DailyLimits defines per-channel daily limits.
	DailyLimits DailyLimits

	// LevelChannels maps levels to allowed channels.
	LevelChannels LevelChannels

	// DigestSchedule defines digest email settings.
	DigestSchedule DigestSchedule

	// AllowDigestSend allows digest email sending (even if not circle-authored).
	AllowDigestSend bool

	// IsPrivate marks this circle as private (never shared with intersection).
	IsPrivate bool
}

// CanonicalString returns a deterministic string representation.
func (n NotificationPolicy) CanonicalString() string {
	var sb strings.Builder
	sb.WriteString("notify_policy|circle:")
	sb.WriteString(n.CircleID)
	sb.WriteString("|")
	sb.WriteString(n.QuietHours.CanonicalString())
	sb.WriteString("|")
	sb.WriteString(n.DailyLimits.CanonicalString())
	sb.WriteString("|")
	sb.WriteString(n.LevelChannels.CanonicalString())
	sb.WriteString("|")
	sb.WriteString(n.DigestSchedule.CanonicalString())
	sb.WriteString("|allow_digest:")
	sb.WriteString(strconv.FormatBool(n.AllowDigestSend))
	sb.WriteString("|private:")
	sb.WriteString(strconv.FormatBool(n.IsPrivate))
	return sb.String()
}

// ComputeHash computes the SHA256 hash of the canonical string.
func (n NotificationPolicy) ComputeHash() string {
	hash := sha256.Sum256([]byte(n.CanonicalString()))
	return hex.EncodeToString(hash[:])
}

// Validate checks that the notification policy is valid.
func (n NotificationPolicy) Validate() error {
	if n.CircleID == "" {
		return errors.New("circle_id is required")
	}
	if err := n.QuietHours.Validate(); err != nil {
		return fmt.Errorf("quiet_hours: %w", err)
	}
	if err := n.DailyLimits.Validate(); err != nil {
		return fmt.Errorf("daily_limits: %w", err)
	}
	if err := n.DigestSchedule.Validate(); err != nil {
		return fmt.Errorf("digest_schedule: %w", err)
	}
	return nil
}

// DefaultNotificationPolicy returns a sensible default policy.
func DefaultNotificationPolicy(circleID string) NotificationPolicy {
	return NotificationPolicy{
		CircleID: circleID,
		QuietHours: QuietHoursPolicy{
			Enabled:     true,
			StartMinute: 22 * 60, // 10 PM
			EndMinute:   7 * 60,  // 7 AM
			AllowUrgent: true,
			DowngradeTo: ChannelWebBadge,
		},
		DailyLimits: DailyLimits{
			Push:        5,
			SMS:         2,
			EmailAlert:  10,
			EmailDigest: 1,
		},
		LevelChannels: LevelChannels{
			Ambient: []NotificationChannel{ChannelWebBadge},
			Queued:  []NotificationChannel{ChannelWebBadge, ChannelEmailDigest},
			Notify:  []NotificationChannel{ChannelWebBadge, ChannelEmailAlert},
			Urgent:  []NotificationChannel{ChannelWebBadge, ChannelEmailAlert, ChannelPush},
		},
		DigestSchedule: DigestSchedule{
			Enabled:       true,
			PreferredDay:  0, // Sunday
			PreferredHour: 9, // 9 AM UTC
			AutoSend:      false,
		},
		AllowDigestSend: true,
		IsPrivate:       false,
	}
}
