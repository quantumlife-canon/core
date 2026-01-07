// Package demo_phase35_push_transport contains demo tests for Phase 35.
//
// Phase 35: Push Transport (Abstract Interrupt Delivery)
//
// CRITICAL INVARIANTS:
//   - Transport-only. No new decision logic (uses Phase 33/34 output).
//   - Abstract payload only. No identifiers in push body.
//   - TokenHash only. Raw token NEVER stored.
//   - No goroutines. Synchronous delivery only.
//   - Commerce never interrupts.
//   - Daily cap: max 2 deliveries.
//
// Reference: docs/ADR/ADR-0071-phase35-push-transport-abstract-interrupt-delivery.md
package demo_phase35_push_transport

import (
	"context"
	"strings"
	"testing"
	"time"

	"quantumlife/internal/persist"
	"quantumlife/internal/pushtransport"
	"quantumlife/internal/pushtransport/transport"
	pt "quantumlife/pkg/domain/pushtransport"
)

// ═══════════════════════════════════════════════════════════════════════════
// Test 1: Provider Kind Validation
// ═══════════════════════════════════════════════════════════════════════════

func TestProviderKind_ValidKinds(t *testing.T) {
	validKinds := []pt.PushProviderKind{
		pt.ProviderAPNs,
		pt.ProviderWebhook,
		pt.ProviderStub,
	}

	for _, kind := range validKinds {
		if err := kind.Validate(); err != nil {
			t.Errorf("expected valid kind %s to pass validation, got: %v", kind, err)
		}
	}
}

func TestProviderKind_InvalidKind(t *testing.T) {
	invalid := pt.PushProviderKind("invalid")
	if err := invalid.Validate(); err == nil {
		t.Error("expected invalid kind to fail validation")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 2: Token Hashing — Raw token never stored
// ═══════════════════════════════════════════════════════════════════════════

func TestTokenHashing_NeverStoreRaw(t *testing.T) {
	rawToken := "device-token-abc123-secret"
	tokenHash := pt.HashToken(rawToken)

	// Hash should not contain raw token
	if strings.Contains(tokenHash, rawToken) {
		t.Error("tokenHash contains raw token — security violation")
	}

	// Hash should be deterministic
	tokenHash2 := pt.HashToken(rawToken)
	if tokenHash != tokenHash2 {
		t.Error("token hashing is not deterministic")
	}

	// Hash should be 64 hex chars (SHA256)
	if len(tokenHash) != 64 {
		t.Errorf("expected 64 char hash, got %d", len(tokenHash))
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 3: Registration Canonical String
// ═══════════════════════════════════════════════════════════════════════════

func TestRegistration_CanonicalString(t *testing.T) {
	reg := &pt.PushRegistration{
		CircleIDHash:          "circle-hash-123",
		DeviceFingerprintHash: "device-fp-456",
		ProviderKind:          pt.ProviderStub,
		TokenKind:             pt.TokenKindDeviceToken,
		TokenHash:             "token-hash-789",
		CreatedPeriodKey:      "2026-01-07",
		Enabled:               true,
	}

	canonical := reg.CanonicalString()

	// Must be pipe-delimited
	if !strings.HasPrefix(canonical, "PUSH_REG|v1|") {
		t.Errorf("canonical string missing prefix, got: %s", canonical)
	}

	// Must contain all fields
	if !strings.Contains(canonical, "circle-hash-123") {
		t.Error("canonical string missing CircleIDHash")
	}
	if !strings.Contains(canonical, "stub") {
		t.Error("canonical string missing ProviderKind")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 4: Attempt Deduplication — same circle+candidate+period = same ID
// ═══════════════════════════════════════════════════════════════════════════

func TestAttempt_Deduplication(t *testing.T) {
	attempt1 := &pt.PushDeliveryAttempt{
		CircleIDHash:  "circle-hash-123",
		CandidateHash: "candidate-hash-456",
		ProviderKind:  pt.ProviderStub,
		Status:        pt.StatusSent,
		FailureBucket: pt.FailureNone,
		PeriodKey:     "2026-01-07",
		AttemptBucket: "12:00",
	}

	attempt2 := &pt.PushDeliveryAttempt{
		CircleIDHash:  "circle-hash-123",
		CandidateHash: "candidate-hash-456",
		ProviderKind:  pt.ProviderWebhook, // Different provider
		Status:        pt.StatusFailed,    // Different status
		FailureBucket: pt.FailureTransportError,
		PeriodKey:     "2026-01-07",
		AttemptBucket: "12:15", // Different bucket
	}

	// Same dedup key (circle+candidate+period) = same attempt ID
	id1 := attempt1.ComputeAttemptID()
	id2 := attempt2.ComputeAttemptID()

	if id1 != id2 {
		t.Errorf("same circle+candidate+period should produce same attempt ID: %s != %s", id1, id2)
	}
}

func TestAttempt_DifferentPeriod_DifferentID(t *testing.T) {
	attempt1 := &pt.PushDeliveryAttempt{
		CircleIDHash:  "circle-hash-123",
		CandidateHash: "candidate-hash-456",
		PeriodKey:     "2026-01-07",
	}

	attempt2 := &pt.PushDeliveryAttempt{
		CircleIDHash:  "circle-hash-123",
		CandidateHash: "candidate-hash-456",
		PeriodKey:     "2026-01-08", // Different day
	}

	id1 := attempt1.ComputeAttemptID()
	id2 := attempt2.ComputeAttemptID()

	if id1 == id2 {
		t.Error("different period should produce different attempt ID")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 5: Abstract Payload — Constant literals only
// ═══════════════════════════════════════════════════════════════════════════

func TestPayload_AbstractOnly(t *testing.T) {
	payload := pt.DefaultTransportPayload("status-hash-123")

	// Title must be constant
	if payload.Title != pt.PushTitle {
		t.Errorf("payload title is not constant: got %s, want %s", payload.Title, pt.PushTitle)
	}

	// Body must be constant
	if payload.Body != pt.PushBody {
		t.Errorf("payload body is not constant: got %s, want %s", payload.Body, pt.PushBody)
	}

	// No identifiers in payload
	forbiddenPatterns := []string{"@", "http", "merchant", "amount", "sender"}
	for _, p := range forbiddenPatterns {
		if strings.Contains(payload.Title, p) || strings.Contains(payload.Body, p) {
			t.Errorf("payload contains forbidden pattern: %s", p)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 6: Engine — No candidate means skipped
// ═══════════════════════════════════════════════════════════════════════════

func TestEngine_NoCandidate_Skipped(t *testing.T) {
	engine := pushtransport.NewEngine()

	input := &pt.DeliveryEligibilityInput{
		CircleIDHash:  "circle-hash-123",
		PeriodKey:     "2026-01-07",
		TimeBucket:    "12:00",
		HasCandidate:  false,
		CandidateHash: "",
		PolicyEnabled: true,
		PushEnabled:   true,
	}

	attempt, request := engine.ComputeDeliveryAttempt(input)

	if request != nil {
		t.Error("expected nil request when no candidate")
	}

	if attempt.Status != pt.StatusSkipped {
		t.Errorf("expected skipped status, got %s", attempt.Status)
	}

	if attempt.FailureBucket != pt.FailureNoCandidate {
		t.Errorf("expected no_candidate failure bucket, got %s", attempt.FailureBucket)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 7: Engine — Policy not enabled means skipped
// ═══════════════════════════════════════════════════════════════════════════

func TestEngine_PolicyNotEnabled_Skipped(t *testing.T) {
	engine := pushtransport.NewEngine()

	input := &pt.DeliveryEligibilityInput{
		CircleIDHash:  "circle-hash-123",
		PeriodKey:     "2026-01-07",
		TimeBucket:    "12:00",
		HasCandidate:  true,
		CandidateHash: "candidate-hash-456",
		PolicyEnabled: false, // Policy not enabled
		PushEnabled:   true,
	}

	attempt, request := engine.ComputeDeliveryAttempt(input)

	if request != nil {
		t.Error("expected nil request when policy not enabled")
	}

	if attempt.Status != pt.StatusSkipped {
		t.Errorf("expected skipped status, got %s", attempt.Status)
	}

	if attempt.FailureBucket != pt.FailureNotPermitted {
		t.Errorf("expected not_permitted failure bucket, got %s", attempt.FailureBucket)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 8: Engine — No registration means skipped
// ═══════════════════════════════════════════════════════════════════════════

func TestEngine_NoRegistration_Skipped(t *testing.T) {
	engine := pushtransport.NewEngine()

	input := &pt.DeliveryEligibilityInput{
		CircleIDHash:    "circle-hash-123",
		PeriodKey:       "2026-01-07",
		TimeBucket:      "12:00",
		HasCandidate:    true,
		CandidateHash:   "candidate-hash-456",
		PolicyEnabled:   true,
		PushEnabled:     true,
		HasRegistration: false, // No registration
		Registration:    nil,
	}

	attempt, request := engine.ComputeDeliveryAttempt(input)

	if request != nil {
		t.Error("expected nil request when no registration")
	}

	if attempt.Status != pt.StatusSkipped {
		t.Errorf("expected skipped status, got %s", attempt.Status)
	}

	if attempt.FailureBucket != pt.FailureNotConfigured {
		t.Errorf("expected not_configured failure bucket, got %s", attempt.FailureBucket)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 9: Engine — Daily cap exceeded means skipped
// ═══════════════════════════════════════════════════════════════════════════

func TestEngine_DailyCapExceeded_Skipped(t *testing.T) {
	engine := pushtransport.NewEngine()

	reg := &pt.PushRegistration{
		ProviderKind: pt.ProviderStub,
		TokenHash:    "token-hash-123",
		Enabled:      true,
	}

	input := &pt.DeliveryEligibilityInput{
		CircleIDHash:      "circle-hash-123",
		PeriodKey:         "2026-01-07",
		TimeBucket:        "12:00",
		HasCandidate:      true,
		CandidateHash:     "candidate-hash-456",
		PolicyEnabled:     true,
		PushEnabled:       true,
		HasRegistration:   true,
		Registration:      reg,
		DailyAttemptCount: 2, // Already at cap
		MaxPerDay:         2,
	}

	attempt, request := engine.ComputeDeliveryAttempt(input)

	if request != nil {
		t.Error("expected nil request when cap exceeded")
	}

	if attempt.Status != pt.StatusSkipped {
		t.Errorf("expected skipped status, got %s", attempt.Status)
	}

	if attempt.FailureBucket != pt.FailureCapReached {
		t.Errorf("expected cap_reached failure bucket, got %s", attempt.FailureBucket)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 10: Engine — All conditions met means eligible
// ═══════════════════════════════════════════════════════════════════════════

func TestEngine_AllConditionsMet_Eligible(t *testing.T) {
	engine := pushtransport.NewEngine()

	reg := &pt.PushRegistration{
		ProviderKind: pt.ProviderStub,
		TokenHash:    "token-hash-123",
		Enabled:      true,
	}

	input := &pt.DeliveryEligibilityInput{
		CircleIDHash:      "circle-hash-123",
		PeriodKey:         "2026-01-07",
		TimeBucket:        "12:00",
		HasCandidate:      true,
		CandidateHash:     "candidate-hash-456",
		PolicyEnabled:     true,
		PushEnabled:       true,
		HasRegistration:   true,
		Registration:      reg,
		DailyAttemptCount: 0, // Under cap
		MaxPerDay:         2,
	}

	attempt, request := engine.ComputeDeliveryAttempt(input)

	if request == nil {
		t.Fatal("expected transport request when all conditions met")
	}

	if attempt.Status != pt.StatusSent {
		t.Errorf("expected sent status, got %s", attempt.Status)
	}

	if attempt.FailureBucket != pt.FailureNone {
		t.Errorf("expected none failure bucket, got %s", attempt.FailureBucket)
	}

	// Verify request payload is abstract
	if request.Payload.Title != pt.PushTitle {
		t.Errorf("expected abstract title, got %s", request.Payload.Title)
	}

	if request.Payload.Body != pt.PushBody {
		t.Errorf("expected abstract body, got %s", request.Payload.Body)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 11: Stub Transport — Deterministic success
// ═══════════════════════════════════════════════════════════════════════════

func TestStubTransport_DeterministicSuccess(t *testing.T) {
	stub := transport.NewStubTransport()

	req := &pt.TransportRequest{
		ProviderKind: pt.ProviderStub,
		TokenHash:    "token-hash-123",
		Payload:      pt.DefaultTransportPayload("status-hash-456"),
		AttemptID:    "attempt-id-789",
	}

	ctx := context.Background()

	result1, err1 := stub.Send(ctx, req)
	result2, err2 := stub.Send(ctx, req)

	if err1 != nil || err2 != nil {
		t.Errorf("stub transport should not fail: %v, %v", err1, err2)
	}

	if !result1.Success || !result2.Success {
		t.Error("stub transport should always succeed")
	}

	// Same request should produce same response hash
	if result1.ResponseHash != result2.ResponseHash {
		t.Errorf("stub transport not deterministic: %s != %s", result1.ResponseHash, result2.ResponseHash)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 12: Stub Transport — Provider kind matches
// ═══════════════════════════════════════════════════════════════════════════

func TestStubTransport_ProviderKind(t *testing.T) {
	stub := transport.NewStubTransport()

	if stub.ProviderKind() != pt.ProviderStub {
		t.Errorf("expected stub provider kind, got %s", stub.ProviderKind())
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 13: Webhook Transport — Provider kind matches
// ═══════════════════════════════════════════════════════════════════════════

func TestWebhookTransport_ProviderKind(t *testing.T) {
	webhook := transport.NewWebhookTransport("https://example.com/webhook")

	if webhook.ProviderKind() != pt.ProviderWebhook {
		t.Errorf("expected webhook provider kind, got %s", webhook.ProviderKind())
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 14: Webhook Transport — No endpoint configured
// ═══════════════════════════════════════════════════════════════════════════

func TestWebhookTransport_NoEndpoint_Error(t *testing.T) {
	webhook := transport.NewWebhookTransport("") // No default endpoint

	req := &pt.TransportRequest{
		ProviderKind: pt.ProviderWebhook,
		TokenHash:    "token-hash-123",
		Endpoint:     "", // No endpoint in request
		Payload:      pt.DefaultTransportPayload("status-hash-456"),
		AttemptID:    "attempt-id-789",
	}

	ctx := context.Background()
	result, err := webhook.Send(ctx, req)

	if err == nil {
		t.Error("expected error when no endpoint configured")
	}

	if result.Success {
		t.Error("expected failure when no endpoint configured")
	}

	if result.ErrorBucket != pt.FailureNotConfigured {
		t.Errorf("expected not_configured error bucket, got %s", result.ErrorBucket)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 15: Transport Registry — Get and Register
// ═══════════════════════════════════════════════════════════════════════════

func TestTransportRegistry_GetAndRegister(t *testing.T) {
	reg := transport.NewRegistry()

	stub := transport.NewStubTransport()
	reg.Register(stub)

	// Get registered transport
	got, ok := reg.Get(pt.ProviderStub)
	if !ok {
		t.Error("expected to find registered stub transport")
	}

	if got.ProviderKind() != pt.ProviderStub {
		t.Errorf("expected stub provider, got %s", got.ProviderKind())
	}

	// Get unregistered transport
	_, ok = reg.Get(pt.ProviderAPNs)
	if ok {
		t.Error("expected not to find unregistered APNs transport")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 16: Registration Store — Append and Get
// ═══════════════════════════════════════════════════════════════════════════

func TestRegistrationStore_AppendAndGet(t *testing.T) {
	store := persist.NewPushRegistrationStore(persist.DefaultPushRegistrationStoreConfig())

	reg := &pt.PushRegistration{
		CircleIDHash:          "circle-hash-123",
		DeviceFingerprintHash: "device-fp-456",
		ProviderKind:          pt.ProviderStub,
		TokenKind:             pt.TokenKindDeviceToken,
		TokenHash:             "token-hash-789",
		CreatedPeriodKey:      "2026-01-07",
		Enabled:               true,
	}

	err := store.Append(reg)
	if err != nil {
		t.Fatalf("failed to append registration: %v", err)
	}

	// Get active registration
	active := store.GetActive("circle-hash-123", pt.ProviderStub)
	if active == nil {
		t.Fatal("expected to find active registration")
	}

	if active.TokenHash != "token-hash-789" {
		t.Errorf("expected token hash 'token-hash-789', got %s", active.TokenHash)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 17: Registration Store — Duplicate rejected
// ═══════════════════════════════════════════════════════════════════════════

func TestRegistrationStore_DuplicateRejected(t *testing.T) {
	store := persist.NewPushRegistrationStore(persist.DefaultPushRegistrationStoreConfig())

	reg := &pt.PushRegistration{
		CircleIDHash:          "circle-hash-123",
		DeviceFingerprintHash: "device-fp-456",
		ProviderKind:          pt.ProviderStub,
		TokenKind:             pt.TokenKindDeviceToken,
		TokenHash:             "token-hash-789",
		CreatedPeriodKey:      "2026-01-07",
		Enabled:               true,
	}

	err := store.Append(reg)
	if err != nil {
		t.Fatalf("failed to append first registration: %v", err)
	}

	// Same registration again
	err = store.Append(reg)
	if err == nil {
		t.Error("expected error on duplicate registration")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 18: Attempt Store — Append and Get
// ═══════════════════════════════════════════════════════════════════════════

func TestAttemptStore_AppendAndGet(t *testing.T) {
	store := persist.NewPushAttemptStore(persist.DefaultPushAttemptStoreConfig())

	attempt := &pt.PushDeliveryAttempt{
		CircleIDHash:  "circle-hash-123",
		CandidateHash: "candidate-hash-456",
		ProviderKind:  pt.ProviderStub,
		Status:        pt.StatusSent,
		FailureBucket: pt.FailureNone,
		PeriodKey:     "2026-01-07",
		AttemptBucket: "12:00",
	}
	attempt.AttemptID = attempt.ComputeAttemptID()
	attempt.StatusHash = attempt.ComputeStatusHash()

	err := store.Append(attempt)
	if err != nil {
		t.Fatalf("failed to append attempt: %v", err)
	}

	// Get by period
	attempts := store.GetByPeriod("2026-01-07")
	if len(attempts) != 1 {
		t.Errorf("expected 1 attempt, got %d", len(attempts))
	}

	// Count sent today
	count := store.CountSentToday("circle-hash-123", "2026-01-07")
	if count != 1 {
		t.Errorf("expected 1 sent attempt, got %d", count)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 19: Attempt Store — Dedup key rejects duplicate
// ═══════════════════════════════════════════════════════════════════════════

func TestAttemptStore_DedupRejects(t *testing.T) {
	store := persist.NewPushAttemptStore(persist.DefaultPushAttemptStoreConfig())

	attempt1 := &pt.PushDeliveryAttempt{
		CircleIDHash:  "circle-hash-123",
		CandidateHash: "candidate-hash-456",
		ProviderKind:  pt.ProviderStub,
		Status:        pt.StatusSent,
		FailureBucket: pt.FailureNone,
		PeriodKey:     "2026-01-07",
		AttemptBucket: "12:00",
	}
	attempt1.AttemptID = attempt1.ComputeAttemptID()

	err := store.Append(attempt1)
	if err != nil {
		t.Fatalf("failed to append first attempt: %v", err)
	}

	// Same circle+candidate+period = same attempt ID = rejected
	attempt2 := &pt.PushDeliveryAttempt{
		CircleIDHash:  "circle-hash-123",
		CandidateHash: "candidate-hash-456",
		ProviderKind:  pt.ProviderWebhook, // Different provider
		Status:        pt.StatusFailed,    // Different status
		PeriodKey:     "2026-01-07",
		AttemptBucket: "12:15", // Different bucket
	}
	attempt2.AttemptID = attempt2.ComputeAttemptID()

	err = store.Append(attempt2)
	if err == nil {
		t.Error("expected duplicate rejection for same circle+candidate+period")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 20: Proof Page — Calm copy for sent
// ═══════════════════════════════════════════════════════════════════════════

func TestProofPage_SentCalmCopy(t *testing.T) {
	engine := pushtransport.NewEngine()

	attempt := &pt.PushDeliveryAttempt{
		CircleIDHash:  "circle-hash-123",
		CandidateHash: "candidate-hash-456",
		ProviderKind:  pt.ProviderStub,
		Status:        pt.StatusSent,
		FailureBucket: pt.FailureNone,
		PeriodKey:     "2026-01-07",
		AttemptBucket: "12:00",
	}
	attempt.AttemptID = attempt.ComputeAttemptID()
	attempt.StatusHash = attempt.ComputeStatusHash()

	page := engine.BuildProofPage(attempt, "2026-01-07")

	// Title should be calm
	if !strings.Contains(page.Title, "quietly") {
		t.Errorf("expected calm title, got: %s", page.Title)
	}

	// Should have evidence hashes
	if len(page.EvidenceHashes) == 0 {
		t.Error("expected evidence hashes in proof page")
	}

	// Should have back link
	if page.BackLink != "/today" {
		t.Errorf("expected /today back link, got: %s", page.BackLink)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 21: Proof Page — Skipped has reason
// ═══════════════════════════════════════════════════════════════════════════

func TestProofPage_SkippedHasReason(t *testing.T) {
	engine := pushtransport.NewEngine()

	attempt := &pt.PushDeliveryAttempt{
		CircleIDHash:  "circle-hash-123",
		CandidateHash: "",
		ProviderKind:  pt.ProviderStub,
		Status:        pt.StatusSkipped,
		FailureBucket: pt.FailureNoCandidate,
		PeriodKey:     "2026-01-07",
		AttemptBucket: "12:00",
	}
	attempt.AttemptID = attempt.ComputeAttemptID()
	attempt.StatusHash = attempt.ComputeStatusHash()

	page := engine.BuildProofPage(attempt, "2026-01-07")

	if page.Status != pt.StatusSkipped {
		t.Errorf("expected skipped status, got: %s", page.Status)
	}

	// Lines should explain the skip
	found := false
	for _, line := range page.Lines {
		if strings.Contains(line, "Nothing needed") || strings.Contains(line, "Silence") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected calm explanation for skip reason")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 22: Bounded retention — eviction works
// ═══════════════════════════════════════════════════════════════════════════

func TestAttemptStore_BoundedRetention(t *testing.T) {
	cfg := persist.DefaultPushAttemptStoreConfig()
	cfg.MaxRetentionDays = 7 // Short retention for test
	store := persist.NewPushAttemptStore(cfg)

	// Add old attempt
	oldAttempt := &pt.PushDeliveryAttempt{
		CircleIDHash:  "circle-hash-123",
		CandidateHash: "candidate-old",
		ProviderKind:  pt.ProviderStub,
		Status:        pt.StatusSent,
		FailureBucket: pt.FailureNone,
		PeriodKey:     "2025-12-01", // Old
		AttemptBucket: "12:00",
	}
	oldAttempt.AttemptID = oldAttempt.ComputeAttemptID()
	_ = store.Append(oldAttempt)

	// Add new attempt
	newAttempt := &pt.PushDeliveryAttempt{
		CircleIDHash:  "circle-hash-123",
		CandidateHash: "candidate-new",
		ProviderKind:  pt.ProviderStub,
		Status:        pt.StatusSent,
		FailureBucket: pt.FailureNone,
		PeriodKey:     "2026-01-07",
		AttemptBucket: "12:00",
	}
	newAttempt.AttemptID = newAttempt.ComputeAttemptID()
	_ = store.Append(newAttempt)

	// Evict old
	store.EvictOldPeriods(time.Date(2026, 1, 7, 0, 0, 0, 0, time.UTC))

	// Old should be gone
	old := store.GetByPeriod("2025-12-01")
	if len(old) != 0 {
		t.Error("old period should be evicted")
	}

	// New should remain
	new := store.GetByPeriod("2026-01-07")
	if len(new) != 1 {
		t.Error("new period should remain")
	}
}
