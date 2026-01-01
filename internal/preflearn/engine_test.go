package preflearn

import (
	"testing"
	"time"

	"quantumlife/pkg/domain/feedback"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/policy"
	"quantumlife/pkg/domain/suppress"
)

// mockHistory implements FeedbackHistory for testing.
type mockHistory struct {
	records map[string][]feedback.FeedbackRecord // circleID|trigger -> records
}

func newMockHistory() *mockHistory {
	return &mockHistory{
		records: make(map[string][]feedback.FeedbackRecord),
	}
}

func (h *mockHistory) Add(circleID identity.EntityID, trigger string, fr feedback.FeedbackRecord) {
	key := string(circleID) + "|" + trigger
	h.records[key] = append(h.records[key], fr)
}

func (h *mockHistory) GetRecentByCircleAndTrigger(circleID identity.EntityID, trigger string, since time.Time) []feedback.FeedbackRecord {
	key := string(circleID) + "|" + trigger
	var result []feedback.FeedbackRecord
	for _, fr := range h.records[key] {
		if !fr.CapturedAt.Before(since) && fr.Signal == feedback.SignalUnnecessary {
			result = append(result, fr)
		}
	}
	return result
}

func TestApplyFeedbackEmpty(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	ps := policy.DefaultPolicySet(now)
	ss := suppress.NewSuppressionSet()

	result, err := engine.ApplyFeedback(
		[]feedback.FeedbackRecord{},
		map[string]InterruptContext{},
		&ps,
		ss,
		nil,
		now,
	)

	if err != nil {
		t.Fatalf("ApplyFeedback error: %v", err)
	}

	if result.PolicyChanged {
		t.Error("PolicyChanged should be false for empty feedback")
	}

	if len(result.SuppressAdded) != 0 {
		t.Error("SuppressAdded should be empty")
	}

	if len(result.Decisions) != 0 {
		t.Error("Decisions should be empty")
	}
}

func TestApplyFeedbackUnnecessary(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	ps := policy.DefaultPolicySet(now)
	ss := suppress.NewSuppressionSet()

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
		map[string]InterruptContext{},
		&ps,
		ss,
		nil,
		now,
	)

	if err != nil {
		t.Fatalf("ApplyFeedback error: %v", err)
	}

	if !result.PolicyChanged {
		t.Error("PolicyChanged should be true")
	}

	if len(result.Decisions) != 1 {
		t.Fatalf("Expected 1 decision, got %d", len(result.Decisions))
	}

	if result.Decisions[0].Action != "threshold_increase" {
		t.Errorf("Expected threshold_increase, got %s", result.Decisions[0].Action)
	}

	// Check threshold was increased
	workPolicy := result.NewPolicy.GetCircle("work")
	if workPolicy == nil {
		t.Fatal("work circle should exist")
	}

	originalWork := ps.GetCircle("work")
	if workPolicy.RegretThreshold != originalWork.RegretThreshold+5 {
		t.Errorf("RegretThreshold should increase by 5: got %d, want %d",
			workPolicy.RegretThreshold, originalWork.RegretThreshold+5)
	}
}

func TestApplyFeedbackHelpful(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	ps := policy.DefaultPolicySet(now)
	ss := suppress.NewSuppressionSet()

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
		map[string]InterruptContext{},
		&ps,
		ss,
		nil,
		now,
	)

	if err != nil {
		t.Fatalf("ApplyFeedback error: %v", err)
	}

	if !result.PolicyChanged {
		t.Error("PolicyChanged should be true")
	}

	if len(result.Decisions) != 1 {
		t.Fatalf("Expected 1 decision, got %d", len(result.Decisions))
	}

	if result.Decisions[0].Action != "threshold_decrease" {
		t.Errorf("Expected threshold_decrease, got %s", result.Decisions[0].Action)
	}

	// Check threshold was decreased
	workPolicy := result.NewPolicy.GetCircle("work")
	if workPolicy == nil {
		t.Fatal("work circle should exist")
	}

	originalWork := ps.GetCircle("work")
	if workPolicy.RegretThreshold != originalWork.RegretThreshold-3 {
		t.Errorf("RegretThreshold should decrease by 3: got %d, want %d",
			workPolicy.RegretThreshold, originalWork.RegretThreshold-3)
	}
}

func TestApplyFeedbackTriggerBias(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	ps := policy.DefaultPolicySet(now)
	ss := suppress.NewSuppressionSet()

	fr := feedback.NewFeedbackRecord(
		feedback.TargetInterruption,
		"int-001",
		"work",
		now,
		feedback.SignalHelpful,
		"",
	)

	ctx := InterruptContext{
		InterruptID: "int-001",
		CircleID:    "work",
		Trigger:     "new_trigger",
	}

	result, err := engine.ApplyFeedback(
		[]feedback.FeedbackRecord{fr},
		map[string]InterruptContext{fr.FeedbackID: ctx},
		&ps,
		ss,
		nil,
		now,
	)

	if err != nil {
		t.Fatalf("ApplyFeedback error: %v", err)
	}

	if !result.PolicyChanged {
		t.Error("PolicyChanged should be true")
	}

	if result.Decisions[0].Action != "trigger_bias_increase" {
		t.Errorf("Expected trigger_bias_increase, got %s", result.Decisions[0].Action)
	}

	// Check trigger was added
	triggerPolicy := result.NewPolicy.GetTrigger("new_trigger")
	if triggerPolicy == nil {
		t.Fatal("new_trigger should exist")
	}

	if triggerPolicy.RegretBias != 5 {
		t.Errorf("RegretBias should be 5, got %d", triggerPolicy.RegretBias)
	}
}

func TestApplyFeedbackRepeatedUnnecessary(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	ps := policy.DefaultPolicySet(now)
	ss := suppress.NewSuppressionSet()

	// Create history with one previous unnecessary
	history := newMockHistory()
	prevFr := feedback.NewFeedbackRecord(
		feedback.TargetInterruption,
		"int-000",
		"work",
		now.Add(-1*time.Hour),
		feedback.SignalUnnecessary,
		"",
	)
	history.Add("work", "newsletter", prevFr)

	// New unnecessary feedback
	fr := feedback.NewFeedbackRecord(
		feedback.TargetInterruption,
		"int-001",
		"work",
		now,
		feedback.SignalUnnecessary,
		"",
	)

	ctx := InterruptContext{
		InterruptID: "int-001",
		CircleID:    "work",
		Trigger:     "newsletter",
	}

	result, err := engine.ApplyFeedback(
		[]feedback.FeedbackRecord{fr},
		map[string]InterruptContext{fr.FeedbackID: ctx},
		&ps,
		ss,
		history,
		now,
	)

	if err != nil {
		t.Fatalf("ApplyFeedback error: %v", err)
	}

	if len(result.SuppressAdded) != 1 {
		t.Fatalf("Expected 1 suppression rule, got %d", len(result.SuppressAdded))
	}

	rule := result.SuppressAdded[0]
	if rule.CircleID != "work" {
		t.Errorf("CircleID should be work, got %s", rule.CircleID)
	}
	if rule.Scope != suppress.ScopeTrigger {
		t.Errorf("Scope should be scope_trigger, got %s", rule.Scope)
	}
	if rule.Key != "newsletter" {
		t.Errorf("Key should be newsletter, got %s", rule.Key)
	}
	if rule.Source != suppress.SourceFeedback {
		t.Errorf("Source should be feedback, got %s", rule.Source)
	}

	if result.Decisions[0].Action != "suppression_add" {
		t.Errorf("Expected suppression_add, got %s", result.Decisions[0].Action)
	}
}

func TestApplyFeedbackPersonSuppression(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	ps := policy.DefaultPolicySet(now)
	ss := suppress.NewSuppressionSet()

	// Create history with one previous unnecessary
	history := newMockHistory()
	prevFr := feedback.NewFeedbackRecord(
		feedback.TargetInterruption,
		"int-000",
		"work",
		now.Add(-1*time.Hour),
		feedback.SignalUnnecessary,
		"",
	)
	history.Add("work", "newsletter", prevFr)

	// New unnecessary feedback with person context
	fr := feedback.NewFeedbackRecord(
		feedback.TargetInterruption,
		"int-001",
		"work",
		now,
		feedback.SignalUnnecessary,
		"",
	)

	ctx := InterruptContext{
		InterruptID: "int-001",
		CircleID:    "work",
		Trigger:     "newsletter",
		PersonID:    "person-alice",
	}

	result, err := engine.ApplyFeedback(
		[]feedback.FeedbackRecord{fr},
		map[string]InterruptContext{fr.FeedbackID: ctx},
		&ps,
		ss,
		history,
		now,
	)

	if err != nil {
		t.Fatalf("ApplyFeedback error: %v", err)
	}

	if len(result.SuppressAdded) != 1 {
		t.Fatalf("Expected 1 suppression rule, got %d", len(result.SuppressAdded))
	}

	rule := result.SuppressAdded[0]
	// Should prefer person scope
	if rule.Scope != suppress.ScopePerson {
		t.Errorf("Scope should be scope_person, got %s", rule.Scope)
	}
	if rule.Key != "person-alice" {
		t.Errorf("Key should be person-alice, got %s", rule.Key)
	}
}

func TestApplyFeedbackThresholdFloor(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	// Create policy with threshold already at floor
	ps := policy.PolicySet{
		Version:    1,
		CapturedAt: now,
		Circles: map[string]policy.CirclePolicy{
			"work": {
				CircleID:         "work",
				RegretThreshold:  5, // At floor
				NotifyThreshold:  50,
				UrgentThreshold:  75,
				DailyNotifyQuota: 10,
				DailyQueuedQuota: 50,
			},
		},
		Triggers: make(map[string]policy.TriggerPolicy),
	}
	ps.ComputeHash()

	ss := suppress.NewSuppressionSet()

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
		map[string]InterruptContext{},
		&ps,
		ss,
		nil,
		now,
	)

	if err != nil {
		t.Fatalf("ApplyFeedback error: %v", err)
	}

	// Threshold should stay at floor
	workPolicy := result.NewPolicy.GetCircle("work")
	if workPolicy.RegretThreshold != 5 {
		t.Errorf("RegretThreshold should stay at floor 5, got %d", workPolicy.RegretThreshold)
	}
}

func TestApplyFeedbackThresholdCeiling(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	// Create policy with threshold near ceiling
	ps := policy.PolicySet{
		Version:    1,
		CapturedAt: now,
		Circles: map[string]policy.CirclePolicy{
			"work": {
				CircleID:         "work",
				RegretThreshold:  92, // Near ceiling
				NotifyThreshold:  93,
				UrgentThreshold:  95,
				DailyNotifyQuota: 10,
				DailyQueuedQuota: 50,
			},
		},
		Triggers: make(map[string]policy.TriggerPolicy),
	}
	ps.ComputeHash()

	ss := suppress.NewSuppressionSet()

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
		map[string]InterruptContext{},
		&ps,
		ss,
		nil,
		now,
	)

	if err != nil {
		t.Fatalf("ApplyFeedback error: %v", err)
	}

	// Threshold should cap at ceiling
	workPolicy := result.NewPolicy.GetCircle("work")
	if workPolicy.RegretThreshold > 95 {
		t.Errorf("RegretThreshold should cap at 95, got %d", workPolicy.RegretThreshold)
	}
}

func TestApplyFeedbackDeterminism(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	ps := policy.DefaultPolicySet(now)
	ss := suppress.NewSuppressionSet()

	fr := feedback.NewFeedbackRecord(
		feedback.TargetInterruption,
		"int-001",
		"work",
		now,
		feedback.SignalUnnecessary,
		"",
	)

	result1, _ := engine.ApplyFeedback(
		[]feedback.FeedbackRecord{fr},
		map[string]InterruptContext{},
		&ps,
		ss,
		nil,
		now,
	)

	result2, _ := engine.ApplyFeedback(
		[]feedback.FeedbackRecord{fr},
		map[string]InterruptContext{},
		&ps,
		ss,
		nil,
		now,
	)

	if result1.AfterPolicyHash != result2.AfterPolicyHash {
		t.Error("Policy hash should be deterministic")
	}

	if len(result1.Decisions) != len(result2.Decisions) {
		t.Error("Decisions count should be deterministic")
	}
}

func TestDecisionRecordCanonicalString(t *testing.T) {
	d := DecisionRecord{
		FeedbackID: "fb-001",
		Action:     "threshold_increase",
		Reason:     "unnecessary_feedback",
		Details:    "circle:work threshold+5",
	}

	s1 := d.CanonicalString()
	s2 := d.CanonicalString()

	if s1 != s2 {
		t.Error("CanonicalString should be deterministic")
	}

	expected := "decision|fb-001|threshold_increase|unnecessary_feedback|circle:work threshold+5"
	if s1 != expected {
		t.Errorf("CanonicalString = %q, want %q", s1, expected)
	}
}
