// Package demo_phase19_2_shadow_mode provides demo tests for Phase 19.2.
//
// Phase 19.2: LLM Shadow Mode Contract
//
// CRITICAL INVARIANTS TESTED:
//   - Determinism: same inputs + same clock => same receipt hash
//   - Privacy: receipt contains no forbidden strings (emails, @, amounts, dates)
//   - Sorting: suggestions are stable order
//   - Replay: storelog replay yields identical receipt hash
//   - No influence: shadow does NOT change obligations/drafts/interruptions
//
// Reference: docs/ADR/ADR-0043-phase19-2-shadow-mode-contract.md
package demo_phase19_2_shadow_mode

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"quantumlife/internal/persist"
	"quantumlife/internal/shadowllm"
	"quantumlife/internal/shadowllm/stub"
	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/identity"
	domainshadow "quantumlife/pkg/domain/shadowllm"
)

// createTestClock creates a deterministic clock for testing.
func createTestClock() clock.Clock {
	// Use a fixed time for determinism
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	return clock.NewFunc(func() time.Time {
		return fixedTime
	})
}

// createTestDigest creates a test input digest with abstract data only.
func createTestDigest(circleID string, triggersSet bool) domainshadow.ShadowInputDigest {
	digest := domainshadow.ShadowInputDigest{
		CircleID:                  identity.EntityID(circleID),
		ObligationCountByCategory: make(map[domainshadow.AbstractCategory]domainshadow.MagnitudeBucket),
		HeldCountByCategory:       make(map[domainshadow.AbstractCategory]domainshadow.MagnitudeBucket),
		SurfaceCandidateCount:     domainshadow.MagnitudeNothing,
		DraftCandidateCount:       domainshadow.MagnitudeNothing,
		TriggersSeen:              triggersSet,
		MirrorBucket:              domainshadow.MagnitudeNothing,
	}

	if triggersSet {
		digest.ObligationCountByCategory[domainshadow.CategoryMoney] = domainshadow.MagnitudeAFew
		digest.ObligationCountByCategory[domainshadow.CategoryWork] = domainshadow.MagnitudeSeveral
		digest.HeldCountByCategory[domainshadow.CategoryMoney] = domainshadow.MagnitudeAFew
		digest.MirrorBucket = domainshadow.MagnitudeSeveral
	}

	return digest
}

// TestDeterminism verifies that same inputs + same clock => same receipt hash.
func TestDeterminism(t *testing.T) {
	clk := createTestClock()
	provider := stub.NewStubModel()
	engine := shadowllm.NewEngine(clk, provider)

	circleID := "test-circle-determinism"
	digest := createTestDigest(circleID, true)

	// Run twice with same inputs
	input := shadowllm.RunInput{
		CircleID: identity.EntityID(circleID),
		Digest:   digest,
	}

	output1, err := engine.Run(input)
	if err != nil {
		t.Fatalf("First run failed: %v", err)
	}

	output2, err := engine.Run(input)
	if err != nil {
		t.Fatalf("Second run failed: %v", err)
	}

	// Hashes must be identical
	hash1 := output1.Receipt.Hash()
	hash2 := output2.Receipt.Hash()

	if hash1 != hash2 {
		t.Errorf("Determinism failed: hash1=%s, hash2=%s", hash1, hash2)
	}

	t.Logf("Determinism verified: same inputs => same hash (%s)", hash1[:16])
}

// TestDeterminismWithDifferentInputs verifies different inputs => different hashes.
func TestDeterminismWithDifferentInputs(t *testing.T) {
	clk := createTestClock()
	provider := stub.NewStubModel()
	engine := shadowllm.NewEngine(clk, provider)

	// Two different digests
	digest1 := createTestDigest("circle-1", true)
	digest2 := createTestDigest("circle-2", false) // Different triggers

	output1, _ := engine.Run(shadowllm.RunInput{
		CircleID: identity.EntityID("circle-1"),
		Digest:   digest1,
	})

	output2, _ := engine.Run(shadowllm.RunInput{
		CircleID: identity.EntityID("circle-2"),
		Digest:   digest2,
	})

	if output1.Receipt.Hash() == output2.Receipt.Hash() {
		t.Errorf("Different inputs should produce different hashes")
	}

	t.Log("Different inputs correctly produce different hashes")
}

// TestPrivacyNoForbiddenStrings verifies receipt contains no PII/sensitive data.
func TestPrivacyNoForbiddenStrings(t *testing.T) {
	clk := createTestClock()
	provider := stub.NewStubModel()
	engine := shadowllm.NewEngine(clk, provider)

	output, err := engine.Run(shadowllm.RunInput{
		CircleID: identity.EntityID("privacy-test"),
		Digest:   createTestDigest("privacy-test", true),
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	receipt := output.Receipt
	canonical := receipt.CanonicalString()

	// Forbidden patterns - actual PII and sensitive content
	// Note: Technical metadata like timestamps and the word "receipt" in type names are allowed
	forbiddenPatterns := []struct {
		pattern string
		desc    string
	}{
		{`@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`, "email addresses"},
		{`http://`, "URLs"},
		{`https://`, "URLs"},
		{`\$\d+`, "dollar amounts"},
		{`\d{2}/\d{2}/\d{4}`, "dates MM/DD/YYYY"},
		{`subject:`, "email subjects"},
		{`from:`, "sender info"},
		{`body:`, "email body"},
		{`amazon|google|apple|microsoft`, "vendor names"},
		{`invoice\s+\d+|payment\s+\d+`, "financial IDs"},
	}

	for _, fp := range forbiddenPatterns {
		re := regexp.MustCompile("(?i)" + fp.pattern)
		if re.MatchString(canonical) {
			t.Errorf("Forbidden pattern found (%s): %s", fp.desc, fp.pattern)
		}
	}

	t.Log("Privacy check passed: no forbidden patterns in receipt")
}

// TestSuggestionSorting verifies suggestions are in stable order.
func TestSuggestionSorting(t *testing.T) {
	clk := createTestClock()
	provider := stub.NewStubModel()
	engine := shadowllm.NewEngine(clk, provider)

	// Run multiple times
	input := shadowllm.RunInput{
		CircleID: identity.EntityID("sort-test"),
		Digest:   createTestDigest("sort-test", true),
	}

	var prevOrder []string
	for i := 0; i < 5; i++ {
		output, _ := engine.Run(input)
		currentOrder := make([]string, len(output.Receipt.Suggestions))
		for j, sug := range output.Receipt.Suggestions {
			currentOrder[j] = string(sug.Category) + "|" + string(sug.Horizon)
		}

		if prevOrder != nil {
			if len(currentOrder) != len(prevOrder) {
				t.Errorf("Suggestion count changed: %d vs %d", len(prevOrder), len(currentOrder))
			}
			for j := range currentOrder {
				if currentOrder[j] != prevOrder[j] {
					t.Errorf("Suggestion order changed at index %d: %s vs %s", j, prevOrder[j], currentOrder[j])
				}
			}
		}
		prevOrder = currentOrder
	}

	t.Log("Suggestion sorting verified: stable order across runs")
}

// TestStoreReplay verifies storelog replay yields identical receipt hash.
func TestStoreReplay(t *testing.T) {
	clk := createTestClock()
	provider := stub.NewStubModel()
	engine := shadowllm.NewEngine(clk, provider)
	store := persist.NewShadowReceiptStore(clk.Now)

	input := shadowllm.RunInput{
		CircleID: identity.EntityID("replay-test"),
		Digest:   createTestDigest("replay-test", true),
	}

	// Run and store
	output, _ := engine.Run(input)
	originalHash := output.Receipt.Hash()

	err := store.Append(&output.Receipt)
	if err != nil {
		t.Fatalf("Store append failed: %v", err)
	}

	// Retrieve and verify hash
	retrieved, ok := store.GetByID(output.Receipt.ReceiptID)
	if !ok {
		t.Fatalf("Receipt not found in store")
	}

	if retrieved.Hash() != originalHash {
		t.Errorf("Replay hash mismatch: %s vs %s", retrieved.Hash(), originalHash)
	}

	// Verify via store method
	if !store.VerifyHash(output.Receipt.ReceiptID, originalHash) {
		t.Error("Store.VerifyHash failed")
	}

	t.Logf("Replay verified: stored receipt hash matches (%s)", originalHash[:16])
}

// TestNoStateInfluence verifies shadow does NOT affect other state.
func TestNoStateInfluence(t *testing.T) {
	clk := createTestClock()
	provider := stub.NewStubModel()
	engine := shadowllm.NewEngine(clk, provider)

	// Create a digest and capture its hash BEFORE shadow run
	digest := createTestDigest("influence-test", true)
	beforeHash := digest.Hash()

	// Run shadow analysis
	_, err := engine.Run(shadowllm.RunInput{
		CircleID: identity.EntityID("influence-test"),
		Digest:   digest,
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify digest hash unchanged AFTER shadow run
	afterHash := digest.Hash()

	if beforeHash != afterHash {
		t.Errorf("Shadow run modified digest: before=%s, after=%s", beforeHash, afterHash)
	}

	t.Log("No state influence verified: digest unchanged after shadow run")
}

// TestReceiptValidation verifies receipt passes validation.
func TestReceiptValidation(t *testing.T) {
	clk := createTestClock()
	provider := stub.NewStubModel()
	engine := shadowllm.NewEngine(clk, provider)

	output, err := engine.Run(shadowllm.RunInput{
		CircleID: identity.EntityID("validation-test"),
		Digest:   createTestDigest("validation-test", true),
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if err := output.Receipt.Validate(); err != nil {
		t.Errorf("Receipt validation failed: %v", err)
	}

	t.Log("Receipt validation passed")
}

// TestSuggestionValidation verifies all suggestions pass validation.
func TestSuggestionValidation(t *testing.T) {
	clk := createTestClock()
	provider := stub.NewStubModel()
	engine := shadowllm.NewEngine(clk, provider)

	output, _ := engine.Run(shadowllm.RunInput{
		CircleID: identity.EntityID("sug-validation-test"),
		Digest:   createTestDigest("sug-validation-test", true),
	})

	for i, sug := range output.Receipt.Suggestions {
		if err := sug.Validate(); err != nil {
			t.Errorf("Suggestion %d validation failed: %v", i, err)
		}
	}

	t.Logf("All %d suggestions passed validation", len(output.Receipt.Suggestions))
}

// TestCanonicalStringFormat verifies canonical string uses pipe delimiter.
func TestCanonicalStringFormat(t *testing.T) {
	clk := createTestClock()
	provider := stub.NewStubModel()
	engine := shadowllm.NewEngine(clk, provider)

	output, _ := engine.Run(shadowllm.RunInput{
		CircleID: identity.EntityID("canonical-test"),
		Digest:   createTestDigest("canonical-test", true),
	})

	canonical := output.Receipt.CanonicalString()

	// Must start with type prefix
	if !strings.HasPrefix(canonical, "SHADOW_RECEIPT|v1|") {
		t.Errorf("Canonical string missing prefix: %s", canonical[:50])
	}

	// Must use pipe delimiter, NOT JSON
	if strings.Contains(canonical, "{") || strings.Contains(canonical, "}") {
		t.Error("Canonical string contains JSON braces - should use pipe delimiter")
	}

	t.Log("Canonical string format verified: pipe-delimited, not JSON")
}

// TestStubModelName verifies stub model has a valid name.
func TestStubModelName(t *testing.T) {
	provider := stub.NewStubModel()

	name := provider.Name()
	if name == "" {
		t.Error("Stub model name is empty")
	}

	// Name should indicate it's a stub/deterministic model
	if !strings.Contains(strings.ToLower(name), "stub") {
		t.Errorf("Model name should contain 'stub': %s", name)
	}

	t.Logf("Stub model name: %s", name)
}

// TestMaxSuggestions verifies max suggestions limit is enforced.
func TestMaxSuggestions(t *testing.T) {
	clk := createTestClock()
	provider := stub.NewStubModel()
	engine := shadowllm.NewEngine(clk, provider)

	output, _ := engine.Run(shadowllm.RunInput{
		CircleID: identity.EntityID("max-sug-test"),
		Digest:   createTestDigest("max-sug-test", true),
	})

	if len(output.Receipt.Suggestions) > domainshadow.MaxSuggestionsPerReceipt {
		t.Errorf("Too many suggestions: %d > %d",
			len(output.Receipt.Suggestions), domainshadow.MaxSuggestionsPerReceipt)
	}

	t.Logf("Max suggestions enforced: %d suggestions (max: %d)",
		len(output.Receipt.Suggestions), domainshadow.MaxSuggestionsPerReceipt)
}

// TestWindowBucket verifies window bucket is a date string.
func TestWindowBucket(t *testing.T) {
	clk := createTestClock()
	provider := stub.NewStubModel()
	engine := shadowllm.NewEngine(clk, provider)

	output, _ := engine.Run(shadowllm.RunInput{
		CircleID: identity.EntityID("window-test"),
		Digest:   createTestDigest("window-test", true),
	})

	// Window bucket should be YYYY-MM-DD format
	windowBucket := output.Receipt.WindowBucket
	if len(windowBucket) != 10 || windowBucket[4] != '-' || windowBucket[7] != '-' {
		t.Errorf("Invalid window bucket format: %s", windowBucket)
	}

	// Should match clock date
	expected := clk.Now().UTC().Format("2006-01-02")
	if windowBucket != expected {
		t.Errorf("Window bucket mismatch: got %s, expected %s", windowBucket, expected)
	}

	t.Logf("Window bucket verified: %s", windowBucket)
}

// TestEmptyDigest verifies engine handles empty digest correctly.
func TestEmptyDigest(t *testing.T) {
	clk := createTestClock()
	provider := stub.NewStubModel()
	engine := shadowllm.NewEngine(clk, provider)

	emptyDigest := domainshadow.ShadowInputDigest{
		CircleID:                  identity.EntityID("empty-test"),
		ObligationCountByCategory: make(map[domainshadow.AbstractCategory]domainshadow.MagnitudeBucket),
		HeldCountByCategory:       make(map[domainshadow.AbstractCategory]domainshadow.MagnitudeBucket),
		TriggersSeen:              false,
	}

	output, err := engine.Run(shadowllm.RunInput{
		CircleID: identity.EntityID("empty-test"),
		Digest:   emptyDigest,
	})

	if err != nil {
		t.Fatalf("Empty digest run failed: %v", err)
	}

	if output.Status != shadowllm.RunStatusSuccess {
		t.Errorf("Expected success status, got: %s", output.Status)
	}

	t.Log("Empty digest handled correctly")
}

// TestMissingCircleID verifies engine rejects missing circle ID.
func TestMissingCircleID(t *testing.T) {
	clk := createTestClock()
	provider := stub.NewStubModel()
	engine := shadowllm.NewEngine(clk, provider)

	_, err := engine.Run(shadowllm.RunInput{
		CircleID: "", // Missing
		Digest:   createTestDigest("", true),
	})

	if err == nil {
		t.Error("Expected error for missing circle ID")
	}

	if err != domainshadow.ErrMissingCircleID {
		t.Errorf("Expected ErrMissingCircleID, got: %v", err)
	}

	t.Log("Missing circle ID correctly rejected")
}
