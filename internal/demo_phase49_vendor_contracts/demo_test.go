package demo_phase49_vendor_contracts

import (
	"strings"
	"testing"
	"time"

	"quantumlife/internal/persist"
	internalvendorcontract "quantumlife/internal/vendorcontract"
	domain "quantumlife/pkg/domain/vendorcontract"
)

// ============================================================================
// Section 1: Enum Validation Tests
// ============================================================================

func TestContractScopeValidation(t *testing.T) {
	tests := []struct {
		scope   domain.ContractScope
		wantErr bool
	}{
		{domain.ScopeCommerce, false},
		{domain.ScopeInstitution, false},
		{domain.ScopeHealth, false},
		{domain.ScopeTransport, false},
		{domain.ScopeUnknown, false},
		{"invalid_scope", true},
		{"", true},
	}

	for _, tt := range tests {
		err := tt.scope.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("ContractScope(%q).Validate() error = %v, wantErr %v", tt.scope, err, tt.wantErr)
		}
	}
}

func TestPressureAllowanceValidation(t *testing.T) {
	tests := []struct {
		allowance domain.PressureAllowance
		wantErr   bool
	}{
		{domain.AllowHoldOnly, false},
		{domain.AllowSurfaceOnly, false},
		{domain.AllowInterruptCandidate, false},
		{"invalid_allowance", true},
		{"", true},
	}

	for _, tt := range tests {
		err := tt.allowance.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("PressureAllowance(%q).Validate() error = %v, wantErr %v", tt.allowance, err, tt.wantErr)
		}
	}
}

func TestFrequencyBucketValidation(t *testing.T) {
	tests := []struct {
		bucket  domain.FrequencyBucket
		wantErr bool
	}{
		{domain.FreqPerDay, false},
		{domain.FreqPerWeek, false},
		{domain.FreqPerEvent, false},
		{"invalid_freq", true},
	}

	for _, tt := range tests {
		err := tt.bucket.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("FrequencyBucket(%q).Validate() error = %v, wantErr %v", tt.bucket, err, tt.wantErr)
		}
	}
}

func TestEmergencyBucketValidation(t *testing.T) {
	tests := []struct {
		bucket  domain.EmergencyBucket
		wantErr bool
	}{
		{domain.EmergencyNone, false},
		{domain.EmergencyHumanOnly, false},
		{domain.EmergencyInstitutionOnly, false},
		{"invalid_emergency", true},
	}

	for _, tt := range tests {
		err := tt.bucket.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("EmergencyBucket(%q).Validate() error = %v, wantErr %v", tt.bucket, err, tt.wantErr)
		}
	}
}

func TestDeclaredByKindValidation(t *testing.T) {
	tests := []struct {
		kind    domain.DeclaredByKind
		wantErr bool
	}{
		{domain.DeclaredVendorSelf, false},
		{domain.DeclaredRegulator, false},
		{domain.DeclaredMarketplace, false},
		{"invalid_declared_by", true},
	}

	for _, tt := range tests {
		err := tt.kind.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("DeclaredByKind(%q).Validate() error = %v, wantErr %v", tt.kind, err, tt.wantErr)
		}
	}
}

func TestContractStatusValidation(t *testing.T) {
	tests := []struct {
		status  domain.ContractStatus
		wantErr bool
	}{
		{domain.StatusActive, false},
		{domain.StatusRevoked, false},
		{"invalid_status", true},
	}

	for _, tt := range tests {
		err := tt.status.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("ContractStatus(%q).Validate() error = %v, wantErr %v", tt.status, err, tt.wantErr)
		}
	}
}

func TestContractReasonBucketValidation(t *testing.T) {
	tests := []struct {
		reason  domain.ContractReasonBucket
		wantErr bool
	}{
		{domain.ReasonOK, false},
		{domain.ReasonInvalid, false},
		{domain.ReasonCommerceCapped, false},
		{domain.ReasonNoPower, false},
		{domain.ReasonRejected, false},
		{"invalid_reason", true},
	}

	for _, tt := range tests {
		err := tt.reason.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("ContractReasonBucket(%q).Validate() error = %v, wantErr %v", tt.reason, err, tt.wantErr)
		}
	}
}

// ============================================================================
// Section 2: CanonicalString Determinism Tests
// ============================================================================

func TestVendorContractCanonicalStringDeterminism(t *testing.T) {
	contract := domain.VendorContract{
		VendorCircleHash:   strings.Repeat("a", 64),
		Scope:              domain.ScopeCommerce,
		AllowedPressure:    domain.AllowHoldOnly,
		MaxFrequency:       domain.FreqPerDay,
		EmergencyException: domain.EmergencyNone,
		DeclaredBy:         domain.DeclaredVendorSelf,
		PeriodKey:          "2024-01-15",
	}

	// Call multiple times - must be deterministic
	s1 := contract.CanonicalString()
	s2 := contract.CanonicalString()
	s3 := contract.CanonicalString()

	if s1 != s2 || s2 != s3 {
		t.Errorf("CanonicalString is not deterministic: %q != %q != %q", s1, s2, s3)
	}

	// Must be pipe-delimited
	if !strings.Contains(s1, "|") {
		t.Error("CanonicalString should be pipe-delimited")
	}
}

func TestVendorContractRecordCanonicalStringDeterminism(t *testing.T) {
	record := domain.VendorContractRecord{
		ContractHash:     strings.Repeat("b", 64),
		VendorCircleHash: strings.Repeat("a", 64),
		Scope:            domain.ScopeHealth,
		EffectiveCap:     domain.AllowSurfaceOnly,
		Status:           domain.StatusActive,
		CreatedAtBucket:  "2024-01-15",
		PeriodKey:        "2024-01-15",
	}

	s1 := record.CanonicalString()
	s2 := record.CanonicalString()

	if s1 != s2 {
		t.Errorf("CanonicalString is not deterministic: %q != %q", s1, s2)
	}
}

// ============================================================================
// Section 3: Hash Determinism Tests
// ============================================================================

func TestHashContractStringDeterminism(t *testing.T) {
	input := "test|input|string"

	h1 := domain.HashContractString(input)
	h2 := domain.HashContractString(input)
	h3 := domain.HashContractString(input)

	if h1 != h2 || h2 != h3 {
		t.Errorf("HashContractString is not deterministic")
	}

	// Must be 64 hex chars (SHA256)
	if len(h1) != 64 {
		t.Errorf("Hash length = %d, want 64", len(h1))
	}
}

func TestComputeContractHashDeterminism(t *testing.T) {
	contract := domain.VendorContract{
		VendorCircleHash:   strings.Repeat("c", 64),
		Scope:              domain.ScopeInstitution,
		AllowedPressure:    domain.AllowInterruptCandidate,
		MaxFrequency:       domain.FreqPerWeek,
		EmergencyException: domain.EmergencyHumanOnly,
		DeclaredBy:         domain.DeclaredRegulator,
		PeriodKey:          "2024-02-20",
	}

	h1 := contract.ComputeContractHash()
	h2 := contract.ComputeContractHash()

	if h1 != h2 {
		t.Errorf("ComputeContractHash is not deterministic: %q != %q", h1, h2)
	}
}

// ============================================================================
// Section 4: Pressure Allowance Level Ordering Tests
// ============================================================================

func TestPressureAllowanceLevelOrdering(t *testing.T) {
	holdLevel := domain.AllowHoldOnly.Level()
	surfaceLevel := domain.AllowSurfaceOnly.Level()
	interruptLevel := domain.AllowInterruptCandidate.Level()

	// Must be: hold_only < surface_only < interrupt_candidate
	if !(holdLevel < surfaceLevel) {
		t.Errorf("Level ordering violation: hold(%d) should be < surface(%d)", holdLevel, surfaceLevel)
	}
	if !(surfaceLevel < interruptLevel) {
		t.Errorf("Level ordering violation: surface(%d) should be < interrupt(%d)", surfaceLevel, interruptLevel)
	}
}

// ============================================================================
// Section 5: Commerce Cap Tests
// ============================================================================

func TestCommerceCapAtSurfaceOnly(t *testing.T) {
	clock := func() time.Time { return time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC) }
	engine := internalvendorcontract.NewEngine(clock)

	// Commerce vendor requesting interrupt_candidate should be capped at surface_only
	contract := domain.VendorContract{
		VendorCircleHash:   strings.Repeat("d", 64),
		Scope:              domain.ScopeCommerce,
		AllowedPressure:    domain.AllowInterruptCandidate, // Requesting high
		MaxFrequency:       domain.FreqPerDay,
		EmergencyException: domain.EmergencyNone,
		DeclaredBy:         domain.DeclaredVendorSelf,
		PeriodKey:          "2024-01-15",
	}

	cap, reason := engine.ComputeEffectiveCap(contract, "", true)

	if cap != domain.AllowSurfaceOnly {
		t.Errorf("Commerce cap = %v, want %v", cap, domain.AllowSurfaceOnly)
	}
	if reason != domain.ReasonCommerceCapped {
		t.Errorf("Commerce reason = %v, want %v", reason, domain.ReasonCommerceCapped)
	}
}

func TestCommerceCapWithScopeCommerce(t *testing.T) {
	clock := func() time.Time { return time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC) }
	engine := internalvendorcontract.NewEngine(clock)

	// Scope is commerce - should be capped regardless of isCommerce flag
	contract := domain.VendorContract{
		VendorCircleHash:   strings.Repeat("e", 64),
		Scope:              domain.ScopeCommerce,
		AllowedPressure:    domain.AllowInterruptCandidate,
		MaxFrequency:       domain.FreqPerDay,
		EmergencyException: domain.EmergencyNone,
		DeclaredBy:         domain.DeclaredVendorSelf,
		PeriodKey:          "2024-01-15",
	}

	cap, reason := engine.ComputeEffectiveCap(contract, "", false) // isCommerce=false but scope is commerce

	if cap != domain.AllowSurfaceOnly {
		t.Errorf("Commerce scope cap = %v, want %v", cap, domain.AllowSurfaceOnly)
	}
	if reason != domain.ReasonCommerceCapped {
		t.Errorf("Commerce scope reason = %v, want %v", reason, domain.ReasonCommerceCapped)
	}
}

func TestNonCommerceCanRequestInterrupt(t *testing.T) {
	clock := func() time.Time { return time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC) }
	engine := internalvendorcontract.NewEngine(clock)

	// Non-commerce vendor requesting interrupt_candidate should get it
	contract := domain.VendorContract{
		VendorCircleHash:   strings.Repeat("f", 64),
		Scope:              domain.ScopeHealth,
		AllowedPressure:    domain.AllowInterruptCandidate,
		MaxFrequency:       domain.FreqPerDay,
		EmergencyException: domain.EmergencyHumanOnly,
		DeclaredBy:         domain.DeclaredRegulator,
		PeriodKey:          "2024-01-15",
	}

	cap, reason := engine.ComputeEffectiveCap(contract, "", false)

	if cap != domain.AllowInterruptCandidate {
		t.Errorf("Non-commerce cap = %v, want %v", cap, domain.AllowInterruptCandidate)
	}
	if reason != domain.ReasonOK {
		t.Errorf("Non-commerce reason = %v, want %v", reason, domain.ReasonOK)
	}
}

// ============================================================================
// Section 6: Clamp Ordering Tests
// ============================================================================

func TestClampPressureAllowanceOrdering(t *testing.T) {
	clock := func() time.Time { return time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC) }
	engine := internalvendorcontract.NewEngine(clock)

	tests := []struct {
		current  domain.PressureAllowance
		cap      domain.PressureAllowance
		expected domain.PressureAllowance
	}{
		// Current at or below cap - no change
		{domain.AllowHoldOnly, domain.AllowHoldOnly, domain.AllowHoldOnly},
		{domain.AllowHoldOnly, domain.AllowSurfaceOnly, domain.AllowHoldOnly},
		{domain.AllowHoldOnly, domain.AllowInterruptCandidate, domain.AllowHoldOnly},
		{domain.AllowSurfaceOnly, domain.AllowSurfaceOnly, domain.AllowSurfaceOnly},
		{domain.AllowSurfaceOnly, domain.AllowInterruptCandidate, domain.AllowSurfaceOnly},
		{domain.AllowInterruptCandidate, domain.AllowInterruptCandidate, domain.AllowInterruptCandidate},
		// Current above cap - clamp down
		{domain.AllowSurfaceOnly, domain.AllowHoldOnly, domain.AllowHoldOnly},
		{domain.AllowInterruptCandidate, domain.AllowHoldOnly, domain.AllowHoldOnly},
		{domain.AllowInterruptCandidate, domain.AllowSurfaceOnly, domain.AllowSurfaceOnly},
	}

	for _, tt := range tests {
		result := engine.ClampPressureAllowance(tt.current, tt.cap)
		if result != tt.expected {
			t.Errorf("ClampPressureAllowance(%v, %v) = %v, want %v", tt.current, tt.cap, result, tt.expected)
		}
	}
}

func TestClampNeverRaisesPressure(t *testing.T) {
	clock := func() time.Time { return time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC) }
	engine := internalvendorcontract.NewEngine(clock)

	// Property test: for all (current, cap), result <= max(current, cap)
	// More specifically: result <= current (can only reduce)
	allAllowances := []domain.PressureAllowance{
		domain.AllowHoldOnly,
		domain.AllowSurfaceOnly,
		domain.AllowInterruptCandidate,
	}

	for _, current := range allAllowances {
		for _, cap := range allAllowances {
			result := engine.ClampPressureAllowance(current, cap)
			// Result should never be higher than current
			if result.Level() > current.Level() {
				t.Errorf("Clamp raised pressure: current=%v, cap=%v, result=%v", current, cap, result)
			}
		}
	}
}

// ============================================================================
// Section 7: Invalid Contract Tests
// ============================================================================

func TestInvalidContractReturnsHoldOnly(t *testing.T) {
	clock := func() time.Time { return time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC) }
	engine := internalvendorcontract.NewEngine(clock)

	// Invalid contract (empty VendorCircleHash)
	contract := domain.VendorContract{
		VendorCircleHash:   "", // Invalid
		Scope:              domain.ScopeHealth,
		AllowedPressure:    domain.AllowInterruptCandidate,
		MaxFrequency:       domain.FreqPerDay,
		EmergencyException: domain.EmergencyNone,
		DeclaredBy:         domain.DeclaredVendorSelf,
		PeriodKey:          "2024-01-15",
	}

	cap, reason := engine.ComputeEffectiveCap(contract, "", false)

	if cap != domain.AllowHoldOnly {
		t.Errorf("Invalid contract cap = %v, want %v", cap, domain.AllowHoldOnly)
	}
	if reason != domain.ReasonInvalid {
		t.Errorf("Invalid contract reason = %v, want %v", reason, domain.ReasonInvalid)
	}
}

func TestInvalidContractOutcomeNotAccepted(t *testing.T) {
	clock := func() time.Time { return time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC) }
	engine := internalvendorcontract.NewEngine(clock)

	// Invalid contract
	contract := domain.VendorContract{
		VendorCircleHash: "short", // Invalid - not 64 chars
		Scope:            domain.ScopeHealth,
		AllowedPressure:  domain.AllowInterruptCandidate,
		MaxFrequency:     domain.FreqPerDay,
	}

	outcome := engine.DecideOutcome(contract, false)

	if outcome.Accepted {
		t.Error("Invalid contract should not be accepted")
	}
	if outcome.EffectiveCap != domain.AllowHoldOnly {
		t.Errorf("Invalid contract effective cap = %v, want %v", outcome.EffectiveCap, domain.AllowHoldOnly)
	}
}

// ============================================================================
// Section 8: Store Idempotency Tests
// ============================================================================

func TestStoreUpsertIdempotent(t *testing.T) {
	clock := func() time.Time { return time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC) }
	store := persist.NewVendorContractStore(clock)

	vendorHash := strings.Repeat("a", 64)
	periodKey := "2024-01-15"
	contractHash := strings.Repeat("b", 64)

	// Upsert twice with same data
	err1 := store.UpsertActiveContract(vendorHash, periodKey, contractHash, domain.AllowHoldOnly, domain.ScopeCommerce)
	err2 := store.UpsertActiveContract(vendorHash, periodKey, contractHash, domain.AllowHoldOnly, domain.ScopeCommerce)

	if err1 != nil || err2 != nil {
		t.Errorf("Upsert errors: %v, %v", err1, err2)
	}

	// Should still be only 1 record (idempotent)
	count := store.Count()
	if count != 1 {
		t.Errorf("Store count = %d, want 1 (idempotent)", count)
	}
}

func TestStoreRevokeIdempotent(t *testing.T) {
	clock := func() time.Time { return time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC) }
	store := persist.NewVendorContractStore(clock)

	vendorHash := strings.Repeat("a", 64)
	periodKey := "2024-01-15"
	contractHash := strings.Repeat("b", 64)

	// Upsert then revoke twice
	_ = store.UpsertActiveContract(vendorHash, periodKey, contractHash, domain.AllowHoldOnly, domain.ScopeCommerce)
	err1 := store.RevokeContract(vendorHash, periodKey, contractHash)
	err2 := store.RevokeContract(vendorHash, periodKey, contractHash) // Second revoke

	if err1 != nil || err2 != nil {
		t.Errorf("Revoke errors: %v, %v", err1, err2)
	}

	// Should be revoked
	contract := store.GetActiveContract(vendorHash, periodKey)
	if contract != nil {
		t.Error("Contract should be nil after revocation")
	}
}

// ============================================================================
// Section 9: Store Retention Tests
// ============================================================================

func TestStoreRetentionEviction(t *testing.T) {
	baseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	currentTime := baseTime
	clock := func() time.Time { return currentTime }
	store := persist.NewVendorContractStore(clock)

	// Add records with old period keys
	for i := 0; i < 5; i++ {
		oldPeriodKey := baseTime.AddDate(0, 0, -35-i).Format("2006-01-02") // 35+ days old
		vendorHash := domain.HashContractString("vendor-" + string(rune('a'+i)))
		contractHash := domain.HashContractString("contract-" + string(rune('a'+i)))
		_ = store.UpsertActiveContract(vendorHash, oldPeriodKey, contractHash, domain.AllowHoldOnly, domain.ScopeUnknown)
	}

	// Add a new record - should trigger eviction
	currentPeriodKey := baseTime.Format("2006-01-02")
	vendorHash := domain.HashContractString("vendor-new")
	contractHash := domain.HashContractString("contract-new")
	_ = store.UpsertActiveContract(vendorHash, currentPeriodKey, contractHash, domain.AllowSurfaceOnly, domain.ScopeHealth)

	// Old records should be evicted, only new one remains
	all := store.ListAll()
	for _, rec := range all {
		if rec.PeriodKey < baseTime.AddDate(0, 0, -30).Format("2006-01-02") {
			t.Errorf("Found record with old period key: %s", rec.PeriodKey)
		}
	}
}

// ============================================================================
// Section 10: Proof Line Tests
// ============================================================================

func TestProofLineContainsNoForbiddenPatterns(t *testing.T) {
	clock := func() time.Time { return time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC) }
	engine := internalvendorcontract.NewEngine(clock)

	proofLine := engine.BuildProofLine(
		strings.Repeat("a", 64),
		domain.ScopeCommerce,
		domain.AllowSurfaceOnly,
		"2024-01-15",
	)

	canonical := proofLine.CanonicalString()

	forbiddenPatterns := []string{
		"@",
		"http://",
		"https://",
		"$",
		"£",
	}

	for _, pattern := range forbiddenPatterns {
		if strings.Contains(canonical, pattern) {
			t.Errorf("Proof line contains forbidden pattern %q: %s", pattern, canonical)
		}
	}
}

func TestProofLineHashDeterminism(t *testing.T) {
	clock := func() time.Time { return time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC) }
	engine := internalvendorcontract.NewEngine(clock)

	line1 := engine.BuildProofLine(strings.Repeat("a", 64), domain.ScopeHealth, domain.AllowHoldOnly, "2024-01-15")
	line2 := engine.BuildProofLine(strings.Repeat("a", 64), domain.ScopeHealth, domain.AllowHoldOnly, "2024-01-15")

	if line1.ProofHash != line2.ProofHash {
		t.Errorf("Proof line hash not deterministic: %s != %s", line1.ProofHash, line2.ProofHash)
	}
}

// ============================================================================
// Section 11: Decision Outcome Tests
// ============================================================================

func TestDecideOutcomeValid(t *testing.T) {
	clock := func() time.Time { return time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC) }
	engine := internalvendorcontract.NewEngine(clock)

	contract := domain.VendorContract{
		VendorCircleHash:   strings.Repeat("a", 64),
		Scope:              domain.ScopeHealth,
		AllowedPressure:    domain.AllowSurfaceOnly,
		MaxFrequency:       domain.FreqPerDay,
		EmergencyException: domain.EmergencyNone,
		DeclaredBy:         domain.DeclaredVendorSelf,
		PeriodKey:          "2024-01-15",
	}

	outcome := engine.DecideOutcome(contract, false)

	if !outcome.Accepted {
		t.Error("Valid contract should be accepted")
	}
	if outcome.EffectiveCap != domain.AllowSurfaceOnly {
		t.Errorf("Effective cap = %v, want %v", outcome.EffectiveCap, domain.AllowSurfaceOnly)
	}
	if outcome.Reason != domain.ReasonOK {
		t.Errorf("Reason = %v, want %v", outcome.Reason, domain.ReasonOK)
	}
}

// ============================================================================
// Section 12: Clamp Decision Kind Tests
// ============================================================================

func TestClampDecisionKindMapping(t *testing.T) {
	clock := func() time.Time { return time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC) }
	engine := internalvendorcontract.NewEngine(clock)

	tests := []struct {
		rawDecision string
		cap         domain.PressureAllowance
		expected    string
	}{
		// No clamping needed
		{"hold", domain.AllowHoldOnly, "hold"},
		{"HOLD", domain.AllowInterruptCandidate, "hold"},
		{"surface", domain.AllowSurfaceOnly, "surface"},
		{"SURFACE", domain.AllowInterruptCandidate, "surface"},
		{"interrupt_candidate", domain.AllowInterruptCandidate, "interrupt_candidate"},
		// Clamping needed
		{"surface", domain.AllowHoldOnly, "hold"},
		{"interrupt_candidate", domain.AllowHoldOnly, "hold"},
		{"interrupt_candidate", domain.AllowSurfaceOnly, "surface"},
	}

	for _, tt := range tests {
		result := engine.ClampDecisionKind(tt.rawDecision, tt.cap)
		if result != tt.expected {
			t.Errorf("ClampDecisionKind(%q, %v) = %q, want %q", tt.rawDecision, tt.cap, result, tt.expected)
		}
	}
}

func TestWasClamped(t *testing.T) {
	clock := func() time.Time { return time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC) }
	engine := internalvendorcontract.NewEngine(clock)

	tests := []struct {
		rawDecision string
		cap         domain.PressureAllowance
		wasClamped  bool
	}{
		{"hold", domain.AllowHoldOnly, false},
		{"hold", domain.AllowInterruptCandidate, false},
		{"surface", domain.AllowSurfaceOnly, false},
		{"surface", domain.AllowHoldOnly, true}, // Clamped
		{"interrupt_candidate", domain.AllowInterruptCandidate, false},
		{"interrupt_candidate", domain.AllowSurfaceOnly, true}, // Clamped
		{"interrupt_candidate", domain.AllowHoldOnly, true},    // Clamped
	}

	for _, tt := range tests {
		result := engine.WasClamped(tt.rawDecision, tt.cap)
		if result != tt.wasClamped {
			t.Errorf("WasClamped(%q, %v) = %v, want %v", tt.rawDecision, tt.cap, result, tt.wasClamped)
		}
	}
}

// ============================================================================
// Section 13: Proof Page and Cue Tests
// ============================================================================

func TestBuildProofPageEmpty(t *testing.T) {
	clock := func() time.Time { return time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC) }
	engine := internalvendorcontract.NewEngine(clock)

	page := engine.BuildProofPage(nil)

	if page.Title == "" {
		t.Error("Proof page title should not be empty")
	}
	if len(page.Lines) == 0 {
		t.Error("Proof page should have lines")
	}
}

func TestBuildProofPageWithContracts(t *testing.T) {
	clock := func() time.Time { return time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC) }
	engine := internalvendorcontract.NewEngine(clock)

	proofLines := []domain.VendorContractProofLine{
		engine.BuildProofLine(strings.Repeat("a", 64), domain.ScopeCommerce, domain.AllowHoldOnly, "2024-01-15"),
	}

	page := engine.BuildProofPage(proofLines)

	if len(page.ProofLines) != 1 {
		t.Errorf("Proof page proof lines = %d, want 1", len(page.ProofLines))
	}
}

func TestBuildCueAvailable(t *testing.T) {
	clock := func() time.Time { return time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC) }
	engine := internalvendorcontract.NewEngine(clock)

	proofLines := []domain.VendorContractProofLine{
		engine.BuildProofLine(strings.Repeat("a", 64), domain.ScopeCommerce, domain.AllowHoldOnly, "2024-01-15"),
	}

	cue := engine.BuildCue(proofLines, false) // Not dismissed

	if !cue.Available {
		t.Error("Cue should be available when contracts exist and not dismissed")
	}
	if cue.Path == "" {
		t.Error("Cue path should not be empty")
	}
}

func TestBuildCueDismissed(t *testing.T) {
	clock := func() time.Time { return time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC) }
	engine := internalvendorcontract.NewEngine(clock)

	proofLines := []domain.VendorContractProofLine{
		engine.BuildProofLine(strings.Repeat("a", 64), domain.ScopeCommerce, domain.AllowHoldOnly, "2024-01-15"),
	}

	cue := engine.BuildCue(proofLines, true) // Dismissed

	if cue.Available {
		t.Error("Cue should not be available when dismissed")
	}
}

// ============================================================================
// Section 14: VendorProofAck Tests
// ============================================================================

func TestVendorProofAckStore(t *testing.T) {
	clock := func() time.Time { return time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC) }
	store := persist.NewVendorProofAckStore(clock)

	vendorHash := strings.Repeat("a", 64)
	periodKey := "2024-01-15"

	// Initially not dismissed
	if store.IsProofDismissed(vendorHash, periodKey) {
		t.Error("Should not be dismissed initially")
	}

	// Dismiss
	ack := domain.VendorProofAck{
		VendorCircleHash: vendorHash,
		PeriodKey:        periodKey,
		AckKind:          domain.VendorAckDismissed,
	}
	ack.StatusHash = ack.ComputeStatusHash()

	_ = store.AppendAck(ack)

	// Now dismissed
	if !store.IsProofDismissed(vendorHash, periodKey) {
		t.Error("Should be dismissed after ack")
	}
}

// ============================================================================
// Section 15: Contract Record Building Tests
// ============================================================================

func TestBuildContractRecord(t *testing.T) {
	clock := func() time.Time { return time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC) }
	engine := internalvendorcontract.NewEngine(clock)

	contract := domain.VendorContract{
		VendorCircleHash:   strings.Repeat("a", 64),
		Scope:              domain.ScopeHealth,
		AllowedPressure:    domain.AllowSurfaceOnly,
		MaxFrequency:       domain.FreqPerDay,
		EmergencyException: domain.EmergencyNone,
		DeclaredBy:         domain.DeclaredVendorSelf,
		PeriodKey:          "2024-01-15",
	}

	outcome := domain.VendorContractOutcome{
		Accepted:     true,
		EffectiveCap: domain.AllowSurfaceOnly,
		Reason:       domain.ReasonOK,
	}

	record := engine.BuildContractRecord(contract, outcome, "2024-01-15")

	if record.Status != domain.StatusActive {
		t.Errorf("Record status = %v, want %v", record.Status, domain.StatusActive)
	}
	if record.ContractHash == "" {
		t.Error("Record contract hash should not be empty")
	}
}
