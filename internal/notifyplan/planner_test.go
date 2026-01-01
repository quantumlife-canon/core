package notifyplan

import (
	"testing"
	"time"

	"quantumlife/pkg/domain/interrupt"
	"quantumlife/pkg/domain/notify"
	"quantumlife/pkg/domain/policy"
	"quantumlife/pkg/domain/suppress"
)

func TestPlannerDeterminism(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	expires := now.Add(24 * time.Hour)

	// Create interruption
	intr := interrupt.NewInterruption(
		"circle-work",
		interrupt.TriggerEmailActionNeeded,
		"event-001",
		"",
		75,
		80,
		interrupt.LevelNotify,
		expires,
		now,
		"Action needed on email from Bob",
	)

	// Create policy
	pol := policy.DefaultNotificationPolicy("circle-work")

	input := PlannerInput{
		Interruptions:        []*interrupt.Interruption{intr},
		NotificationPolicies: map[string]policy.NotificationPolicy{"circle-work": pol},
		Suppressions:         nil,
		Now:                  now,
	}

	planner := NewPlanner()

	// Plan twice
	out1 := planner.Plan(input)
	out2 := planner.Plan(input)

	// Should produce identical results
	if out1.Plan.Hash != out2.Plan.Hash {
		t.Errorf("plans should have same hash: %s != %s", out1.Plan.Hash, out2.Plan.Hash)
	}

	if out1.PolicyHash != out2.PolicyHash {
		t.Errorf("policy hashes should match: %s != %s", out1.PolicyHash, out2.PolicyHash)
	}

	if len(out1.Plan.Notifications) != len(out2.Plan.Notifications) {
		t.Fatalf("notification counts should match: %d != %d",
			len(out1.Plan.Notifications), len(out2.Plan.Notifications))
	}

	for i := range out1.Plan.Notifications {
		if out1.Plan.Notifications[i].NotificationID != out2.Plan.Notifications[i].NotificationID {
			t.Errorf("notification IDs should match at index %d", i)
		}
	}
}

func TestPlannerSilentLevel(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	expires := now.Add(24 * time.Hour)

	// Create silent interruption
	intr := interrupt.NewInterruption(
		"circle-work",
		interrupt.TriggerEmailActionNeeded,
		"event-001",
		"",
		10,
		50,
		interrupt.LevelSilent,
		expires,
		now,
		"Silent item",
	)

	pol := policy.DefaultNotificationPolicy("circle-work")

	input := PlannerInput{
		Interruptions:        []*interrupt.Interruption{intr},
		NotificationPolicies: map[string]policy.NotificationPolicy{"circle-work": pol},
		Now:                  now,
	}

	planner := NewPlanner()
	out := planner.Plan(input)

	// Should have no notifications
	if len(out.Plan.Notifications) != 0 {
		t.Errorf("silent level should produce no notifications: got %d", len(out.Plan.Notifications))
	}

	// Should have suppressed reason
	if len(out.Reasons) != 1 || out.Reasons[0].Action != "suppressed" {
		t.Error("reason should indicate suppression")
	}
}

func TestPlannerQuietHours(t *testing.T) {
	// 11 PM UTC - during quiet hours (10 PM - 7 AM)
	now := time.Date(2025, 1, 15, 23, 0, 0, 0, time.UTC)
	expires := now.Add(24 * time.Hour)

	// Create notify-level interruption (would normally get email_alert)
	intr := interrupt.NewInterruption(
		"circle-work",
		interrupt.TriggerEmailActionNeeded,
		"event-001",
		"",
		75,
		80,
		interrupt.LevelNotify,
		expires,
		now,
		"Action needed",
	)

	pol := policy.DefaultNotificationPolicy("circle-work")
	// Ensure quiet hours are enabled (they are by default)

	input := PlannerInput{
		Interruptions:        []*interrupt.Interruption{intr},
		NotificationPolicies: map[string]policy.NotificationPolicy{"circle-work": pol},
		Now:                  now,
	}

	planner := NewPlanner()
	out := planner.Plan(input)

	if len(out.Plan.Notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(out.Plan.Notifications))
	}

	n := out.Plan.Notifications[0]

	// Should be downgraded to web_badge during quiet hours
	if n.Channel != notify.ChannelWebBadge {
		t.Errorf("channel should be downgraded to web_badge during quiet hours: got %s", n.Channel)
	}

	if !n.WasDowngraded() {
		t.Error("notification should be marked as downgraded")
	}

	if n.SuppressionReason != notify.ReasonQuietHours {
		t.Errorf("suppression reason should be quiet_hours: got %s", n.SuppressionReason)
	}
}

func TestPlannerUrgentDuringQuietHours(t *testing.T) {
	// 11 PM UTC - during quiet hours
	now := time.Date(2025, 1, 15, 23, 0, 0, 0, time.UTC)
	expires := now.Add(24 * time.Hour)

	// Create urgent-level interruption
	intr := interrupt.NewInterruption(
		"circle-work",
		interrupt.TriggerEmailActionNeeded,
		"event-001",
		"",
		95,
		90,
		interrupt.LevelUrgent,
		expires,
		now,
		"URGENT: Action needed",
	)

	pol := policy.DefaultNotificationPolicy("circle-work")
	// Quiet hours allow urgent by default

	input := PlannerInput{
		Interruptions:        []*interrupt.Interruption{intr},
		NotificationPolicies: map[string]policy.NotificationPolicy{"circle-work": pol},
		Now:                  now,
	}

	planner := NewPlanner()
	out := planner.Plan(input)

	if len(out.Plan.Notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(out.Plan.Notifications))
	}

	n := out.Plan.Notifications[0]

	// Urgent should NOT be downgraded (AllowUrgent is true by default)
	if n.Channel == notify.ChannelWebBadge {
		t.Error("urgent notifications should not be downgraded during quiet hours")
	}

	if n.WasDowngraded() {
		t.Error("urgent notification should not be marked as downgraded")
	}
}

func TestPlannerUserSuppression(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	expires := now.Add(24 * time.Hour)

	intr := interrupt.NewInterruption(
		"circle-work",
		interrupt.TriggerEmailActionNeeded,
		"event-001",
		"",
		75,
		80,
		interrupt.LevelNotify,
		expires,
		now,
		"Action needed",
	)

	// Create suppression matching the interruption's dedup key
	suppExpiry := now.Add(24 * time.Hour)
	supp := suppress.NewSuppressionRule(
		"circle-work",
		suppress.ScopeItemKey,
		intr.DedupKey,
		now.Add(-1*time.Hour),
		&suppExpiry,
		"test suppression",
		suppress.SourceManual,
	)

	pol := policy.DefaultNotificationPolicy("circle-work")

	input := PlannerInput{
		Interruptions:        []*interrupt.Interruption{intr},
		NotificationPolicies: map[string]policy.NotificationPolicy{"circle-work": pol},
		Suppressions:         []suppress.SuppressionRule{supp},
		Now:                  now,
	}

	planner := NewPlanner()
	out := planner.Plan(input)

	// Should have no notifications due to suppression
	if len(out.Plan.Notifications) != 0 {
		t.Errorf("suppressed items should produce no notifications: got %d", len(out.Plan.Notifications))
	}

	// Check reason
	if len(out.Reasons) != 1 || out.Reasons[0].Reason != "person suppression" {
		t.Errorf("reason should indicate person suppression")
	}
}

func TestPlannerIntersectionAudience(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	expires := now.Add(24 * time.Hour)

	intr := interrupt.NewInterruption(
		"circle-family",
		interrupt.TriggerCalendarInvitePending,
		"event-001",
		"",
		70,
		80,
		interrupt.LevelNotify,
		expires,
		now,
		"Family dinner invite",
	)
	intr.IntersectionID = "intersection-household"

	pol := policy.DefaultNotificationPolicy("circle-family")

	intersectionRules := map[string]IntersectionAudienceRule{
		"intersection-household": {
			IntersectionID:  "intersection-household",
			DefaultAudience: notify.AudienceBoth,
			OwnerPersonID:   "person-satish",
			SpousePersonID:  "person-wife",
			AllowSharedView: true,
		},
	}

	input := PlannerInput{
		Interruptions:        []*interrupt.Interruption{intr},
		NotificationPolicies: map[string]policy.NotificationPolicy{"circle-family": pol},
		IntersectionRules:    intersectionRules,
		Now:                  now,
	}

	planner := NewPlanner()
	out := planner.Plan(input)

	if len(out.Plan.Notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(out.Plan.Notifications))
	}

	n := out.Plan.Notifications[0]

	if n.Audience != notify.AudienceBoth {
		t.Errorf("audience should be 'both': got %s", n.Audience)
	}

	if len(n.PersonIDs) != 2 {
		t.Errorf("should have 2 person IDs: got %d", len(n.PersonIDs))
	}
}

func TestPlannerPrivacyBoundary(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	expires := now.Add(24 * time.Hour)

	// Personal circle item that happens to have intersection context
	intr := interrupt.NewInterruption(
		"circle-personal",
		interrupt.TriggerEmailActionNeeded,
		"event-001",
		"",
		70,
		80,
		interrupt.LevelNotify,
		expires,
		now,
		"Personal email",
	)
	intr.IntersectionID = "intersection-household"

	pol := policy.DefaultNotificationPolicy("circle-personal")
	pol.IsPrivate = true // Mark as private

	intersectionRules := map[string]IntersectionAudienceRule{
		"intersection-household": {
			IntersectionID:  "intersection-household",
			DefaultAudience: notify.AudienceBoth, // Would normally share
			OwnerPersonID:   "person-satish",
			SpousePersonID:  "person-wife",
		},
	}

	input := PlannerInput{
		Interruptions:        []*interrupt.Interruption{intr},
		NotificationPolicies: map[string]policy.NotificationPolicy{"circle-personal": pol},
		IntersectionRules:    intersectionRules,
		Now:                  now,
	}

	planner := NewPlanner()
	out := planner.Plan(input)

	if len(out.Plan.Notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(out.Plan.Notifications))
	}

	n := out.Plan.Notifications[0]

	// Privacy boundary should force owner-only despite intersection rule
	if n.Audience != notify.AudienceOwnerOnly {
		t.Errorf("private circle should force owner_only audience: got %s", n.Audience)
	}
}

func TestPlannerDailyQuota(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	expires := now.Add(24 * time.Hour)

	intr := interrupt.NewInterruption(
		"circle-work",
		interrupt.TriggerEmailActionNeeded,
		"event-001",
		"",
		75,
		80,
		interrupt.LevelNotify,
		expires,
		now,
		"Action needed",
	)

	pol := policy.DefaultNotificationPolicy("circle-work")
	// Default limit is 10 email alerts

	// Simulate already at quota
	dailyUsage := map[string]ChannelUsage{
		"circle-work": {
			EmailAlert: 10, // At limit
		},
	}

	input := PlannerInput{
		Interruptions:        []*interrupt.Interruption{intr},
		NotificationPolicies: map[string]policy.NotificationPolicy{"circle-work": pol},
		DailyUsage:           dailyUsage,
		Now:                  now,
	}

	planner := NewPlanner()
	out := planner.Plan(input)

	if len(out.Plan.Notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(out.Plan.Notifications))
	}

	n := out.Plan.Notifications[0]

	// Should be downgraded to web_badge due to quota
	if n.Channel != notify.ChannelWebBadge {
		t.Errorf("should be downgraded to web_badge due to quota: got %s", n.Channel)
	}

	if n.SuppressionReason != notify.ReasonDailyQuota {
		t.Errorf("suppression reason should be daily_quota: got %s", n.SuppressionReason)
	}
}

func TestPlanDigest(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	items := []DigestItem{
		{
			Summary:   "Important email from Bob",
			Level:     interrupt.LevelNotify,
			Count:     3,
			FirstSeen: now.Add(-48 * time.Hour),
			LastSeen:  now.Add(-1 * time.Hour),
			CircleID:  "circle-work",
			Trigger:   interrupt.TriggerEmailActionNeeded,
		},
		{
			Summary:   "Calendar conflict on Friday",
			Level:     interrupt.LevelUrgent,
			Count:     1,
			FirstSeen: now.Add(-24 * time.Hour),
			LastSeen:  now.Add(-24 * time.Hour),
			CircleID:  "circle-work",
			Trigger:   interrupt.TriggerCalendarConflict,
		},
	}

	pol := policy.DefaultNotificationPolicy("circle-work")

	input := DigestPlanInput{
		RollupItems: items,
		CircleID:    "circle-work",
		Policy:      pol,
		Now:         now,
		PersonID:    "person-satish",
	}

	planner := NewPlanner()
	out := planner.PlanDigest(input)

	if out.Suppressed {
		t.Fatalf("digest should not be suppressed: %s", out.SuppressedReason)
	}

	if out.Notification == nil {
		t.Fatal("notification should be created")
	}

	if out.ItemCount != 2 {
		t.Errorf("item count should be 2: got %d", out.ItemCount)
	}

	// Check subject mentions urgent
	if !containsSubstring(out.Subject, "urgent") {
		t.Errorf("subject should mention urgent: %s", out.Subject)
	}
}

func TestPlanDigestDisabled(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	items := []DigestItem{
		{Summary: "Test item", Level: interrupt.LevelNotify, Count: 1},
	}

	pol := policy.DefaultNotificationPolicy("circle-work")
	pol.DigestSchedule.Enabled = false

	input := DigestPlanInput{
		RollupItems: items,
		CircleID:    "circle-work",
		Policy:      pol,
		Now:         now,
		PersonID:    "person-satish",
	}

	planner := NewPlanner()
	out := planner.PlanDigest(input)

	if !out.Suppressed {
		t.Error("digest should be suppressed when disabled")
	}
	if out.SuppressedReason != "digest disabled" {
		t.Errorf("wrong suppression reason: %s", out.SuppressedReason)
	}
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
