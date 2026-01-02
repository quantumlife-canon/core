// Package demo_phase19_3_azure_shadow provides demo tests for Phase 19.3.
//
// Phase 19.3: Azure OpenAI Shadow Provider
//
// CRITICAL INVARIANTS TESTED:
//   - Privacy guard blocks raw content
//   - Output validator rejects identifiers
//   - Stub provider remains deterministic
//   - Provenance is correctly populated
//   - Config correctly gates real providers
//
// Reference: docs/ADR/ADR-0044-phase19-3-azure-openai-shadow-provider.md
package demo_phase19_3_azure_shadow

import (
	"strings"
	"testing"
	"time"

	"quantumlife/internal/shadowllm"
	"quantumlife/internal/shadowllm/privacy"
	"quantumlife/internal/shadowllm/prompt"
	"quantumlife/internal/shadowllm/stub"
	"quantumlife/internal/shadowllm/validate"
	"quantumlife/pkg/clock"
	pkgconfig "quantumlife/pkg/domain/config"
	"quantumlife/pkg/domain/identity"
	domainshadow "quantumlife/pkg/domain/shadowllm"
)

// createTestClock creates a deterministic clock for testing.
func createTestClock() clock.Clock {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	return clock.NewFunc(func() time.Time {
		return fixedTime
	})
}

// createTestDigest creates a test input digest with abstract data only.
func createTestDigest(circleID string) domainshadow.ShadowInputDigest {
	return domainshadow.ShadowInputDigest{
		CircleID: identity.EntityID(circleID),
		ObligationCountByCategory: map[domainshadow.AbstractCategory]domainshadow.MagnitudeBucket{
			domainshadow.CategoryMoney: domainshadow.MagnitudeAFew,
			domainshadow.CategoryWork:  domainshadow.MagnitudeSeveral,
		},
		HeldCountByCategory: map[domainshadow.AbstractCategory]domainshadow.MagnitudeBucket{
			domainshadow.CategoryMoney: domainshadow.MagnitudeAFew,
		},
		SurfaceCandidateCount: domainshadow.MagnitudeAFew,
		DraftCandidateCount:   domainshadow.MagnitudeNothing,
		TriggersSeen:          true,
		MirrorBucket:          domainshadow.MagnitudeSeveral,
	}
}

// =============================================================================
// Privacy Guard Tests
// =============================================================================

// TestPrivacyGuardBlocksEmail verifies email patterns are blocked.
func TestPrivacyGuardBlocksEmail(t *testing.T) {
	guard := privacy.NewGuard()

	input := &privacy.ShadowInput{
		CircleID:          "test-circle",
		TimeBucket:        "2024-01-15",
		StateSnapshotHash: "abc123def456",
		InputDigestHash:   "fed987cba654",
	}

	// Valid input should pass
	err := guard.ValidateInput(input)
	if err != nil {
		t.Errorf("Valid input should pass: %v", err)
	}

	// Invalid circle ID with email should fail
	input.CircleID = "test@example.com"
	err = guard.ValidateInput(input)
	if err == nil {
		t.Error("Input with email in CircleID should be blocked")
	}

	t.Log("Privacy guard correctly blocks email patterns")
}

// TestPrivacyGuardBlocksURL verifies URL patterns are blocked.
func TestPrivacyGuardBlocksURL(t *testing.T) {
	guard := privacy.NewGuard()

	// CircleID with URL should fail
	input := &privacy.ShadowInput{
		CircleID:          "https://example.com/path",
		TimeBucket:        "2024-01-15",
		StateSnapshotHash: "abc123",
		InputDigestHash:   "def456",
	}

	err := guard.ValidateInput(input)
	if err == nil {
		t.Error("Input with URL should be blocked")
	}

	t.Log("Privacy guard correctly blocks URL patterns")
}

// TestPrivacyGuardBlocksAmount verifies currency amounts are blocked.
func TestPrivacyGuardBlocksAmount(t *testing.T) {
	guard := privacy.NewGuard()

	input := &privacy.ShadowInput{
		CircleID:          "$500-payment",
		TimeBucket:        "2024-01-15",
		StateSnapshotHash: "abc123",
		InputDigestHash:   "def456",
	}

	err := guard.ValidateInput(input)
	if err == nil {
		t.Error("Input with currency amount should be blocked")
	}

	t.Log("Privacy guard correctly blocks currency amounts")
}

// TestPrivacyGuardAllowsAbstract verifies abstract data passes.
func TestPrivacyGuardAllowsAbstract(t *testing.T) {
	guard := privacy.NewGuard()

	input := &privacy.ShadowInput{
		CircleID:   "family-circle",
		TimeBucket: "2024-01-15",
		ObligationMagnitudes: map[domainshadow.AbstractCategory]domainshadow.MagnitudeBucket{
			domainshadow.CategoryMoney: domainshadow.MagnitudeAFew,
		},
		CategoryPresence: map[domainshadow.AbstractCategory]bool{
			domainshadow.CategoryMoney: true,
		},
		StateSnapshotHash: "abcdef123456",
		InputDigestHash:   "654321fedcba",
	}

	err := guard.ValidateInput(input)
	if err != nil {
		t.Errorf("Abstract data should pass: %v", err)
	}

	t.Log("Privacy guard correctly allows abstract data")
}

// =============================================================================
// Output Validator Tests
// =============================================================================

// TestValidatorParsesValidJSON verifies valid JSON is parsed correctly.
func TestValidatorParsesValidJSON(t *testing.T) {
	validator := validate.NewValidator()

	validJSON := `{
		"confidence_bucket": "high",
		"horizon_bucket": "soon",
		"magnitude_bucket": "a_few",
		"category": "money",
		"why_generic": "There are a few items that may need attention.",
		"suggested_action_class": "surface"
	}`

	output := validator.ParseAndValidate(validJSON)

	if !output.IsValid {
		t.Errorf("Valid JSON should pass validation: %s", output.ValidationError)
	}

	if output.Confidence != domainshadow.ConfidenceHigh {
		t.Errorf("Expected ConfidenceHigh, got %s", output.Confidence)
	}

	if output.Horizon != domainshadow.HorizonSoon {
		t.Errorf("Expected HorizonSoon, got %s", output.Horizon)
	}

	if output.Category != domainshadow.CategoryMoney {
		t.Errorf("Expected CategoryMoney, got %s", output.Category)
	}

	t.Log("Validator correctly parses valid JSON")
}

// TestValidatorRejectsEmailInWhyGeneric verifies email in output is rejected.
func TestValidatorRejectsEmailInWhyGeneric(t *testing.T) {
	validator := validate.NewValidator()

	jsonWithEmail := `{
		"confidence_bucket": "high",
		"horizon_bucket": "soon",
		"magnitude_bucket": "a_few",
		"category": "money",
		"why_generic": "Check the email from john@example.com about the payment.",
		"suggested_action_class": "surface"
	}`

	output := validator.ParseAndValidate(jsonWithEmail)

	if output.IsValid {
		t.Error("JSON with email in why_generic should be rejected")
	}

	if !strings.Contains(output.ValidationError, "why_generic") {
		t.Errorf("Error should mention why_generic: %s", output.ValidationError)
	}

	t.Log("Validator correctly rejects email in why_generic")
}

// TestValidatorRejectsVendorName verifies vendor names are rejected.
func TestValidatorRejectsVendorName(t *testing.T) {
	validator := validate.NewValidator()

	jsonWithVendor := `{
		"confidence_bucket": "medium",
		"horizon_bucket": "now",
		"magnitude_bucket": "several",
		"category": "money",
		"why_generic": "Your Amazon order needs attention.",
		"suggested_action_class": "surface"
	}`

	output := validator.ParseAndValidate(jsonWithVendor)

	if output.IsValid {
		t.Error("JSON with vendor name in why_generic should be rejected")
	}

	t.Log("Validator correctly rejects vendor names")
}

// TestValidatorRejectsInvalidJSON verifies invalid JSON returns defaults.
func TestValidatorRejectsInvalidJSON(t *testing.T) {
	validator := validate.NewValidator()

	invalidJSON := `not valid json at all`

	output := validator.ParseAndValidate(invalidJSON)

	if output.IsValid {
		t.Error("Invalid JSON should not be valid")
	}

	// Should have safe defaults
	if output.Confidence != domainshadow.ConfidenceLow {
		t.Errorf("Expected default ConfidenceLow, got %s", output.Confidence)
	}

	if output.SuggestedActionClass != domainshadow.SuggestHold {
		t.Errorf("Expected default SuggestHold, got %s", output.SuggestedActionClass)
	}

	t.Log("Validator correctly handles invalid JSON with safe defaults")
}

// =============================================================================
// Provenance Tests
// =============================================================================

// TestProvenancePopulatedForStub verifies provenance is set for stub provider.
func TestProvenancePopulatedForStub(t *testing.T) {
	clk := createTestClock()
	provider := stub.NewStubModel()
	engine := shadowllm.NewEngine(clk, provider)

	output, err := engine.Run(shadowllm.RunInput{
		CircleID: identity.EntityID("provenance-test"),
		Digest:   createTestDigest("provenance-test"),
	})

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	prov := output.Receipt.Provenance

	if prov.ProviderKind != domainshadow.ProviderKindStub {
		t.Errorf("Expected ProviderKindStub, got %s", prov.ProviderKind)
	}

	if prov.Status != domainshadow.ReceiptStatusSuccess {
		t.Errorf("Expected ReceiptStatusSuccess, got %s", prov.Status)
	}

	if prov.LatencyBucket != domainshadow.LatencyNA {
		t.Errorf("Expected LatencyNA for stub, got %s", prov.LatencyBucket)
	}

	if prov.RequestPolicyHash == "" {
		t.Error("RequestPolicyHash should be set")
	}

	if prov.PromptTemplateVersion == "" {
		t.Error("PromptTemplateVersion should be set")
	}

	t.Logf("Provenance correctly populated: provider=%s, status=%s",
		prov.ProviderKind, prov.Status)
}

// TestProvenanceIncludedInHash verifies provenance affects receipt hash.
func TestProvenanceIncludedInHash(t *testing.T) {
	clk := createTestClock()
	provider := stub.NewStubModel()
	engine := shadowllm.NewEngine(clk, provider)

	output, _ := engine.Run(shadowllm.RunInput{
		CircleID: identity.EntityID("hash-test"),
		Digest:   createTestDigest("hash-test"),
	})

	hash1 := output.Receipt.Hash()

	// The hash should include provenance
	canonical := output.Receipt.CanonicalString()

	if !strings.Contains(canonical, "PROVENANCE|v1") {
		t.Error("Canonical string should include PROVENANCE section")
	}

	if !strings.Contains(canonical, string(domainshadow.ProviderKindStub)) {
		t.Errorf("Canonical string should include provider kind: %s", canonical)
	}

	t.Logf("Provenance included in hash: %s", hash1[:16])
}

// =============================================================================
// Config Tests
// =============================================================================

// TestDefaultConfigDisablesReal verifies real providers are off by default.
func TestDefaultConfigDisablesReal(t *testing.T) {
	cfg := pkgconfig.DefaultShadowConfig()

	if cfg.RealAllowed {
		t.Error("RealAllowed should be false by default")
	}

	if cfg.Mode != "off" {
		t.Errorf("Mode should be 'off' by default, got %s", cfg.Mode)
	}

	if cfg.ProviderKind != "stub" {
		t.Errorf("ProviderKind should be 'stub' by default, got %s", cfg.ProviderKind)
	}

	t.Log("Default config correctly disables real providers")
}

// TestProviderKindValidation verifies provider kind enum validation.
func TestProviderKindValidation(t *testing.T) {
	testCases := []struct {
		kind  domainshadow.ProviderKind
		valid bool
		real  bool
	}{
		{domainshadow.ProviderKindNone, true, false},
		{domainshadow.ProviderKindStub, true, false},
		{domainshadow.ProviderKindAzureOpenAI, true, true},
		{domainshadow.ProviderKindLocalSLM, true, true},
		{"invalid_provider", false, false},
	}

	for _, tc := range testCases {
		valid := tc.kind.Validate()
		if valid != tc.valid {
			t.Errorf("ProviderKind %q: expected valid=%v, got %v", tc.kind, tc.valid, valid)
		}

		if tc.valid && tc.kind.IsReal() != tc.real {
			t.Errorf("ProviderKind %q: expected IsReal=%v, got %v", tc.kind, tc.real, tc.kind.IsReal())
		}
	}

	t.Log("ProviderKind validation works correctly")
}

// =============================================================================
// Prompt Template Tests
// =============================================================================

// TestPromptRenderDoesNotContainSecrets verifies prompt has no secrets.
func TestPromptRenderDoesNotContainSecrets(t *testing.T) {
	input := &privacy.ShadowInput{
		CircleID:   "test-circle",
		TimeBucket: "2024-01-15",
		ObligationMagnitudes: map[domainshadow.AbstractCategory]domainshadow.MagnitudeBucket{
			domainshadow.CategoryMoney: domainshadow.MagnitudeAFew,
		},
		CategoryPresence: map[domainshadow.AbstractCategory]bool{
			domainshadow.CategoryMoney: true,
		},
		StateSnapshotHash: "abc123",
		InputDigestHash:   "def456",
	}

	system, user := prompt.RenderPrompt(input)

	// Check for forbidden patterns in rendered prompt
	// Note: "$" alone is allowed in JSON schema examples like "$500"
	// We check for actual secrets, not schema documentation
	forbidden := []string{
		"api_key=", "api-key=", "password=", "secret=", "bearer ", "authorization:",
	}

	for _, f := range forbidden {
		if strings.Contains(system, f) {
			t.Errorf("System prompt contains forbidden pattern: %s", f)
		}
		if strings.Contains(user, f) {
			t.Errorf("User prompt contains forbidden pattern: %s", f)
		}
	}

	// Should contain abstract data
	if !strings.Contains(user, "money") {
		t.Error("User prompt should contain category names")
	}

	if !strings.Contains(user, "a_few") || !strings.Contains(user, "nothing") {
		t.Error("User prompt should contain magnitude buckets")
	}

	t.Log("Prompt render correctly excludes secrets and includes abstract data")
}

// TestPromptVersioning verifies prompt template has version.
func TestPromptVersioning(t *testing.T) {
	version := prompt.TemplateVersion
	hash := prompt.TemplateHash()

	if version == "" {
		t.Error("TemplateVersion should not be empty")
	}

	if hash == "" {
		t.Error("TemplateHash should not be empty")
	}

	// Hash should be deterministic
	hash2 := prompt.TemplateHash()
	if hash != hash2 {
		t.Errorf("TemplateHash should be deterministic: %s != %s", hash, hash2)
	}

	t.Logf("Prompt template version: %s, hash: %s", version, hash)
}

// =============================================================================
// Stub Provider Determinism Tests
// =============================================================================

// TestStubProviderDeterministic verifies stub remains deterministic.
func TestStubProviderDeterministic(t *testing.T) {
	clk := createTestClock()
	provider := stub.NewStubModel()
	engine := shadowllm.NewEngine(clk, provider)

	input := shadowllm.RunInput{
		CircleID: identity.EntityID("determinism-test"),
		Digest:   createTestDigest("determinism-test"),
	}

	// Run twice
	output1, _ := engine.Run(input)
	output2, _ := engine.Run(input)

	hash1 := output1.Receipt.Hash()
	hash2 := output2.Receipt.Hash()

	if hash1 != hash2 {
		t.Errorf("Stub provider should be deterministic: %s != %s", hash1, hash2)
	}

	t.Logf("Stub provider determinism verified: %s", hash1[:16])
}

// =============================================================================
// Privacy Policy Hash Tests
// =============================================================================

// TestPrivacyPolicyHash verifies policy hash is deterministic.
func TestPrivacyPolicyHash(t *testing.T) {
	hash1 := privacy.PolicyHash()
	hash2 := privacy.PolicyHash()

	if hash1 != hash2 {
		t.Errorf("PolicyHash should be deterministic: %s != %s", hash1, hash2)
	}

	if hash1 == "" {
		t.Error("PolicyHash should not be empty")
	}

	t.Logf("Privacy policy hash: %s", hash1)
}

// =============================================================================
// Build Privacy Safe Input Tests
// =============================================================================

// TestBuildPrivacySafeInput verifies input construction.
func TestBuildPrivacySafeInput(t *testing.T) {
	digest := createTestDigest("test-circle")

	input, err := privacy.BuildPrivacySafeInput(
		"test-circle",
		"2024-01-15",
		digest,
		"state-hash-abc123",
	)

	if err != nil {
		t.Fatalf("BuildPrivacySafeInput failed: %v", err)
	}

	if input.CircleID != "test-circle" {
		t.Errorf("CircleID mismatch: %s", input.CircleID)
	}

	if input.TimeBucket != "2024-01-15" {
		t.Errorf("TimeBucket mismatch: %s", input.TimeBucket)
	}

	if input.InputDigestHash == "" {
		t.Error("InputDigestHash should be set")
	}

	// Category presence should be populated
	if !input.CategoryPresence[domainshadow.CategoryMoney] {
		t.Error("CategoryPresence should include money")
	}

	t.Log("BuildPrivacySafeInput correctly constructs privacy-safe input")
}
