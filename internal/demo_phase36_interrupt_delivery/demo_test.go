// Package demo_phase36_interrupt_delivery contains demo tests for Phase 36.
//
// These tests verify the Interrupt Delivery Orchestrator implementation.
//
// CRITICAL INVARIANTS TESTED:
//   - Delivery is EXPLICIT. POST-only.
//   - Max 2 deliveries per day.
//   - Deterministic ordering (sorted by hash).
//   - Deduplication by (candidate_hash, period).
//   - Policy must allow for delivery.
//   - Trust must not be fragile.
//   - Transport-agnostic.
//   - Hash-only persistence.
//
// Reference: docs/ADR/ADR-0073-phase36-interrupt-delivery-orchestrator.md
package demo_phase36_interrupt_delivery

import (
	"testing"
	"time"

	"quantumlife/internal/interruptdelivery"
	"quantumlife/internal/persist"
	delivery "quantumlife/pkg/domain/interruptdelivery"
)

// testPeriodKey returns a period key that won't be evicted during tests.
// Uses today's date to ensure it's within retention window.
func testPeriodKey() string {
	return time.Now().Format("2006-01-02")
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test: Domain Model Validation
// ═══════════════════════════════════════════════════════════════════════════════

func TestDeliveryCandidate_Validate(t *testing.T) {
	periodKey := testPeriodKey()

	t.Run("valid candidate passes validation", func(t *testing.T) {
		candidate := &delivery.DeliveryCandidate{
			CandidateHash: "abc123def456",
			CircleIDHash:  "circle_hash_123",
			DecisionHash:  "decision_hash_456",
			PeriodKey:     periodKey,
		}

		err := candidate.Validate()
		if err != nil {
			t.Errorf("expected valid candidate to pass, got: %v", err)
		}
	})

	t.Run("missing candidate_hash fails", func(t *testing.T) {
		candidate := &delivery.DeliveryCandidate{
			CircleIDHash: "circle_hash_123",
			DecisionHash: "decision_hash_456",
			PeriodKey:    periodKey,
		}

		err := candidate.Validate()
		if err == nil {
			t.Error("expected missing candidate_hash to fail")
		}
	})

	t.Run("missing period_key fails", func(t *testing.T) {
		candidate := &delivery.DeliveryCandidate{
			CandidateHash: "abc123def456",
			CircleIDHash:  "circle_hash_123",
			DecisionHash:  "decision_hash_456",
		}

		err := candidate.Validate()
		if err == nil {
			t.Error("expected missing period_key to fail")
		}
	})
}

func TestDeliveryAttempt_CanonicalString(t *testing.T) {
	periodKey := testPeriodKey()
	attempt := &delivery.DeliveryAttempt{
		CandidateHash: "candidate_123",
		CircleIDHash:  "circle_456",
		TransportKind: delivery.TransportStub,
		ResultBucket:  delivery.ResultSent,
		ReasonBucket:  delivery.ReasonNone,
		PeriodKey:     periodKey,
		AttemptBucket: "14:00",
	}

	canonical := attempt.CanonicalString()

	// Verify canonical string is pipe-delimited
	if canonical == "" {
		t.Error("canonical string should not be empty")
	}

	// Verify it starts with version prefix
	expected := "DELIVERY_ATTEMPT|v1|"
	if len(canonical) < len(expected) || canonical[:len(expected)] != expected {
		t.Errorf("expected canonical to start with %q, got %q", expected, canonical)
	}
}

func TestDeliveryReceipt_ComputeHashes(t *testing.T) {
	periodKey := testPeriodKey()
	receipt := &delivery.DeliveryReceipt{
		CircleIDHash: "circle_123",
		SentCount:    1,
		SkippedCount: 2,
		DedupedCount: 0,
		PeriodKey:    periodKey,
		TimeBucket:   "14:00",
		Attempts:     []delivery.AttemptSummary{},
	}

	// Compute hashes
	receipt.StatusHash = receipt.ComputeStatusHash()
	receipt.ReceiptID = receipt.ComputeReceiptID()

	// Verify hashes are computed
	if receipt.StatusHash == "" {
		t.Error("StatusHash should be computed")
	}
	if receipt.ReceiptID == "" {
		t.Error("ReceiptID should be computed")
	}

	// Verify hashes are deterministic
	statusHash2 := receipt.ComputeStatusHash()
	receiptID2 := receipt.ComputeReceiptID()

	if receipt.StatusHash != statusHash2 {
		t.Error("StatusHash should be deterministic")
	}
	if receipt.ReceiptID != receiptID2 {
		t.Error("ReceiptID should be deterministic")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test: Engine - Basic Flow
// ═══════════════════════════════════════════════════════════════════════════════

func TestEngine_ComputeDeliveryRun_NoCandidates(t *testing.T) {
	engine := interruptdelivery.NewEngine()
	periodKey := testPeriodKey()

	input := &delivery.DeliveryInput{
		CircleIDHash:  "circle_123",
		PeriodKey:     periodKey,
		TimeBucket:    "14:00",
		Candidates:    []*delivery.DeliveryCandidate{},
		PolicyAllowed: true,
		PushEnabled:   true,
		MaxPerDay:     2,
	}

	attempts, receipt := engine.ComputeDeliveryRun(input)

	// No candidates = no delivery
	if attempts != nil {
		t.Error("expected nil attempts for empty candidates")
	}
	if receipt != nil {
		t.Error("expected nil receipt for empty candidates")
	}
}

func TestEngine_ComputeDeliveryRun_SingleCandidate_Sent(t *testing.T) {
	engine := interruptdelivery.NewEngine()
	periodKey := testPeriodKey()

	input := &delivery.DeliveryInput{
		CircleIDHash: "circle_123",
		PeriodKey:    periodKey,
		TimeBucket:   "14:00",
		Candidates: []*delivery.DeliveryCandidate{
			{
				CandidateHash: "candidate_abc",
				CircleIDHash:  "circle_123",
				DecisionHash:  "decision_xyz",
				PeriodKey:     periodKey,
			},
		},
		PolicyAllowed: true,
		PushEnabled:   true,
		SentToday:     0,
		MaxPerDay:     2,
		PriorAttempts: make(map[string]bool),
	}

	attempts, receipt := engine.ComputeDeliveryRun(input)

	// Should have one attempt
	if len(attempts) != 1 {
		t.Fatalf("expected 1 attempt, got %d", len(attempts))
	}

	// Should be sent
	if attempts[0].ResultBucket != delivery.ResultSent {
		t.Errorf("expected ResultSent, got %v", attempts[0].ResultBucket)
	}

	// Receipt should have SentCount = 1
	if receipt.SentCount != 1 {
		t.Errorf("expected SentCount=1, got %d", receipt.SentCount)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test: Engine - Policy Denial
// ═══════════════════════════════════════════════════════════════════════════════

func TestEngine_ComputeDeliveryRun_PolicyDenied(t *testing.T) {
	engine := interruptdelivery.NewEngine()
	periodKey := testPeriodKey()

	input := &delivery.DeliveryInput{
		CircleIDHash: "circle_123",
		PeriodKey:    periodKey,
		TimeBucket:   "14:00",
		Candidates: []*delivery.DeliveryCandidate{
			{
				CandidateHash: "candidate_abc",
				CircleIDHash:  "circle_123",
				DecisionHash:  "decision_xyz",
				PeriodKey:     periodKey,
			},
		},
		PolicyAllowed: false, // Policy denies
		PushEnabled:   true,
		MaxPerDay:     2,
		PriorAttempts: make(map[string]bool),
	}

	attempts, receipt := engine.ComputeDeliveryRun(input)

	// Should have one attempt
	if len(attempts) != 1 {
		t.Fatalf("expected 1 attempt, got %d", len(attempts))
	}

	// Should be skipped due to policy
	if attempts[0].ResultBucket != delivery.ResultSkipped {
		t.Errorf("expected ResultSkipped, got %v", attempts[0].ResultBucket)
	}
	if attempts[0].ReasonBucket != delivery.ReasonPolicyDenies {
		t.Errorf("expected ReasonPolicyDenies, got %v", attempts[0].ReasonBucket)
	}

	// Receipt should have SkippedCount = 1
	if receipt.SkippedCount != 1 {
		t.Errorf("expected SkippedCount=1, got %d", receipt.SkippedCount)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test: Engine - Trust Fragile
// ═══════════════════════════════════════════════════════════════════════════════

func TestEngine_ComputeDeliveryRun_TrustFragile(t *testing.T) {
	engine := interruptdelivery.NewEngine()
	periodKey := testPeriodKey()

	input := &delivery.DeliveryInput{
		CircleIDHash: "circle_123",
		PeriodKey:    periodKey,
		TimeBucket:   "14:00",
		Candidates: []*delivery.DeliveryCandidate{
			{
				CandidateHash: "candidate_abc",
				CircleIDHash:  "circle_123",
				DecisionHash:  "decision_xyz",
				PeriodKey:     periodKey,
			},
		},
		PolicyAllowed: true,
		TrustFragile:  true, // Trust is fragile
		PushEnabled:   true,
		MaxPerDay:     2,
		PriorAttempts: make(map[string]bool),
	}

	attempts, receipt := engine.ComputeDeliveryRun(input)

	// Should be skipped due to fragile trust
	if len(attempts) != 1 {
		t.Fatalf("expected 1 attempt, got %d", len(attempts))
	}
	if attempts[0].ResultBucket != delivery.ResultSkipped {
		t.Errorf("expected ResultSkipped, got %v", attempts[0].ResultBucket)
	}
	if attempts[0].ReasonBucket != delivery.ReasonTrustFragile {
		t.Errorf("expected ReasonTrustFragile, got %v", attempts[0].ReasonBucket)
	}

	if receipt.SkippedCount != 1 {
		t.Errorf("expected SkippedCount=1, got %d", receipt.SkippedCount)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test: Engine - Daily Cap Enforcement
// ═══════════════════════════════════════════════════════════════════════════════

func TestEngine_ComputeDeliveryRun_DailyCapEnforced(t *testing.T) {
	engine := interruptdelivery.NewEngine()
	periodKey := testPeriodKey()

	input := &delivery.DeliveryInput{
		CircleIDHash: "circle_123",
		PeriodKey:    periodKey,
		TimeBucket:   "14:00",
		Candidates: []*delivery.DeliveryCandidate{
			{
				CandidateHash: "candidate_abc",
				CircleIDHash:  "circle_123",
				DecisionHash:  "decision_xyz",
				PeriodKey:     periodKey,
			},
		},
		PolicyAllowed: true,
		PushEnabled:   true,
		SentToday:     2, // Already at cap
		MaxPerDay:     2,
		PriorAttempts: make(map[string]bool),
	}

	attempts, receipt := engine.ComputeDeliveryRun(input)

	// Should be skipped due to cap
	if len(attempts) != 1 {
		t.Fatalf("expected 1 attempt, got %d", len(attempts))
	}
	if attempts[0].ResultBucket != delivery.ResultSkipped {
		t.Errorf("expected ResultSkipped, got %v", attempts[0].ResultBucket)
	}
	if attempts[0].ReasonBucket != delivery.ReasonCapReached {
		t.Errorf("expected ReasonCapReached, got %v", attempts[0].ReasonBucket)
	}

	if receipt.SkippedCount != 1 {
		t.Errorf("expected SkippedCount=1, got %d", receipt.SkippedCount)
	}
}

func TestEngine_MaxDeliveriesPerDay_EnforcedAt2(t *testing.T) {
	// Verify the constant is 2
	if delivery.MaxDeliveriesPerDay != 2 {
		t.Errorf("expected MaxDeliveriesPerDay=2, got %d", delivery.MaxDeliveriesPerDay)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test: Engine - Deduplication
// ═══════════════════════════════════════════════════════════════════════════════

func TestEngine_ComputeDeliveryRun_Deduplication(t *testing.T) {
	engine := interruptdelivery.NewEngine()
	periodKey := testPeriodKey()

	// Mark candidate as already sent
	priorAttempts := map[string]bool{
		"candidate_abc": true,
	}

	input := &delivery.DeliveryInput{
		CircleIDHash: "circle_123",
		PeriodKey:    periodKey,
		TimeBucket:   "14:00",
		Candidates: []*delivery.DeliveryCandidate{
			{
				CandidateHash: "candidate_abc",
				CircleIDHash:  "circle_123",
				DecisionHash:  "decision_xyz",
				PeriodKey:     periodKey,
			},
		},
		PolicyAllowed: true,
		PushEnabled:   true,
		MaxPerDay:     2,
		PriorAttempts: priorAttempts,
	}

	attempts, receipt := engine.ComputeDeliveryRun(input)

	// Should be deduped
	if len(attempts) != 1 {
		t.Fatalf("expected 1 attempt, got %d", len(attempts))
	}
	if attempts[0].ResultBucket != delivery.ResultDeduped {
		t.Errorf("expected ResultDeduped, got %v", attempts[0].ResultBucket)
	}
	if attempts[0].ReasonBucket != delivery.ReasonAlreadySent {
		t.Errorf("expected ReasonAlreadySent, got %v", attempts[0].ReasonBucket)
	}

	if receipt.DedupedCount != 1 {
		t.Errorf("expected DedupedCount=1, got %d", receipt.DedupedCount)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test: Engine - Deterministic Ordering
// ═══════════════════════════════════════════════════════════════════════════════

func TestEngine_ComputeDeliveryRun_DeterministicOrdering(t *testing.T) {
	engine := interruptdelivery.NewEngine()
	periodKey := testPeriodKey()

	// Candidates in reverse hash order
	candidates := []*delivery.DeliveryCandidate{
		{CandidateHash: "zzz_last", CircleIDHash: "c", DecisionHash: "d", PeriodKey: periodKey},
		{CandidateHash: "aaa_first", CircleIDHash: "c", DecisionHash: "d", PeriodKey: periodKey},
		{CandidateHash: "mmm_middle", CircleIDHash: "c", DecisionHash: "d", PeriodKey: periodKey},
	}

	input := &delivery.DeliveryInput{
		CircleIDHash:  "circle_123",
		PeriodKey:     periodKey,
		TimeBucket:    "14:00",
		Candidates:    candidates,
		PolicyAllowed: true,
		PushEnabled:   true,
		MaxPerDay:     2, // Only 2 slots
		PriorAttempts: make(map[string]bool),
	}

	attempts, _ := engine.ComputeDeliveryRun(input)

	if len(attempts) != 3 {
		t.Fatalf("expected 3 attempts, got %d", len(attempts))
	}

	// First two should be sent (sorted by hash: aaa, mmm)
	// Third should be skipped (cap reached)
	if attempts[0].CandidateHash != "aaa_first" {
		t.Errorf("expected first candidate to be aaa_first, got %s", attempts[0].CandidateHash)
	}
	if attempts[1].CandidateHash != "mmm_middle" {
		t.Errorf("expected second candidate to be mmm_middle, got %s", attempts[1].CandidateHash)
	}
	if attempts[2].CandidateHash != "zzz_last" {
		t.Errorf("expected third candidate to be zzz_last, got %s", attempts[2].CandidateHash)
	}

	// Verify sent/skipped counts
	sentCount := 0
	skippedCount := 0
	for _, a := range attempts {
		if a.ResultBucket == delivery.ResultSent {
			sentCount++
		}
		if a.ResultBucket == delivery.ResultSkipped {
			skippedCount++
		}
	}

	if sentCount != 2 {
		t.Errorf("expected 2 sent, got %d", sentCount)
	}
	if skippedCount != 1 {
		t.Errorf("expected 1 skipped (cap), got %d", skippedCount)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test: Engine - Proof Page
// ═══════════════════════════════════════════════════════════════════════════════

func TestEngine_BuildProofPage_WithDelivery(t *testing.T) {
	engine := interruptdelivery.NewEngine()
	periodKey := testPeriodKey()

	receipt := &delivery.DeliveryReceipt{
		ReceiptID:    "receipt_123",
		CircleIDHash: "circle_456",
		SentCount:    1,
		SkippedCount: 0,
		PeriodKey:    periodKey,
		TimeBucket:   "14:00",
		Attempts: []delivery.AttemptSummary{
			{ResultBucket: delivery.ResultSent, AttemptHash: "hash1"},
		},
	}

	page := engine.BuildProofPage(receipt, periodKey, "circle_456")

	// Verify page content
	if page.Title != "Delivered, quietly." {
		t.Errorf("expected title 'Delivered, quietly.', got %q", page.Title)
	}
	if page.SentLabel == "" {
		t.Error("expected SentLabel to be set")
	}
	if page.StatusHash == "" {
		t.Error("expected StatusHash to be computed")
	}
}

func TestEngine_BuildProofPage_NoDelivery(t *testing.T) {
	engine := interruptdelivery.NewEngine()
	periodKey := testPeriodKey()

	page := engine.BuildProofPage(nil, periodKey, "circle_456")

	// Should return default page
	if page.Title != "Delivery, quietly." {
		t.Errorf("expected default title, got %q", page.Title)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test: Engine - Delivery Cue
// ═══════════════════════════════════════════════════════════════════════════════

func TestEngine_ShouldShowDeliveryCue(t *testing.T) {
	engine := interruptdelivery.NewEngine()

	// Should show if sent today and not dismissed
	if !engine.ShouldShowDeliveryCue(true, false) {
		t.Error("expected cue to show when sent and not dismissed")
	}

	// Should not show if dismissed
	if engine.ShouldShowDeliveryCue(true, true) {
		t.Error("expected cue to be hidden when dismissed")
	}

	// Should not show if nothing sent
	if engine.ShouldShowDeliveryCue(false, false) {
		t.Error("expected cue to be hidden when nothing sent")
	}
}

func TestEngine_BuildDeliveryCue(t *testing.T) {
	engine := interruptdelivery.NewEngine()

	cue := engine.BuildDeliveryCue(true, false)

	if !cue.Available {
		t.Error("expected cue to be available")
	}
	if cue.Text != delivery.DefaultDeliveryCueText {
		t.Errorf("expected default cue text, got %q", cue.Text)
	}
	if cue.LinkPath != delivery.DefaultDeliveryCuePath {
		t.Errorf("expected default link path, got %q", cue.LinkPath)
	}
	if cue.Priority != delivery.DefaultDeliveryCuePriority {
		t.Errorf("expected priority %d, got %d", delivery.DefaultDeliveryCuePriority, cue.Priority)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test: Persistence Store
// ═══════════════════════════════════════════════════════════════════════════════

func TestInterruptDeliveryStore_AppendAttempt(t *testing.T) {
	store := persist.NewInterruptDeliveryStore(persist.DefaultInterruptDeliveryStoreConfig())
	periodKey := testPeriodKey()

	attempt := &delivery.DeliveryAttempt{
		CandidateHash: "candidate_123",
		CircleIDHash:  "circle_456",
		TransportKind: delivery.TransportStub,
		ResultBucket:  delivery.ResultSent,
		ReasonBucket:  delivery.ReasonNone,
		PeriodKey:     periodKey,
		AttemptBucket: "14:00",
	}

	err := store.AppendAttempt(attempt)
	if err != nil {
		t.Fatalf("failed to append attempt: %v", err)
	}

	// Verify stored
	attempts := store.GetAttemptsByPeriod(periodKey)
	if len(attempts) != 1 {
		t.Errorf("expected 1 attempt, got %d", len(attempts))
	}

	// Verify dedup index updated
	hasSent := store.HasSentCandidate(periodKey, "candidate_123")
	if !hasSent {
		t.Error("expected HasSentCandidate to return true")
	}
}

func TestInterruptDeliveryStore_Deduplication(t *testing.T) {
	store := persist.NewInterruptDeliveryStore(persist.DefaultInterruptDeliveryStoreConfig())
	periodKey := testPeriodKey()

	attempt := &delivery.DeliveryAttempt{
		CandidateHash: "candidate_123",
		CircleIDHash:  "circle_456",
		TransportKind: delivery.TransportStub,
		ResultBucket:  delivery.ResultSent,
		ReasonBucket:  delivery.ReasonNone,
		PeriodKey:     periodKey,
		AttemptBucket: "14:00",
	}

	// First append should succeed
	err := store.AppendAttempt(attempt)
	if err != nil {
		t.Fatalf("first append failed: %v", err)
	}

	// Duplicate append should fail
	err = store.AppendAttempt(attempt)
	if err == nil {
		t.Error("expected duplicate append to fail")
	}
}

func TestInterruptDeliveryStore_CountSentToday(t *testing.T) {
	store := persist.NewInterruptDeliveryStore(persist.DefaultInterruptDeliveryStoreConfig())
	periodKey := testPeriodKey()

	// Add two sent attempts
	for i, hash := range []string{"candidate_1", "candidate_2"} {
		attempt := &delivery.DeliveryAttempt{
			CandidateHash: hash,
			CircleIDHash:  "circle_456",
			TransportKind: delivery.TransportStub,
			ResultBucket:  delivery.ResultSent,
			ReasonBucket:  delivery.ReasonNone,
			PeriodKey:     periodKey,
			AttemptBucket: "14:00",
		}
		// Need unique attempt IDs - add suffix
		attempt.AttemptID = hash + "_attempt"

		err := store.AppendAttempt(attempt)
		if err != nil {
			t.Fatalf("failed to append attempt %d: %v", i, err)
		}
	}

	count := store.CountSentToday(periodKey)
	if count != 2 {
		t.Errorf("expected CountSentToday=2, got %d", count)
	}
}

func TestInterruptDeliveryStore_GetSentCandidates(t *testing.T) {
	store := persist.NewInterruptDeliveryStore(persist.DefaultInterruptDeliveryStoreConfig())
	periodKey := testPeriodKey()

	// Add sent attempts
	for i, hash := range []string{"candidate_a", "candidate_b"} {
		attempt := &delivery.DeliveryAttempt{
			CandidateHash: hash,
			CircleIDHash:  "circle_456",
			TransportKind: delivery.TransportStub,
			ResultBucket:  delivery.ResultSent,
			ReasonBucket:  delivery.ReasonNone,
			PeriodKey:     periodKey,
			AttemptBucket: "14:00",
		}
		attempt.AttemptID = hash + "_attempt"

		err := store.AppendAttempt(attempt)
		if err != nil {
			t.Fatalf("failed to append attempt %d: %v", i, err)
		}
	}

	sentCandidates := store.GetSentCandidates(periodKey)
	if len(sentCandidates) != 2 {
		t.Errorf("expected 2 sent candidates, got %d", len(sentCandidates))
	}
	if !sentCandidates["candidate_a"] {
		t.Error("expected candidate_a to be in sent candidates")
	}
	if !sentCandidates["candidate_b"] {
		t.Error("expected candidate_b to be in sent candidates")
	}
}

func TestInterruptDeliveryStore_BoundedRetention(t *testing.T) {
	store := persist.NewInterruptDeliveryStore(persist.InterruptDeliveryStoreConfig{
		MaxRetentionDays: 30,
	})

	// Use relative dates: today and 45 days ago (beyond 30-day retention)
	now := time.Now()
	recentPeriod := now.Format("2006-01-02")
	oldPeriod := now.AddDate(0, 0, -45).Format("2006-01-02")

	// Add attempts for two periods
	attempt1 := &delivery.DeliveryAttempt{
		CandidateHash: "old_candidate",
		CircleIDHash:  "circle_456",
		TransportKind: delivery.TransportStub,
		ResultBucket:  delivery.ResultSent,
		ReasonBucket:  delivery.ReasonNone,
		PeriodKey:     oldPeriod, // Old period (45 days ago)
		AttemptBucket: "14:00",
	}
	attempt1.AttemptID = "old_attempt"

	attempt2 := &delivery.DeliveryAttempt{
		CandidateHash: "new_candidate",
		CircleIDHash:  "circle_456",
		TransportKind: delivery.TransportStub,
		ResultBucket:  delivery.ResultSent,
		ReasonBucket:  delivery.ReasonNone,
		PeriodKey:     recentPeriod, // Recent period (today)
		AttemptBucket: "14:00",
	}
	attempt2.AttemptID = "new_attempt"

	_ = store.AppendAttempt(attempt1)
	_ = store.AppendAttempt(attempt2)

	// Evict with current time - old period should be evicted
	store.EvictOldPeriods(now)

	// Old period should be evicted
	oldAttempts := store.GetAttemptsByPeriod(oldPeriod)
	if len(oldAttempts) != 0 {
		t.Errorf("expected old period to be evicted, got %d attempts", len(oldAttempts))
	}

	// New period should remain
	newAttempts := store.GetAttemptsByPeriod(recentPeriod)
	if len(newAttempts) != 1 {
		t.Errorf("expected new period to remain, got %d attempts", len(newAttempts))
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test: Enum Validation
// ═══════════════════════════════════════════════════════════════════════════════

func TestTransportKind_Validate(t *testing.T) {
	validKinds := []delivery.TransportKind{
		delivery.TransportStub,
		delivery.TransportAPNs,
		delivery.TransportWebhook,
	}

	for _, kind := range validKinds {
		if err := kind.Validate(); err != nil {
			t.Errorf("expected %v to be valid, got error: %v", kind, err)
		}
	}

	// Invalid kind
	invalid := delivery.TransportKind("invalid")
	if err := invalid.Validate(); err == nil {
		t.Error("expected invalid transport kind to fail validation")
	}
}

func TestResultBucket_Validate(t *testing.T) {
	validBuckets := []delivery.ResultBucket{
		delivery.ResultSent,
		delivery.ResultSkipped,
		delivery.ResultRejected,
		delivery.ResultDeduped,
	}

	for _, bucket := range validBuckets {
		if err := bucket.Validate(); err != nil {
			t.Errorf("expected %v to be valid, got error: %v", bucket, err)
		}
	}

	// Invalid bucket
	invalid := delivery.ResultBucket("invalid")
	if err := invalid.Validate(); err == nil {
		t.Error("expected invalid result bucket to fail validation")
	}
}

func TestReasonBucket_Validate(t *testing.T) {
	validBuckets := []delivery.ReasonBucket{
		delivery.ReasonNone,
		delivery.ReasonPolicyDenies,
		delivery.ReasonCapReached,
		delivery.ReasonNotConfigured,
		delivery.ReasonAlreadySent,
		delivery.ReasonTransportError,
		delivery.ReasonNoCandidate,
		delivery.ReasonTrustFragile,
	}

	for _, bucket := range validBuckets {
		if err := bucket.Validate(); err != nil {
			t.Errorf("expected %v to be valid, got error: %v", bucket, err)
		}
	}

	// Invalid bucket
	invalid := delivery.ReasonBucket("invalid")
	if err := invalid.Validate(); err == nil {
		t.Error("expected invalid reason bucket to fail validation")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test: Hash-Only Storage
// ═══════════════════════════════════════════════════════════════════════════════

func TestDeliveryAttempt_ComputeAttemptID_Deterministic(t *testing.T) {
	periodKey := testPeriodKey()
	attempt := &delivery.DeliveryAttempt{
		CandidateHash: "candidate_123",
		CircleIDHash:  "circle_456",
		PeriodKey:     periodKey,
	}

	id1 := attempt.ComputeAttemptID()
	id2 := attempt.ComputeAttemptID()

	if id1 != id2 {
		t.Error("ComputeAttemptID should be deterministic")
	}
	if id1 == "" {
		t.Error("AttemptID should not be empty")
	}
}

func TestDeliveryAttempt_ComputeStatusHash_Deterministic(t *testing.T) {
	periodKey := testPeriodKey()
	attempt := &delivery.DeliveryAttempt{
		CandidateHash: "candidate_123",
		CircleIDHash:  "circle_456",
		TransportKind: delivery.TransportStub,
		ResultBucket:  delivery.ResultSent,
		ReasonBucket:  delivery.ReasonNone,
		PeriodKey:     periodKey,
		AttemptBucket: "14:00",
	}

	hash1 := attempt.ComputeStatusHash()
	hash2 := attempt.ComputeStatusHash()

	if hash1 != hash2 {
		t.Error("ComputeStatusHash should be deterministic")
	}
	if hash1 == "" {
		t.Error("StatusHash should not be empty")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test: Magnitude Bucket
// ═══════════════════════════════════════════════════════════════════════════════

func TestMagnitudeFromCount(t *testing.T) {
	tests := []struct {
		count    int
		expected delivery.MagnitudeBucket
	}{
		{0, delivery.MagnitudeNothing},
		{1, delivery.MagnitudeAFew},
		{2, delivery.MagnitudeAFew},
		{3, delivery.MagnitudeSeveral},
		{10, delivery.MagnitudeSeveral},
	}

	for _, tt := range tests {
		result := delivery.MagnitudeFromCount(tt.count)
		if result != tt.expected {
			t.Errorf("MagnitudeFromCount(%d) = %v, expected %v", tt.count, result, tt.expected)
		}
	}
}
