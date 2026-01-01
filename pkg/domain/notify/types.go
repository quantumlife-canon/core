// Package notify defines the notification domain model for Phase 16.
//
// Notifications are the outbound projection of earned interruptions.
// They determine which channels to use and who to notify based on:
// - Circle policies (quiet hours, daily limits, allowed channels)
// - Intersection rules (audience: satish-only, spouse-only, both)
// - Privacy boundaries (never leak personal circle content to spouse)
//
// CRITICAL: Deterministic computation. Same inputs = same outputs.
// CRITICAL: Uses canonical string hashing (NOT JSON).
// CRITICAL: Web is primary UI; email is first outbound channel.
// CRITICAL: Push/SMS are interfaces only (mocks in this phase).
//
// Reference: docs/ADR/ADR-0032-phase16-notification-projection.md
package notify

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/interrupt"
)

// Channel represents a notification delivery channel.
type Channel string

const (
	// ChannelWebBadge is a passive badge count on the web UI.
	ChannelWebBadge Channel = "web_badge"

	// ChannelEmailDigest is a periodic digest email (weekly).
	ChannelEmailDigest Channel = "email_digest"

	// ChannelEmailAlert is an immediate email for urgent items.
	ChannelEmailAlert Channel = "email_alert"

	// ChannelPush is a mobile push notification (mock only in Phase 16).
	ChannelPush Channel = "push"

	// ChannelSMS is an SMS notification (mock only in Phase 16).
	ChannelSMS Channel = "sms"
)

// ChannelOrder returns a sort key for channel priority (higher = more intrusive).
func ChannelOrder(c Channel) int {
	switch c {
	case ChannelSMS:
		return 5
	case ChannelPush:
		return 4
	case ChannelEmailAlert:
		return 3
	case ChannelEmailDigest:
		return 2
	case ChannelWebBadge:
		return 1
	default:
		return 0
	}
}

// Audience represents who should receive the notification.
type Audience string

const (
	// AudienceOwnerOnly means only the primary owner sees this.
	AudienceOwnerOnly Audience = "owner_only"

	// AudienceSpouseOnly means only the spouse sees this.
	AudienceSpouseOnly Audience = "spouse_only"

	// AudienceBoth means both owner and spouse see this.
	AudienceBoth Audience = "both"

	// AudienceIntersection means all intersection members see this.
	AudienceIntersection Audience = "intersection"
)

// NotificationStatus represents the lifecycle status.
type NotificationStatus string

const (
	StatusPlanned   NotificationStatus = "planned"
	StatusPending   NotificationStatus = "pending"
	StatusDelivered NotificationStatus = "delivered"
	StatusFailed    NotificationStatus = "failed"
	StatusExpired   NotificationStatus = "expired"
	// StatusSuppressed means policy/quiet hours prevented delivery.
	StatusSuppressed NotificationStatus = "suppressed"
)

// SuppressionReason explains why a notification was suppressed or downgraded.
type SuppressionReason string

const (
	ReasonNone           SuppressionReason = ""
	ReasonQuietHours     SuppressionReason = "quiet_hours"
	ReasonDailyQuota     SuppressionReason = "daily_quota"
	ReasonPrivacyBlock   SuppressionReason = "privacy_block"
	ReasonSilentLevel    SuppressionReason = "silent_level"
	ReasonChannelBlocked SuppressionReason = "channel_blocked"
	ReasonUserSuppressed SuppressionReason = "user_suppressed"
	ReasonExpired        SuppressionReason = "expired"
)

// Notification represents a single notification to be delivered.
type Notification struct {
	// NotificationID is deterministic: sha256(canonical)[:16]
	NotificationID string

	// Source identification
	InterruptionID string
	CircleID       identity.EntityID
	IntersectionID string // Optional; may be empty

	// Classification
	Level   interrupt.Level
	Channel Channel
	Trigger interrupt.Trigger

	// Audience
	Audience  Audience
	PersonIDs []identity.EntityID // Resolved person IDs to notify

	// Content
	Summary  string // Short summary from interruption
	Template string // Template name for rendering

	// Timing
	PlannedAt   time.Time
	DeliveredAt *time.Time
	ExpiresAt   time.Time

	// Status
	Status            NotificationStatus
	SuppressionReason SuppressionReason
	OriginalChannel   Channel // If downgraded, what was the original?

	// Deduplication
	DedupKey string

	// Audit
	Hash string
}

// NewNotification creates a notification with computed fields.
func NewNotification(
	interruptionID string,
	circleID identity.EntityID,
	level interrupt.Level,
	channel Channel,
	trigger interrupt.Trigger,
	audience Audience,
	summary string,
	plannedAt time.Time,
	expiresAt time.Time,
) *Notification {
	n := &Notification{
		InterruptionID:  interruptionID,
		CircleID:        circleID,
		Level:           level,
		Channel:         channel,
		OriginalChannel: channel,
		Trigger:         trigger,
		Audience:        audience,
		Summary:         summary,
		PlannedAt:       plannedAt,
		ExpiresAt:       expiresAt,
		Status:          StatusPlanned,
	}

	n.DedupKey = n.computeDedupKey()
	n.NotificationID = n.computeID()
	n.Hash = n.ComputeHash()

	return n
}

// computeDedupKey generates a stable dedup key.
// Format: notify|circle|channel|trigger|hour_bucket
func (n *Notification) computeDedupKey() string {
	bucket := n.PlannedAt.UTC().Format("2006-01-02T15")
	return fmt.Sprintf("notify|%s|%s|%s|%s",
		n.CircleID, n.Channel, n.Trigger, bucket)
}

// computeID generates a deterministic notification ID.
func (n *Notification) computeID() string {
	canonical := n.CanonicalString()
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:8]) // 16 hex chars
}

// CanonicalString returns the canonical representation for hashing.
func (n *Notification) CanonicalString() string {
	var sb strings.Builder

	sb.WriteString("interruption:")
	sb.WriteString(n.InterruptionID)
	sb.WriteString("|circle:")
	sb.WriteString(string(n.CircleID))
	sb.WriteString("|intersection:")
	sb.WriteString(n.IntersectionID)
	sb.WriteString("|level:")
	sb.WriteString(string(n.Level))
	sb.WriteString("|channel:")
	sb.WriteString(string(n.Channel))
	sb.WriteString("|original_channel:")
	sb.WriteString(string(n.OriginalChannel))
	sb.WriteString("|trigger:")
	sb.WriteString(string(n.Trigger))
	sb.WriteString("|audience:")
	sb.WriteString(string(n.Audience))
	sb.WriteString("|summary:")
	sb.WriteString(n.Summary)
	sb.WriteString("|planned:")
	sb.WriteString(fmt.Sprintf("%d", n.PlannedAt.Unix()))
	sb.WriteString("|expires:")
	sb.WriteString(fmt.Sprintf("%d", n.ExpiresAt.Unix()))
	sb.WriteString("|status:")
	sb.WriteString(string(n.Status))
	sb.WriteString("|suppression:")
	sb.WriteString(string(n.SuppressionReason))

	// Include person IDs in canonical string (sorted)
	sb.WriteString("|persons:[")
	sortedPersons := sortEntityIDs(n.PersonIDs)
	for i, p := range sortedPersons {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(string(p))
	}
	sb.WriteString("]")

	return sb.String()
}

// ComputeHash computes the SHA256 hash of the canonical string.
func (n *Notification) ComputeHash() string {
	hash := sha256.Sum256([]byte(n.CanonicalString()))
	return hex.EncodeToString(hash[:])
}

// WithIntersection sets the intersection ID.
func (n *Notification) WithIntersection(intersectionID string) *Notification {
	n.IntersectionID = intersectionID
	n.Hash = n.ComputeHash()
	return n
}

// WithPersons sets the person IDs to notify.
func (n *Notification) WithPersons(personIDs []identity.EntityID) *Notification {
	n.PersonIDs = personIDs
	n.Hash = n.ComputeHash()
	return n
}

// WithTemplate sets the template name.
func (n *Notification) WithTemplate(template string) *Notification {
	n.Template = template
	return n
}

// Downgrade changes the channel to a less intrusive one.
func (n *Notification) Downgrade(newChannel Channel, reason SuppressionReason) {
	if n.OriginalChannel == n.Channel {
		n.OriginalChannel = n.Channel
	}
	n.Channel = newChannel
	n.SuppressionReason = reason
	n.Hash = n.ComputeHash()
}

// Suppress marks the notification as suppressed.
func (n *Notification) Suppress(reason SuppressionReason) {
	n.Status = StatusSuppressed
	n.SuppressionReason = reason
	n.Hash = n.ComputeHash()
}

// MarkDelivered marks the notification as delivered.
func (n *Notification) MarkDelivered(deliveredAt time.Time) {
	n.Status = StatusDelivered
	n.DeliveredAt = &deliveredAt
	n.Hash = n.ComputeHash()
}

// MarkFailed marks the notification as failed.
func (n *Notification) MarkFailed() {
	n.Status = StatusFailed
	n.Hash = n.ComputeHash()
}

// IsExpired checks if the notification has expired.
func (n *Notification) IsExpired(now time.Time) bool {
	return now.After(n.ExpiresAt)
}

// WasDowngraded returns true if the channel was changed.
func (n *Notification) WasDowngraded() bool {
	return n.OriginalChannel != n.Channel
}

// NotificationPlan represents a batch of planned notifications.
type NotificationPlan struct {
	// PlanID is deterministic from contents.
	PlanID string

	// Notifications in this plan.
	Notifications []*Notification

	// Counts by status.
	PlannedCount    int
	SuppressedCount int
	DowngradedCount int

	// Counts by channel.
	ChannelCounts map[Channel]int

	// Planning metadata.
	PlannedAt        time.Time
	PolicyHash       string
	SuppressionsHash string

	// Hash of the entire plan.
	Hash string
}

// NewNotificationPlan creates an empty plan.
func NewNotificationPlan(plannedAt time.Time, policyHash, suppressionsHash string) *NotificationPlan {
	return &NotificationPlan{
		Notifications:    make([]*Notification, 0),
		ChannelCounts:    make(map[Channel]int),
		PlannedAt:        plannedAt,
		PolicyHash:       policyHash,
		SuppressionsHash: suppressionsHash,
	}
}

// Add adds a notification to the plan.
func (p *NotificationPlan) Add(n *Notification) {
	p.Notifications = append(p.Notifications, n)

	switch n.Status {
	case StatusPlanned, StatusPending:
		p.PlannedCount++
		p.ChannelCounts[n.Channel]++
	case StatusSuppressed:
		p.SuppressedCount++
	}

	if n.WasDowngraded() {
		p.DowngradedCount++
	}
}

// CanonicalString returns the canonical representation.
func (p *NotificationPlan) CanonicalString() string {
	var sb strings.Builder

	sb.WriteString("plan|")
	sb.WriteString("planned_at:")
	sb.WriteString(fmt.Sprintf("%d", p.PlannedAt.Unix()))
	sb.WriteString("|policy_hash:")
	sb.WriteString(p.PolicyHash)
	sb.WriteString("|suppressions_hash:")
	sb.WriteString(p.SuppressionsHash)
	sb.WriteString("|count:")
	sb.WriteString(fmt.Sprintf("%d", len(p.Notifications)))
	sb.WriteString("|notifications:[")

	// Sort notifications by ID for determinism
	sortedNotifs := make([]*Notification, len(p.Notifications))
	copy(sortedNotifs, p.Notifications)
	sortNotificationsByID(sortedNotifs)

	for i, n := range sortedNotifs {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(n.NotificationID)
	}
	sb.WriteString("]")

	return sb.String()
}

// ComputeHash computes and sets the PlanID and Hash.
func (p *NotificationPlan) ComputeHash() string {
	canonical := p.CanonicalString()
	hash := sha256.Sum256([]byte(canonical))
	p.Hash = hex.EncodeToString(hash[:])
	p.PlanID = hex.EncodeToString(hash[:8])
	return p.Hash
}

// BadgeCounts represents web badge counts by level.
type BadgeCounts struct {
	Urgent  int
	Notify  int
	Queued  int
	Ambient int
	Total   int

	// ByCircle breaks down counts by circle.
	ByCircle map[identity.EntityID]*CircleBadgeCounts
}

// CircleBadgeCounts represents badge counts for a single circle.
type CircleBadgeCounts struct {
	CircleID identity.EntityID
	Urgent   int
	Notify   int
	Queued   int
	Ambient  int
	Total    int
}

// NewBadgeCounts creates empty badge counts.
func NewBadgeCounts() *BadgeCounts {
	return &BadgeCounts{
		ByCircle: make(map[identity.EntityID]*CircleBadgeCounts),
	}
}

// Add adds a notification to the badge counts.
func (b *BadgeCounts) Add(n *Notification) {
	if n.Channel != ChannelWebBadge {
		return
	}
	if n.Status != StatusPlanned && n.Status != StatusPending {
		return
	}

	b.Total++
	switch n.Level {
	case interrupt.LevelUrgent:
		b.Urgent++
	case interrupt.LevelNotify:
		b.Notify++
	case interrupt.LevelQueued:
		b.Queued++
	case interrupt.LevelAmbient:
		b.Ambient++
	}

	// Update per-circle counts
	if b.ByCircle[n.CircleID] == nil {
		b.ByCircle[n.CircleID] = &CircleBadgeCounts{CircleID: n.CircleID}
	}
	circle := b.ByCircle[n.CircleID]
	circle.Total++
	switch n.Level {
	case interrupt.LevelUrgent:
		circle.Urgent++
	case interrupt.LevelNotify:
		circle.Notify++
	case interrupt.LevelQueued:
		circle.Queued++
	case interrupt.LevelAmbient:
		circle.Ambient++
	}
}

// IsEmpty returns true if there are no badges.
func (b *BadgeCounts) IsEmpty() bool {
	return b.Total == 0
}

// sortEntityIDs sorts entity IDs alphabetically using bubble sort (stdlib only).
func sortEntityIDs(ids []identity.EntityID) []identity.EntityID {
	result := make([]identity.EntityID, len(ids))
	copy(result, ids)

	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i] > result[j] {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}

// sortNotificationsByID sorts notifications by ID using bubble sort.
func sortNotificationsByID(notifs []*Notification) {
	for i := 0; i < len(notifs); i++ {
		for j := i + 1; j < len(notifs); j++ {
			if notifs[i].NotificationID > notifs[j].NotificationID {
				notifs[i], notifs[j] = notifs[j], notifs[i]
			}
		}
	}
}

// SortNotifications sorts notifications deterministically.
// Order: Level DESC, Channel DESC, PlannedAt ASC, NotificationID ASC
func SortNotifications(notifs []*Notification) {
	for i := 0; i < len(notifs); i++ {
		for j := i + 1; j < len(notifs); j++ {
			if !lessNotification(notifs[i], notifs[j]) {
				notifs[i], notifs[j] = notifs[j], notifs[i]
			}
		}
	}
}

func lessNotification(a, b *Notification) bool {
	// Level descending
	la := interrupt.LevelOrder(a.Level)
	lb := interrupt.LevelOrder(b.Level)
	if la != lb {
		return la > lb
	}

	// Channel descending
	ca := ChannelOrder(a.Channel)
	cb := ChannelOrder(b.Channel)
	if ca != cb {
		return ca > cb
	}

	// PlannedAt ascending
	if !a.PlannedAt.Equal(b.PlannedAt) {
		return a.PlannedAt.Before(b.PlannedAt)
	}

	// ID ascending for determinism
	return a.NotificationID < b.NotificationID
}
