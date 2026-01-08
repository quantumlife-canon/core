// Package demo_phase37_device_registration contains demo tests for Phase 37.
//
// These tests verify the Device Registration + Deep-Link implementation.
//
// CRITICAL INVARIANTS TESTED:
//   - Raw device_token ONLY in sealed secret boundary.
//   - Hash-only storage for registration records.
//   - No identifiers in deep links.
//   - Bounded retention (max 200 records OR 30 days).
//   - Deterministic: same inputs => same outputs.
//   - Whisper cue at lowest priority.
//
// Reference: docs/ADR/ADR-0074-phase37-device-registration-deeplink.md
package demo_phase37_device_registration

import (
	"fmt"
	"testing"
	"time"

	"quantumlife/internal/devicereg"
	"quantumlife/internal/persist"
	domainreg "quantumlife/pkg/domain/devicereg"
)

// testPeriodKey returns a period key that won't be evicted during tests.
func testPeriodKey() string {
	return time.Now().Format("2006-01-02")
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test: Enum Validation
// ═══════════════════════════════════════════════════════════════════════════════

func TestDevicePlatform_Validate(t *testing.T) {
	t.Run("ios is valid", func(t *testing.T) {
		p := domainreg.DevicePlatformIOS
		if err := p.Validate(); err != nil {
			t.Errorf("expected ios to be valid, got: %v", err)
		}
	})

	t.Run("invalid platform fails", func(t *testing.T) {
		p := domainreg.DevicePlatform("android")
		if err := p.Validate(); err == nil {
			t.Error("expected invalid platform to fail")
		}
	})
}

func TestDeviceRegState_Validate(t *testing.T) {
	validStates := []domainreg.DeviceRegState{
		domainreg.DeviceRegStateRegistered,
		domainreg.DeviceRegStateRevoked,
	}

	for _, s := range validStates {
		if err := s.Validate(); err != nil {
			t.Errorf("expected %v to be valid, got: %v", s, err)
		}
	}

	invalid := domainreg.DeviceRegState("unknown")
	if err := invalid.Validate(); err == nil {
		t.Error("expected invalid state to fail")
	}
}

func TestDeepLinkTarget_Validate(t *testing.T) {
	validTargets := []domainreg.DeepLinkTarget{
		domainreg.DeepLinkTargetInterrupts,
		domainreg.DeepLinkTargetTrust,
		domainreg.DeepLinkTargetShadow,
		domainreg.DeepLinkTargetReality,
		domainreg.DeepLinkTargetToday,
	}

	for _, target := range validTargets {
		if err := target.Validate(); err != nil {
			t.Errorf("expected %v to be valid, got: %v", target, err)
		}
	}

	invalid := domainreg.DeepLinkTarget("profile")
	if err := invalid.Validate(); err == nil {
		t.Error("expected invalid target to fail")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test: DeepLinkTarget ToPath
// ═══════════════════════════════════════════════════════════════════════════════

func TestDeepLinkTarget_ToPath(t *testing.T) {
	tests := []struct {
		target   domainreg.DeepLinkTarget
		expected string
	}{
		{domainreg.DeepLinkTargetInterrupts, "/interrupts/preview"},
		{domainreg.DeepLinkTargetTrust, "/trust/action/receipt"},
		{domainreg.DeepLinkTargetShadow, "/shadow/receipt"},
		{domainreg.DeepLinkTargetReality, "/reality"},
		{domainreg.DeepLinkTargetToday, "/today"},
	}

	for _, tt := range tests {
		path := tt.target.ToPath()
		if path != tt.expected {
			t.Errorf("ToPath(%v) = %v, expected %v", tt.target, path, tt.expected)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test: CanonicalString Stability
// ═══════════════════════════════════════════════════════════════════════════════

func TestDevicePlatform_CanonicalString_Stable(t *testing.T) {
	p := domainreg.DevicePlatformIOS
	cs1 := p.CanonicalString()
	cs2 := p.CanonicalString()

	if cs1 != cs2 {
		t.Error("CanonicalString should be deterministic")
	}
	if cs1 == "" {
		t.Error("CanonicalString should not be empty")
	}
}

func TestDeviceRegistrationReceipt_CanonicalString_Stable(t *testing.T) {
	periodKey := testPeriodKey()
	receipt := &domainreg.DeviceRegistrationReceipt{
		PeriodKey:     periodKey,
		Platform:      domainreg.DevicePlatformIOS,
		CircleIDHash:  "abc123",
		TokenHash:     "def456",
		SealedRefHash: "ghi789",
		State:         domainreg.DeviceRegStateRegistered,
	}

	cs1 := receipt.CanonicalString()
	cs2 := receipt.CanonicalString()

	if cs1 != cs2 {
		t.Error("CanonicalString should be deterministic")
	}
	if cs1 == "" {
		t.Error("CanonicalString should not be empty")
	}
}

func TestDeviceRegistrationReceipt_ComputeStatusHash_Deterministic(t *testing.T) {
	periodKey := testPeriodKey()
	receipt := &domainreg.DeviceRegistrationReceipt{
		PeriodKey:     periodKey,
		Platform:      domainreg.DevicePlatformIOS,
		CircleIDHash:  "abc123",
		TokenHash:     "def456",
		SealedRefHash: "ghi789",
		State:         domainreg.DeviceRegStateRegistered,
	}

	hash1 := receipt.ComputeStatusHash()
	hash2 := receipt.ComputeStatusHash()

	if hash1 != hash2 {
		t.Error("ComputeStatusHash should be deterministic")
	}
	if hash1 == "" {
		t.Error("StatusHash should not be empty")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test: Receipt Validation
// ═══════════════════════════════════════════════════════════════════════════════

func TestDeviceRegistrationReceipt_Validate(t *testing.T) {
	periodKey := testPeriodKey()

	t.Run("valid receipt passes", func(t *testing.T) {
		receipt := &domainreg.DeviceRegistrationReceipt{
			PeriodKey:     periodKey,
			Platform:      domainreg.DevicePlatformIOS,
			CircleIDHash:  "abc123",
			TokenHash:     "def456",
			SealedRefHash: "ghi789",
			State:         domainreg.DeviceRegStateRegistered,
		}

		if err := receipt.Validate(); err != nil {
			t.Errorf("expected valid receipt to pass: %v", err)
		}
	})

	t.Run("missing period_key fails", func(t *testing.T) {
		receipt := &domainreg.DeviceRegistrationReceipt{
			Platform:      domainreg.DevicePlatformIOS,
			CircleIDHash:  "abc123",
			TokenHash:     "def456",
			SealedRefHash: "ghi789",
			State:         domainreg.DeviceRegStateRegistered,
		}

		if err := receipt.Validate(); err == nil {
			t.Error("expected missing period_key to fail")
		}
	})

	t.Run("missing token_hash fails", func(t *testing.T) {
		receipt := &domainreg.DeviceRegistrationReceipt{
			PeriodKey:     periodKey,
			Platform:      domainreg.DevicePlatformIOS,
			CircleIDHash:  "abc123",
			SealedRefHash: "ghi789",
			State:         domainreg.DeviceRegStateRegistered,
		}

		if err := receipt.Validate(); err == nil {
			t.Error("expected missing token_hash to fail")
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test: Forbidden Patterns
// ═══════════════════════════════════════════════════════════════════════════════

func TestCheckForbiddenPatterns(t *testing.T) {
	t.Run("email is forbidden", func(t *testing.T) {
		err := domainreg.CheckForbiddenPatterns("test@example.com")
		if err == nil {
			t.Error("expected email to be forbidden")
		}
	})

	t.Run("url is forbidden", func(t *testing.T) {
		err := domainreg.CheckForbiddenPatterns("https://example.com")
		if err == nil {
			t.Error("expected URL to be forbidden")
		}
	})

	t.Run("currency is forbidden", func(t *testing.T) {
		err := domainreg.CheckForbiddenPatterns("$100.00")
		if err == nil {
			t.Error("expected currency to be forbidden")
		}
	})

	t.Run("hash is allowed", func(t *testing.T) {
		err := domainreg.CheckForbiddenPatterns("abc123def456")
		if err != nil {
			t.Errorf("expected hash to be allowed: %v", err)
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test: ValidateOpenParam
// ═══════════════════════════════════════════════════════════════════════════════

func TestValidateOpenParam(t *testing.T) {
	t.Run("valid targets", func(t *testing.T) {
		validParams := []string{"interrupts", "trust", "shadow", "reality", "today"}
		for _, p := range validParams {
			target, err := domainreg.ValidateOpenParam(p)
			if err != nil {
				t.Errorf("expected %q to be valid: %v", p, err)
			}
			if target == "" {
				t.Errorf("expected non-empty target for %q", p)
			}
		}
	})

	t.Run("empty param fails", func(t *testing.T) {
		_, err := domainreg.ValidateOpenParam("")
		if err == nil {
			t.Error("expected empty param to fail")
		}
	})

	t.Run("invalid param fails", func(t *testing.T) {
		_, err := domainreg.ValidateOpenParam("invalid")
		if err == nil {
			t.Error("expected invalid param to fail")
		}
	})

	t.Run("special chars rejected", func(t *testing.T) {
		_, err := domainreg.ValidateOpenParam("today&foo=bar")
		if err == nil {
			t.Error("expected special chars to be rejected")
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test: Engine - BuildRegistrationReceipt
// ═══════════════════════════════════════════════════════════════════════════════

func TestEngine_BuildRegistrationReceipt(t *testing.T) {
	engine := devicereg.NewEngine()
	periodKey := testPeriodKey()

	receipt := engine.BuildRegistrationReceipt(
		periodKey,
		domainreg.DevicePlatformIOS,
		"circle_hash_123",
		"token_hash_456",
		"sealed_ref_hash_789",
	)

	if receipt == nil {
		t.Fatal("expected receipt to be created")
	}

	if receipt.PeriodKey != periodKey {
		t.Errorf("expected period_key %q, got %q", periodKey, receipt.PeriodKey)
	}

	if receipt.Platform != domainreg.DevicePlatformIOS {
		t.Errorf("expected platform ios, got %v", receipt.Platform)
	}

	if receipt.StatusHash == "" {
		t.Error("expected StatusHash to be computed")
	}

	if receipt.ReceiptID == "" {
		t.Error("expected ReceiptID to be computed")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test: Engine - BuildProofPage
// ═══════════════════════════════════════════════════════════════════════════════

func TestEngine_BuildProofPage_WithReceipt(t *testing.T) {
	engine := devicereg.NewEngine()
	periodKey := testPeriodKey()

	receipt := &domainreg.DeviceRegistrationReceipt{
		PeriodKey:     periodKey,
		Platform:      domainreg.DevicePlatformIOS,
		CircleIDHash:  "abc123",
		TokenHash:     "def456789012345678901234567890123456789012345678901234567890123",
		SealedRefHash: "ghi789",
		State:         domainreg.DeviceRegStateRegistered,
		StatusHash:    "status123456789012345678901234567890",
	}

	page := engine.BuildProofPage(receipt)

	if page.Title != "Sealed, quietly." {
		t.Errorf("expected title 'Sealed, quietly.', got %q", page.Title)
	}

	if !page.HasRegistration {
		t.Error("expected HasRegistration to be true")
	}

	if page.TokenHashPrefix == "" {
		t.Error("expected TokenHashPrefix to be set")
	}

	// Verify prefix is at most 8 chars
	if len(page.TokenHashPrefix) > 8 {
		t.Errorf("TokenHashPrefix should be max 8 chars, got %d", len(page.TokenHashPrefix))
	}
}

func TestEngine_BuildProofPage_NoReceipt(t *testing.T) {
	engine := devicereg.NewEngine()

	page := engine.BuildProofPage(nil)

	if page.Title != "Device, quietly." {
		t.Errorf("expected default title, got %q", page.Title)
	}

	if page.HasRegistration {
		t.Error("expected HasRegistration to be false")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test: Engine - ComputeDeepLinkTarget
// ═══════════════════════════════════════════════════════════════════════════════

func TestEngine_ComputeDeepLinkTarget_Priority(t *testing.T) {
	engine := devicereg.NewEngine()

	t.Run("interrupt preview has highest priority", func(t *testing.T) {
		input := &domainreg.DeepLinkComputeInput{
			InterruptPreviewAvailable:   true,
			InterruptPreviewAcked:       false,
			TrustActionReceiptAvailable: true,
			ShadowReceiptCueAvailable:   true,
			RealityCueAvailable:         true,
		}

		target := engine.ComputeDeepLinkTarget(input)
		if target != domainreg.DeepLinkTargetInterrupts {
			t.Errorf("expected interrupts, got %v", target)
		}
	})

	t.Run("trust is second priority", func(t *testing.T) {
		input := &domainreg.DeepLinkComputeInput{
			InterruptPreviewAvailable:   false,
			TrustActionReceiptAvailable: true,
			TrustActionReceiptDismissed: false,
			ShadowReceiptCueAvailable:   true,
			RealityCueAvailable:         true,
		}

		target := engine.ComputeDeepLinkTarget(input)
		if target != domainreg.DeepLinkTargetTrust {
			t.Errorf("expected trust, got %v", target)
		}
	})

	t.Run("shadow is third priority", func(t *testing.T) {
		input := &domainreg.DeepLinkComputeInput{
			InterruptPreviewAvailable:   false,
			TrustActionReceiptAvailable: false,
			ShadowReceiptCueAvailable:   true,
			ShadowReceiptCueDismissed:   false,
			RealityCueAvailable:         true,
		}

		target := engine.ComputeDeepLinkTarget(input)
		if target != domainreg.DeepLinkTargetShadow {
			t.Errorf("expected shadow, got %v", target)
		}
	})

	t.Run("reality is fourth priority", func(t *testing.T) {
		input := &domainreg.DeepLinkComputeInput{
			InterruptPreviewAvailable: false,
			ShadowReceiptCueAvailable: false,
			RealityCueAvailable:       true,
		}

		target := engine.ComputeDeepLinkTarget(input)
		if target != domainreg.DeepLinkTargetReality {
			t.Errorf("expected reality, got %v", target)
		}
	})

	t.Run("today is default", func(t *testing.T) {
		input := &domainreg.DeepLinkComputeInput{}

		target := engine.ComputeDeepLinkTarget(input)
		if target != domainreg.DeepLinkTargetToday {
			t.Errorf("expected today, got %v", target)
		}
	})

	t.Run("nil input defaults to today", func(t *testing.T) {
		target := engine.ComputeDeepLinkTarget(nil)
		if target != domainreg.DeepLinkTargetToday {
			t.Errorf("expected today for nil input, got %v", target)
		}
	})
}

func TestEngine_ComputeDeepLinkTarget_Deterministic(t *testing.T) {
	engine := devicereg.NewEngine()

	input := &domainreg.DeepLinkComputeInput{
		InterruptPreviewAvailable:   true,
		TrustActionReceiptAvailable: true,
		ShadowReceiptCueAvailable:   true,
		RealityCueAvailable:         true,
	}

	target1 := engine.ComputeDeepLinkTarget(input)
	target2 := engine.ComputeDeepLinkTarget(input)

	if target1 != target2 {
		t.Error("ComputeDeepLinkTarget should be deterministic")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test: Engine - Device Registration Cue
// ═══════════════════════════════════════════════════════════════════════════════

func TestEngine_ShouldShowDeviceRegCue(t *testing.T) {
	engine := devicereg.NewEngine()

	t.Run("shows when connected and no device", func(t *testing.T) {
		if !engine.ShouldShowDeviceRegCue(true, false) {
			t.Error("expected cue to show when connected and no device")
		}
	})

	t.Run("hidden when not connected", func(t *testing.T) {
		if engine.ShouldShowDeviceRegCue(false, false) {
			t.Error("expected cue to be hidden when not connected")
		}
	})

	t.Run("hidden when device registered", func(t *testing.T) {
		if engine.ShouldShowDeviceRegCue(true, true) {
			t.Error("expected cue to be hidden when device registered")
		}
	})
}

func TestEngine_BuildDeviceRegCue(t *testing.T) {
	engine := devicereg.NewEngine()

	cue := engine.BuildDeviceRegCue(true)

	if !cue.Available {
		t.Error("expected cue to be available")
	}

	if cue.Text != domainreg.DefaultDeviceRegCueText {
		t.Errorf("expected default text, got %q", cue.Text)
	}

	if cue.Priority != domainreg.DefaultDeviceRegCuePriority {
		t.Errorf("expected priority %d, got %d", domainreg.DefaultDeviceRegCuePriority, cue.Priority)
	}

	// Verify it's the lowest priority (highest number)
	if cue.Priority < 50 {
		t.Errorf("expected lowest priority (>=50), got %d", cue.Priority)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test: Persistence Store
// ═══════════════════════════════════════════════════════════════════════════════

func TestDeviceRegistrationStore_AppendRegistration(t *testing.T) {
	store := persist.NewDeviceRegistrationStore(persist.DefaultDeviceRegistrationStoreConfig())
	periodKey := testPeriodKey()

	receipt := &domainreg.DeviceRegistrationReceipt{
		PeriodKey:     periodKey,
		Platform:      domainreg.DevicePlatformIOS,
		CircleIDHash:  "circle_abc",
		TokenHash:     "token_def",
		SealedRefHash: "sealed_ghi",
		State:         domainreg.DeviceRegStateRegistered,
	}

	err := store.AppendRegistration(receipt)
	if err != nil {
		t.Fatalf("failed to append registration: %v", err)
	}

	// Verify stored
	latest := store.LatestByCircle("circle_abc")
	if latest == nil {
		t.Fatal("expected registration to be stored")
	}

	if latest.TokenHash != "token_def" {
		t.Errorf("expected token_hash token_def, got %s", latest.TokenHash)
	}
}

func TestDeviceRegistrationStore_HasActiveRegistration(t *testing.T) {
	store := persist.NewDeviceRegistrationStore(persist.DefaultDeviceRegistrationStoreConfig())
	periodKey := testPeriodKey()

	// No registration yet
	if store.HasActiveRegistration("circle_abc", domainreg.DevicePlatformIOS) {
		t.Error("expected no active registration initially")
	}

	// Add registration
	receipt := &domainreg.DeviceRegistrationReceipt{
		PeriodKey:     periodKey,
		Platform:      domainreg.DevicePlatformIOS,
		CircleIDHash:  "circle_abc",
		TokenHash:     "token_def",
		SealedRefHash: "sealed_ghi",
		State:         domainreg.DeviceRegStateRegistered,
	}
	_ = store.AppendRegistration(receipt)

	// Now should have active registration
	if !store.HasActiveRegistration("circle_abc", domainreg.DevicePlatformIOS) {
		t.Error("expected active registration after append")
	}
}

func TestDeviceRegistrationStore_BoundedRetention_ByCount(t *testing.T) {
	cfg := persist.DeviceRegistrationStoreConfig{
		MaxRecords:       5, // Small limit for testing
		MaxRetentionDays: 30,
	}
	store := persist.NewDeviceRegistrationStore(cfg)
	periodKey := testPeriodKey()

	// Add more records than limit
	for i := 0; i < 10; i++ {
		receipt := &domainreg.DeviceRegistrationReceipt{
			PeriodKey:     periodKey,
			Platform:      domainreg.DevicePlatformIOS,
			CircleIDHash:  fmt.Sprintf("circle_%d", i),
			TokenHash:     fmt.Sprintf("token_%d", i),
			SealedRefHash: fmt.Sprintf("sealed_%d", i),
			State:         domainreg.DeviceRegStateRegistered,
		}
		_ = store.AppendRegistration(receipt)
	}

	// Should have at most maxRecords
	total := store.TotalRecords()
	if total > 5 {
		t.Errorf("expected at most 5 records after eviction, got %d", total)
	}
}

func TestDeviceRegistrationStore_BoundedRetention_ByDate(t *testing.T) {
	cfg := persist.DeviceRegistrationStoreConfig{
		MaxRecords:       200,
		MaxRetentionDays: 30,
	}
	store := persist.NewDeviceRegistrationStore(cfg)

	// Use relative dates: today and 45 days ago
	now := time.Now()
	recentPeriod := now.Format("2006-01-02")
	oldPeriod := now.AddDate(0, 0, -45).Format("2006-01-02")

	// Add old receipt
	oldReceipt := &domainreg.DeviceRegistrationReceipt{
		PeriodKey:     oldPeriod,
		Platform:      domainreg.DevicePlatformIOS,
		CircleIDHash:  "circle_old",
		TokenHash:     "token_old",
		SealedRefHash: "sealed_old",
		State:         domainreg.DeviceRegStateRegistered,
	}
	_ = store.AppendRegistration(oldReceipt)

	// Add recent receipt
	recentReceipt := &domainreg.DeviceRegistrationReceipt{
		PeriodKey:     recentPeriod,
		Platform:      domainreg.DevicePlatformIOS,
		CircleIDHash:  "circle_new",
		TokenHash:     "token_new",
		SealedRefHash: "sealed_new",
		State:         domainreg.DeviceRegStateRegistered,
	}
	_ = store.AppendRegistration(recentReceipt)

	// Evict with current time
	store.EvictOldPeriods(now)

	// Old period should be evicted
	oldRecords := store.GetByPeriod(oldPeriod)
	if len(oldRecords) != 0 {
		t.Errorf("expected old period to be evicted, got %d records", len(oldRecords))
	}

	// New period should remain
	newRecords := store.GetByPeriod(recentPeriod)
	if len(newRecords) != 1 {
		t.Errorf("expected new period to remain, got %d records", len(newRecords))
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test: Hash-Only Storage
// ═══════════════════════════════════════════════════════════════════════════════

func TestHashString(t *testing.T) {
	hash1 := domainreg.HashString("test")
	hash2 := domainreg.HashString("test")

	if hash1 != hash2 {
		t.Error("HashString should be deterministic")
	}

	if len(hash1) != 64 {
		t.Errorf("expected 64-char hash, got %d", len(hash1))
	}
}

func TestHashStringShort(t *testing.T) {
	hash := domainreg.HashStringShort("test")

	if len(hash) != 32 {
		t.Errorf("expected 32-char hash, got %d", len(hash))
	}
}

func TestMagnitudeFromCount(t *testing.T) {
	tests := []struct {
		count    int
		expected domainreg.MagnitudeBucket
	}{
		{0, domainreg.MagnitudeNothing},
		{1, domainreg.MagnitudeAFew},
		{3, domainreg.MagnitudeAFew},
		{4, domainreg.MagnitudeSeveral},
		{10, domainreg.MagnitudeSeveral},
	}

	for _, tt := range tests {
		result := domainreg.MagnitudeFromCount(tt.count)
		if result != tt.expected {
			t.Errorf("MagnitudeFromCount(%d) = %v, expected %v", tt.count, result, tt.expected)
		}
	}
}
