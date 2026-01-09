package demo_phase55_observer_consent

import (
	"strings"
	"testing"
	"time"

	"quantumlife/internal/observerconsent"
	"quantumlife/internal/persist"
	domaincoverageplan "quantumlife/pkg/domain/coverageplan"
	domain "quantumlife/pkg/domain/observerconsent"
)

// ============================================================================
// Section 1: Enum Validation Tests
// ============================================================================

func TestObserverKindValidation(t *testing.T) {
	tests := []struct {
		kind    domain.ObserverKind
		wantErr bool
	}{
		{domain.KindReceipt, false},
		{domain.KindCalendar, false},
		{domain.KindFinanceCommerce, false},
		{domain.KindCommerce, false},
		{domain.KindNotification, false},
		{domain.KindDeviceHint, false},
		{domain.KindUnknown, false},
		{"invalid_kind", true},
		{"", true},
	}

	for _, tt := range tests {
		err := tt.kind.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("ObserverKind(%q).Validate() error = %v, wantErr %v", tt.kind, err, tt.wantErr)
		}
	}
}

func TestConsentActionValidation(t *testing.T) {
	tests := []struct {
		action  domain.ConsentAction
		wantErr bool
	}{
		{domain.ActionEnable, false},
		{domain.ActionDisable, false},
		{"invalid_action", true},
		{"", true},
	}

	for _, tt := range tests {
		err := tt.action.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("ConsentAction(%q).Validate() error = %v, wantErr %v", tt.action, err, tt.wantErr)
		}
	}
}

func TestConsentResultValidation(t *testing.T) {
	tests := []struct {
		result  domain.ConsentResult
		wantErr bool
	}{
		{domain.ResultApplied, false},
		{domain.ResultNoChange, false},
		{domain.ResultRejected, false},
		{"invalid_result", true},
		{"", true},
	}

	for _, tt := range tests {
		err := tt.result.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("ConsentResult(%q).Validate() error = %v, wantErr %v", tt.result, err, tt.wantErr)
		}
	}
}

func TestRejectReasonValidation(t *testing.T) {
	tests := []struct {
		reason  domain.RejectReason
		wantErr bool
	}{
		{domain.RejectNone, false},
		{domain.RejectInvalid, false},
		{domain.RejectNotAllowlisted, false},
		{domain.RejectMissingCircle, false},
		{domain.RejectPeriodInvalid, false},
		{"invalid_reason", true},
	}

	for _, tt := range tests {
		err := tt.reason.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("RejectReason(%q).Validate() error = %v, wantErr %v", tt.reason, err, tt.wantErr)
		}
	}
}

func TestConsentAckKindValidation(t *testing.T) {
	tests := []struct {
		kind    domain.ConsentAckKind
		wantErr bool
	}{
		{domain.AckDismissed, false},
		{"invalid_ack", true},
		{"", true},
	}

	for _, tt := range tests {
		err := tt.kind.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("ConsentAckKind(%q).Validate() error = %v, wantErr %v", tt.kind, err, tt.wantErr)
		}
	}
}

// ============================================================================
// Section 2: CanonicalString Determinism Tests
// ============================================================================

func TestReceiptCanonicalStringDeterminism(t *testing.T) {
	receipt := domain.ObserverConsentReceipt{
		PeriodKey:    "2024-01-15",
		CircleIDHash: strings.Repeat("a", 64),
		Action:       domain.ActionEnable,
		Capability:   domaincoverageplan.CapReceiptObserver,
		Kind:         domain.KindReceipt,
		Result:       domain.ResultApplied,
		RejectReason: domain.RejectNone,
	}

	// Call multiple times - must be deterministic
	s1 := receipt.CanonicalStringV1()
	s2 := receipt.CanonicalStringV1()
	s3 := receipt.CanonicalStringV1()

	if s1 != s2 || s2 != s3 {
		t.Errorf("CanonicalStringV1 is not deterministic: %q != %q != %q", s1, s2, s3)
	}

	// Must be pipe-delimited
	if !strings.Contains(s1, "|") {
		t.Error("CanonicalStringV1 should be pipe-delimited")
	}
}

func TestAckCanonicalStringDeterminism(t *testing.T) {
	ack := domain.ObserverConsentAck{
		PeriodKey:    "2024-01-15",
		CircleIDHash: strings.Repeat("b", 64),
		AckKind:      domain.AckDismissed,
	}

	s1 := ack.CanonicalStringV1()
	s2 := ack.CanonicalStringV1()

	if s1 != s2 {
		t.Errorf("CanonicalStringV1 is not deterministic: %q != %q", s1, s2)
	}

	// Must be pipe-delimited
	if !strings.Contains(s1, "|") {
		t.Error("CanonicalStringV1 should be pipe-delimited")
	}
}

// ============================================================================
// Section 3: Hash Determinism Tests
// ============================================================================

func TestHashStringDeterminism(t *testing.T) {
	input := "test-input-string"

	h1 := domain.HashString(input)
	h2 := domain.HashString(input)
	h3 := domain.HashString(input)

	if h1 != h2 || h2 != h3 {
		t.Errorf("HashString is not deterministic: %q != %q != %q", h1, h2, h3)
	}

	// Must be 64 characters (SHA256 hex)
	if len(h1) != 64 {
		t.Errorf("HashString should return 64 hex chars, got %d", len(h1))
	}
}

func TestHashCircleIDDeterminism(t *testing.T) {
	circleID := "test-circle-123"

	h1 := domain.HashCircleID(circleID)
	h2 := domain.HashCircleID(circleID)

	if h1 != h2 {
		t.Errorf("HashCircleID is not deterministic: %q != %q", h1, h2)
	}
}

func TestReceiptHashDeterminism(t *testing.T) {
	receipt := domain.ObserverConsentReceipt{
		PeriodKey:    "2024-01-15",
		CircleIDHash: strings.Repeat("c", 64),
		Action:       domain.ActionDisable,
		Capability:   domaincoverageplan.CapCommerceObserver,
		Kind:         domain.KindCommerce,
		Result:       domain.ResultApplied,
		RejectReason: domain.RejectNone,
	}

	h1 := receipt.ComputeReceiptHash()
	h2 := receipt.ComputeReceiptHash()

	if h1 != h2 {
		t.Errorf("ComputeReceiptHash is not deterministic: %q != %q", h1, h2)
	}
}

// ============================================================================
// Section 4: KindFromCapability Tests
// ============================================================================

func TestKindFromCapability(t *testing.T) {
	tests := []struct {
		cap  domaincoverageplan.CoverageCapability
		want domain.ObserverKind
	}{
		{domaincoverageplan.CapReceiptObserver, domain.KindReceipt},
		{domaincoverageplan.CapCommerceObserver, domain.KindCommerce},
		{domaincoverageplan.CapFinanceCommerceObserver, domain.KindFinanceCommerce},
		{domaincoverageplan.CapNotificationMetadata, domain.KindNotification},
		{domaincoverageplan.CapPressureMap, domain.KindUnknown}, // Not an observer capability
	}

	for _, tt := range tests {
		got := domain.KindFromCapability(tt.cap)
		if got != tt.want {
			t.Errorf("KindFromCapability(%q) = %q, want %q", tt.cap, got, tt.want)
		}
	}
}

// ============================================================================
// Section 5: Allowlist Tests
// ============================================================================

func TestAllowlistedCapabilities(t *testing.T) {
	allowed := domain.AllowlistedCapabilities()

	if len(allowed) == 0 {
		t.Error("AllowlistedCapabilities should not be empty")
	}

	// Verify expected capabilities are in the allowlist
	expected := []domaincoverageplan.CoverageCapability{
		domaincoverageplan.CapReceiptObserver,
		domaincoverageplan.CapCommerceObserver,
		domaincoverageplan.CapFinanceCommerceObserver,
		domaincoverageplan.CapNotificationMetadata,
	}

	for _, exp := range expected {
		found := false
		for _, a := range allowed {
			if a == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected capability %q in allowlist", exp)
		}
	}
}

func TestIsAllowlisted(t *testing.T) {
	tests := []struct {
		cap  domaincoverageplan.CoverageCapability
		want bool
	}{
		{domaincoverageplan.CapReceiptObserver, true},
		{domaincoverageplan.CapCommerceObserver, true},
		{domaincoverageplan.CapFinanceCommerceObserver, true},
		{domaincoverageplan.CapNotificationMetadata, true},
		{domaincoverageplan.CapPressureMap, false},
		{domaincoverageplan.CapTimeWindowSources, false},
	}

	for _, tt := range tests {
		got := domain.IsAllowlisted(tt.cap)
		if got != tt.want {
			t.Errorf("IsAllowlisted(%q) = %v, want %v", tt.cap, got, tt.want)
		}
	}
}

// ============================================================================
// Section 6: Engine ApplyConsent Tests
// ============================================================================

func newTestEngine() *observerconsent.Engine {
	return observerconsent.NewEngine(func() string {
		return "2024-01-15"
	})
}

func TestEngineApplyConsentEnableAllowlisted(t *testing.T) {
	engine := newTestEngine()

	input := observerconsent.ApplyConsentInput{
		Request: domain.ObserverConsentRequest{
			CircleIDHash: strings.Repeat("d", 64),
			Action:       domain.ActionEnable,
			Capability:   domaincoverageplan.CapReceiptObserver,
		},
		CurrentCaps: []domaincoverageplan.CoverageCapability{},
	}

	output := engine.ApplyConsent(input)

	if output.Receipt.Result != domain.ResultApplied {
		t.Errorf("Expected ResultApplied, got %q", output.Receipt.Result)
	}

	// Verify capability was added
	found := false
	for _, cap := range output.NewCaps {
		if cap == domaincoverageplan.CapReceiptObserver {
			found = true
			break
		}
	}
	if !found {
		t.Error("Capability should have been added to NewCaps")
	}
}

func TestEngineApplyConsentDisableAllowlisted(t *testing.T) {
	engine := newTestEngine()

	input := observerconsent.ApplyConsentInput{
		Request: domain.ObserverConsentRequest{
			CircleIDHash: strings.Repeat("e", 64),
			Action:       domain.ActionDisable,
			Capability:   domaincoverageplan.CapReceiptObserver,
		},
		CurrentCaps: []domaincoverageplan.CoverageCapability{
			domaincoverageplan.CapReceiptObserver,
			domaincoverageplan.CapCommerceObserver,
		},
	}

	output := engine.ApplyConsent(input)

	if output.Receipt.Result != domain.ResultApplied {
		t.Errorf("Expected ResultApplied, got %q", output.Receipt.Result)
	}

	// Verify capability was removed
	for _, cap := range output.NewCaps {
		if cap == domaincoverageplan.CapReceiptObserver {
			t.Error("Capability should have been removed from NewCaps")
		}
	}
}

func TestEngineApplyConsentEnableAlreadyEnabled(t *testing.T) {
	engine := newTestEngine()

	input := observerconsent.ApplyConsentInput{
		Request: domain.ObserverConsentRequest{
			CircleIDHash: strings.Repeat("f", 64),
			Action:       domain.ActionEnable,
			Capability:   domaincoverageplan.CapReceiptObserver,
		},
		CurrentCaps: []domaincoverageplan.CoverageCapability{
			domaincoverageplan.CapReceiptObserver,
		},
	}

	output := engine.ApplyConsent(input)

	if output.Receipt.Result != domain.ResultNoChange {
		t.Errorf("Expected ResultNoChange, got %q", output.Receipt.Result)
	}
}

func TestEngineApplyConsentDisableAlreadyDisabled(t *testing.T) {
	engine := newTestEngine()

	input := observerconsent.ApplyConsentInput{
		Request: domain.ObserverConsentRequest{
			CircleIDHash: strings.Repeat("g", 64),
			Action:       domain.ActionDisable,
			Capability:   domaincoverageplan.CapReceiptObserver,
		},
		CurrentCaps: []domaincoverageplan.CoverageCapability{},
	}

	output := engine.ApplyConsent(input)

	if output.Receipt.Result != domain.ResultNoChange {
		t.Errorf("Expected ResultNoChange, got %q", output.Receipt.Result)
	}
}

func TestEngineApplyConsentRejectNotAllowlisted(t *testing.T) {
	engine := newTestEngine()

	input := observerconsent.ApplyConsentInput{
		Request: domain.ObserverConsentRequest{
			CircleIDHash: strings.Repeat("h", 64),
			Action:       domain.ActionEnable,
			Capability:   domaincoverageplan.CapPressureMap, // Not allowlisted
		},
		CurrentCaps: []domaincoverageplan.CoverageCapability{},
	}

	output := engine.ApplyConsent(input)

	if output.Receipt.Result != domain.ResultRejected {
		t.Errorf("Expected ResultRejected, got %q", output.Receipt.Result)
	}

	if output.Receipt.RejectReason != domain.RejectNotAllowlisted {
		t.Errorf("Expected RejectNotAllowlisted, got %q", output.Receipt.RejectReason)
	}
}

func TestEngineApplyConsentRejectMissingCircle(t *testing.T) {
	engine := newTestEngine()

	input := observerconsent.ApplyConsentInput{
		Request: domain.ObserverConsentRequest{
			CircleIDHash: "", // Missing
			Action:       domain.ActionEnable,
			Capability:   domaincoverageplan.CapReceiptObserver,
		},
		CurrentCaps: []domaincoverageplan.CoverageCapability{},
	}

	output := engine.ApplyConsent(input)

	if output.Receipt.Result != domain.ResultRejected {
		t.Errorf("Expected ResultRejected, got %q", output.Receipt.Result)
	}

	if output.Receipt.RejectReason != domain.RejectMissingCircle {
		t.Errorf("Expected RejectMissingCircle, got %q", output.Receipt.RejectReason)
	}
}

func TestEngineApplyConsentRejectInvalidAction(t *testing.T) {
	engine := newTestEngine()

	input := observerconsent.ApplyConsentInput{
		Request: domain.ObserverConsentRequest{
			CircleIDHash: strings.Repeat("i", 64),
			Action:       "invalid", // Invalid action
			Capability:   domaincoverageplan.CapReceiptObserver,
		},
		CurrentCaps: []domaincoverageplan.CoverageCapability{},
	}

	output := engine.ApplyConsent(input)

	if output.Receipt.Result != domain.ResultRejected {
		t.Errorf("Expected ResultRejected, got %q", output.Receipt.Result)
	}
}

// ============================================================================
// Section 7: Forbidden Field Validation Tests
// ============================================================================

func TestValidateNoForbiddenFieldsPass(t *testing.T) {
	fields := map[string]string{
		"circleIdHash": "abc123",
		"action":       "enable",
		"capability":   "cap_receipt_observer",
	}

	err := observerconsent.ValidateNoForbiddenFields(fields)
	if err != nil {
		t.Errorf("Expected no error for allowed fields, got %v", err)
	}
}

func TestValidateNoForbiddenFieldsRejectPeriod(t *testing.T) {
	fields := map[string]string{
		"circleIdHash": "abc123",
		"period":       "2024-01-15", // Forbidden
	}

	err := observerconsent.ValidateNoForbiddenFields(fields)
	if err == nil {
		t.Error("Expected error for forbidden 'period' field")
	}
}

func TestValidateNoForbiddenFieldsRejectEmail(t *testing.T) {
	fields := map[string]string{
		"circleIdHash": "abc123",
		"email":        "test@example.com", // Forbidden
	}

	err := observerconsent.ValidateNoForbiddenFields(fields)
	if err == nil {
		t.Error("Expected error for forbidden 'email' field")
	}
}

func TestForbiddenClientFieldsContainsPeriodKey(t *testing.T) {
	forbidden := domain.ForbiddenClientFields()
	found := false
	for _, f := range forbidden {
		if f == "periodKey" {
			found = true
			break
		}
	}
	if !found {
		t.Error("ForbiddenClientFields should contain 'periodKey'")
	}
}

// ============================================================================
// Section 8: Settings Page Tests
// ============================================================================

func TestEngineBuildSettingsPage(t *testing.T) {
	engine := newTestEngine()

	currentCaps := []domaincoverageplan.CoverageCapability{
		domaincoverageplan.CapReceiptObserver,
	}

	page := engine.BuildSettingsPage(currentCaps)

	if page.Title == "" {
		t.Error("Settings page should have a title")
	}

	if len(page.Capabilities) == 0 {
		t.Error("Settings page should have capabilities")
	}

	// Verify receipt observer shows as enabled
	for _, cap := range page.Capabilities {
		if cap.Capability == domaincoverageplan.CapReceiptObserver {
			if !cap.Enabled {
				t.Error("ReceiptObserver should be enabled")
			}
		}
	}

	// Verify status hash is not empty
	if page.StatusHash == "" {
		t.Error("Settings page should have a status hash")
	}
}

func TestEngineBuildSettingsPageAllDisabled(t *testing.T) {
	engine := newTestEngine()

	page := engine.BuildSettingsPage([]domaincoverageplan.CoverageCapability{})

	for _, cap := range page.Capabilities {
		if cap.Enabled {
			t.Errorf("Capability %q should be disabled", cap.Capability)
		}
	}
}

// ============================================================================
// Section 9: Proof Page Tests
// ============================================================================

func TestEngineBuildProofPageEmpty(t *testing.T) {
	engine := newTestEngine()

	page := engine.BuildProofPage([]domain.ObserverConsentReceipt{})

	if page.Title == "" {
		t.Error("Proof page should have a title")
	}

	if len(page.Lines) == 0 {
		t.Error("Proof page should have lines")
	}

	if page.StatusHash == "" {
		t.Error("Proof page should have a status hash")
	}
}

func TestEngineBuildProofPageWithReceipts(t *testing.T) {
	engine := newTestEngine()

	receipts := []domain.ObserverConsentReceipt{
		{
			PeriodKey:    "2024-01-15",
			CircleIDHash: strings.Repeat("j", 64),
			Action:       domain.ActionEnable,
			Capability:   domaincoverageplan.CapReceiptObserver,
			Kind:         domain.KindReceipt,
			Result:       domain.ResultApplied,
			ReceiptHash:  strings.Repeat("k", 64),
		},
	}

	page := engine.BuildProofPage(receipts)

	if len(page.Receipts) != 1 {
		t.Errorf("Proof page should have 1 receipt, got %d", len(page.Receipts))
	}
}

func TestEngineBuildProofPageMaxReceipts(t *testing.T) {
	engine := newTestEngine()

	// Create more than max receipts
	receipts := make([]domain.ObserverConsentReceipt, 20)
	for i := range receipts {
		receipts[i] = domain.ObserverConsentReceipt{
			PeriodKey:    "2024-01-15",
			CircleIDHash: strings.Repeat("l", 64),
			Action:       domain.ActionEnable,
			Capability:   domaincoverageplan.CapReceiptObserver,
			Kind:         domain.KindReceipt,
			Result:       domain.ResultApplied,
			ReceiptHash:  domain.HashString(string(rune(i))),
		}
	}

	page := engine.BuildProofPage(receipts)

	if len(page.Receipts) > domain.MaxProofDisplayReceipts {
		t.Errorf("Proof page should limit to %d receipts, got %d", domain.MaxProofDisplayReceipts, len(page.Receipts))
	}
}

// ============================================================================
// Section 10: Consent Store Tests
// ============================================================================

func TestConsentStoreAppendAndList(t *testing.T) {
	clock := func() time.Time {
		return time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	}
	store := persist.NewObserverConsentStore(clock)

	receipt := domain.ObserverConsentReceipt{
		PeriodKey:    "2024-01-15",
		CircleIDHash: strings.Repeat("m", 64),
		Action:       domain.ActionEnable,
		Capability:   domaincoverageplan.CapReceiptObserver,
		Kind:         domain.KindReceipt,
		Result:       domain.ResultApplied,
		ReceiptHash:  strings.Repeat("n", 64),
	}

	stored := store.AppendReceipt(receipt)
	if !stored {
		t.Error("Receipt should have been stored")
	}

	all := store.ListAll()
	if len(all) != 1 {
		t.Errorf("Expected 1 receipt, got %d", len(all))
	}
}

func TestConsentStoreDedup(t *testing.T) {
	clock := func() time.Time {
		return time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	}
	store := persist.NewObserverConsentStore(clock)

	receipt := domain.ObserverConsentReceipt{
		PeriodKey:    "2024-01-15",
		CircleIDHash: strings.Repeat("o", 64),
		Action:       domain.ActionEnable,
		Capability:   domaincoverageplan.CapReceiptObserver,
		Kind:         domain.KindReceipt,
		Result:       domain.ResultApplied,
		ReceiptHash:  strings.Repeat("p", 64),
	}

	// First append should succeed
	stored1 := store.AppendReceipt(receipt)
	if !stored1 {
		t.Error("First append should succeed")
	}

	// Second append should be rejected (duplicate)
	stored2 := store.AppendReceipt(receipt)
	if stored2 {
		t.Error("Second append should be rejected as duplicate")
	}

	if store.Count() != 1 {
		t.Errorf("Expected 1 receipt after dedup, got %d", store.Count())
	}
}

func TestConsentStoreListByCircle(t *testing.T) {
	clock := func() time.Time {
		return time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	}
	store := persist.NewObserverConsentStore(clock)

	circleHash1 := strings.Repeat("q", 64)
	circleHash2 := strings.Repeat("r", 64)

	store.AppendReceipt(domain.ObserverConsentReceipt{
		PeriodKey:    "2024-01-15",
		CircleIDHash: circleHash1,
		Action:       domain.ActionEnable,
		Capability:   domaincoverageplan.CapReceiptObserver,
		Kind:         domain.KindReceipt,
		Result:       domain.ResultApplied,
		ReceiptHash:  strings.Repeat("s", 64),
	})

	store.AppendReceipt(domain.ObserverConsentReceipt{
		PeriodKey:    "2024-01-15",
		CircleIDHash: circleHash2,
		Action:       domain.ActionEnable,
		Capability:   domaincoverageplan.CapCommerceObserver,
		Kind:         domain.KindCommerce,
		Result:       domain.ResultApplied,
		ReceiptHash:  strings.Repeat("t", 64),
	})

	receipts := store.ListByCircle(circleHash1)
	if len(receipts) != 1 {
		t.Errorf("Expected 1 receipt for circle1, got %d", len(receipts))
	}
}

// ============================================================================
// Section 11: Ack Store Tests
// ============================================================================

func TestAckStoreAppendAndList(t *testing.T) {
	clock := func() time.Time {
		return time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	}
	store := persist.NewObserverConsentAckStore(clock)

	ack := domain.ObserverConsentAck{
		PeriodKey:    "2024-01-15",
		CircleIDHash: strings.Repeat("u", 64),
		AckKind:      domain.AckDismissed,
		StatusHash:   strings.Repeat("v", 64),
	}

	stored := store.AppendAck(ack)
	if !stored {
		t.Error("Ack should have been stored")
	}

	all := store.ListAll()
	if len(all) != 1 {
		t.Errorf("Expected 1 ack, got %d", len(all))
	}
}

func TestAckStoreIsProofDismissed(t *testing.T) {
	clock := func() time.Time {
		return time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	}
	store := persist.NewObserverConsentAckStore(clock)

	circleHash := strings.Repeat("w", 64)
	periodKey := "2024-01-15"

	// Initially not dismissed
	if store.IsProofDismissed(circleHash, periodKey) {
		t.Error("Proof should not be dismissed initially")
	}

	// Add dismissal ack
	ack := domain.ObserverConsentAck{
		PeriodKey:    periodKey,
		CircleIDHash: circleHash,
		AckKind:      domain.AckDismissed,
	}
	ack.StatusHash = ack.ComputeStatusHash()
	store.AppendAck(ack)

	// Now should be dismissed
	if !store.IsProofDismissed(circleHash, periodKey) {
		t.Error("Proof should be dismissed after ack")
	}
}

// ============================================================================
// Section 12: NormalizeCapabilities Tests
// ============================================================================

func TestNormalizeCapabilitiesDedup(t *testing.T) {
	caps := []domaincoverageplan.CoverageCapability{
		domaincoverageplan.CapReceiptObserver,
		domaincoverageplan.CapReceiptObserver,
		domaincoverageplan.CapCommerceObserver,
	}

	normalized := domain.NormalizeCapabilities(caps)

	if len(normalized) != 2 {
		t.Errorf("Expected 2 unique capabilities, got %d", len(normalized))
	}
}

func TestNormalizeCapabilitiesSorted(t *testing.T) {
	caps := []domaincoverageplan.CoverageCapability{
		domaincoverageplan.CapReceiptObserver,
		domaincoverageplan.CapCommerceObserver,
		domaincoverageplan.CapFinanceCommerceObserver,
	}

	normalized := domain.NormalizeCapabilities(caps)

	// Verify sorted lexicographically
	for i := 1; i < len(normalized); i++ {
		if normalized[i-1] > normalized[i] {
			t.Errorf("Capabilities not sorted: %s > %s", normalized[i-1], normalized[i])
		}
	}
}

// ============================================================================
// Section 13: Request Validation Tests
// ============================================================================

func TestObserverConsentRequestValidation(t *testing.T) {
	tests := []struct {
		name    string
		req     domain.ObserverConsentRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: domain.ObserverConsentRequest{
				CircleIDHash: strings.Repeat("x", 64),
				Action:       domain.ActionEnable,
				Capability:   domaincoverageplan.CapReceiptObserver,
			},
			wantErr: false,
		},
		{
			name: "missing circle id hash",
			req: domain.ObserverConsentRequest{
				CircleIDHash: "",
				Action:       domain.ActionEnable,
				Capability:   domaincoverageplan.CapReceiptObserver,
			},
			wantErr: true,
		},
		{
			name: "invalid action",
			req: domain.ObserverConsentRequest{
				CircleIDHash: strings.Repeat("y", 64),
				Action:       "invalid",
				Capability:   domaincoverageplan.CapReceiptObserver,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// ============================================================================
// Section 14: Display Label Tests
// ============================================================================

func TestObserverKindDisplayLabel(t *testing.T) {
	tests := []struct {
		kind     domain.ObserverKind
		wantText string // Just check it's not empty
	}{
		{domain.KindReceipt, "Receipt"},
		{domain.KindCalendar, "Calendar"},
		{domain.KindCommerce, "Commerce"},
		{domain.KindFinanceCommerce, "Finance"},
		{domain.KindNotification, "Notification"},
	}

	for _, tt := range tests {
		label := tt.kind.DisplayLabel()
		if label == "" {
			t.Errorf("DisplayLabel for %q should not be empty", tt.kind)
		}
		if !strings.Contains(label, tt.wantText) {
			t.Errorf("DisplayLabel for %q should contain %q, got %q", tt.kind, tt.wantText, label)
		}
	}
}

// ============================================================================
// Section 15: DedupKey Tests
// ============================================================================

func TestReceiptDedupKey(t *testing.T) {
	receipt := domain.ObserverConsentReceipt{
		PeriodKey:    "2024-01-15",
		CircleIDHash: strings.Repeat("z", 64),
		Action:       domain.ActionEnable,
		Capability:   domaincoverageplan.CapReceiptObserver,
	}

	key1 := receipt.DedupKey()
	key2 := receipt.DedupKey()

	if key1 != key2 {
		t.Errorf("DedupKey should be deterministic: %q != %q", key1, key2)
	}

	// Key should be pipe-delimited
	if !strings.Contains(key1, "|") {
		t.Error("DedupKey should be pipe-delimited")
	}
}

func TestAckDedupKey(t *testing.T) {
	ack := domain.ObserverConsentAck{
		PeriodKey:    "2024-01-15",
		CircleIDHash: strings.Repeat("0", 64),
		AckKind:      domain.AckDismissed,
	}

	key1 := ack.DedupKey()
	key2 := ack.DedupKey()

	if key1 != key2 {
		t.Errorf("DedupKey should be deterministic: %q != %q", key1, key2)
	}
}
