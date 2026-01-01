package notify

import (
	"testing"
	"time"

	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/interrupt"
)

func TestNotificationDeterminism(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	expires := now.Add(24 * time.Hour)

	n1 := NewNotification(
		"int-001",
		"circle-work",
		interrupt.LevelNotify,
		ChannelEmailAlert,
		interrupt.TriggerEmailActionNeeded,
		AudienceOwnerOnly,
		"Action needed on email",
		now,
		expires,
	)

	n2 := NewNotification(
		"int-001",
		"circle-work",
		interrupt.LevelNotify,
		ChannelEmailAlert,
		interrupt.TriggerEmailActionNeeded,
		AudienceOwnerOnly,
		"Action needed on email",
		now,
		expires,
	)

	if n1.NotificationID != n2.NotificationID {
		t.Errorf("same inputs should produce same ID: %s != %s", n1.NotificationID, n2.NotificationID)
	}

	if n1.Hash != n2.Hash {
		t.Errorf("same inputs should produce same hash: %s != %s", n1.Hash, n2.Hash)
	}

	if n1.DedupKey != n2.DedupKey {
		t.Errorf("same inputs should produce same dedup key: %s != %s", n1.DedupKey, n2.DedupKey)
	}
}

func TestNotificationDowngrade(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	expires := now.Add(24 * time.Hour)

	n := NewNotification(
		"int-001",
		"circle-work",
		interrupt.LevelNotify,
		ChannelPush,
		interrupt.TriggerEmailActionNeeded,
		AudienceOwnerOnly,
		"Action needed on email",
		now,
		expires,
	)

	originalHash := n.Hash

	// Downgrade due to quiet hours
	n.Downgrade(ChannelEmailAlert, ReasonQuietHours)

	if n.Channel != ChannelEmailAlert {
		t.Errorf("channel should be downgraded: %s", n.Channel)
	}
	if n.OriginalChannel != ChannelPush {
		t.Errorf("original channel should be preserved: %s", n.OriginalChannel)
	}
	if n.SuppressionReason != ReasonQuietHours {
		t.Errorf("suppression reason should be set: %s", n.SuppressionReason)
	}
	if !n.WasDowngraded() {
		t.Error("WasDowngraded should return true")
	}
	if n.Hash == originalHash {
		t.Error("hash should change after downgrade")
	}
}

func TestNotificationSuppress(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	expires := now.Add(24 * time.Hour)

	n := NewNotification(
		"int-001",
		"circle-work",
		interrupt.LevelQueued,
		ChannelWebBadge,
		interrupt.TriggerEmailActionNeeded,
		AudienceOwnerOnly,
		"Test",
		now,
		expires,
	)

	n.Suppress(ReasonUserSuppressed)

	if n.Status != StatusSuppressed {
		t.Errorf("status should be suppressed: %s", n.Status)
	}
	if n.SuppressionReason != ReasonUserSuppressed {
		t.Errorf("suppression reason should be set: %s", n.SuppressionReason)
	}
}

func TestNotificationExpiry(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	expires := now.Add(1 * time.Hour)

	n := NewNotification(
		"int-001",
		"circle-work",
		interrupt.LevelQueued,
		ChannelWebBadge,
		interrupt.TriggerEmailActionNeeded,
		AudienceOwnerOnly,
		"Test",
		now,
		expires,
	)

	// Not expired at planning time
	if n.IsExpired(now) {
		t.Error("should not be expired at planning time")
	}

	// Not expired 30 minutes later
	if n.IsExpired(now.Add(30 * time.Minute)) {
		t.Error("should not be expired 30 minutes later")
	}

	// Expired 2 hours later
	if !n.IsExpired(now.Add(2 * time.Hour)) {
		t.Error("should be expired 2 hours later")
	}
}

func TestNotificationPlanDeterminism(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	expires := now.Add(24 * time.Hour)

	// Create two identical plans
	p1 := NewNotificationPlan(now, "policy-hash-1", "supp-hash-1")
	p2 := NewNotificationPlan(now, "policy-hash-1", "supp-hash-1")

	// Add notifications in different order
	n1 := NewNotification("int-001", "circle-work", interrupt.LevelNotify, ChannelEmailAlert, interrupt.TriggerEmailActionNeeded, AudienceOwnerOnly, "Test 1", now, expires)
	n2 := NewNotification("int-002", "circle-family", interrupt.LevelQueued, ChannelWebBadge, interrupt.TriggerCalendarUpcoming, AudienceBoth, "Test 2", now, expires)

	p1.Add(n1)
	p1.Add(n2)

	p2.Add(n2)
	p2.Add(n1)

	p1.ComputeHash()
	p2.ComputeHash()

	// Plans should have same hash regardless of insertion order
	if p1.Hash != p2.Hash {
		t.Errorf("plans with same notifications should have same hash: %s != %s", p1.Hash, p2.Hash)
	}

	if p1.PlanID != p2.PlanID {
		t.Errorf("plans with same notifications should have same ID: %s != %s", p1.PlanID, p2.PlanID)
	}
}

func TestBadgeCounts(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	expires := now.Add(24 * time.Hour)

	badges := NewBadgeCounts()

	// Add web badge notifications
	n1 := NewNotification("int-001", "circle-work", interrupt.LevelUrgent, ChannelWebBadge, interrupt.TriggerEmailActionNeeded, AudienceOwnerOnly, "Urgent", now, expires)
	n2 := NewNotification("int-002", "circle-work", interrupt.LevelNotify, ChannelWebBadge, interrupt.TriggerEmailActionNeeded, AudienceOwnerOnly, "Notify", now, expires)
	n3 := NewNotification("int-003", "circle-family", interrupt.LevelQueued, ChannelWebBadge, interrupt.TriggerCalendarUpcoming, AudienceBoth, "Queued", now, expires)

	// Email alerts should not count
	n4 := NewNotification("int-004", "circle-work", interrupt.LevelNotify, ChannelEmailAlert, interrupt.TriggerEmailActionNeeded, AudienceOwnerOnly, "Alert", now, expires)

	badges.Add(n1)
	badges.Add(n2)
	badges.Add(n3)
	badges.Add(n4)

	if badges.Total != 3 {
		t.Errorf("total should be 3: %d", badges.Total)
	}
	if badges.Urgent != 1 {
		t.Errorf("urgent should be 1: %d", badges.Urgent)
	}
	if badges.Notify != 1 {
		t.Errorf("notify should be 1: %d", badges.Notify)
	}
	if badges.Queued != 1 {
		t.Errorf("queued should be 1: %d", badges.Queued)
	}

	// Check per-circle counts
	workBadges := badges.ByCircle["circle-work"]
	if workBadges == nil {
		t.Fatal("work circle badges should exist")
	}
	if workBadges.Total != 2 {
		t.Errorf("work total should be 2: %d", workBadges.Total)
	}

	familyBadges := badges.ByCircle["circle-family"]
	if familyBadges == nil {
		t.Fatal("family circle badges should exist")
	}
	if familyBadges.Total != 1 {
		t.Errorf("family total should be 1: %d", familyBadges.Total)
	}
}

func TestChannelOrder(t *testing.T) {
	tests := []struct {
		channel Channel
		order   int
	}{
		{ChannelSMS, 5},
		{ChannelPush, 4},
		{ChannelEmailAlert, 3},
		{ChannelEmailDigest, 2},
		{ChannelWebBadge, 1},
	}

	for _, tc := range tests {
		got := ChannelOrder(tc.channel)
		if got != tc.order {
			t.Errorf("ChannelOrder(%s) = %d, want %d", tc.channel, got, tc.order)
		}
	}
}

func TestSortNotifications(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	expires := now.Add(24 * time.Hour)

	// Create notifications in random order
	n1 := NewNotification("int-001", "circle-work", interrupt.LevelQueued, ChannelWebBadge, interrupt.TriggerEmailActionNeeded, AudienceOwnerOnly, "Low", now, expires)
	n2 := NewNotification("int-002", "circle-work", interrupt.LevelUrgent, ChannelPush, interrupt.TriggerEmailActionNeeded, AudienceOwnerOnly, "Urgent", now, expires)
	n3 := NewNotification("int-003", "circle-work", interrupt.LevelNotify, ChannelEmailAlert, interrupt.TriggerEmailActionNeeded, AudienceOwnerOnly, "Notify", now, expires)

	notifs := []*Notification{n1, n2, n3}
	SortNotifications(notifs)

	// Should be sorted by level DESC, then channel DESC
	if notifs[0].Level != interrupt.LevelUrgent {
		t.Errorf("first should be urgent: %s", notifs[0].Level)
	}
	if notifs[1].Level != interrupt.LevelNotify {
		t.Errorf("second should be notify: %s", notifs[1].Level)
	}
	if notifs[2].Level != interrupt.LevelQueued {
		t.Errorf("third should be queued: %s", notifs[2].Level)
	}
}

func TestWithPersons(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	expires := now.Add(24 * time.Hour)

	n := NewNotification(
		"int-001",
		"circle-family",
		interrupt.LevelNotify,
		ChannelEmailAlert,
		interrupt.TriggerCalendarInvitePending,
		AudienceBoth,
		"Family calendar invite",
		now,
		expires,
	)

	originalHash := n.Hash

	// Add persons
	persons := []identity.EntityID{"person-satish", "person-wife"}
	n.WithPersons(persons)

	if len(n.PersonIDs) != 2 {
		t.Errorf("should have 2 persons: %d", len(n.PersonIDs))
	}

	if n.Hash == originalHash {
		t.Error("hash should change after adding persons")
	}
}

func TestCanonicalStringStability(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	expires := now.Add(24 * time.Hour)

	n := NewNotification(
		"int-001",
		"circle-work",
		interrupt.LevelNotify,
		ChannelEmailAlert,
		interrupt.TriggerEmailActionNeeded,
		AudienceOwnerOnly,
		"Test notification",
		now,
		expires,
	)

	canonical := n.CanonicalString()

	// Verify it contains expected fields
	if !contains(canonical, "interruption:int-001") {
		t.Error("canonical should contain interruption ID")
	}
	if !contains(canonical, "circle:circle-work") {
		t.Error("canonical should contain circle ID")
	}
	if !contains(canonical, "channel:email_alert") {
		t.Error("canonical should contain channel")
	}
	if !contains(canonical, "status:planned") {
		t.Error("canonical should contain status")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr) >= 0
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
