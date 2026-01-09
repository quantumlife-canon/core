// Package demo_phase44_2_enforcement_audit provides demonstration tests for Phase 44.2.
//
// These tests verify the Enforcement Wiring Audit functionality.
// Target: 28-36 tests covering all Phase 44.2 invariants.
//
// CRITICAL INVARIANTS TESTED:
//   - Clamp enforces HOLD-only when contract present
//   - Envelope cannot override HOLD-only clamp
//   - Interrupt policy cannot override HOLD-only clamp
//   - Commerce is always clamped to HOLD
//   - Manifest tracks all enforcement wrappers
//   - Audit fails when manifest is incomplete
//   - Hash computation is deterministic
//
// Reference: docs/ADR/ADR-0082-phase44-2-enforcement-wiring-audit.md
package demo_phase44_2_enforcement_audit

import (
	"testing"
	"time"

	"quantumlife/internal/enforcementaudit"
	"quantumlife/internal/enforcementclamp"
	ea "quantumlife/pkg/domain/enforcementaudit"
)

// ============================================================================
// Test Clock
// ============================================================================

type testClock struct {
	now time.Time
}

func (c *testClock) Now() time.Time {
	return c.now
}

func newTestClock() *testClock {
	return &testClock{
		now: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
	}
}

// ============================================================================
// Test Stores
// ============================================================================

type testAuditStore struct {
	runs []ea.AuditRun
}

func (s *testAuditStore) AppendRun(run ea.AuditRun) error {
	s.runs = append(s.runs, run)
	return nil
}

func (s *testAuditStore) GetLatestRun() *ea.AuditRun {
	if len(s.runs) == 0 {
		return nil
	}
	run := s.runs[len(s.runs)-1]
	return &run
}

func (s *testAuditStore) ListRuns() []ea.AuditRun {
	return s.runs
}

type testAckStore struct {
	acks map[string]bool
}

func (s *testAckStore) AppendAck(ack ea.AuditAck) error {
	if s.acks == nil {
		s.acks = make(map[string]bool)
	}
	s.acks[ack.RunHash] = true
	return nil
}

func (s *testAckStore) IsAcked(runHash string) bool {
	return s.acks[runHash]
}

// ============================================================================
// Manifest Tests
// ============================================================================

func TestManifestCanonicalDeterministic(t *testing.T) {
	manifest1 := enforcementaudit.BuildCompleteManifest()
	manifest2 := enforcementaudit.BuildCompleteManifest()

	if manifest1.CanonicalString() != manifest2.CanonicalString() {
		t.Error("Manifest canonical string is not deterministic")
	}

	if manifest1.ComputeHash() != manifest2.ComputeHash() {
		t.Error("Manifest hash is not deterministic")
	}
}

func TestManifestIsComplete_AllTrue(t *testing.T) {
	manifest := enforcementaudit.BuildCompleteManifest()

	if !manifest.IsComplete() {
		t.Error("BuildCompleteManifest should return complete manifest")
	}

	missing := manifest.MissingComponents()
	if len(missing) != 0 {
		t.Errorf("Complete manifest should have no missing components, got: %v", missing)
	}
}

func TestManifestIsComplete_MissingComponent(t *testing.T) {
	manifest := enforcementaudit.NewManifestBuilder().
		WithPressureGate().
		WithDelegatedHolding().
		// Missing TrustTransfer
		WithInterruptPreview().
		WithDeliveryOrchestrator().
		WithTimeWindowAdapter().
		WithCommerceExcluded().
		WithEnvelopeCannotOverride().
		WithInterruptPolicyCannotOverride().
		WithClampWrapper().
		Build()

	if manifest.IsComplete() {
		t.Error("Manifest should not be complete when TrustTransfer is missing")
	}

	missing := manifest.MissingComponents()
	found := false
	for _, m := range missing {
		if m == "trust_transfer" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Missing components should include trust_transfer")
	}
}

// ============================================================================
// Clamp Tests
// ============================================================================

func TestClampHoldOnlyOverridesSurface(t *testing.T) {
	engine := enforcementclamp.NewEngine()

	input := enforcementclamp.ClampInput{
		CircleIDHash:    ea.ComputeEvidenceHash("test_circle"),
		RawDecisionKind: "surface",
		RawReasonBucket: "test",
		ContractsSummary: enforcementclamp.ContractsSummary{
			HasHoldOnlyContract: true,
		},
	}

	output := engine.ClampOutcome(input)

	if output.ClampedDecisionKind != ea.ClampedHold {
		t.Errorf("Expected ClampedHold, got %s", output.ClampedDecisionKind)
	}
	if !output.WasClamped {
		t.Error("Expected WasClamped to be true")
	}
}

func TestClampHoldOnlyOverridesInterruptCandidate(t *testing.T) {
	engine := enforcementclamp.NewEngine()

	input := enforcementclamp.ClampInput{
		CircleIDHash:    ea.ComputeEvidenceHash("test_circle"),
		RawDecisionKind: "interrupt_candidate",
		RawReasonBucket: "test",
		ContractsSummary: enforcementclamp.ContractsSummary{
			HasHoldOnlyContract: true,
		},
	}

	output := engine.ClampOutcome(input)

	if output.ClampedDecisionKind != ea.ClampedHold {
		t.Errorf("Expected ClampedHold, got %s", output.ClampedDecisionKind)
	}
	if !output.WasClamped {
		t.Error("Expected WasClamped to be true")
	}
}

func TestClampHoldOnlyOverridesDeliver(t *testing.T) {
	engine := enforcementclamp.NewEngine()

	input := enforcementclamp.ClampInput{
		CircleIDHash:    ea.ComputeEvidenceHash("test_circle"),
		RawDecisionKind: "deliver",
		RawReasonBucket: "test",
		ContractsSummary: enforcementclamp.ContractsSummary{
			HasHoldOnlyContract: true,
		},
	}

	output := engine.ClampOutcome(input)

	if output.ClampedDecisionKind != ea.ClampedHold {
		t.Errorf("Expected ClampedHold, got %s", output.ClampedDecisionKind)
	}
	if !output.WasClamped {
		t.Error("Expected WasClamped to be true")
	}
}

func TestClampHoldOnlyOverridesExecute(t *testing.T) {
	engine := enforcementclamp.NewEngine()

	input := enforcementclamp.ClampInput{
		CircleIDHash:    ea.ComputeEvidenceHash("test_circle"),
		RawDecisionKind: "execute",
		RawReasonBucket: "test",
		ContractsSummary: enforcementclamp.ContractsSummary{
			HasHoldOnlyContract: true,
		},
	}

	output := engine.ClampOutcome(input)

	if output.ClampedDecisionKind != ea.ClampedHold {
		t.Errorf("Expected ClampedHold, got %s", output.ClampedDecisionKind)
	}
	if !output.WasClamped {
		t.Error("Expected WasClamped to be true")
	}
}

func TestClampQueueProofAllowed(t *testing.T) {
	engine := enforcementclamp.NewEngine()

	input := enforcementclamp.ClampInput{
		CircleIDHash:    ea.ComputeEvidenceHash("test_circle"),
		RawDecisionKind: "surface",
		RawReasonBucket: "test",
		ContractsSummary: enforcementclamp.ContractsSummary{
			HasHoldOnlyContract: true,
			QueueProofRequested: true,
		},
	}

	output := engine.ClampOutcome(input)

	if output.ClampedDecisionKind != ea.ClampedQueueProof {
		t.Errorf("Expected ClampedQueueProof, got %s", output.ClampedDecisionKind)
	}
}

func TestClampNeverEnablesDeliver(t *testing.T) {
	engine := enforcementclamp.NewEngine()

	// Even if we try to pass through "deliver" with a contract
	input := enforcementclamp.ClampInput{
		CircleIDHash:    ea.ComputeEvidenceHash("test_circle"),
		RawDecisionKind: "deliver",
		RawReasonBucket: "test",
		ContractsSummary: enforcementclamp.ContractsSummary{
			HasHoldOnlyContract: true,
		},
	}

	output := engine.ClampOutcome(input)

	// Should never be "deliver" in output
	if output.ClampedDecisionKind.CanonicalString() == "deliver" {
		t.Error("Clamp should never return deliver when contract active")
	}
}

func TestClampCommerceAlwaysHold(t *testing.T) {
	engine := enforcementclamp.NewEngine()

	input := enforcementclamp.ClampInput{
		CircleIDHash:    ea.ComputeEvidenceHash("test_circle"),
		RawDecisionKind: "surface",
		RawReasonBucket: "commerce",
		ContractsSummary: enforcementclamp.ContractsSummary{
			IsCommerce: true,
			// Note: no contract, but commerce should still be clamped
		},
	}

	output := engine.ClampOutcome(input)

	if output.ClampedDecisionKind != ea.ClampedHold {
		t.Errorf("Commerce should always be clamped to HOLD, got %s", output.ClampedDecisionKind)
	}
	if !output.WasClamped {
		t.Error("Commerce should be marked as clamped")
	}
	if output.ClampReason != "commerce" {
		t.Errorf("ClampReason should be 'commerce', got %s", output.ClampReason)
	}
}

func TestClampTransferContract(t *testing.T) {
	engine := enforcementclamp.NewEngine()

	input := enforcementclamp.ClampInput{
		CircleIDHash:    ea.ComputeEvidenceHash("test_circle"),
		RawDecisionKind: "surface",
		RawReasonBucket: "test",
		ContractsSummary: enforcementclamp.ContractsSummary{
			HasTransferContract: true,
		},
	}

	output := engine.ClampOutcome(input)

	if output.ClampedDecisionKind != ea.ClampedHold {
		t.Errorf("Transfer contract should clamp to HOLD, got %s", output.ClampedDecisionKind)
	}
	if output.ClampReason != "transfer_contract" {
		t.Errorf("ClampReason should be 'transfer_contract', got %s", output.ClampReason)
	}
}

func TestEnvelopeCannotOverrideHoldOnly(t *testing.T) {
	summary := enforcementclamp.ContractsSummary{
		HasHoldOnlyContract: true,
		EnvelopeActive:      true,
	}

	if enforcementclamp.CanEnvelopeOverride(summary) {
		t.Error("Envelope should not be able to override when HOLD-only contract active")
	}
}

func TestInterruptPolicyCannotOverrideHoldOnly(t *testing.T) {
	summary := enforcementclamp.ContractsSummary{
		HasHoldOnlyContract:   true,
		InterruptPolicyActive: true,
	}

	if enforcementclamp.CanInterruptPolicyOverride(summary) {
		t.Error("Interrupt policy should not be able to override when HOLD-only contract active")
	}
}

func TestEnvelopeCanOverrideWhenNoContract(t *testing.T) {
	summary := enforcementclamp.ContractsSummary{
		EnvelopeActive: true,
	}

	if !enforcementclamp.CanEnvelopeOverride(summary) {
		t.Error("Envelope should be able to override when no contract")
	}
}

func TestIsForbiddenDecision(t *testing.T) {
	forbidden := []string{"surface", "SURFACE", "interrupt_candidate", "INTERRUPT_CANDIDATE", "deliver", "DELIVER", "execute", "EXECUTE"}
	allowed := []string{"hold", "HOLD", "queue_proof", "QUEUE_PROOF", "no_effect", "NO_EFFECT"}

	for _, d := range forbidden {
		if !enforcementclamp.IsForbiddenDecision(d) {
			t.Errorf("%s should be forbidden", d)
		}
	}

	for _, d := range allowed {
		if enforcementclamp.IsForbiddenDecision(d) {
			t.Errorf("%s should not be forbidden", d)
		}
	}
}

// ============================================================================
// Audit Engine Tests
// ============================================================================

func TestAuditRunHashDeterministic(t *testing.T) {
	clk := newTestClock()
	store := &testAuditStore{}
	ackStore := &testAckStore{}
	engine := enforcementaudit.NewEngine(store, ackStore, clk)

	manifest := enforcementaudit.BuildCompleteManifest()
	probes := enforcementaudit.DefaultProbeInputs()

	run1 := engine.RunAudit(manifest, probes)
	run2 := engine.RunAudit(manifest, probes)

	if run1.RunHash != run2.RunHash {
		t.Error("Audit run hash should be deterministic with same inputs")
	}
}

func TestAuditPassesWhenManifestComplete(t *testing.T) {
	clk := newTestClock()
	store := &testAuditStore{}
	ackStore := &testAckStore{}
	engine := enforcementaudit.NewEngine(store, ackStore, clk)

	manifest := enforcementaudit.BuildCompleteManifest()
	probes := enforcementaudit.DefaultProbeInputs()

	run := engine.RunAudit(manifest, probes)

	if run.Status != ea.StatusPass {
		t.Errorf("Expected status pass, got %s", run.Status)
	}
}

func TestAuditFailsWhenManifestMissingClamp(t *testing.T) {
	clk := newTestClock()
	store := &testAuditStore{}
	ackStore := &testAckStore{}
	engine := enforcementaudit.NewEngine(store, ackStore, clk)

	// Build incomplete manifest (missing clamp wrapper)
	manifest := enforcementaudit.NewManifestBuilder().
		WithPressureGate().
		WithDelegatedHolding().
		WithTrustTransfer().
		WithInterruptPreview().
		WithDeliveryOrchestrator().
		WithTimeWindowAdapter().
		WithCommerceExcluded().
		WithEnvelopeCannotOverride().
		WithInterruptPolicyCannotOverride().
		// Missing: WithClampWrapper()
		Build()

	probes := enforcementaudit.DefaultProbeInputs()

	run := engine.RunAudit(manifest, probes)

	if run.Status != ea.StatusFail {
		t.Error("Expected status fail when clamp wrapper missing")
	}
}

func TestAuditCriticalFailsBucket(t *testing.T) {
	cases := []struct {
		count    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{2, "2-5"},
		{5, "2-5"},
		{6, "6+"},
		{10, "6+"},
	}

	for _, tc := range cases {
		bucket := ea.ComputeCriticalFailsBucket(tc.count)
		if bucket != tc.expected {
			t.Errorf("Count %d: expected bucket %s, got %s", tc.count, tc.expected, bucket)
		}
	}
}

func TestAuditPersistence(t *testing.T) {
	clk := newTestClock()
	store := &testAuditStore{}
	ackStore := &testAckStore{}
	engine := enforcementaudit.NewEngine(store, ackStore, clk)

	manifest := enforcementaudit.BuildCompleteManifest()
	probes := enforcementaudit.DefaultProbeInputs()

	_ = engine.RunAudit(manifest, probes)

	if len(store.runs) != 1 {
		t.Errorf("Expected 1 run in store, got %d", len(store.runs))
	}
}

func TestAuditAcknowledgment(t *testing.T) {
	clk := newTestClock()
	store := &testAuditStore{}
	ackStore := &testAckStore{}
	engine := enforcementaudit.NewEngine(store, ackStore, clk)

	manifest := enforcementaudit.BuildCompleteManifest()
	probes := enforcementaudit.DefaultProbeInputs()

	run := engine.RunAudit(manifest, probes)

	if engine.IsRunAcked(run.RunHash) {
		t.Error("Run should not be acked initially")
	}

	_, _ = engine.AcknowledgeRun(run.RunHash)

	if !engine.IsRunAcked(run.RunHash) {
		t.Error("Run should be acked after acknowledgment")
	}
}

func TestProofPageNoIdentifiers(t *testing.T) {
	clk := newTestClock()
	store := &testAuditStore{}
	ackStore := &testAckStore{}
	engine := enforcementaudit.NewEngine(store, ackStore, clk)

	manifest := enforcementaudit.BuildCompleteManifest()
	probes := enforcementaudit.DefaultProbeInputs()

	_ = engine.RunAudit(manifest, probes)

	page := engine.BuildProofPage()

	// Check that lines don't contain identifiers
	forbiddenPatterns := []string{"@", "http://", "https://", "gmail", "truelayer"}
	for _, line := range page.Lines {
		for _, pattern := range forbiddenPatterns {
			if contains(line, pattern) {
				t.Errorf("Proof page line contains forbidden pattern '%s': %s", pattern, line)
			}
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsImpl(s, substr))
}

func containsImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ============================================================================
// Domain Type Tests
// ============================================================================

func TestAuditCheckValidation(t *testing.T) {
	check := &ea.AuditCheck{
		Target:       ea.TargetPressurePipeline,
		Check:        ea.CheckContractApplied,
		Status:       ea.StatusPass,
		Severity:     ea.SeverityInfo,
		Component:    "pressure_gate",
		EvidenceHash: ea.ComputeEvidenceHash("test evidence"),
	}

	if err := check.Validate(); err != nil {
		t.Errorf("Valid check should pass validation: %v", err)
	}
}

func TestAuditCheckValidation_InvalidComponent(t *testing.T) {
	check := &ea.AuditCheck{
		Target:       ea.TargetPressurePipeline,
		Check:        ea.CheckContractApplied,
		Status:       ea.StatusPass,
		Severity:     ea.SeverityInfo,
		Component:    "invalid_component_not_in_allowlist",
		EvidenceHash: ea.ComputeEvidenceHash("test evidence"),
	}

	if err := check.Validate(); err == nil {
		t.Error("Check with invalid component should fail validation")
	}
}

func TestAuditRunValidation_MaxChecks(t *testing.T) {
	checks := make([]ea.AuditCheck, ea.MaxChecksPerRun+1)
	for i := range checks {
		checks[i] = ea.AuditCheck{
			Target:       ea.TargetPressurePipeline,
			Check:        ea.CheckContractApplied,
			Status:       ea.StatusPass,
			Severity:     ea.SeverityInfo,
			Component:    "pressure_gate",
			EvidenceHash: ea.ComputeEvidenceHash("test"),
		}
	}

	run := &ea.AuditRun{
		PeriodKey: "2024-01-15-10",
		Status:    ea.StatusPass,
		Checks:    checks,
	}

	if err := run.Validate(); err == nil {
		t.Error("Run with too many checks should fail validation")
	}
}

func TestAllowedComponents(t *testing.T) {
	allowed := []string{
		"pressure_gate",
		"interrupt_preview",
		"delivery_orchestrator",
		"delegated_holding",
		"trust_transfer",
		"clamp_wrapper",
	}

	for _, comp := range allowed {
		if !ea.IsAllowedComponent(comp) {
			t.Errorf("%s should be an allowed component", comp)
		}
	}

	if ea.IsAllowedComponent("not_allowed") {
		t.Error("not_allowed should not be an allowed component")
	}
}

func TestClampedDecisionValidation(t *testing.T) {
	validDecisions := []ea.ClampedDecisionKind{
		ea.ClampedNoEffect,
		ea.ClampedHold,
		ea.ClampedQueueProof,
	}

	for _, d := range validDecisions {
		if err := d.Validate(); err != nil {
			t.Errorf("%s should be valid: %v", d, err)
		}
	}

	invalid := ea.ClampedDecisionKind("surface")
	if err := invalid.Validate(); err == nil {
		t.Error("'surface' should not be a valid ClampedDecisionKind")
	}
}

func TestEvidenceHash(t *testing.T) {
	hash1 := ea.ComputeEvidenceHash("test")
	hash2 := ea.ComputeEvidenceHash("test")

	if hash1 != hash2 {
		t.Error("Evidence hash should be deterministic")
	}

	if len(hash1) != 64 {
		t.Errorf("Evidence hash should be 64 hex chars, got %d", len(hash1))
	}
}

// ============================================================================
// Batch Clamp Tests
// ============================================================================

func TestClampOutcomesBatch(t *testing.T) {
	engine := enforcementclamp.NewEngine()

	inputs := []enforcementclamp.ClampInput{
		{
			CircleIDHash:    ea.ComputeEvidenceHash("circle1"),
			RawDecisionKind: "surface",
			ContractsSummary: enforcementclamp.ContractsSummary{
				HasHoldOnlyContract: true,
			},
		},
		{
			CircleIDHash:    ea.ComputeEvidenceHash("circle2"),
			RawDecisionKind: "hold",
			ContractsSummary: enforcementclamp.ContractsSummary{
				HasHoldOnlyContract: true,
			},
		},
	}

	outputs := engine.ClampOutcomes(inputs)

	if len(outputs) != 2 {
		t.Errorf("Expected 2 outputs, got %d", len(outputs))
	}

	// First should be clamped
	if !outputs[0].WasClamped {
		t.Error("First output should be clamped")
	}

	// Second should not be clamped (already HOLD)
	if outputs[1].WasClamped {
		t.Error("Second output should not be clamped")
	}
}

func TestClampStats(t *testing.T) {
	outputs := []enforcementclamp.ClampOutput{
		{WasClamped: true, ClampReason: "commerce"},
		{WasClamped: true, ClampReason: "hold_only_contract"},
		{WasClamped: true, ClampReason: "transfer_contract"},
		{WasClamped: false},
	}

	stats := enforcementclamp.ComputeStats(outputs)

	if stats.TotalClamped != 3 {
		t.Errorf("Expected 3 total clamped, got %d", stats.TotalClamped)
	}
	if stats.CommerceBlocked != 1 {
		t.Errorf("Expected 1 commerce blocked, got %d", stats.CommerceBlocked)
	}
	if stats.ContractBlocked != 1 {
		t.Errorf("Expected 1 contract blocked, got %d", stats.ContractBlocked)
	}
	if stats.TransferBlocked != 1 {
		t.Errorf("Expected 1 transfer blocked, got %d", stats.TransferBlocked)
	}
}
