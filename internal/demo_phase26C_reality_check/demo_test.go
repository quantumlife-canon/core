// Package demo_phase26C_reality_check provides determinism tests for Phase 26C.
//
// Phase 26C: Connected Reality Check
// This is NOT analytics. This is a trust proof page.
//
// These tests verify:
// - Determinism: same inputs + clock => same StatusHash and identical page output
// - Privacy: no identifiers leak in rendered strings
// - State mapping: all connection/sync/shadow states map correctly
// - Ack store: acknowledgements suppress cue correctly
// - Single whisper rule: reality cue respects priority
//
// Reference: docs/ADR/ADR-0057-phase26C-connected-reality-check.md
package demo_phase26C_reality_check

import (
	"strings"
	"testing"
	"time"

	"quantumlife/internal/persist"
	"quantumlife/internal/reality"
	domainreality "quantumlife/pkg/domain/reality"
)

// fixedClock implements reality.Clock for deterministic testing.
type fixedClock struct {
	t time.Time
}

func (c *fixedClock) Now() time.Time {
	return c.t
}

func TestDeterminism(t *testing.T) {
	// Same inputs => same StatusHash
	clock := &fixedClock{t: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)}
	engine := reality.NewEngine(clock)

	inputs := &domainreality.RealityInputs{
		CircleID:           "test-circle",
		NowBucket:          "2025-01-15",
		GmailConnected:     true,
		SyncBucket:         domainreality.SyncBucketRecent,
		SyncMagnitude:      domainreality.MagnitudeAFew,
		ObligationsHeld:    true,
		AutoSurface:        false,
		ShadowProviderKind: domainreality.ProviderStub,
		ShadowRealAllowed:  false,
		ShadowMagnitude:    domainreality.MagnitudeNothing,
		ChatConfigured:     true,
		EmbedConfigured:    false,
		EndpointConfigured: true,
	}

	page1 := engine.BuildPage(inputs)
	page2 := engine.BuildPage(inputs)

	if page1.StatusHash != page2.StatusHash {
		t.Errorf("Determinism violated: StatusHash differs between runs: %s vs %s", page1.StatusHash, page2.StatusHash)
	}

	if page1.CalmLine != page2.CalmLine {
		t.Errorf("Determinism violated: CalmLine differs between runs: %s vs %s", page1.CalmLine, page2.CalmLine)
	}

	if len(page1.Lines) != len(page2.Lines) {
		t.Errorf("Determinism violated: Lines count differs: %d vs %d", len(page1.Lines), len(page2.Lines))
	}
}

func TestNoIdentifiersLeak(t *testing.T) {
	// Rendered strings must not contain identifiers
	clock := &fixedClock{t: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)}
	engine := reality.NewEngine(clock)

	inputs := &domainreality.RealityInputs{
		CircleID:           "test@example.com", // Potentially identifiable
		NowBucket:          "2025-01-15",
		GmailConnected:     true,
		SyncBucket:         domainreality.SyncBucketRecent,
		SyncMagnitude:      domainreality.MagnitudeAFew,
		ObligationsHeld:    true,
		AutoSurface:        false,
		ShadowProviderKind: domainreality.ProviderAzureChat,
		ShadowRealAllowed:  true,
		ShadowMagnitude:    domainreality.MagnitudeSeveral,
		ChatConfigured:     true,
		EmbedConfigured:    true,
		EndpointConfigured: true,
		Region:             "uksouth",
	}

	page := engine.BuildPage(inputs)

	// Check page does not contain identifiable patterns
	forbiddenPatterns := []string{
		"@",           // Email addresses
		"http://",     // URLs
		"https://",    // URLs
		"Bearer",      // Auth tokens
		"api-key",     // API keys
		"secret",      // Secrets
		"password",    // Passwords
		"example.com", // Domain from test
	}

	for _, pattern := range forbiddenPatterns {
		if strings.Contains(page.Title, pattern) {
			t.Errorf("Title contains forbidden pattern: %s", pattern)
		}
		if strings.Contains(page.Subtitle, pattern) {
			t.Errorf("Subtitle contains forbidden pattern: %s", pattern)
		}
		if strings.Contains(page.CalmLine, pattern) {
			t.Errorf("CalmLine contains forbidden pattern: %s", pattern)
		}
		for _, line := range page.Lines {
			if strings.Contains(line.Label, pattern) {
				t.Errorf("Line label contains forbidden pattern: %s in %s", pattern, line.Label)
			}
			if strings.Contains(line.Value, pattern) {
				t.Errorf("Line value contains forbidden pattern: %s in %s", pattern, line.Value)
			}
		}
	}

	// Check StatusHash is 32 hex chars
	if len(page.StatusHash) != 32 {
		t.Errorf("StatusHash should be 32 hex chars, got %d", len(page.StatusHash))
	}
}

func TestConnectedNoYesCalmLines(t *testing.T) {
	clock := &fixedClock{t: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)}
	engine := reality.NewEngine(clock)

	tests := []struct {
		name           string
		gmailConnected bool
		syncBucket     domainreality.SyncBucket
		expectedCalm   string
	}{
		{
			name:           "not connected",
			gmailConnected: false,
			syncBucket:     domainreality.SyncBucketNever,
			expectedCalm:   "Nothing is connected yet. Quiet is still the baseline.",
		},
		{
			name:           "connected but never synced",
			gmailConnected: true,
			syncBucket:     domainreality.SyncBucketNever,
			expectedCalm:   "Connected. Waiting for your explicit sync.",
		},
		{
			name:           "connected and synced",
			gmailConnected: true,
			syncBucket:     domainreality.SyncBucketRecent,
			expectedCalm:   "Quiet baseline verified.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			inputs := &domainreality.RealityInputs{
				CircleID:           "test",
				NowBucket:          "2025-01-15",
				GmailConnected:     tc.gmailConnected,
				SyncBucket:         tc.syncBucket,
				SyncMagnitude:      domainreality.MagnitudeNA,
				ObligationsHeld:    true,
				AutoSurface:        false,
				ShadowProviderKind: domainreality.ProviderOff,
			}

			page := engine.BuildPage(inputs)

			if page.CalmLine != tc.expectedCalm {
				t.Errorf("Expected calm line %q, got %q", tc.expectedCalm, page.CalmLine)
			}
		})
	}
}

func TestShadowProviderKindMapping(t *testing.T) {
	tests := []struct {
		configKind string
		mode       string
		expected   domainreality.ShadowProviderKind
	}{
		{"", "off", domainreality.ProviderOff},
		{"", "", domainreality.ProviderOff},
		{"stub", "observe", domainreality.ProviderStub},
		{"none", "observe", domainreality.ProviderStub},
		{"azure_openai", "observe", domainreality.ProviderAzureChat},
	}

	for _, tc := range tests {
		t.Run(tc.configKind+"_"+tc.mode, func(t *testing.T) {
			result := reality.MapProviderKind(tc.configKind, tc.mode)
			if result != tc.expected {
				t.Errorf("MapProviderKind(%q, %q) = %q, want %q", tc.configKind, tc.mode, result, tc.expected)
			}
		})
	}
}

func TestAckStoreSuppressesCue(t *testing.T) {
	clock := &fixedClock{t: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)}
	engine := reality.NewEngine(clock)
	store := persist.NewRealityAckStore(func() time.Time { return clock.t })

	inputs := &domainreality.RealityInputs{
		CircleID:           "test",
		NowBucket:          "2025-01-15",
		GmailConnected:     true,
		SyncBucket:         domainreality.SyncBucketRecent,
		SyncMagnitude:      domainreality.MagnitudeAFew,
		ObligationsHeld:    true,
		AutoSurface:        false,
		ShadowProviderKind: domainreality.ProviderOff,
	}

	page := engine.BuildPage(inputs)
	period := "2025-01-15"

	// Before ack, cue should be available
	cue := engine.ComputeCue(inputs, false)
	if !cue.Available {
		t.Error("Cue should be available before acknowledgement")
	}

	// Record ack
	if err := store.RecordAck(period, page.StatusHash); err != nil {
		t.Fatalf("RecordAck failed: %v", err)
	}

	// Check ack is recorded
	if !store.IsAcked(period, page.StatusHash) {
		t.Error("IsAcked should return true after recording")
	}

	// After ack, cue should not be available
	cue = engine.ComputeCue(inputs, true)
	if cue.Available {
		t.Error("Cue should not be available after acknowledgement")
	}
}

func TestSingleWhisperRule(t *testing.T) {
	clock := &fixedClock{t: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)}
	engine := reality.NewEngine(clock)

	inputs := &domainreality.RealityInputs{
		CircleID:           "test",
		NowBucket:          "2025-01-15",
		GmailConnected:     true,
		SyncBucket:         domainreality.SyncBucketRecent,
		SyncMagnitude:      domainreality.MagnitudeAFew,
		ObligationsHeld:    true,
		AutoSurface:        false,
		ShadowProviderKind: domainreality.ProviderOff,
	}

	tests := []struct {
		name               string
		surfaceCueActive   bool
		proofCueActive     bool
		firstMinutesActive bool
		expectRealityCue   bool
	}{
		{"no other cues", false, false, false, true},
		{"surface cue active", true, false, false, false},
		{"proof cue active", false, true, false, false},
		{"first-minutes cue active", false, false, true, false},
		{"multiple cues active", true, true, true, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			shouldShow := engine.ShouldShowRealityCue(
				inputs,
				false, // not acked
				tc.surfaceCueActive,
				tc.proofCueActive,
				false, // journeyCueActive
				tc.firstMinutesActive,
			)

			if shouldShow != tc.expectRealityCue {
				t.Errorf("ShouldShowRealityCue = %v, want %v", shouldShow, tc.expectRealityCue)
			}
		})
	}
}

func TestSyncBucketComputation(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name         string
		lastSyncTime time.Time
		expected     domainreality.SyncBucket
	}{
		{
			name:         "zero time",
			lastSyncTime: time.Time{},
			expected:     domainreality.SyncBucketNever,
		},
		{
			name:         "recent (same bucket)",
			lastSyncTime: time.Date(2025, 1, 15, 10, 25, 0, 0, time.UTC),
			expected:     domainreality.SyncBucketRecent,
		},
		{
			name:         "stale (hours ago)",
			lastSyncTime: time.Date(2025, 1, 15, 8, 0, 0, 0, time.UTC),
			expected:     domainreality.SyncBucketStale,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := reality.ComputeSyncBucket(tc.lastSyncTime, now)
			if result != tc.expected {
				t.Errorf("ComputeSyncBucket = %q, want %q", result, tc.expected)
			}
		})
	}
}

func TestSyncReceiptMagnitudeMapping(t *testing.T) {
	tests := []struct {
		input    string
		expected domainreality.MagnitudeBucket
	}{
		{"none", domainreality.MagnitudeNothing},
		{"handful", domainreality.MagnitudeAFew},
		{"several", domainreality.MagnitudeSeveral},
		{"many", domainreality.MagnitudeSeveral},
		{"unknown", domainreality.MagnitudeNA},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := reality.MapSyncReceiptMagnitude(tc.input)
			if result != tc.expected {
				t.Errorf("MapSyncReceiptMagnitude(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestShadowReceiptCountMapping(t *testing.T) {
	tests := []struct {
		count    int
		expected domainreality.MagnitudeBucket
	}{
		{0, domainreality.MagnitudeNothing},
		{1, domainreality.MagnitudeAFew},
		{5, domainreality.MagnitudeAFew},
		{6, domainreality.MagnitudeSeveral},
		{100, domainreality.MagnitudeSeveral},
	}

	for _, tc := range tests {
		t.Run(string(rune(tc.count)), func(t *testing.T) {
			result := reality.MapShadowReceiptCount(tc.count)
			if result != tc.expected {
				t.Errorf("MapShadowReceiptCount(%d) = %q, want %q", tc.count, result, tc.expected)
			}
		})
	}
}

func TestCanonicalStringFormat(t *testing.T) {
	inputs := &domainreality.RealityInputs{
		CircleID:           "test",
		NowBucket:          "2025-01-15",
		GmailConnected:     true,
		SyncBucket:         domainreality.SyncBucketRecent,
		SyncMagnitude:      domainreality.MagnitudeAFew,
		ObligationsHeld:    true,
		AutoSurface:        false,
		ShadowProviderKind: domainreality.ProviderStub,
		ShadowRealAllowed:  false,
		ShadowMagnitude:    domainreality.MagnitudeNothing,
		ChatConfigured:     true,
		EmbedConfigured:    false,
		EndpointConfigured: true,
	}

	canonical := inputs.CanonicalString()

	// Verify pipe-delimited format
	if !strings.HasPrefix(canonical, "REALITY_INPUTS|v1|") {
		t.Errorf("CanonicalString should start with 'REALITY_INPUTS|v1|', got: %s", canonical)
	}

	// Verify no JSON brackets
	if strings.Contains(canonical, "{") || strings.Contains(canonical, "[") {
		t.Error("CanonicalString should not contain JSON brackets")
	}

	// Verify deterministic
	canonical2 := inputs.CanonicalString()
	if canonical != canonical2 {
		t.Errorf("CanonicalString not deterministic: %s vs %s", canonical, canonical2)
	}
}

func TestStatusHashLength(t *testing.T) {
	clock := &fixedClock{t: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)}
	engine := reality.NewEngine(clock)

	inputs := &domainreality.RealityInputs{
		CircleID:           "test",
		NowBucket:          "2025-01-15",
		GmailConnected:     true,
		SyncBucket:         domainreality.SyncBucketRecent,
		ObligationsHeld:    true,
		ShadowProviderKind: domainreality.ProviderOff,
	}

	page := engine.BuildPage(inputs)

	// Should be 32 hex chars (128 bits)
	if len(page.StatusHash) != 32 {
		t.Errorf("StatusHash should be 32 hex chars, got %d: %s", len(page.StatusHash), page.StatusHash)
	}

	// Should be valid hex
	for _, c := range page.StatusHash {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("StatusHash contains non-hex character: %c", c)
		}
	}
}

func TestAckStoreBoundedRetention(t *testing.T) {
	clock := &fixedClock{t: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)}
	store := persist.NewRealityAckStore(func() time.Time { return clock.t })

	// Add 35 acks (exceeds 30 day limit)
	for i := 0; i < 35; i++ {
		period := time.Date(2025, 1, i+1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
		hash := "hash" + string(rune('a'+i))
		_ = store.RecordAck(period, hash)
	}

	// Should have at most 30 entries
	if store.Count() > 30 {
		t.Errorf("Store should have at most 30 entries, has %d", store.Count())
	}
}

func TestDefaultCueText(t *testing.T) {
	if domainreality.DefaultCueText == "" {
		t.Error("DefaultCueText should not be empty")
	}
	if domainreality.DefaultLinkText == "" {
		t.Error("DefaultLinkText should not be empty")
	}

	// Verify calm language
	if strings.Contains(strings.ToLower(domainreality.DefaultCueText), "urgency") {
		t.Error("DefaultCueText should not contain urgency language")
	}
	if strings.Contains(strings.ToLower(domainreality.DefaultCueText), "must") {
		t.Error("DefaultCueText should not contain demanding language")
	}
}

func TestLinesOrderDeterministic(t *testing.T) {
	clock := &fixedClock{t: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)}
	engine := reality.NewEngine(clock)

	inputs := &domainreality.RealityInputs{
		CircleID:           "test",
		NowBucket:          "2025-01-15",
		GmailConnected:     true,
		SyncBucket:         domainreality.SyncBucketRecent,
		SyncMagnitude:      domainreality.MagnitudeAFew,
		ObligationsHeld:    true,
		AutoSurface:        false,
		ShadowProviderKind: domainreality.ProviderAzureChat,
		ShadowRealAllowed:  true,
		ShadowMagnitude:    domainreality.MagnitudeSeveral,
		ChatConfigured:     true,
		EmbedConfigured:    true,
		EndpointConfigured: true,
		Region:             "uksouth",
	}

	page1 := engine.BuildPage(inputs)
	page2 := engine.BuildPage(inputs)

	for i := range page1.Lines {
		if page1.Lines[i].Label != page2.Lines[i].Label {
			t.Errorf("Line %d label differs: %s vs %s", i, page1.Lines[i].Label, page2.Lines[i].Label)
		}
		if page1.Lines[i].Value != page2.Lines[i].Value {
			t.Errorf("Line %d value differs: %s vs %s", i, page1.Lines[i].Value, page2.Lines[i].Value)
		}
	}
}

func TestCueNotAvailableWhenNotConnected(t *testing.T) {
	clock := &fixedClock{t: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)}
	engine := reality.NewEngine(clock)

	inputs := &domainreality.RealityInputs{
		CircleID:           "test",
		NowBucket:          "2025-01-15",
		GmailConnected:     false, // Not connected
		SyncBucket:         domainreality.SyncBucketNever,
		ObligationsHeld:    true,
		ShadowProviderKind: domainreality.ProviderOff,
	}

	cue := engine.ComputeCue(inputs, false)
	if cue.Available {
		t.Error("Cue should not be available when Gmail is not connected")
	}
}

func TestCueNotAvailableWhenNeverSynced(t *testing.T) {
	clock := &fixedClock{t: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)}
	engine := reality.NewEngine(clock)

	inputs := &domainreality.RealityInputs{
		CircleID:           "test",
		NowBucket:          "2025-01-15",
		GmailConnected:     true,                          // Connected
		SyncBucket:         domainreality.SyncBucketNever, // But never synced
		ObligationsHeld:    true,
		ShadowProviderKind: domainreality.ProviderOff,
	}

	cue := engine.ComputeCue(inputs, false)
	if cue.Available {
		t.Error("Cue should not be available when never synced")
	}
}
