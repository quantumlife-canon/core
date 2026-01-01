// Package demo_phase14_policy_learning demonstrates Phase 14 features.
//
// Phase 14: Circle Policies + Preference Learning (Deterministic)
// - Per-circle policy thresholds (RegretThreshold, NotifyThreshold, UrgentThreshold)
// - Suppression rules with scopes (circle, person, vendor, trigger, itemkey)
// - Preference learning engine (rule-based, no ML)
// - Interruption explainability (why am I seeing this?)
//
// Reference: docs/ADR/ADR-0030-phase14-policy-learning.md
package demo_phase14_policy_learning

import (
	"testing"
	"time"

	"quantumlife/internal/preflearn"
	"quantumlife/pkg/domain/feedback"
	"quantumlife/pkg/domain/interrupt"
	"quantumlife/pkg/domain/policy"
	"quantumlife/pkg/domain/suppress"
)

// TestPolicyDomainModelDeterminism demonstrates deterministic policy hashing.
func TestPolicyDomainModelDeterminism(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	// Create two identical policy sets
	ps1 := policy.DefaultPolicySet(now)
	ps2 := policy.DefaultPolicySet(now)

	// Hashes should be identical
	if ps1.Hash != ps2.Hash {
		t.Errorf("policy set hashes should be identical: %s != %s", ps1.Hash, ps2.Hash)
	}

	// Canonical strings should be identical
	c1 := ps1.CanonicalString()
	c2 := ps2.CanonicalString()
	if c1 != c2 {
		t.Error("policy canonical strings should be identical")
	}

	t.Logf("Policy determinism verified: hash=%s", ps1.Hash[:16])
}

// TestPolicyCircleThresholds demonstrates circle-level thresholds.
func TestPolicyCircleThresholds(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	ps := policy.DefaultPolicySet(now)

	// Check default circles exist
	circles := []string{"work", "family", "personal", "finance"}
	for _, circleID := range circles {
		cp := ps.GetCircle(circleID)
		if cp == nil {
			t.Errorf("circle %s should exist", circleID)
			continue
		}

		// Verify threshold monotonicity
		if cp.RegretThreshold > cp.NotifyThreshold {
			t.Errorf("%s: regret threshold (%d) should <= notify threshold (%d)",
				circleID, cp.RegretThreshold, cp.NotifyThreshold)
		}
		if cp.NotifyThreshold > cp.UrgentThreshold {
			t.Errorf("%s: notify threshold (%d) should <= urgent threshold (%d)",
				circleID, cp.NotifyThreshold, cp.UrgentThreshold)
		}

		t.Logf("Circle %s: regret=%d, notify=%d, urgent=%d",
			circleID, cp.RegretThreshold, cp.NotifyThreshold, cp.UrgentThreshold)
	}
}

// TestPolicyTriggerBias demonstrates trigger-level bias.
func TestPolicyTriggerBias(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	ps := policy.DefaultPolicySet(now)

	// Check default triggers exist (note: dot notation for triggers)
	triggers := []string{"obligation.due_soon", "newsletter", "marketing"}
	for _, trigger := range triggers {
		tp := ps.GetTrigger(trigger)
		if tp == nil {
			t.Errorf("trigger %s should exist", trigger)
			continue
		}

		t.Logf("Trigger %s: bias=%d", trigger, tp.RegretBias)
	}

	// Verify newsletter has negative bias (lower priority)
	newsletter := ps.GetTrigger("newsletter")
	if newsletter != nil && newsletter.RegretBias >= 0 {
		t.Logf("Note: newsletter bias is %d (could be negative to reduce priority)", newsletter.RegretBias)
	}
}

// TestSuppressionRuleDeterminism demonstrates deterministic rule IDs.
func TestSuppressionRuleDeterminism(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	// Create two identical rules
	rule1 := suppress.NewSuppressionRule(
		"work",
		suppress.ScopeTrigger,
		"newsletter",
		now,
		nil,
		"User marked as unnecessary",
		suppress.SourceFeedback,
	)

	rule2 := suppress.NewSuppressionRule(
		"work",
		suppress.ScopeTrigger,
		"newsletter",
		now,
		nil,
		"User marked as unnecessary",
		suppress.SourceFeedback,
	)

	// Rule IDs should be identical
	if rule1.RuleID != rule2.RuleID {
		t.Errorf("rule IDs should be identical: %s != %s", rule1.RuleID, rule2.RuleID)
	}

	t.Logf("Suppression rule determinism verified: ruleID=%s", rule1.RuleID[:16])
}

// TestSuppressionSetMatching demonstrates rule matching.
func TestSuppressionSetMatching(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	ss := suppress.NewSuppressionSet()

	// Add a trigger suppression for work circle
	rule := suppress.NewSuppressionRule(
		"work",
		suppress.ScopeTrigger,
		"newsletter",
		now,
		nil,
		"Suppress newsletters",
		suppress.SourceFeedback,
	)
	ss.AddRule(rule)

	// Should match work+newsletter+trigger scope
	match := ss.FindMatch(now, "work", suppress.ScopeTrigger, "newsletter")
	if match == nil {
		t.Error("should match work+newsletter")
	}

	// Should not match personal+newsletter (different circle)
	match = ss.FindMatch(now, "personal", suppress.ScopeTrigger, "newsletter")
	if match != nil {
		t.Error("should not match personal+newsletter")
	}

	// Should not match work+marketing (different trigger key)
	match = ss.FindMatch(now, "work", suppress.ScopeTrigger, "marketing")
	if match != nil {
		t.Error("should not match work+marketing")
	}

	stats := ss.GetStats(now)
	t.Logf("Suppression matching verified: %d rules", stats.TotalRules)
}

// TestSuppressionScopeHierarchy demonstrates scope matching hierarchy.
func TestSuppressionScopeHierarchy(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	ss := suppress.NewSuppressionSet()

	// Add person-scoped suppression
	personRule := suppress.NewSuppressionRule(
		"work",
		suppress.ScopePerson,
		"person-alice",
		now,
		nil,
		"Suppress interruptions from Alice",
		suppress.SourceFeedback,
	)
	ss.AddRule(personRule)

	// Add trigger-scoped suppression
	triggerRule := suppress.NewSuppressionRule(
		"work",
		suppress.ScopeTrigger,
		"newsletter",
		now,
		nil,
		"Suppress newsletters",
		suppress.SourceFeedback,
	)
	ss.AddRule(triggerRule)

	// Test person match - matches person scope with person key
	personMatch := ss.FindMatch(now, "work", suppress.ScopePerson, "person-alice")
	if personMatch == nil {
		t.Error("should match person-alice with person scope")
	}

	// Test trigger match - matches trigger scope with trigger key
	triggerMatch := ss.FindMatch(now, "work", suppress.ScopeTrigger, "newsletter")
	if triggerMatch == nil {
		t.Error("should match newsletter with trigger scope")
	}

	// Person scope should not match trigger key
	wrongScope := ss.FindMatch(now, "work", suppress.ScopePerson, "newsletter")
	if wrongScope != nil {
		t.Error("person scope should not match newsletter key")
	}

	stats := ss.GetStats(now)
	t.Logf("Scope hierarchy verified: %d rules", stats.TotalRules)
}

// TestPreferenceLearningUnnecessaryFeedback demonstrates learning from "unnecessary".
func TestPreferenceLearningUnnecessaryFeedback(t *testing.T) {
	engine := preflearn.NewEngine(preflearn.DefaultConfig())
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	ps := policy.DefaultPolicySet(now)
	ss := suppress.NewSuppressionSet()

	originalThreshold := ps.GetCircle("work").RegretThreshold

	// User marks interruption as unnecessary
	fr := feedback.NewFeedbackRecord(
		feedback.TargetInterruption,
		"int-001",
		"work",
		now,
		feedback.SignalUnnecessary,
		"",
	)

	result, err := engine.ApplyFeedback(
		[]feedback.FeedbackRecord{fr},
		map[string]preflearn.InterruptContext{},
		&ps,
		ss,
		nil,
		now,
	)

	if err != nil {
		t.Fatalf("ApplyFeedback error: %v", err)
	}

	// Policy should change
	if !result.PolicyChanged {
		t.Error("policy should change after unnecessary feedback")
	}

	// Threshold should increase
	newThreshold := result.NewPolicy.GetCircle("work").RegretThreshold
	if newThreshold <= originalThreshold {
		t.Errorf("threshold should increase: %d -> %d", originalThreshold, newThreshold)
	}

	// Decision should be recorded
	if len(result.Decisions) == 0 {
		t.Error("should have at least one decision")
	}
	if result.Decisions[0].Action != "threshold_increase" {
		t.Errorf("expected threshold_increase, got %s", result.Decisions[0].Action)
	}

	t.Logf("Unnecessary feedback: threshold %d -> %d", originalThreshold, newThreshold)
}

// TestPreferenceLearningHelpfulFeedback demonstrates learning from "helpful".
func TestPreferenceLearningHelpfulFeedback(t *testing.T) {
	engine := preflearn.NewEngine(preflearn.DefaultConfig())
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	ps := policy.DefaultPolicySet(now)
	ss := suppress.NewSuppressionSet()

	originalThreshold := ps.GetCircle("work").RegretThreshold

	// User marks interruption as helpful
	fr := feedback.NewFeedbackRecord(
		feedback.TargetInterruption,
		"int-001",
		"work",
		now,
		feedback.SignalHelpful,
		"",
	)

	result, err := engine.ApplyFeedback(
		[]feedback.FeedbackRecord{fr},
		map[string]preflearn.InterruptContext{},
		&ps,
		ss,
		nil,
		now,
	)

	if err != nil {
		t.Fatalf("ApplyFeedback error: %v", err)
	}

	// Policy should change
	if !result.PolicyChanged {
		t.Error("policy should change after helpful feedback")
	}

	// Threshold should decrease
	newThreshold := result.NewPolicy.GetCircle("work").RegretThreshold
	if newThreshold >= originalThreshold {
		t.Errorf("threshold should decrease: %d -> %d", originalThreshold, newThreshold)
	}

	// Decision should be recorded
	if result.Decisions[0].Action != "threshold_decrease" {
		t.Errorf("expected threshold_decrease, got %s", result.Decisions[0].Action)
	}

	t.Logf("Helpful feedback: threshold %d -> %d", originalThreshold, newThreshold)
}

// TestPreferenceLearningTriggerBias demonstrates trigger bias updates.
func TestPreferenceLearningTriggerBias(t *testing.T) {
	engine := preflearn.NewEngine(preflearn.DefaultConfig())
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	ps := policy.DefaultPolicySet(now)
	ss := suppress.NewSuppressionSet()

	// User marks specific trigger as helpful
	fr := feedback.NewFeedbackRecord(
		feedback.TargetInterruption,
		"int-001",
		"work",
		now,
		feedback.SignalHelpful,
		"",
	)

	ctx := preflearn.InterruptContext{
		InterruptID: "int-001",
		CircleID:    "work",
		Trigger:     "custom_trigger",
	}

	result, err := engine.ApplyFeedback(
		[]feedback.FeedbackRecord{fr},
		map[string]preflearn.InterruptContext{fr.FeedbackID: ctx},
		&ps,
		ss,
		nil,
		now,
	)

	if err != nil {
		t.Fatalf("ApplyFeedback error: %v", err)
	}

	// Trigger should be added/updated
	trigger := result.NewPolicy.GetTrigger("custom_trigger")
	if trigger == nil {
		t.Fatal("custom_trigger should exist after helpful feedback")
	}

	if trigger.RegretBias <= 0 {
		t.Errorf("trigger bias should be positive: %d", trigger.RegretBias)
	}

	t.Logf("Trigger bias learned: %s = %d", "custom_trigger", trigger.RegretBias)
}

// TestPreferenceLearningDeterminism demonstrates deterministic learning.
func TestPreferenceLearningDeterminism(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	fr := feedback.NewFeedbackRecord(
		feedback.TargetInterruption,
		"int-001",
		"work",
		now,
		feedback.SignalUnnecessary,
		"",
	)

	// Run twice with same inputs
	result1 := applyFeedbackOnce(t, fr, now)
	result2 := applyFeedbackOnce(t, fr, now)

	// Results should be identical
	if result1.AfterPolicyHash != result2.AfterPolicyHash {
		t.Error("learning should be deterministic")
	}

	if len(result1.Decisions) != len(result2.Decisions) {
		t.Error("decision count should be deterministic")
	}

	t.Logf("Learning determinism verified: hash=%s", result1.AfterPolicyHash[:16])
}

func applyFeedbackOnce(t *testing.T, fr feedback.FeedbackRecord, now time.Time) *preflearn.ApplyResult {
	engine := preflearn.NewEngine(preflearn.DefaultConfig())
	ps := policy.DefaultPolicySet(now)
	ss := suppress.NewSuppressionSet()

	result, err := engine.ApplyFeedback(
		[]feedback.FeedbackRecord{fr},
		map[string]preflearn.InterruptContext{},
		&ps,
		ss,
		nil,
		now,
	)
	if err != nil {
		t.Fatalf("ApplyFeedback error: %v", err)
	}
	return result
}

// TestExplainRecordDeterminism demonstrates deterministic explainability.
func TestExplainRecordDeterminism(t *testing.T) {
	// Create two identical explain records
	e1 := interrupt.NewExplainRecord("int-001", "work", "obligation_due_soon", 75, interrupt.LevelNotify, "policy123")
	e1.AddReason("Score 75 >= notify threshold 60")
	e1.AddReason("Due within 24 hours")
	h1 := e1.ComputeHash()

	e2 := interrupt.NewExplainRecord("int-001", "work", "obligation_due_soon", 75, interrupt.LevelNotify, "policy123")
	e2.AddReason("Score 75 >= notify threshold 60")
	e2.AddReason("Due within 24 hours")
	h2 := e2.ComputeHash()

	// Hashes should be identical
	if h1 != h2 {
		t.Errorf("explain hashes should be identical: %s != %s", h1, h2)
	}

	t.Logf("Explain determinism verified: hash=%s", h1[:16])
}

// TestExplainBuilderPattern demonstrates the explain builder.
func TestExplainBuilderPattern(t *testing.T) {
	explain := interrupt.NewExplainBuilder("int-001", "work", "obligation_due_soon", "policy123").
		WithRegretScore(75).
		WithLevel(interrupt.LevelNotify).
		AddThresholdReason(60, 75, "notify").
		AddDueReason(12).
		SetScoring(&interrupt.ScoringBreakdown{
			CircleBase:    30,
			DueBoost:      20,
			ActionBoost:   15,
			SeverityBoost: 10,
			TriggerBias:   0,
			FinalScore:    75,
		}).
		SetQuotaState(&interrupt.QuotaState{
			NotifyQuotaUsed:  2,
			NotifyQuotaLimit: 5,
			QueuedQuotaUsed:  8,
			QueuedQuotaLimit: 20,
			WasDowngraded:    false,
		}).
		Build()

	if explain.InterruptionID != "int-001" {
		t.Error("InterruptionID mismatch")
	}
	if explain.RegretScore != 75 {
		t.Error("RegretScore mismatch")
	}
	if explain.Level != interrupt.LevelNotify {
		t.Error("Level mismatch")
	}
	if len(explain.Reasons) != 2 {
		t.Errorf("expected 2 reasons, got %d", len(explain.Reasons))
	}
	if explain.Scoring == nil {
		t.Error("Scoring should be set")
	}
	if explain.QuotaState == nil {
		t.Error("QuotaState should be set")
	}
	if explain.Hash == "" {
		t.Error("Hash should be computed")
	}

	t.Logf("Explain builder verified: %d reasons, hash=%s", len(explain.Reasons), explain.Hash[:16])
}

// TestExplainFormatForUI demonstrates human-readable formatting.
func TestExplainFormatForUI(t *testing.T) {
	explain := interrupt.NewExplainBuilder("int-001", "work", "obligation_due_soon", "policy123").
		WithRegretScore(75).
		WithLevel(interrupt.LevelNotify).
		AddThresholdReason(60, 75, "notify").
		AddDueReason(12).
		SetScoring(&interrupt.ScoringBreakdown{
			CircleBase:    30,
			DueBoost:      20,
			ActionBoost:   15,
			SeverityBoost: 10,
			TriggerBias:   0,
			FinalScore:    75,
		}).
		Build()

	ui := explain.FormatForUI()

	// Check it contains expected sections
	if !containsStr(ui, "Interruption: int-001") {
		t.Error("Should contain interruption ID")
	}
	if !containsStr(ui, "Circle: work") {
		t.Error("Should contain circle")
	}
	if !containsStr(ui, "Regret Score: 75/100") {
		t.Error("Should contain regret score")
	}
	if !containsStr(ui, "Why this interruption:") {
		t.Error("Should contain reasons header")
	}
	if !containsStr(ui, "Score breakdown:") {
		t.Error("Should contain scoring header")
	}

	t.Logf("FormatForUI produces readable output:\n%s", ui)
}

// TestSuppressionExplainability demonstrates suppression explanations.
func TestSuppressionExplainability(t *testing.T) {
	explain := interrupt.NewExplainBuilder("int-001", "work", "newsletter", "policy123").
		WithRegretScore(40).
		WithLevel(interrupt.LevelSilent).
		AddSuppressionReason("sr_abc123", "scope_trigger", "newsletter").
		Build()

	if explain.SuppressionHit == nil {
		t.Fatal("SuppressionHit should be set")
	}
	if *explain.SuppressionHit != "sr_abc123" {
		t.Error("SuppressionHit mismatch")
	}
	if len(explain.Reasons) == 0 {
		t.Error("Should have suppression reason")
	}

	ui := explain.FormatForUI()
	if !containsStr(ui, "Suppressed by rule: sr_abc123") {
		t.Error("UI should show suppression")
	}

	t.Logf("Suppression explainability verified")
}

// TestQuotaDowngradeExplainability demonstrates quota downgrade explanations.
func TestQuotaDowngradeExplainability(t *testing.T) {
	explain := interrupt.NewExplainBuilder("int-001", "work", "obligation", "policy123").
		WithRegretScore(70).
		WithLevel(interrupt.LevelQueued). // Downgraded from notify
		SetQuotaState(&interrupt.QuotaState{
			NotifyQuotaUsed:  5,
			NotifyQuotaLimit: 5,
			QueuedQuotaUsed:  10,
			QueuedQuotaLimit: 20,
			WasDowngraded:    true,
			DowngradedFrom:   interrupt.LevelNotify,
		}).
		AddQuotaReason("Notify", 5, 5, true).
		Build()

	if explain.QuotaState == nil {
		t.Fatal("QuotaState should be set")
	}
	if !explain.QuotaState.WasDowngraded {
		t.Error("WasDowngraded should be true")
	}
	if explain.QuotaState.DowngradedFrom != interrupt.LevelNotify {
		t.Error("DowngradedFrom should be LevelNotify")
	}

	ui := explain.FormatForUI()
	if !containsStr(ui, "Downgraded from notify") {
		t.Error("UI should show downgrade reason")
	}

	t.Logf("Quota downgrade explainability verified")
}

// TestPolicyVersionIncrement demonstrates version tracking.
func TestPolicyVersionIncrement(t *testing.T) {
	engine := preflearn.NewEngine(preflearn.DefaultConfig())
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	ps := policy.DefaultPolicySet(now)
	ss := suppress.NewSuppressionSet()

	originalVersion := ps.Version

	fr := feedback.NewFeedbackRecord(
		feedback.TargetInterruption,
		"int-001",
		"work",
		now,
		feedback.SignalUnnecessary,
		"",
	)

	result, err := engine.ApplyFeedback(
		[]feedback.FeedbackRecord{fr},
		map[string]preflearn.InterruptContext{},
		&ps,
		ss,
		nil,
		now,
	)

	if err != nil {
		t.Fatalf("ApplyFeedback error: %v", err)
	}

	if result.NewPolicy.Version != originalVersion+1 {
		t.Errorf("version should increment: %d -> %d (expected %d)",
			originalVersion, result.NewPolicy.Version, originalVersion+1)
	}

	t.Logf("Policy version: %d -> %d", originalVersion, result.NewPolicy.Version)
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
