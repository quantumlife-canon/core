// Package demo_phase23_gentle_invitation contains tests for Phase 23.
//
// Phase 23: Gentle Action Invitation (Trust-Preserving)
//
// This file demonstrates:
//   - Invitation appears only after quiet proof
//   - Only one invitation per period
//   - Accept hides future invitations
//   - Dismiss suppresses for period
//   - Deterministic output
//   - No invitation without Gmail sync
//
// CRITICAL INVARIANTS:
//   - Never auto-execute
//   - Never create urgency
//   - Never surface identifiers
//   - No goroutines. No time.Now().
//   - Stdlib only.
//
// Reference: docs/ADR/ADR-0053-phase23-gentle-invitation.md
package demo_phase23_gentle_invitation

import (
	"testing"
	"time"

	"quantumlife/internal/invitation"
	"quantumlife/internal/persist"
	"quantumlife/pkg/domain/identity"
	domaininvitation "quantumlife/pkg/domain/invitation"
)

// =============================================================================
// Test: Invitation Appears Only After Quiet Proof
// =============================================================================

func TestInvitationOnlyAfterQuietProof(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	engine := invitation.NewEngine(func() time.Time { return fixedTime })

	// No Gmail, no sync, no mirror - should NOT be eligible
	eligibility := engine.ComputeEligibility(
		"personal",
		false, // hasGmailConnection
		false, // hasSyncReceipt
		nil,   // no trust inputs
		false, // dismissedThisPeriod
		false, // acceptedThisPeriod
	)

	if eligibility.IsEligible() {
		t.Error("Expected not eligible when Gmail not connected")
	}

	summary := engine.Compute(eligibility)
	if summary != nil {
		t.Error("Expected no invitation when not eligible")
	}

	t.Log("No invitation shown without quiet proof - correct")
}

// =============================================================================
// Test: Invitation Requires Gmail Sync
// =============================================================================

func TestInvitationRequiresGmailSync(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	engine := invitation.NewEngine(func() time.Time { return fixedTime })

	// Gmail connected but no sync
	eligibility := engine.ComputeEligibility(
		"personal",
		true,  // hasGmailConnection
		false, // hasSyncReceipt - no sync yet
		&invitation.TrustInputs{
			HasQuietMirrorSummary: true,
			HasTrustAccrual:       true,
			TrustScore:            0.5,
		},
		false,
		false,
	)

	if eligibility.IsEligible() {
		t.Error("Expected not eligible without sync receipt")
	}

	t.Log("No invitation without Gmail sync - correct")
}

// =============================================================================
// Test: Invitation Requires Mirror Viewed
// =============================================================================

func TestInvitationRequiresMirrorViewed(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	engine := invitation.NewEngine(func() time.Time { return fixedTime })

	// Gmail connected and synced, but mirror not viewed
	eligibility := engine.ComputeEligibility(
		"personal",
		true, // hasGmailConnection
		true, // hasSyncReceipt
		&invitation.TrustInputs{
			HasQuietMirrorSummary: false, // mirror not viewed
			HasTrustAccrual:       true,
			TrustScore:            0.5,
		},
		false,
		false,
	)

	if eligibility.IsEligible() {
		t.Error("Expected not eligible without mirror viewed")
	}

	t.Log("No invitation without mirror viewed - correct")
}

// =============================================================================
// Test: Eligible When All Conditions Met
// =============================================================================

func TestEligibleWhenAllConditionsMet(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	engine := invitation.NewEngine(func() time.Time { return fixedTime })

	// All conditions met
	eligibility := engine.ComputeEligibility(
		"personal",
		true, // hasGmailConnection
		true, // hasSyncReceipt
		&invitation.TrustInputs{
			HasQuietMirrorSummary: true,
			HasTrustAccrual:       true,
			TrustScore:            0.5,
			HeldMagnitude:         "a_few",
		},
		false, // not dismissed
		false, // not accepted
	)

	if !eligibility.IsEligible() {
		t.Error("Expected eligible when all conditions met")
	}

	summary := engine.Compute(eligibility)
	if summary == nil {
		t.Fatal("Expected invitation when eligible")
	}

	// Should get hold_continue kind when held items exist
	if summary.Kind != domaininvitation.KindHoldContinue {
		t.Errorf("Expected kind hold_continue, got %s", summary.Kind)
	}

	t.Logf("Invitation shown: %s", summary.Text)
}

// =============================================================================
// Test: Only One Invitation Per Period
// =============================================================================

func TestOnlyOneInvitationPerPeriod(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	engine := invitation.NewEngine(func() time.Time { return fixedTime })
	store := persist.NewInvitationStore(func() time.Time { return fixedTime })

	period := engine.CurrentPeriod()
	circleID := identity.EntityID("personal")

	// First invitation should be eligible
	eligibility1 := engine.ComputeEligibility(
		"personal",
		true, true,
		&invitation.TrustInputs{
			HasQuietMirrorSummary: true,
			HasTrustAccrual:       true,
			TrustScore:            0.5,
		},
		false, false,
	)

	summary1 := engine.Compute(eligibility1)
	if summary1 == nil {
		t.Fatal("Expected first invitation to be shown")
	}

	// Record acceptance
	err := store.RecordDecision(circleID, summary1.Hash(), domaininvitation.DecisionAccepted, period.PeriodHash)
	if err != nil {
		t.Fatalf("Failed to record decision: %v", err)
	}

	// Second check in same period should NOT be eligible
	acceptedThisPeriod := store.IsAcceptedForPeriod(circleID, period.PeriodHash)

	eligibility2 := engine.ComputeEligibility(
		"personal",
		true, true,
		&invitation.TrustInputs{
			HasQuietMirrorSummary: true,
			HasTrustAccrual:       true,
			TrustScore:            0.5,
		},
		false,
		acceptedThisPeriod, // should be true now
	)

	if eligibility2.IsEligible() {
		t.Error("Expected not eligible after accepting in same period")
	}

	t.Log("Only one invitation per period - correct")
}

// =============================================================================
// Test: Dismiss Suppresses For Period
// =============================================================================

func TestDismissSuppressesForPeriod(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	engine := invitation.NewEngine(func() time.Time { return fixedTime })
	store := persist.NewInvitationStore(func() time.Time { return fixedTime })

	period := engine.CurrentPeriod()
	circleID := identity.EntityID("personal")

	// Get first invitation
	eligibility := engine.ComputeEligibility(
		"personal",
		true, true,
		&invitation.TrustInputs{
			HasQuietMirrorSummary: true,
			HasTrustAccrual:       true,
			TrustScore:            0.5,
		},
		false, false,
	)

	summary := engine.Compute(eligibility)
	if summary == nil {
		t.Fatal("Expected invitation to be shown")
	}

	// Dismiss it
	err := store.RecordDecision(circleID, summary.Hash(), domaininvitation.DecisionDismissed, period.PeriodHash)
	if err != nil {
		t.Fatalf("Failed to record dismissal: %v", err)
	}

	// Check dismissal is recorded
	dismissedThisPeriod := store.IsDismissedForPeriod(circleID, period.PeriodHash)
	if !dismissedThisPeriod {
		t.Error("Expected dismissal to be recorded")
	}

	// Next check should NOT be eligible
	eligibility2 := engine.ComputeEligibility(
		"personal",
		true, true,
		&invitation.TrustInputs{
			HasQuietMirrorSummary: true,
			HasTrustAccrual:       true,
			TrustScore:            0.5,
		},
		dismissedThisPeriod,
		false,
	)

	if eligibility2.IsEligible() {
		t.Error("Expected not eligible after dismissing")
	}

	t.Log("Dismiss suppresses for period - correct")
}

// =============================================================================
// Test: Deterministic Output
// =============================================================================

func TestDeterministicOutput(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	engine := invitation.NewEngine(func() time.Time { return fixedTime })

	trustInputs := &invitation.TrustInputs{
		HasQuietMirrorSummary: true,
		HasTrustAccrual:       true,
		TrustScore:            0.5,
		HeldMagnitude:         "several",
	}

	// Compute twice with same inputs
	eligibility1 := engine.ComputeEligibility("personal", true, true, trustInputs, false, false)
	summary1 := engine.Compute(eligibility1)

	eligibility2 := engine.ComputeEligibility("personal", true, true, trustInputs, false, false)
	summary2 := engine.Compute(eligibility2)

	if summary1 == nil || summary2 == nil {
		t.Fatal("Expected both summaries to exist")
	}

	// Hashes must match
	if summary1.Hash() != summary2.Hash() {
		t.Errorf("Hashes differ: %s vs %s", summary1.Hash(), summary2.Hash())
	}

	// Kinds must match
	if summary1.Kind != summary2.Kind {
		t.Errorf("Kinds differ: %s vs %s", summary1.Kind, summary2.Kind)
	}

	t.Logf("Deterministic hash: %s", summary1.Hash())
}

// =============================================================================
// Test: Canonical String Format
// =============================================================================

func TestCanonicalStringFormat(t *testing.T) {
	summary := &domaininvitation.InvitationSummary{
		CircleID: "personal",
		Period:   domaininvitation.NewInvitationPeriod("2024-01-15"),
		Kind:     domaininvitation.KindHoldContinue,
		Text:     "We can keep holding this.",
	}

	canonical := summary.CanonicalString()

	// Should be pipe-delimited
	if !containsString(canonical, "|") {
		t.Error("Canonical string should be pipe-delimited")
	}

	// Should start with INVITATION
	if len(canonical) < 10 || canonical[:10] != "INVITATION" {
		t.Errorf("Canonical should start with INVITATION, got: %s", canonical[:20])
	}

	// Hash should be deterministic
	hash1 := summary.Hash()
	hash2 := summary.Hash()
	if hash1 != hash2 {
		t.Error("Hash should be deterministic")
	}

	t.Logf("Canonical: %s", canonical)
	t.Logf("Hash: %s", summary.Hash())
}

// =============================================================================
// Test: Page When No Invitation
// =============================================================================

func TestPageWhenNoInvitation(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	engine := invitation.NewEngine(func() time.Time { return fixedTime })

	// Build page with nil summary
	page := engine.BuildPage(nil)

	if page.HasInvitation {
		t.Error("Expected no invitation")
	}

	if page.Title == "" {
		t.Error("Expected title even when no invitation")
	}

	t.Logf("Empty page title: %s", page.Title)
	t.Logf("Empty page statement: %s", page.Statement)
}

// =============================================================================
// Test: Page When Invitation Exists
// =============================================================================

func TestPageWhenInvitationExists(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	engine := invitation.NewEngine(func() time.Time { return fixedTime })

	eligibility := engine.ComputeEligibility(
		"personal",
		true, true,
		&invitation.TrustInputs{
			HasQuietMirrorSummary: true,
			HasTrustAccrual:       true,
			TrustScore:            0.5,
		},
		false, false,
	)

	summary := engine.Compute(eligibility)
	if summary == nil {
		t.Fatal("Expected invitation")
	}

	page := engine.BuildPage(summary)

	if !page.HasInvitation {
		t.Error("Expected has invitation to be true")
	}

	if page.Statement == "" {
		t.Error("Expected statement in page")
	}

	// Verify no urgency language
	urgentWords := []string{"urgent", "immediately", "now", "hurry", "asap", "important"}
	for _, word := range urgentWords {
		if containsStringIgnoreCase(page.Statement, word) {
			t.Errorf("Statement contains forbidden urgency word: %s", word)
		}
	}

	t.Logf("Page title: %s", page.Title)
	t.Logf("Page statement: %s", page.Statement)
}

// =============================================================================
// Test: Whisper Cue
// =============================================================================

func TestWhisperCue(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	engine := invitation.NewEngine(func() time.Time { return fixedTime })

	eligibility := engine.ComputeEligibility(
		"personal",
		true, true,
		&invitation.TrustInputs{
			HasQuietMirrorSummary: true,
			HasTrustAccrual:       true,
			TrustScore:            0.5,
		},
		false, false,
	)

	summary := engine.Compute(eligibility)
	if summary == nil {
		t.Fatal("Expected invitation")
	}

	cue := engine.BuildWhisperCue(summary)

	if !cue.Show {
		t.Error("Expected whisper cue to show")
	}

	if cue.Link != "/invite" {
		t.Errorf("Expected link to /invite, got %s", cue.Link)
	}

	// Cue text should be calm
	if cue.Text == "" {
		t.Error("Expected whisper cue text")
	}

	t.Logf("Whisper cue: %s", cue.Text)
}

// =============================================================================
// Test: No Whisper Cue When No Summary
// =============================================================================

func TestNoWhisperCueWhenNoSummary(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	engine := invitation.NewEngine(func() time.Time { return fixedTime })

	cue := engine.BuildWhisperCue(nil)

	if cue.Show {
		t.Error("Expected no whisper cue when no summary")
	}

	t.Log("No whisper cue when no summary - correct")
}

// =============================================================================
// Test: Store Persistence
// =============================================================================

func TestStorePersistence(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	store := persist.NewInvitationStore(func() time.Time { return fixedTime })

	circleID := identity.EntityID("personal")
	periodHash := "test-period-hash"
	invitationHash := "test-invitation-hash"

	// Record a decision
	err := store.RecordDecision(circleID, invitationHash, domaininvitation.DecisionAccepted, periodHash)
	if err != nil {
		t.Fatalf("Failed to store: %v", err)
	}

	// Verify count
	if store.Count() != 1 {
		t.Errorf("Expected count 1, got %d", store.Count())
	}

	// Verify retrieval
	records := store.GetForPeriod(circleID, periodHash)
	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	if records[0].Decision != domaininvitation.DecisionAccepted {
		t.Errorf("Expected accepted decision, got %s", records[0].Decision)
	}

	t.Log("Store persistence works correctly")
}

// =============================================================================
// Test: Kind Selection Based On Context
// =============================================================================

func TestKindSelectionBasedOnContext(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	engine := invitation.NewEngine(func() time.Time { return fixedTime })

	tests := []struct {
		name          string
		heldMagnitude string
		hasShadow     bool
		expectedKind  domaininvitation.InvitationKind
	}{
		{
			name:          "With held items",
			heldMagnitude: "several",
			hasShadow:     false,
			expectedKind:  domaininvitation.KindHoldContinue,
		},
		{
			name:          "With shadow, no held",
			heldMagnitude: "",
			hasShadow:     true,
			expectedKind:  domaininvitation.KindReviewOnce,
		},
		{
			name:          "Nothing special",
			heldMagnitude: "",
			hasShadow:     false,
			expectedKind:  domaininvitation.KindNotifyNextTime,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eligibility := engine.ComputeEligibility(
				"personal",
				true, true,
				&invitation.TrustInputs{
					HasQuietMirrorSummary: true,
					HasTrustAccrual:       true,
					TrustScore:            0.5,
					HeldMagnitude:         tt.heldMagnitude,
					HasShadowReceipt:      tt.hasShadow,
				},
				false, false,
			)

			summary := engine.Compute(eligibility)
			if summary == nil {
				t.Fatal("Expected summary")
			}

			if summary.Kind != tt.expectedKind {
				t.Errorf("Expected kind %s, got %s", tt.expectedKind, summary.Kind)
			}
		})
	}
}

// =============================================================================
// Helpers
// =============================================================================

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func containsStringIgnoreCase(s, substr string) bool {
	sLower := toLower(s)
	substrLower := toLower(substr)
	return containsString(sLower, substrLower)
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}
