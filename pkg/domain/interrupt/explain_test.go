package interrupt

import (
	"testing"
)

func TestExplainRecordCanonicalString(t *testing.T) {
	e := NewExplainRecord("int-001", "work", "obligation_due_soon", 75, LevelNotify, "policy123")
	e.AddReason("Score 75 >= notify threshold 60")
	e.AddReason("Due within 24 hours")

	s1 := e.CanonicalString()
	s2 := e.CanonicalString()

	if s1 != s2 {
		t.Error("CanonicalString should be deterministic")
	}

	// Check contains expected parts
	if !contains(s1, "id:int-001") {
		t.Error("Should contain id")
	}
	if !contains(s1, "circle:work") {
		t.Error("Should contain circle")
	}
	if !contains(s1, "level:notify") {
		t.Error("Should contain level")
	}
}

func TestExplainRecordHashDeterminism(t *testing.T) {
	e1 := NewExplainRecord("int-001", "work", "obligation_due_soon", 75, LevelNotify, "policy123")
	e1.AddReason("Test reason")
	h1 := e1.ComputeHash()

	e2 := NewExplainRecord("int-001", "work", "obligation_due_soon", 75, LevelNotify, "policy123")
	e2.AddReason("Test reason")
	h2 := e2.ComputeHash()

	if h1 != h2 {
		t.Errorf("Hash should be deterministic: %s != %s", h1, h2)
	}

	// Different content should have different hash
	e3 := NewExplainRecord("int-002", "work", "obligation_due_soon", 75, LevelNotify, "policy123")
	e3.AddReason("Test reason")
	h3 := e3.ComputeHash()

	if h1 == h3 {
		t.Error("Different content should have different hash")
	}
}

func TestScoringBreakdownCanonicalString(t *testing.T) {
	s := ScoringBreakdown{
		CircleBase:    30,
		DueBoost:      20,
		ActionBoost:   15,
		SeverityBoost: 10,
		TriggerBias:   5,
		FinalScore:    75,
	}

	str := s.CanonicalString()
	expected := "base:30|due_boost:20|action_boost:15|severity_boost:10|trigger_bias:5|final:75"

	if str != expected {
		t.Errorf("CanonicalString = %q, want %q", str, expected)
	}
}

func TestQuotaStateCanonicalString(t *testing.T) {
	q := QuotaState{
		NotifyQuotaUsed:  3,
		NotifyQuotaLimit: 5,
		QueuedQuotaUsed:  10,
		QueuedQuotaLimit: 20,
		WasDowngraded:    false,
	}

	str := q.CanonicalString()
	expected := "notify_used:3|notify_limit:5|queued_used:10|queued_limit:20|downgraded:none"

	if str != expected {
		t.Errorf("CanonicalString = %q, want %q", str, expected)
	}

	// With downgrade
	q.WasDowngraded = true
	q.DowngradedFrom = LevelNotify

	str = q.CanonicalString()
	if !contains(str, "downgraded:notify") {
		t.Error("Should show downgraded level")
	}
}

func TestExplainBuilderBasic(t *testing.T) {
	builder := NewExplainBuilder("int-001", "work", "obligation_due_soon", "policy123")

	explain := builder.
		WithRegretScore(75).
		WithLevel(LevelNotify).
		AddThresholdReason(60, 75, "notify").
		AddDueReason(12).
		Build()

	if explain.InterruptionID != "int-001" {
		t.Error("InterruptionID mismatch")
	}
	if explain.RegretScore != 75 {
		t.Error("RegretScore mismatch")
	}
	if explain.Level != LevelNotify {
		t.Error("Level mismatch")
	}
	if len(explain.Reasons) != 2 {
		t.Errorf("Expected 2 reasons, got %d", len(explain.Reasons))
	}
	if explain.Hash == "" {
		t.Error("Hash should be computed")
	}
}

func TestExplainBuilderWithScoring(t *testing.T) {
	scoring := &ScoringBreakdown{
		CircleBase:    30,
		DueBoost:      20,
		ActionBoost:   15,
		SeverityBoost: 10,
		TriggerBias:   5,
		FinalScore:    75,
	}

	explain := NewExplainBuilder("int-001", "work", "test", "policy123").
		WithRegretScore(75).
		WithLevel(LevelNotify).
		SetScoring(scoring).
		Build()

	if explain.Scoring == nil {
		t.Fatal("Scoring should be set")
	}
	if explain.Scoring.FinalScore != 75 {
		t.Error("Scoring mismatch")
	}
}

func TestExplainBuilderWithQuota(t *testing.T) {
	quota := &QuotaState{
		NotifyQuotaUsed:  4,
		NotifyQuotaLimit: 5,
		QueuedQuotaUsed:  15,
		QueuedQuotaLimit: 20,
		WasDowngraded:    false,
	}

	explain := NewExplainBuilder("int-001", "work", "test", "policy123").
		WithRegretScore(75).
		WithLevel(LevelNotify).
		SetQuotaState(quota).
		AddQuotaReason("Notify", 4, 5, false).
		Build()

	if explain.QuotaState == nil {
		t.Fatal("QuotaState should be set")
	}
	if len(explain.Reasons) == 0 {
		t.Error("Should have quota reason")
	}
}

func TestExplainBuilderWithSuppression(t *testing.T) {
	explain := NewExplainBuilder("int-001", "work", "newsletter", "policy123").
		WithRegretScore(40).
		WithLevel(LevelSilent).
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
}

func TestExplainRecordFormatForUI(t *testing.T) {
	explain := NewExplainBuilder("int-001", "work", "obligation_due_soon", "policy123").
		WithRegretScore(75).
		WithLevel(LevelNotify).
		AddThresholdReason(60, 75, "notify").
		AddDueReason(12).
		SetScoring(&ScoringBreakdown{
			CircleBase:    30,
			DueBoost:      20,
			ActionBoost:   15,
			SeverityBoost: 10,
			TriggerBias:   0,
			FinalScore:    75,
		}).
		SetQuotaState(&QuotaState{
			NotifyQuotaUsed:  2,
			NotifyQuotaLimit: 5,
			QueuedQuotaUsed:  8,
			QueuedQuotaLimit: 20,
			WasDowngraded:    false,
		}).
		Build()

	ui := explain.FormatForUI()

	// Check it contains expected sections
	if !contains(ui, "Interruption: int-001") {
		t.Error("Should contain interruption ID")
	}
	if !contains(ui, "Circle: work") {
		t.Error("Should contain circle")
	}
	if !contains(ui, "Why this interruption:") {
		t.Error("Should contain reasons section")
	}
	if !contains(ui, "Score breakdown:") {
		t.Error("Should contain scoring section")
	}
	if !contains(ui, "Quota status:") {
		t.Error("Should contain quota section")
	}
}

func TestExplainReasonOrdering(t *testing.T) {
	e1 := NewExplainRecord("int-001", "work", "test", 75, LevelNotify, "policy123")
	e1.AddReason("Reason A")
	e1.AddReason("Reason B")
	e1.AddReason("Reason C")
	c1 := e1.CanonicalString()

	e2 := NewExplainRecord("int-001", "work", "test", 75, LevelNotify, "policy123")
	e2.AddReason("Reason A")
	e2.AddReason("Reason B")
	e2.AddReason("Reason C")
	c2 := e2.CanonicalString()

	if c1 != c2 {
		t.Error("Reason order should be preserved and deterministic")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
