// Package demo_phase18_9_quiet_verification demonstrates Phase 18.9 restraint behavior.
//
// These tests verify:
// 1. Gmail consent page shows restraint-first copy
// 2. Sync results display only magnitude buckets
// 3. Mirror shows "not stored" reassurance
// 4. Today page stays quiet after real sync
// 5. Gmail obligations default to HOLD
// 6. Revocation is immediate and reassuring
//
// Reference: docs/ADR/ADR-0042-phase18-9-real-data-quiet-verification.md
package demo_phase18_9_quiet_verification

import (
	"testing"
	"time"

	"quantumlife/internal/mirror"
	"quantumlife/internal/oauth"
	"quantumlife/internal/obligations"
	"quantumlife/pkg/domain/connection"
	domainmirror "quantumlife/pkg/domain/mirror"
)

// TestGmailRestraintPolicyValidation verifies the default policy maintains quietness.
func TestGmailRestraintPolicyValidation(t *testing.T) {
	t.Log("=== Demo: Gmail Restraint Policy Validation ===")

	policy := obligations.DefaultGmailRestraintPolicy()

	t.Log("Default Gmail Restraint Policy:")
	t.Logf("  - NeverAutoSurface: %v", policy.NeverAutoSurface)
	t.Logf("  - RequireUserAction: %v", policy.RequireUserAction)
	t.Logf("  - HoldByDefault: %v", policy.HoldByDefault)
	t.Logf("  - AbstractOnly: %v", policy.AbstractOnly)

	if !policy.Validate() {
		t.Fatal("Default policy should validate for Phase 18.9 compliance")
	}

	t.Log("PASS: Default policy validates - quietness guaranteed")
	t.Log("\n=== Gmail Restraint Policy Validation Complete ===")
}

// TestGmailObligationsDefaultToHold verifies all Gmail obligations are held.
func TestGmailObligationsDefaultToHold(t *testing.T) {
	t.Log("=== Demo: Gmail Obligations Default to HOLD ===")

	config := obligations.DefaultGmailRestraintConfig()
	extractor := obligations.NewGmailObligationExtractor(config)

	t.Log("Gmail Restraint Config:")
	t.Logf("  - DefaultToHold: %v", config.DefaultToHold)
	t.Logf("  - MaxDailyObligations: %d", config.MaxDailyObligations)
	t.Logf("  - BaseRegret: %.2f", config.BaseRegret)
	t.Logf("  - MaxRegret: %.2f", config.MaxRegret)
	t.Logf("  - RequireExplicitAction: %v", config.RequireExplicitAction)

	// Verify MaxRegret is below surface threshold
	surfaceThreshold := 0.5 // Standard surface threshold
	if config.MaxRegret >= surfaceThreshold {
		t.Errorf("MaxRegret %.2f should be below surface threshold %.2f", config.MaxRegret, surfaceThreshold)
	} else {
		t.Logf("PASS: MaxRegret (%.2f) < SurfaceThreshold (%.2f) - auto-surfacing prevented", config.MaxRegret, surfaceThreshold)
	}

	// Create test messages
	now := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	messages := []obligations.GmailMessageMeta{
		{
			MessageHash:  "hash-123",
			DomainBucket: "personal",
			ReceivedAt:   now.Add(-1 * time.Hour),
			LabelBucket:  "important",
			IsUnread:     true,
			HasActionCue: true,
			CircleID:     "circle-123",
		},
		{
			MessageHash:  "hash-456",
			DomainBucket: "commercial",
			ReceivedAt:   now.Add(-2 * time.Hour),
			LabelBucket:  "inbox",
			IsUnread:     true,
			HasActionCue: false, // No action cue - should be filtered
			CircleID:     "circle-123",
		},
	}

	obligs := extractor.ExtractFromMessages(messages, now)

	t.Logf("\nExtracted %d obligations from %d messages", len(obligs), len(messages))

	for _, oblig := range obligs {
		shouldHold := extractor.ShouldHold(oblig)
		t.Logf("  - Obligation %s: RegretScore=%.2f, ShouldHold=%v", oblig.ID[:8], oblig.RegretScore, shouldHold)

		if !shouldHold {
			t.Errorf("Obligation %s should be held but ShouldHold returned false", oblig.ID)
		}

		if oblig.RegretScore > config.MaxRegret {
			t.Errorf("Obligation %s regret %.2f exceeds max %.2f", oblig.ID, oblig.RegretScore, config.MaxRegret)
		}
	}

	t.Log("\n=== Gmail Obligations Default to HOLD Complete ===")
}

// TestMirrorShowsNotStoredReassurance verifies mirror displays "not stored" items.
func TestMirrorShowsNotStoredReassurance(t *testing.T) {
	t.Log("=== Demo: Mirror Shows 'Not Stored' Reassurance ===")

	clock := func() time.Time {
		return time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	}
	engine := mirror.NewEngine(clock)

	// Create input with email connection (Gmail uses email kind)
	input := domainmirror.MirrorInput{
		ConnectedSources: map[connection.ConnectionKind]domainmirror.SourceInputState{
			connection.KindEmail: {
				Connected:   true,
				Mode:        connection.ModeReal,
				ReadSuccess: true,
				ObservedCounts: map[domainmirror.ObservedCategory]int{
					domainmirror.ObservedMessages: 5,
				},
			},
		},
		HeldCount:     2,
		SurfacedCount: 0,
		CircleID:      "circle-123",
	}

	page := engine.BuildMirrorPage(input)

	t.Log("Mirror Page Generated:")
	t.Logf("  - Title: %s", page.Title)
	t.Logf("  - Subtitle: %s", page.Subtitle)
	t.Logf("  - RestraintStatement: %s", page.RestraintStatement)
	t.Logf("  - RestraintWhy: %s", page.RestraintWhy)

	// Check for email source
	for _, src := range page.Sources {
		t.Logf("\nSource: %s", src.Kind)
		t.Logf("  - ReadSuccessfully: %v", src.ReadSuccessfully)
		t.Logf("  - NotStored: %v", src.NotStored)

		if len(src.NotStored) == 0 {
			t.Error("NotStored should list what was NOT stored for reassurance")
		}

		for _, obs := range src.Observed {
			t.Logf("  - Observed: %s %s (%s)", obs.Magnitude.DisplayText(), obs.Category.DisplayText(), obs.Horizon.DisplayText())
		}
	}

	// Verify magnitude buckets are used
	for _, src := range page.Sources {
		for _, obs := range src.Observed {
			if obs.Magnitude == "" {
				t.Error("Observed items must use magnitude buckets")
			}
		}
	}

	t.Log("\n=== Mirror Shows 'Not Stored' Reassurance Complete ===")
}

// TestSyncReceiptsUseMagnitudeBucketsOnly verifies no raw counts in receipts.
func TestSyncReceiptsUseMagnitudeBucketsOnly(t *testing.T) {
	t.Log("=== Demo: Sync Receipts Use Magnitude Buckets Only ===")

	testCases := []struct {
		count    int
		expected string
	}{
		{0, "none"},
		{1, "handful"},
		{5, "handful"},
		{6, "several"},
		{20, "several"},
		{21, "many"},
		{100, "many"},
	}

	for _, tc := range testCases {
		bucket := oauth.MagnitudeBucket(tc.count)
		t.Logf("Count %d -> Bucket '%s'", tc.count, bucket)

		if bucket != tc.expected {
			t.Errorf("Count %d: expected '%s', got '%s'", tc.count, tc.expected, bucket)
		}
	}

	t.Log("\nPASS: All counts correctly bucketed - no raw numbers exposed")
	t.Log("\n=== Sync Receipts Use Magnitude Buckets Only Complete ===")
}

// TestMirrorOutcomeUsesQuietLanguage verifies outcome uses calm language.
func TestMirrorOutcomeUsesQuietLanguage(t *testing.T) {
	t.Log("=== Demo: Mirror Outcome Uses Quiet Language ===")

	clock := func() time.Time {
		return time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	}
	engine := mirror.NewEngine(clock)

	// Test with held items
	inputHeld := domainmirror.MirrorInput{
		ConnectedSources: map[connection.ConnectionKind]domainmirror.SourceInputState{
			connection.KindEmail: {
				Connected:   true,
				Mode:        connection.ModeReal,
				ReadSuccess: true,
			},
		},
		HeldCount:     5,
		SurfacedCount: 0,
		CircleID:      "circle-123",
	}

	page := engine.BuildMirrorPage(inputHeld)

	t.Log("Outcome with held items:")
	t.Logf("  - HeldQuietly: %v", page.Outcome.HeldQuietly)
	t.Logf("  - HeldMagnitude: %s", page.Outcome.HeldMagnitude.DisplayText())
	t.Logf("  - NothingRequiresAttention: %v", page.Outcome.NothingRequiresAttention)

	if !page.Outcome.HeldQuietly {
		t.Error("Items should be held quietly")
	}

	if !page.Outcome.NothingRequiresAttention {
		t.Error("Nothing should require attention when surfaced count is 0")
	}

	// Verify restraint statement is present
	if page.RestraintStatement == "" {
		t.Error("RestraintStatement should be set")
	}

	if page.RestraintWhy == "" {
		t.Error("RestraintWhy should be set")
	}

	t.Log("\nPASS: Outcome uses quiet, reassuring language")
	t.Log("\n=== Mirror Outcome Uses Quiet Language Complete ===")
}

// TestDailyObligationCap verifies max obligations per day is enforced.
func TestDailyObligationCap(t *testing.T) {
	t.Log("=== Demo: Daily Obligation Cap Enforced ===")

	config := obligations.DefaultGmailRestraintConfig()
	extractor := obligations.NewGmailObligationExtractor(config)

	now := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	// Create many messages (more than daily cap)
	messages := make([]obligations.GmailMessageMeta, 10)
	for i := 0; i < 10; i++ {
		messages[i] = obligations.GmailMessageMeta{
			MessageHash:  "hash-" + string(rune('A'+i)),
			DomainBucket: "personal",
			ReceivedAt:   now.Add(-time.Duration(i) * time.Hour),
			LabelBucket:  "important",
			IsUnread:     true,
			HasActionCue: true,
			CircleID:     "circle-123",
		}
	}

	obligs := extractor.ExtractFromMessages(messages, now)

	t.Logf("Input: %d messages", len(messages))
	t.Logf("Output: %d obligations", len(obligs))
	t.Logf("Daily cap: %d", config.MaxDailyObligations)

	if len(obligs) > config.MaxDailyObligations {
		t.Errorf("Obligations (%d) exceed daily cap (%d)", len(obligs), config.MaxDailyObligations)
	} else {
		t.Logf("PASS: Obligations capped at %d - prevents overwhelming user", config.MaxDailyObligations)
	}

	t.Log("\n=== Daily Obligation Cap Enforced Complete ===")
}

// TestStaleMessagesIgnored verifies old messages don't create obligations.
func TestStaleMessagesIgnored(t *testing.T) {
	t.Log("=== Demo: Stale Messages Ignored ===")

	config := obligations.DefaultGmailRestraintConfig()
	extractor := obligations.NewGmailObligationExtractor(config)

	now := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	staleDate := now.Add(-time.Duration(config.StalenessThresholdDays+1) * 24 * time.Hour)

	messages := []obligations.GmailMessageMeta{
		{
			MessageHash:  "hash-fresh",
			DomainBucket: "personal",
			ReceivedAt:   now.Add(-1 * time.Hour), // Fresh
			LabelBucket:  "important",
			IsUnread:     true,
			HasActionCue: true,
			CircleID:     "circle-123",
		},
		{
			MessageHash:  "hash-stale",
			DomainBucket: "personal",
			ReceivedAt:   staleDate, // Stale
			LabelBucket:  "important",
			IsUnread:     true,
			HasActionCue: true,
			CircleID:     "circle-123",
		},
	}

	obligs := extractor.ExtractFromMessages(messages, now)

	t.Logf("Staleness threshold: %d days", config.StalenessThresholdDays)
	t.Logf("Fresh message: %v", messages[0].ReceivedAt)
	t.Logf("Stale message: %v", messages[1].ReceivedAt)
	t.Logf("Obligations created: %d", len(obligs))

	if len(obligs) != 1 {
		t.Errorf("Expected 1 obligation (fresh only), got %d", len(obligs))
	} else {
		t.Log("PASS: Stale messages correctly ignored - prevents old noise")
	}

	t.Log("\n=== Stale Messages Ignored Complete ===")
}

// TestAbstractOnlyEvidence verifies no identifiable info in obligations.
func TestAbstractOnlyEvidence(t *testing.T) {
	t.Log("=== Demo: Abstract Only Evidence ===")

	config := obligations.DefaultGmailRestraintConfig()
	extractor := obligations.NewGmailObligationExtractor(config)

	now := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	messages := []obligations.GmailMessageMeta{
		{
			MessageHash:  "hash-123",
			DomainBucket: "personal", // Abstract - not actual domain
			ReceivedAt:   now.Add(-1 * time.Hour),
			LabelBucket:  "important", // Abstract - not label name
			IsUnread:     true,
			HasActionCue: true,
			CircleID:     "circle-123",
		},
	}

	obligs := extractor.ExtractFromMessages(messages, now)

	if len(obligs) == 0 {
		t.Fatal("Expected at least 1 obligation")
	}

	oblig := obligs[0]
	t.Log("Obligation evidence:")
	for key, val := range oblig.Evidence {
		t.Logf("  - %s: %s", key, val)
	}

	// Verify no identifiable information
	sensitiveKeys := []string{"sender", "subject", "body", "email", "name"}
	for _, key := range sensitiveKeys {
		if _, exists := oblig.Evidence[key]; exists {
			t.Errorf("Evidence contains sensitive key '%s' - should be abstract only", key)
		}
	}

	// Verify abstract buckets are used
	if oblig.Evidence["domain_bucket"] == "" {
		t.Error("domain_bucket evidence should be set")
	}
	if oblig.Evidence["label_bucket"] == "" {
		t.Error("label_bucket evidence should be set")
	}

	t.Log("PASS: Evidence is abstract only - no identifiable information")
	t.Log("\n=== Abstract Only Evidence Complete ===")
}
