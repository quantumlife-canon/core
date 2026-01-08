// Package demo_phase41_interrupt_rehearsal demonstrates Phase 41 Live Interrupt Loop (APNs).
//
// These tests verify the rehearsal delivery flow without real network calls.
//
// CRITICAL INVARIANTS:
//   - NO goroutines. NO time.Now() - clock injection only.
//   - NO new decision logic - reuses Phase 32→33→34 pipeline outputs.
//   - Abstract payload only. No identifiers. No names. No merchants.
//   - Delivery cap: max 2/day per circle.
//   - Deterministic IDs/hashes: same inputs + same clock period => same hashes.
//
// Reference: docs/ADR/ADR-0078-phase41-live-interrupt-loop-apns.md
package demo_phase41_interrupt_rehearsal

import (
	"strings"
	"testing"
	"time"

	engine "quantumlife/internal/interruptrehearsal"
	"quantumlife/internal/persist"
	ir "quantumlife/pkg/domain/interruptrehearsal"
)

// ============================================================================
// Test 1: No device => rejected reject_no_device
// ============================================================================

func TestNoDevice_RejectsWithNoDevice(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	circleIDHash := "test_circle_hash_123"

	eng := engine.NewEngine(
		&engine.StubCandidateSource{CandidateHash: "candidate_hash", HasCandidate: true},
		&engine.StubPolicySource{Allowance: "allow_two_per_day", MaxPerDay: 2, Enabled: true},
		&engine.StubDeviceSource{HasDevice: false, TransportKind: ir.TransportNone},
		&engine.StubRateLimitSource{Allowed: true, RejectReason: ir.RejectNone, DailyCount: 0},
		&engine.StubSealedStatusSource{Ready: true},
		&engine.StubEnvelopeSource{Active: false},
	)

	receipt := eng.EvaluateEligibility(circleIDHash, now)

	if receipt.Status != ir.StatusRejected {
		t.Errorf("expected StatusRejected, got %s", receipt.Status)
	}
	if receipt.RejectReason != ir.RejectNoDevice {
		t.Errorf("expected RejectNoDevice, got %s", receipt.RejectReason)
	}
}

// ============================================================================
// Test 2: Policy disallows => rejected reject_policy_disallows
// ============================================================================

func TestPolicyDisallows_RejectsWithPolicyDisallows(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	circleIDHash := "test_circle_hash_123"

	eng := engine.NewEngine(
		&engine.StubCandidateSource{CandidateHash: "candidate_hash", HasCandidate: true},
		&engine.StubPolicySource{Allowance: "allow_none", MaxPerDay: 0, Enabled: false},
		&engine.StubDeviceSource{HasDevice: true, TransportKind: ir.TransportStub},
		&engine.StubRateLimitSource{Allowed: true, RejectReason: ir.RejectNone, DailyCount: 0},
		&engine.StubSealedStatusSource{Ready: true},
		&engine.StubEnvelopeSource{Active: false},
	)

	receipt := eng.EvaluateEligibility(circleIDHash, now)

	if receipt.Status != ir.StatusRejected {
		t.Errorf("expected StatusRejected, got %s", receipt.Status)
	}
	if receipt.RejectReason != ir.RejectPolicyDisallows {
		t.Errorf("expected RejectPolicyDisallows, got %s", receipt.RejectReason)
	}
}

// ============================================================================
// Test 3: No candidate => rejected reject_no_candidate
// ============================================================================

func TestNoCandidate_RejectsWithNoCandidate(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	circleIDHash := "test_circle_hash_123"

	eng := engine.NewEngine(
		&engine.StubCandidateSource{CandidateHash: "", HasCandidate: false},
		&engine.StubPolicySource{Allowance: "allow_two_per_day", MaxPerDay: 2, Enabled: true},
		&engine.StubDeviceSource{HasDevice: true, TransportKind: ir.TransportStub},
		&engine.StubRateLimitSource{Allowed: true, RejectReason: ir.RejectNone, DailyCount: 0},
		&engine.StubSealedStatusSource{Ready: true},
		&engine.StubEnvelopeSource{Active: false},
	)

	receipt := eng.EvaluateEligibility(circleIDHash, now)

	if receipt.Status != ir.StatusRejected {
		t.Errorf("expected StatusRejected, got %s", receipt.Status)
	}
	if receipt.RejectReason != ir.RejectNoCandidate {
		t.Errorf("expected RejectNoCandidate, got %s", receipt.RejectReason)
	}
}

// ============================================================================
// Test 4: Rate limited => rejected reject_rate_limited
// ============================================================================

func TestRateLimited_RejectsWithRateLimited(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	circleIDHash := "test_circle_hash_123"

	eng := engine.NewEngine(
		&engine.StubCandidateSource{CandidateHash: "candidate_hash", HasCandidate: true},
		&engine.StubPolicySource{Allowance: "allow_two_per_day", MaxPerDay: 2, Enabled: true},
		&engine.StubDeviceSource{HasDevice: true, TransportKind: ir.TransportStub},
		&engine.StubRateLimitSource{Allowed: false, RejectReason: ir.RejectRateLimited, DailyCount: 2},
		&engine.StubSealedStatusSource{Ready: true},
		&engine.StubEnvelopeSource{Active: false},
	)

	receipt := eng.EvaluateEligibility(circleIDHash, now)

	if receipt.Status != ir.StatusRejected {
		t.Errorf("expected StatusRejected, got %s", receipt.Status)
	}
	if receipt.RejectReason != ir.RejectRateLimited {
		t.Errorf("expected RejectRateLimited, got %s", receipt.RejectReason)
	}
}

// ============================================================================
// Test 5: Transport unavailable => rejected reject_transport_unavailable
// ============================================================================

func TestTransportUnavailable_RejectsWithTransportUnavailable(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	circleIDHash := "test_circle_hash_123"

	eng := engine.NewEngine(
		&engine.StubCandidateSource{CandidateHash: "candidate_hash", HasCandidate: true},
		&engine.StubPolicySource{Allowance: "allow_two_per_day", MaxPerDay: 2, Enabled: true},
		&engine.StubDeviceSource{HasDevice: true, TransportKind: ir.TransportNone},
		&engine.StubRateLimitSource{Allowed: true, RejectReason: ir.RejectNone, DailyCount: 0},
		&engine.StubSealedStatusSource{Ready: true},
		&engine.StubEnvelopeSource{Active: false},
	)

	receipt := eng.EvaluateEligibility(circleIDHash, now)

	if receipt.Status != ir.StatusRejected {
		t.Errorf("expected StatusRejected, got %s", receipt.Status)
	}
	if receipt.RejectReason != ir.RejectTransportUnavailable {
		t.Errorf("expected RejectTransportUnavailable, got %s", receipt.RejectReason)
	}
}

// ============================================================================
// Test 6: APNs selected but sealed key missing => rejected reject_sealed_key_missing
// ============================================================================

func TestAPNsSealedKeyMissing_RejectsWithSealedKeyMissing(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	circleIDHash := "test_circle_hash_123"

	eng := engine.NewEngine(
		&engine.StubCandidateSource{CandidateHash: "candidate_hash", HasCandidate: true},
		&engine.StubPolicySource{Allowance: "allow_two_per_day", MaxPerDay: 2, Enabled: true},
		&engine.StubDeviceSource{HasDevice: true, TransportKind: ir.TransportAPNs},
		&engine.StubRateLimitSource{Allowed: true, RejectReason: ir.RejectNone, DailyCount: 0},
		&engine.StubSealedStatusSource{Ready: false},
		&engine.StubEnvelopeSource{Active: false},
	)

	receipt := eng.EvaluateEligibility(circleIDHash, now)

	if receipt.Status != ir.StatusRejected {
		t.Errorf("expected StatusRejected, got %s", receipt.Status)
	}
	if receipt.RejectReason != ir.RejectSealedKeyMissing {
		t.Errorf("expected RejectSealedKeyMissing, got %s", receipt.RejectReason)
	}
}

// ============================================================================
// Test 7: Eligible builds deterministic AttemptIDHash
// ============================================================================

func TestEligible_BuildsDeterministicAttemptIDHash(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	circleIDHash := "test_circle_hash_123"
	candidateHash := "candidate_hash_abc"

	eng := engine.NewEngine(
		&engine.StubCandidateSource{CandidateHash: candidateHash, HasCandidate: true},
		&engine.StubPolicySource{Allowance: "allow_two_per_day", MaxPerDay: 2, Enabled: true},
		&engine.StubDeviceSource{HasDevice: true, TransportKind: ir.TransportStub},
		&engine.StubRateLimitSource{Allowed: true, RejectReason: ir.RejectNone, DailyCount: 0},
		&engine.StubSealedStatusSource{Ready: true},
		&engine.StubEnvelopeSource{Active: false},
	)

	receipt := eng.EvaluateEligibility(circleIDHash, now)

	if receipt.Status != ir.StatusRequested {
		t.Errorf("expected StatusRequested, got %s", receipt.Status)
	}
	if receipt.AttemptIDHash == "" {
		t.Error("expected non-empty AttemptIDHash")
	}

	// Verify determinism
	receipt2 := eng.EvaluateEligibility(circleIDHash, now)
	if receipt.AttemptIDHash != receipt2.AttemptIDHash {
		t.Errorf("expected same AttemptIDHash, got %s vs %s", receipt.AttemptIDHash, receipt2.AttemptIDHash)
	}
}

// ============================================================================
// Test 8: Same inputs + same clock => same receipt hash
// ============================================================================

func TestSameInputsSameClock_SameReceiptHash(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	circleIDHash := "test_circle_hash_123"

	eng := engine.NewEngine(
		&engine.StubCandidateSource{CandidateHash: "candidate_hash", HasCandidate: true},
		&engine.StubPolicySource{Allowance: "allow_two_per_day", MaxPerDay: 2, Enabled: true},
		&engine.StubDeviceSource{HasDevice: true, TransportKind: ir.TransportStub},
		&engine.StubRateLimitSource{Allowed: true, RejectReason: ir.RejectNone, DailyCount: 0},
		&engine.StubSealedStatusSource{Ready: true},
		&engine.StubEnvelopeSource{Active: false},
	)

	receipt1 := eng.EvaluateEligibility(circleIDHash, now)
	receipt2 := eng.EvaluateEligibility(circleIDHash, now)

	if receipt1.StatusHash != receipt2.StatusHash {
		t.Errorf("expected same StatusHash, got %s vs %s", receipt1.StatusHash, receipt2.StatusHash)
	}
}

// ============================================================================
// Test 9: FinalizeAfterAttempt maps latency bucket correctly
// ============================================================================

func TestFinalizeAfterAttempt_MapsLatencyBucketCorrectly(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	circleIDHash := "test_circle_hash_123"

	eng := engine.NewEngine(
		&engine.StubCandidateSource{CandidateHash: "candidate_hash", HasCandidate: true},
		&engine.StubPolicySource{Allowance: "allow_two_per_day", MaxPerDay: 2, Enabled: true},
		&engine.StubDeviceSource{HasDevice: true, TransportKind: ir.TransportStub},
		&engine.StubRateLimitSource{Allowed: true, RejectReason: ir.RejectNone, DailyCount: 0},
		&engine.StubSealedStatusSource{Ready: true},
		&engine.StubEnvelopeSource{Active: false},
	)

	receipt := eng.EvaluateEligibility(circleIDHash, now)

	// Test fast latency
	finalFast := eng.FinalizeAfterAttempt(receipt, true, ir.LatencyFast, ir.ErrorClassNone)
	if finalFast.LatencyBucket != ir.LatencyFast {
		t.Errorf("expected LatencyFast, got %s", finalFast.LatencyBucket)
	}

	// Test slow latency
	finalSlow := eng.FinalizeAfterAttempt(receipt, true, ir.LatencySlow, ir.ErrorClassNone)
	if finalSlow.LatencyBucket != ir.LatencySlow {
		t.Errorf("expected LatencySlow, got %s", finalSlow.LatencyBucket)
	}
}

// ============================================================================
// Test 10: Proof page never contains forbidden patterns
// ============================================================================

func TestProofPage_NeverContainsForbiddenPatterns(t *testing.T) {
	receipt := &ir.RehearsalReceipt{
		Kind:             ir.RehearsalInterruptDelivery,
		Status:           ir.StatusDelivered,
		RejectReason:     ir.RejectNone,
		PeriodKey:        "2026-01-08",
		CircleIDHash:     "circle_hash",
		CandidateHash:    "candidate_hash",
		AttemptIDHash:    "attempt_hash",
		TransportKind:    ir.TransportStub,
		DeliveryBucket:   ir.DeliveryOne,
		LatencyBucket:    ir.LatencyFast,
		ErrorClassBucket: ir.ErrorClassNone,
		TimeBucket:       "10:00",
	}
	receipt.StatusHash = receipt.ComputeStatusHash()

	proofPage := ir.BuildProofPageFromReceipt(receipt)

	// Check title
	if ir.ContainsForbiddenPattern(proofPage.Title) {
		t.Errorf("title contains forbidden pattern: %s", proofPage.Title)
	}

	// Check lines
	for _, line := range proofPage.Lines {
		if ir.ContainsForbiddenPattern(line) {
			t.Errorf("line contains forbidden pattern: %s", line)
		}
	}
}

// ============================================================================
// Test 11: Plan uses correct deep link target
// ============================================================================

func TestBuildPlan_UsesCorrectDeepLinkTarget(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	circleIDHash := "test_circle_hash_123"

	eng := engine.NewEngine(
		&engine.StubCandidateSource{CandidateHash: "candidate_hash", HasCandidate: true},
		&engine.StubPolicySource{Allowance: "allow_two_per_day", MaxPerDay: 2, Enabled: true},
		&engine.StubDeviceSource{HasDevice: true, TransportKind: ir.TransportStub},
		&engine.StubRateLimitSource{Allowed: true, RejectReason: ir.RejectNone, DailyCount: 0},
		&engine.StubSealedStatusSource{Ready: true},
		&engine.StubEnvelopeSource{Active: false},
	)

	receipt := eng.EvaluateEligibility(circleIDHash, now)
	plan := eng.BuildPlan(receipt)

	if plan.DeepLinkTarget != ir.DeepLinkTarget {
		t.Errorf("expected DeepLinkTarget=%s, got %s", ir.DeepLinkTarget, plan.DeepLinkTarget)
	}
}

// ============================================================================
// Test 12: BuildPlan returns nil for rejected receipt
// ============================================================================

func TestBuildPlan_ReturnsNilForRejectedReceipt(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	circleIDHash := "test_circle_hash_123"

	eng := engine.NewEngine(
		&engine.StubCandidateSource{CandidateHash: "", HasCandidate: false},
		&engine.StubPolicySource{Allowance: "allow_two_per_day", MaxPerDay: 2, Enabled: true},
		&engine.StubDeviceSource{HasDevice: true, TransportKind: ir.TransportStub},
		&engine.StubRateLimitSource{Allowed: true, RejectReason: ir.RejectNone, DailyCount: 0},
		&engine.StubSealedStatusSource{Ready: true},
		&engine.StubEnvelopeSource{Active: false},
	)

	receipt := eng.EvaluateEligibility(circleIDHash, now)
	plan := eng.BuildPlan(receipt)

	if plan != nil {
		t.Error("expected nil plan for rejected receipt")
	}
}

// ============================================================================
// Test 13: FinalizeAfterAttempt with delivered=true sets StatusDelivered
// ============================================================================

func TestFinalizeAfterAttempt_DeliveredSetsStatusDelivered(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	circleIDHash := "test_circle_hash_123"

	eng := engine.NewEngine(
		&engine.StubCandidateSource{CandidateHash: "candidate_hash", HasCandidate: true},
		&engine.StubPolicySource{Allowance: "allow_two_per_day", MaxPerDay: 2, Enabled: true},
		&engine.StubDeviceSource{HasDevice: true, TransportKind: ir.TransportStub},
		&engine.StubRateLimitSource{Allowed: true, RejectReason: ir.RejectNone, DailyCount: 0},
		&engine.StubSealedStatusSource{Ready: true},
		&engine.StubEnvelopeSource{Active: false},
	)

	receipt := eng.EvaluateEligibility(circleIDHash, now)
	finalized := eng.FinalizeAfterAttempt(receipt, true, ir.LatencyFast, ir.ErrorClassNone)

	if finalized.Status != ir.StatusDelivered {
		t.Errorf("expected StatusDelivered, got %s", finalized.Status)
	}
	if finalized.DeliveryBucket != ir.DeliveryOne {
		t.Errorf("expected DeliveryOne, got %s", finalized.DeliveryBucket)
	}
}

// ============================================================================
// Test 14: FinalizeAfterAttempt with error sets StatusFailed
// ============================================================================

func TestFinalizeAfterAttempt_ErrorSetsStatusFailed(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	circleIDHash := "test_circle_hash_123"

	eng := engine.NewEngine(
		&engine.StubCandidateSource{CandidateHash: "candidate_hash", HasCandidate: true},
		&engine.StubPolicySource{Allowance: "allow_two_per_day", MaxPerDay: 2, Enabled: true},
		&engine.StubDeviceSource{HasDevice: true, TransportKind: ir.TransportStub},
		&engine.StubRateLimitSource{Allowed: true, RejectReason: ir.RejectNone, DailyCount: 0},
		&engine.StubSealedStatusSource{Ready: true},
		&engine.StubEnvelopeSource{Active: false},
	)

	receipt := eng.EvaluateEligibility(circleIDHash, now)
	finalized := eng.FinalizeAfterAttempt(receipt, false, ir.LatencySlow, ir.ErrorClassTransient)

	if finalized.Status != ir.StatusFailed {
		t.Errorf("expected StatusFailed, got %s", finalized.Status)
	}
	if finalized.DeliveryBucket != ir.DeliveryNone {
		t.Errorf("expected DeliveryNone, got %s", finalized.DeliveryBucket)
	}
}

// ============================================================================
// Test 15: Store cap enforcement (second delivery same day rejected)
// ============================================================================

func TestStore_CapEnforcement_SecondDeliverySameDayRejected(t *testing.T) {
	store := persist.NewInterruptRehearsalStore(nil)
	circleIDHash := "test_circle_hash_123"
	periodKey := "2026-01-08"
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)

	// Add two delivered receipts
	for i := 0; i < 2; i++ {
		receipt := &ir.RehearsalReceipt{
			Kind:             ir.RehearsalInterruptDelivery,
			Status:           ir.StatusDelivered,
			RejectReason:     ir.RejectNone,
			PeriodKey:        periodKey,
			CircleIDHash:     circleIDHash,
			CandidateHash:    "candidate_hash",
			AttemptIDHash:    "attempt_hash_" + string(rune('0'+i)),
			TransportKind:    ir.TransportStub,
			DeliveryBucket:   ir.DeliveryOne,
			LatencyBucket:    ir.LatencyFast,
			ErrorClassBucket: ir.ErrorClassNone,
			TimeBucket:       "10:00",
		}
		receipt.StatusHash = receipt.ComputeStatusHash()
		_ = store.AppendReceipt(receipt, now)
	}

	// Check cap
	allowed, reason := store.CanDeliver(circleIDHash, periodKey)
	if allowed {
		t.Error("expected delivery to be rejected after cap reached")
	}
	if reason != ir.RejectRateLimited {
		t.Errorf("expected RejectRateLimited, got %s", reason)
	}
}

// ============================================================================
// Test 16: Store retention - FIFO evicts beyond max records
// ============================================================================

func TestStore_FIFOEvictionBeyondMaxRecords(t *testing.T) {
	store := persist.NewInterruptRehearsalStore(nil)
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)

	// Add more than MaxRecords (500)
	for i := 0; i < 510; i++ {
		receipt := &ir.RehearsalReceipt{
			Kind:             ir.RehearsalInterruptDelivery,
			Status:           ir.StatusDelivered,
			RejectReason:     ir.RejectNone,
			PeriodKey:        "2026-01-08",
			CircleIDHash:     "circle_" + string(rune('a'+i%26)),
			CandidateHash:    "candidate_hash",
			AttemptIDHash:    "attempt_hash_" + string(rune('0'+i%10)),
			TransportKind:    ir.TransportStub,
			DeliveryBucket:   ir.DeliveryOne,
			LatencyBucket:    ir.LatencyFast,
			ErrorClassBucket: ir.ErrorClassNone,
			TimeBucket:       "10:00",
		}
		receipt.StatusHash = receipt.ComputeStatusHash()
		_ = store.AppendReceipt(receipt, now)
	}

	if store.Count() > ir.MaxRecords {
		t.Errorf("expected at most %d records, got %d", ir.MaxRecords, store.Count())
	}
}

// ============================================================================
// Test 17: Store retention - older than 30 days evicted
// ============================================================================

func TestStore_OldRecordsEvicted(t *testing.T) {
	store := persist.NewInterruptRehearsalStore(nil)
	oldTime := time.Date(2025, 11, 1, 10, 0, 0, 0, time.UTC)
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)

	// Add old receipt
	oldReceipt := &ir.RehearsalReceipt{
		Kind:             ir.RehearsalInterruptDelivery,
		Status:           ir.StatusDelivered,
		RejectReason:     ir.RejectNone,
		PeriodKey:        "2025-11-01",
		CircleIDHash:     "old_circle",
		CandidateHash:    "candidate_hash",
		AttemptIDHash:    "old_attempt",
		TransportKind:    ir.TransportStub,
		DeliveryBucket:   ir.DeliveryOne,
		LatencyBucket:    ir.LatencyFast,
		ErrorClassBucket: ir.ErrorClassNone,
		TimeBucket:       "10:00",
	}
	oldReceipt.StatusHash = oldReceipt.ComputeStatusHash()
	_ = store.AppendReceipt(oldReceipt, oldTime)

	// Force eviction with current time
	store.EvictOldPeriods(now)

	if store.Count() != 0 {
		t.Errorf("expected 0 records after eviction, got %d", store.Count())
	}
}

// ============================================================================
// Test 18: Payload constants are abstract (no identifiers)
// ============================================================================

func TestPayloadConstants_NoIdentifiers(t *testing.T) {
	// Title should be generic
	if ir.ContainsForbiddenPattern(ir.PushTitle) {
		t.Errorf("PushTitle contains forbidden pattern: %s", ir.PushTitle)
	}

	// Body should be generic
	if ir.ContainsForbiddenPattern(ir.PushBody) {
		t.Errorf("PushBody contains forbidden pattern: %s", ir.PushBody)
	}

	// Deep link target should be abstract
	if strings.Contains(ir.DeepLinkTarget, "http") {
		t.Errorf("DeepLinkTarget contains URL: %s", ir.DeepLinkTarget)
	}
}

// ============================================================================
// Test 19: Proof shows only prefixes (first 8 chars) not full hashes
// ============================================================================

func TestProofPage_ShowsOnlyHashPrefixes(t *testing.T) {
	receipt := &ir.RehearsalReceipt{
		Kind:             ir.RehearsalInterruptDelivery,
		Status:           ir.StatusDelivered,
		RejectReason:     ir.RejectNone,
		PeriodKey:        "2026-01-08",
		CircleIDHash:     "a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6",
		CandidateHash:    "x1y2z3a4b5c6d7e8f9g0h1i2j3k4l5m6",
		AttemptIDHash:    "p1q2r3s4t5u6v7w8x9y0z1a2b3c4d5e6",
		TransportKind:    ir.TransportStub,
		DeliveryBucket:   ir.DeliveryOne,
		LatencyBucket:    ir.LatencyFast,
		ErrorClassBucket: ir.ErrorClassNone,
		TimeBucket:       "10:00",
	}
	receipt.StatusHash = receipt.ComputeStatusHash()

	proofPage := ir.BuildProofPageFromReceipt(receipt)

	if proofPage.ReceiptSummary == nil {
		t.Fatal("expected ReceiptSummary to be non-nil")
	}

	// Check prefixes are 8 chars
	if len(proofPage.ReceiptSummary.CandidateHashPrefix) > 8 {
		t.Errorf("CandidateHashPrefix too long: %d", len(proofPage.ReceiptSummary.CandidateHashPrefix))
	}
	if len(proofPage.ReceiptSummary.AttemptHashPrefix) > 8 {
		t.Errorf("AttemptHashPrefix too long: %d", len(proofPage.ReceiptSummary.AttemptHashPrefix))
	}
	if len(proofPage.ReceiptSummary.StatusHashPrefix) > 8 {
		t.Errorf("StatusHashPrefix too long: %d", len(proofPage.ReceiptSummary.StatusHashPrefix))
	}
}

// ============================================================================
// Test 20: Status transitions are valid
// ============================================================================

func TestStatusTransitions_AreValid(t *testing.T) {
	// requested->delivered is valid
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	circleIDHash := "test_circle_hash_123"

	eng := engine.NewEngine(
		&engine.StubCandidateSource{CandidateHash: "candidate_hash", HasCandidate: true},
		&engine.StubPolicySource{Allowance: "allow_two_per_day", MaxPerDay: 2, Enabled: true},
		&engine.StubDeviceSource{HasDevice: true, TransportKind: ir.TransportStub},
		&engine.StubRateLimitSource{Allowed: true, RejectReason: ir.RejectNone, DailyCount: 0},
		&engine.StubSealedStatusSource{Ready: true},
		&engine.StubEnvelopeSource{Active: false},
	)

	receipt := eng.EvaluateEligibility(circleIDHash, now)
	if receipt.Status != ir.StatusRequested {
		t.Errorf("expected StatusRequested, got %s", receipt.Status)
	}

	finalized := eng.FinalizeAfterAttempt(receipt, true, ir.LatencyFast, ir.ErrorClassNone)
	if finalized.Status != ir.StatusDelivered {
		t.Errorf("expected StatusDelivered after delivery, got %s", finalized.Status)
	}
}

// ============================================================================
// Test 21: Reject reason implies status_rejected always
// ============================================================================

func TestRejectReason_ImpliesStatusRejected(t *testing.T) {
	receipt := &ir.RehearsalReceipt{
		Kind:         ir.RehearsalInterruptDelivery,
		Status:       ir.StatusRejected,
		RejectReason: ir.RejectNoDevice,
		PeriodKey:    "2026-01-08",
		CircleIDHash: "circle_hash",
	}

	err := receipt.Validate()
	if err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}

	// Invalid: rejected status without reason
	invalidReceipt := &ir.RehearsalReceipt{
		Kind:         ir.RehearsalInterruptDelivery,
		Status:       ir.StatusRejected,
		RejectReason: ir.RejectNone, // Invalid!
		PeriodKey:    "2026-01-08",
		CircleIDHash: "circle_hash",
	}

	err = invalidReceipt.Validate()
	if err == nil {
		t.Error("expected validation error for rejected without reason")
	}
}

// ============================================================================
// Test 22: Plan payload title and body are constants
// ============================================================================

func TestPlan_PayloadTitleAndBodyAreConstants(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	circleIDHash := "test_circle_hash_123"

	eng := engine.NewEngine(
		&engine.StubCandidateSource{CandidateHash: "candidate_hash", HasCandidate: true},
		&engine.StubPolicySource{Allowance: "allow_two_per_day", MaxPerDay: 2, Enabled: true},
		&engine.StubDeviceSource{HasDevice: true, TransportKind: ir.TransportStub},
		&engine.StubRateLimitSource{Allowed: true, RejectReason: ir.RejectNone, DailyCount: 0},
		&engine.StubSealedStatusSource{Ready: true},
		&engine.StubEnvelopeSource{Active: false},
	)

	receipt := eng.EvaluateEligibility(circleIDHash, now)
	plan := eng.BuildPlan(receipt)

	if plan.PayloadTitle != ir.PushTitle {
		t.Errorf("expected PayloadTitle=%s, got %s", ir.PushTitle, plan.PayloadTitle)
	}
	if plan.PayloadBody != ir.PushBody {
		t.Errorf("expected PayloadBody=%s, got %s", ir.PushBody, plan.PayloadBody)
	}
}

// ============================================================================
// Test 23: BuildRehearsePage shows correct eligibility status
// ============================================================================

func TestBuildRehearsePage_ShowsCorrectEligibilityStatus(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	circleIDHash := "test_circle_hash_123"

	// Eligible scenario
	eng := engine.NewEngine(
		&engine.StubCandidateSource{CandidateHash: "candidate_hash", HasCandidate: true},
		&engine.StubPolicySource{Allowance: "allow_two_per_day", MaxPerDay: 2, Enabled: true},
		&engine.StubDeviceSource{HasDevice: true, TransportKind: ir.TransportStub},
		&engine.StubRateLimitSource{Allowed: true, RejectReason: ir.RejectNone, DailyCount: 0},
		&engine.StubSealedStatusSource{Ready: true},
		&engine.StubEnvelopeSource{Active: false},
	)

	page := eng.BuildRehearsePage(circleIDHash, now)

	if !page.CanSend {
		t.Error("expected CanSend=true for eligible scenario")
	}
	if !page.DeviceRegistered {
		t.Error("expected DeviceRegistered=true")
	}
	if !page.CandidateAvailable {
		t.Error("expected CandidateAvailable=true")
	}
}

// ============================================================================
// Test 24: Different clock periods produce different receipt hashes
// ============================================================================

func TestDifferentClockPeriods_ProduceDifferentReceiptHashes(t *testing.T) {
	circleIDHash := "test_circle_hash_123"

	eng := engine.NewEngine(
		&engine.StubCandidateSource{CandidateHash: "candidate_hash", HasCandidate: true},
		&engine.StubPolicySource{Allowance: "allow_two_per_day", MaxPerDay: 2, Enabled: true},
		&engine.StubDeviceSource{HasDevice: true, TransportKind: ir.TransportStub},
		&engine.StubRateLimitSource{Allowed: true, RejectReason: ir.RejectNone, DailyCount: 0},
		&engine.StubSealedStatusSource{Ready: true},
		&engine.StubEnvelopeSource{Active: false},
	)

	now1 := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	now2 := time.Date(2026, 1, 9, 10, 0, 0, 0, time.UTC) // Different day

	receipt1 := eng.EvaluateEligibility(circleIDHash, now1)
	receipt2 := eng.EvaluateEligibility(circleIDHash, now2)

	if receipt1.PeriodKey == receipt2.PeriodKey {
		t.Error("expected different PeriodKeys for different days")
	}
	if receipt1.AttemptIDHash == receipt2.AttemptIDHash {
		t.Error("expected different AttemptIDHash for different periods")
	}
}

// ============================================================================
// Test 25: Enum validation works correctly
// ============================================================================

func TestEnumValidation(t *testing.T) {
	// Valid rehearsal kind
	if err := ir.RehearsalInterruptDelivery.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Invalid rehearsal kind
	invalidKind := ir.RehearsalKind("invalid_kind")
	if err := invalidKind.Validate(); err == nil {
		t.Error("expected validation error for invalid kind")
	}

	// Valid status
	if err := ir.StatusDelivered.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Invalid status
	invalidStatus := ir.RehearsalStatus("invalid_status")
	if err := invalidStatus.Validate(); err == nil {
		t.Error("expected validation error for invalid status")
	}
}

// ============================================================================
// Test 26: Store GetLatestByCircleAndPeriod returns latest receipt
// ============================================================================

func TestStore_GetLatestByCircleAndPeriod(t *testing.T) {
	store := persist.NewInterruptRehearsalStore(nil)
	circleIDHash := "test_circle_hash_123"
	periodKey := "2026-01-08"
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)

	// Add first receipt
	receipt1 := &ir.RehearsalReceipt{
		Kind:             ir.RehearsalInterruptDelivery,
		Status:           ir.StatusRejected,
		RejectReason:     ir.RejectNoCandidate,
		PeriodKey:        periodKey,
		CircleIDHash:     circleIDHash,
		CandidateHash:    "",
		AttemptIDHash:    "attempt_1",
		TransportKind:    ir.TransportStub,
		DeliveryBucket:   ir.DeliveryNone,
		LatencyBucket:    ir.LatencyNA,
		ErrorClassBucket: ir.ErrorClassNone,
		TimeBucket:       "10:00",
	}
	receipt1.StatusHash = receipt1.ComputeStatusHash()
	_ = store.AppendReceipt(receipt1, now)

	// Add second receipt (later)
	receipt2 := &ir.RehearsalReceipt{
		Kind:             ir.RehearsalInterruptDelivery,
		Status:           ir.StatusDelivered,
		RejectReason:     ir.RejectNone,
		PeriodKey:        periodKey,
		CircleIDHash:     circleIDHash,
		CandidateHash:    "candidate_hash",
		AttemptIDHash:    "attempt_2",
		TransportKind:    ir.TransportStub,
		DeliveryBucket:   ir.DeliveryOne,
		LatencyBucket:    ir.LatencyFast,
		ErrorClassBucket: ir.ErrorClassNone,
		TimeBucket:       "11:00",
	}
	receipt2.StatusHash = receipt2.ComputeStatusHash()
	_ = store.AppendReceipt(receipt2, now.Add(time.Hour))

	// Get latest
	latest := store.GetLatestByCircleAndPeriod(circleIDHash, periodKey)
	if latest == nil {
		t.Fatal("expected non-nil latest receipt")
	}
	if latest.Status != ir.StatusDelivered {
		t.Errorf("expected latest to be delivered, got %s", latest.Status)
	}
}

// ============================================================================
// Test 27: HashPrefix helper returns correct length
// ============================================================================

func TestHashPrefix_ReturnsCorrectLength(t *testing.T) {
	fullHash := "a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6"
	prefix := ir.HashPrefix(fullHash)

	if len(prefix) != 8 {
		t.Errorf("expected prefix length 8, got %d", len(prefix))
	}
	if prefix != "a1b2c3d4" {
		t.Errorf("expected prefix a1b2c3d4, got %s", prefix)
	}

	// Short hash returns empty
	shortHash := "abc"
	shortPrefix := ir.HashPrefix(shortHash)
	if shortPrefix != "" {
		t.Errorf("expected empty prefix for short hash, got %s", shortPrefix)
	}
}
