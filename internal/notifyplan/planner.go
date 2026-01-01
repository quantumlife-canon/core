// Package notifyplan converts interruptions into notifications.
//
// The planner applies circle policies, intersection rules, and suppressions
// to determine which channels to use and who to notify.
//
// CRITICAL: Deterministic computation. Same inputs = same outputs.
// CRITICAL: No goroutines. No time.Now(). Clock must be injected.
// CRITICAL: Never leak personal circle content to intersection.
//
// Reference: docs/ADR/ADR-0032-phase16-notification-projection.md
package notifyplan

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/interrupt"
	"quantumlife/pkg/domain/notify"
	"quantumlife/pkg/domain/policy"
	"quantumlife/pkg/domain/suppress"
)

// PlannerInput contains all inputs for notification planning.
type PlannerInput struct {
	// Interruptions to convert to notifications.
	Interruptions []*interrupt.Interruption

	// NotificationPolicies per circle.
	NotificationPolicies map[string]policy.NotificationPolicy

	// Suppressions from Phase 14.
	Suppressions []suppress.SuppressionRule

	// IntersectionRules map intersection ID -> audience config.
	IntersectionRules map[string]IntersectionAudienceRule

	// PrivateCircles are circles that should never share with intersections.
	PrivateCircles map[string]bool

	// DailyUsage tracks channel usage for the current day.
	DailyUsage map[string]ChannelUsage

	// Clock for deterministic time evaluation.
	Now time.Time
}

// IntersectionAudienceRule defines who sees intersection notifications.
type IntersectionAudienceRule struct {
	IntersectionID  string
	DefaultAudience notify.Audience
	MemberPersonIDs []identity.EntityID
	AllowSharedView bool
	OwnerPersonID   identity.EntityID
	SpousePersonID  identity.EntityID
}

// ChannelUsage tracks daily channel usage.
type ChannelUsage struct {
	Push        int
	SMS         int
	EmailAlert  int
	EmailDigest int
}

// GetUsage returns usage for a channel.
func (u ChannelUsage) GetUsage(channel policy.NotificationChannel) int {
	switch channel {
	case policy.ChannelPush:
		return u.Push
	case policy.ChannelSMS:
		return u.SMS
	case policy.ChannelEmailAlert:
		return u.EmailAlert
	case policy.ChannelEmailDigest:
		return u.EmailDigest
	default:
		return 0
	}
}

// PlannerOutput contains the planning results.
type PlannerOutput struct {
	// Plan is the notification plan.
	Plan *notify.NotificationPlan

	// Reasons explains each decision.
	Reasons []PlanReason

	// PolicyHash is the hash of all policies used.
	PolicyHash string

	// SuppressionsHash is the hash of all suppressions.
	SuppressionsHash string
}

// PlanReason explains a planning decision.
type PlanReason struct {
	InterruptionID  string
	NotificationID  string
	Action          string // "planned", "suppressed", "downgraded"
	Reason          string
	OriginalChannel string
	FinalChannel    string
}

// Planner converts interruptions to notifications.
type Planner struct{}

// NewPlanner creates a new planner.
func NewPlanner() *Planner {
	return &Planner{}
}

// Plan creates a notification plan from interruptions.
func (p *Planner) Plan(input PlannerInput) *PlannerOutput {
	// Compute policy and suppression hashes
	policyHash := p.computePolicyHash(input.NotificationPolicies)
	suppressionsHash := p.computeSuppressionsHash(input.Suppressions)

	plan := notify.NewNotificationPlan(input.Now, policyHash, suppressionsHash)
	var reasons []PlanReason

	// Build suppression lookup
	suppressedKeys := p.buildSuppressionLookup(input.Suppressions, input.Now)

	// Process each interruption
	for _, intr := range input.Interruptions {
		reason := p.planInterruption(intr, input, plan, suppressedKeys)
		reasons = append(reasons, reason)
	}

	// Finalize plan
	plan.ComputeHash()

	return &PlannerOutput{
		Plan:             plan,
		Reasons:          reasons,
		PolicyHash:       policyHash,
		SuppressionsHash: suppressionsHash,
	}
}

func (p *Planner) planInterruption(
	intr *interrupt.Interruption,
	input PlannerInput,
	plan *notify.NotificationPlan,
	suppressedKeys map[string]bool,
) PlanReason {
	circleID := string(intr.CircleID)

	// Get notification policy for this circle
	notifyPolicy, ok := input.NotificationPolicies[circleID]
	if !ok {
		notifyPolicy = policy.DefaultNotificationPolicy(circleID)
	}

	// Check if silent level - no notification
	if intr.Level == interrupt.LevelSilent {
		return PlanReason{
			InterruptionID: intr.InterruptionID,
			Action:         "suppressed",
			Reason:         "silent level",
		}
	}

	// Check if suppressed by person preference
	if suppressedKeys[intr.DedupKey] {
		return PlanReason{
			InterruptionID: intr.InterruptionID,
			Action:         "suppressed",
			Reason:         "person suppression",
		}
	}

	// Determine initial channel based on level
	channel := p.selectChannel(intr.Level, notifyPolicy)
	originalChannel := channel

	// Check quiet hours
	isQuiet := notifyPolicy.QuietHours.IsQuietTime(input.Now)
	if isQuiet && channel != policy.ChannelWebBadge {
		// Check if urgent is allowed during quiet hours
		if intr.Level == interrupt.LevelUrgent && notifyPolicy.QuietHours.AllowUrgent {
			// Keep original channel
		} else {
			// Downgrade to quiet hours channel
			channel = notifyPolicy.QuietHours.DowngradeTo
		}
	}

	// Check daily quota
	circleUsage := input.DailyUsage[circleID]
	limit := notifyPolicy.DailyLimits.GetLimit(channel)
	usage := circleUsage.GetUsage(channel)

	if limit >= 0 && usage >= limit && channel != policy.ChannelWebBadge {
		// Quota exceeded, downgrade to web badge
		channel = policy.ChannelWebBadge
	}

	// Determine audience
	audience := notify.AudienceOwnerOnly
	var personIDs []identity.EntityID

	if intr.IntersectionID != "" {
		// Check privacy boundary
		if input.PrivateCircles[circleID] || notifyPolicy.IsPrivate {
			// Private circle - owner only even for intersection items
			audience = notify.AudienceOwnerOnly
		} else if rule, ok := input.IntersectionRules[intr.IntersectionID]; ok {
			audience = rule.DefaultAudience
			personIDs = p.resolvePersonIDs(audience, rule)
		}
	}

	// Convert to notify channel types
	originalNotifyChannel := p.toNotifyChannel(originalChannel)
	finalNotifyChannel := p.toNotifyChannel(channel)

	// Create notification with ORIGINAL channel
	n := notify.NewNotification(
		intr.InterruptionID,
		intr.CircleID,
		intr.Level,
		originalNotifyChannel, // Use original, not downgraded
		intr.Trigger,
		audience,
		intr.Summary,
		input.Now,
		intr.ExpiresAt,
	)

	if intr.IntersectionID != "" {
		n.WithIntersection(intr.IntersectionID)
	}
	if len(personIDs) > 0 {
		n.WithPersons(personIDs)
	}

	// Check if downgraded and apply downgrade
	wasDowngraded := channel != originalChannel
	if wasDowngraded {
		var reason notify.SuppressionReason
		if isQuiet {
			reason = notify.ReasonQuietHours
		} else {
			reason = notify.ReasonDailyQuota
		}
		n.Downgrade(finalNotifyChannel, reason)
	}

	// Add to plan
	plan.Add(n)

	// Build reason
	action := "planned"
	reasonText := "normal"
	if wasDowngraded {
		action = "downgraded"
		if isQuiet {
			reasonText = "quiet hours"
		} else {
			reasonText = "daily quota exceeded"
		}
	}

	return PlanReason{
		InterruptionID:  intr.InterruptionID,
		NotificationID:  n.NotificationID,
		Action:          action,
		Reason:          reasonText,
		OriginalChannel: string(originalChannel),
		FinalChannel:    string(channel),
	}
}

func (p *Planner) selectChannel(level interrupt.Level, pol policy.NotificationPolicy) policy.NotificationChannel {
	levelStr := string(level)
	channels := pol.LevelChannels.GetChannels(levelStr)

	if len(channels) == 0 {
		return policy.ChannelWebBadge
	}

	// Return highest priority channel for this level
	// Channels are ordered by intrusiveness: sms > push > email_alert > email_digest > web_badge
	var best policy.NotificationChannel
	bestOrder := -1

	for _, ch := range channels {
		order := p.channelOrder(ch)
		if order > bestOrder {
			bestOrder = order
			best = ch
		}
	}

	return best
}

func (p *Planner) channelOrder(ch policy.NotificationChannel) int {
	switch ch {
	case policy.ChannelSMS:
		return 5
	case policy.ChannelPush:
		return 4
	case policy.ChannelEmailAlert:
		return 3
	case policy.ChannelEmailDigest:
		return 2
	case policy.ChannelWebBadge:
		return 1
	default:
		return 0
	}
}

func (p *Planner) toNotifyChannel(ch policy.NotificationChannel) notify.Channel {
	switch ch {
	case policy.ChannelWebBadge:
		return notify.ChannelWebBadge
	case policy.ChannelEmailDigest:
		return notify.ChannelEmailDigest
	case policy.ChannelEmailAlert:
		return notify.ChannelEmailAlert
	case policy.ChannelPush:
		return notify.ChannelPush
	case policy.ChannelSMS:
		return notify.ChannelSMS
	default:
		return notify.ChannelWebBadge
	}
}

func (p *Planner) resolvePersonIDs(audience notify.Audience, rule IntersectionAudienceRule) []identity.EntityID {
	switch audience {
	case notify.AudienceOwnerOnly:
		if rule.OwnerPersonID != "" {
			return []identity.EntityID{rule.OwnerPersonID}
		}
		return nil
	case notify.AudienceSpouseOnly:
		if rule.SpousePersonID != "" {
			return []identity.EntityID{rule.SpousePersonID}
		}
		return nil
	case notify.AudienceBoth:
		var ids []identity.EntityID
		if rule.OwnerPersonID != "" {
			ids = append(ids, rule.OwnerPersonID)
		}
		if rule.SpousePersonID != "" {
			ids = append(ids, rule.SpousePersonID)
		}
		return ids
	case notify.AudienceIntersection:
		return rule.MemberPersonIDs
	default:
		return nil
	}
}

func (p *Planner) buildSuppressionLookup(suppressions []suppress.SuppressionRule, now time.Time) map[string]bool {
	lookup := make(map[string]bool)
	for _, s := range suppressions {
		if s.IsActive(now) && s.Scope == suppress.ScopeItemKey {
			lookup[s.Key] = true
		}
	}
	return lookup
}

func (p *Planner) computePolicyHash(policies map[string]policy.NotificationPolicy) string {
	var sb strings.Builder

	// Sort keys for determinism
	keys := make([]string, 0, len(policies))
	for k := range policies {
		keys = append(keys, k)
	}
	bubbleSort(keys)

	for _, k := range keys {
		sb.WriteString(policies[k].CanonicalString())
		sb.WriteString(";")
	}

	hash := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(hash[:16])
}

func (p *Planner) computeSuppressionsHash(suppressions []suppress.SuppressionRule) string {
	var sb strings.Builder

	// Sort by rule ID for determinism
	ruleIDs := make([]string, len(suppressions))
	for i, s := range suppressions {
		ruleIDs[i] = s.RuleID
	}
	bubbleSort(ruleIDs)

	for _, k := range ruleIDs {
		sb.WriteString(k)
		sb.WriteString(";")
	}

	hash := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(hash[:16])
}

func bubbleSort(s []string) {
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[i] > s[j] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

// DigestPlanInput contains inputs for digest planning.
type DigestPlanInput struct {
	// RollupItems from Phase 3.1.
	RollupItems []DigestItem

	// CircleID for the digest.
	CircleID string

	// Policy for the circle.
	Policy policy.NotificationPolicy

	// Now is the current time.
	Now time.Time

	// PersonID is who receives the digest.
	PersonID identity.EntityID
}

// DigestItem represents an item for the digest.
type DigestItem struct {
	Summary   string
	Level     interrupt.Level
	Count     int
	FirstSeen time.Time
	LastSeen  time.Time
	CircleID  string
	Trigger   interrupt.Trigger
}

// DigestPlanOutput contains the digest planning result.
type DigestPlanOutput struct {
	// Notification is the digest notification (if any).
	Notification *notify.Notification

	// Subject is the email subject.
	Subject string

	// Body is the email body.
	Body string

	// ItemCount is the number of items in the digest.
	ItemCount int

	// Suppressed is true if digest was suppressed.
	Suppressed bool

	// SuppressedReason explains why.
	SuppressedReason string
}

// PlanDigest creates a digest notification.
func (p *Planner) PlanDigest(input DigestPlanInput) *DigestPlanOutput {
	// Check if digest is enabled
	if !input.Policy.DigestSchedule.Enabled {
		return &DigestPlanOutput{
			Suppressed:       true,
			SuppressedReason: "digest disabled",
		}
	}

	// Check if digest sending is allowed
	if !input.Policy.AllowDigestSend {
		return &DigestPlanOutput{
			Suppressed:       true,
			SuppressedReason: "digest send not allowed",
		}
	}

	// No items = no digest
	if len(input.RollupItems) == 0 {
		return &DigestPlanOutput{
			Suppressed:       true,
			SuppressedReason: "no items",
		}
	}

	// Build subject and body from rollup items
	subject, body := p.buildDigestContent(input.RollupItems, input.CircleID)

	// Create digest notification
	n := notify.NewNotification(
		fmt.Sprintf("digest-%s-%d", input.CircleID, input.Now.Unix()),
		identity.EntityID(input.CircleID),
		interrupt.LevelQueued,
		notify.ChannelEmailDigest,
		interrupt.TriggerUnknown,
		notify.AudienceOwnerOnly,
		subject,
		input.Now,
		input.Now.Add(7*24*time.Hour), // Expires in 1 week
	)
	n.WithPersons([]identity.EntityID{input.PersonID})
	n.WithTemplate("digest_weekly")

	return &DigestPlanOutput{
		Notification: n,
		Subject:      subject,
		Body:         body,
		ItemCount:    len(input.RollupItems),
	}
}

func (p *Planner) buildDigestContent(items []DigestItem, circleID string) (subject, body string) {
	// Count by level
	urgentCount := 0
	notifyCount := 0
	queuedCount := 0

	for _, item := range items {
		switch item.Level {
		case interrupt.LevelUrgent:
			urgentCount++
		case interrupt.LevelNotify:
			notifyCount++
		case interrupt.LevelQueued:
			queuedCount++
		}
	}

	// Build subject
	total := len(items)
	if urgentCount > 0 {
		subject = fmt.Sprintf("QuantumLife Weekly: %d items (%d urgent)", total, urgentCount)
	} else {
		subject = fmt.Sprintf("QuantumLife Weekly: %d items", total)
	}

	// Build body
	var sb strings.Builder
	sb.WriteString("Your weekly summary:\n\n")

	if urgentCount > 0 {
		sb.WriteString(fmt.Sprintf("⚠️ %d urgent items need attention\n", urgentCount))
	}
	if notifyCount > 0 {
		sb.WriteString(fmt.Sprintf("📬 %d items to review\n", notifyCount))
	}
	if queuedCount > 0 {
		sb.WriteString(fmt.Sprintf("📋 %d queued items\n", queuedCount))
	}

	sb.WriteString("\n")

	// Add top items (first 5)
	maxItems := 5
	if len(items) < maxItems {
		maxItems = len(items)
	}

	sb.WriteString("Top items:\n")
	for i := 0; i < maxItems; i++ {
		item := items[i]
		levelIcon := p.levelIcon(item.Level)
		if item.Count > 1 {
			sb.WriteString(fmt.Sprintf("  %s %s (x%d)\n", levelIcon, item.Summary, item.Count))
		} else {
			sb.WriteString(fmt.Sprintf("  %s %s\n", levelIcon, item.Summary))
		}
	}

	if len(items) > maxItems {
		sb.WriteString(fmt.Sprintf("\n... and %d more items\n", len(items)-maxItems))
	}

	sb.WriteString("\nOpen QuantumLife to review.\n")

	return subject, sb.String()
}

func (p *Planner) levelIcon(level interrupt.Level) string {
	switch level {
	case interrupt.LevelUrgent:
		return "🔴"
	case interrupt.LevelNotify:
		return "🟠"
	case interrupt.LevelQueued:
		return "🟡"
	case interrupt.LevelAmbient:
		return "⚪"
	default:
		return "⬜"
	}
}
