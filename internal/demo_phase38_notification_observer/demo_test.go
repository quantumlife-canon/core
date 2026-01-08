// Package demo_phase38_notification_observer tests Phase 38: Mobile Notification Metadata Observer.
//
// This package demonstrates:
//   - Notification metadata observation WITHOUT reading content
//   - Abstract app class buckets (transport, health, institution, commerce, unknown)
//   - Magnitude buckets (nothing, a_few, several)
//   - Horizon buckets (now, soon, later)
//   - Hash-only storage
//   - Max 1 signal per app class per period
//   - Bounded retention (200 records, 30 days)
//   - Integration with Phase 31.4 pressure pipeline
//
// CRITICAL INVARIANTS:
//   - NO notification content - only OS-provided category metadata
//   - NO app names - only abstract class buckets
//   - NO device identifiers - hash-only storage
//   - NO time.Now() - clock injection only
//   - NO goroutines
//   - NO network calls - pure local observation
//   - NO decision logic - observation ONLY
//   - NO delivery triggers - cannot send notifications
//   - stdlib only
//
// Reference: docs/ADR/ADR-0075-phase38-notification-metadata-observer.md
package demo_phase38_notification_observer

import (
	"testing"
	"time"

	"quantumlife/internal/notificationobserver"
	"quantumlife/internal/persist"
	domainnotificationobserver "quantumlife/pkg/domain/notificationobserver"
)

// ============================================================================
// Section 1: Domain Model Tests
// ============================================================================

func TestNotificationSourceKindValidation(t *testing.T) {
	// Valid source kind
	valid := domainnotificationobserver.SourceMobileOS
	if err := valid.Validate(); err != nil {
		t.Errorf("SourceMobileOS should be valid: %v", err)
	}

	// Invalid source kind
	invalid := domainnotificationobserver.NotificationSourceKind("invalid_source")
	if err := invalid.Validate(); err == nil {
		t.Error("Invalid source kind should fail validation")
	}
}

func TestNotificationAppClassValidation(t *testing.T) {
	validClasses := []domainnotificationobserver.NotificationAppClass{
		domainnotificationobserver.AppClassTransport,
		domainnotificationobserver.AppClassHealth,
		domainnotificationobserver.AppClassInstitution,
		domainnotificationobserver.AppClassCommerce,
		domainnotificationobserver.AppClassUnknown,
	}

	for _, ac := range validClasses {
		if err := ac.Validate(); err != nil {
			t.Errorf("App class %s should be valid: %v", ac, err)
		}
	}

	// Invalid app class
	invalid := domainnotificationobserver.NotificationAppClass("uber")
	if err := invalid.Validate(); err == nil {
		t.Error("App name 'uber' should fail validation - we don't store app names")
	}
}

func TestMagnitudeBucketValidation(t *testing.T) {
	validMagnitudes := []domainnotificationobserver.MagnitudeBucket{
		domainnotificationobserver.MagnitudeNothing,
		domainnotificationobserver.MagnitudeAFew,
		domainnotificationobserver.MagnitudeSeveral,
	}

	for _, m := range validMagnitudes {
		if err := m.Validate(); err != nil {
			t.Errorf("Magnitude %s should be valid: %v", m, err)
		}
	}

	// Invalid magnitude
	invalid := domainnotificationobserver.MagnitudeBucket("5")
	if err := invalid.Validate(); err == nil {
		t.Error("Raw count '5' should fail validation - we only accept buckets")
	}
}

func TestHorizonBucketValidation(t *testing.T) {
	validHorizons := []domainnotificationobserver.HorizonBucket{
		domainnotificationobserver.HorizonNow,
		domainnotificationobserver.HorizonSoon,
		domainnotificationobserver.HorizonLater,
	}

	for _, h := range validHorizons {
		if err := h.Validate(); err != nil {
			t.Errorf("Horizon %s should be valid: %v", h, err)
		}
	}

	// Invalid horizon
	invalid := domainnotificationobserver.HorizonBucket("3:47PM")
	if err := invalid.Validate(); err == nil {
		t.Error("Raw time '3:47PM' should fail validation - we only accept buckets")
	}
}

func TestToMagnitude(t *testing.T) {
	tests := []struct {
		count    int
		expected domainnotificationobserver.MagnitudeBucket
	}{
		{0, domainnotificationobserver.MagnitudeNothing},
		{1, domainnotificationobserver.MagnitudeAFew},
		{2, domainnotificationobserver.MagnitudeAFew},
		{3, domainnotificationobserver.MagnitudeAFew},
		{4, domainnotificationobserver.MagnitudeSeveral},
		{10, domainnotificationobserver.MagnitudeSeveral},
		{100, domainnotificationobserver.MagnitudeSeveral},
	}

	for _, tc := range tests {
		result := domainnotificationobserver.ToMagnitude(tc.count)
		if result != tc.expected {
			t.Errorf("ToMagnitude(%d) = %s, want %s", tc.count, result, tc.expected)
		}
	}
}

func TestAllAppClasses(t *testing.T) {
	classes := domainnotificationobserver.AllAppClasses()
	if len(classes) != 5 {
		t.Errorf("Expected 5 app classes, got %d", len(classes))
	}

	// Verify deterministic order
	expected := []domainnotificationobserver.NotificationAppClass{
		domainnotificationobserver.AppClassTransport,
		domainnotificationobserver.AppClassHealth,
		domainnotificationobserver.AppClassInstitution,
		domainnotificationobserver.AppClassCommerce,
		domainnotificationobserver.AppClassUnknown,
	}
	for i, class := range classes {
		if class != expected[i] {
			t.Errorf("Class at index %d = %s, want %s", i, class, expected[i])
		}
	}
}

func TestAppClassDisplayText(t *testing.T) {
	tests := []struct {
		class    domainnotificationobserver.NotificationAppClass
		expected string
	}{
		{domainnotificationobserver.AppClassTransport, "Transport"},
		{domainnotificationobserver.AppClassHealth, "Health"},
		{domainnotificationobserver.AppClassInstitution, "Institution"},
		{domainnotificationobserver.AppClassCommerce, "Commerce"},
		{domainnotificationobserver.AppClassUnknown, "External"},
	}

	for _, tc := range tests {
		result := tc.class.DisplayText()
		if result != tc.expected {
			t.Errorf("DisplayText for %s = %s, want %s", tc.class, result, tc.expected)
		}
	}
}

// ============================================================================
// Section 2: Engine Tests
// ============================================================================

func TestEngineObserveNotificationMetadata(t *testing.T) {
	engine := notificationobserver.NewEngine()

	input := &domainnotificationobserver.NotificationObserverInput{
		AppClass:  domainnotificationobserver.AppClassTransport,
		Magnitude: domainnotificationobserver.MagnitudeAFew,
		Horizon:   domainnotificationobserver.HorizonNow,
		PeriodKey: "2026-01-08",
	}

	signal := engine.ObserveNotificationMetadata(input)
	if signal == nil {
		t.Fatal("Expected signal, got nil")
	}

	if signal.AppClass != domainnotificationobserver.AppClassTransport {
		t.Errorf("AppClass = %s, want transport", signal.AppClass)
	}
	if signal.Magnitude != domainnotificationobserver.MagnitudeAFew {
		t.Errorf("Magnitude = %s, want a_few", signal.Magnitude)
	}
	if signal.Horizon != domainnotificationobserver.HorizonNow {
		t.Errorf("Horizon = %s, want now", signal.Horizon)
	}
	if signal.EvidenceHash == "" {
		t.Error("EvidenceHash should not be empty")
	}
	if signal.StatusHash == "" {
		t.Error("StatusHash should not be empty")
	}
	if signal.SignalID == "" {
		t.Error("SignalID should not be empty")
	}
}

func TestEngineObserveReturnsNilForNothing(t *testing.T) {
	engine := notificationobserver.NewEngine()

	input := &domainnotificationobserver.NotificationObserverInput{
		AppClass:  domainnotificationobserver.AppClassTransport,
		Magnitude: domainnotificationobserver.MagnitudeNothing, // No notifications
		Horizon:   domainnotificationobserver.HorizonNow,
		PeriodKey: "2026-01-08",
	}

	signal := engine.ObserveNotificationMetadata(input)
	if signal != nil {
		t.Error("Expected nil signal for magnitude=nothing")
	}
}

func TestEngineObserveReturnsNilForNilInput(t *testing.T) {
	engine := notificationobserver.NewEngine()
	signal := engine.ObserveNotificationMetadata(nil)
	if signal != nil {
		t.Error("Expected nil signal for nil input")
	}
}

func TestEngineShouldHold(t *testing.T) {
	engine := notificationobserver.NewEngine()

	// Valid signal should be held by default
	signal := &domainnotificationobserver.NotificationPressureSignal{
		Source:       domainnotificationobserver.SourceMobileOS,
		AppClass:     domainnotificationobserver.AppClassTransport,
		Magnitude:    domainnotificationobserver.MagnitudeAFew,
		Horizon:      domainnotificationobserver.HorizonNow,
		PeriodKey:    "2026-01-08",
		EvidenceHash: "abc123",
	}

	if !engine.ShouldHold(signal) {
		t.Error("Default behavior should be HOLD")
	}

	// Nil signal should not be held
	if engine.ShouldHold(nil) {
		t.Error("Nil signal should not be held")
	}
}

func TestEngineBuildInputFromParams(t *testing.T) {
	engine := notificationobserver.NewEngine()

	// Valid params
	input := engine.BuildInputFromParams("transport", "a_few", "now", "2026-01-08")
	if input == nil {
		t.Fatal("Expected input, got nil")
	}
	if input.AppClass != domainnotificationobserver.AppClassTransport {
		t.Errorf("AppClass = %s, want transport", input.AppClass)
	}

	// Invalid app class
	input = engine.BuildInputFromParams("uber", "a_few", "now", "2026-01-08")
	if input != nil {
		t.Error("Expected nil for invalid app class 'uber'")
	}

	// Invalid magnitude
	input = engine.BuildInputFromParams("transport", "5", "now", "2026-01-08")
	if input != nil {
		t.Error("Expected nil for invalid magnitude '5'")
	}

	// Empty period key
	input = engine.BuildInputFromParams("transport", "a_few", "now", "")
	if input != nil {
		t.Error("Expected nil for empty period key")
	}
}

func TestEngineMergeSignals(t *testing.T) {
	engine := notificationobserver.NewEngine()

	existing := &domainnotificationobserver.NotificationPressureSignal{
		Source:       domainnotificationobserver.SourceMobileOS,
		AppClass:     domainnotificationobserver.AppClassTransport,
		Magnitude:    domainnotificationobserver.MagnitudeAFew,
		Horizon:      domainnotificationobserver.HorizonSoon,
		PeriodKey:    "2026-01-08",
		EvidenceHash: "abc123",
	}

	// Higher magnitude wins
	newSignal := &domainnotificationobserver.NotificationPressureSignal{
		Source:       domainnotificationobserver.SourceMobileOS,
		AppClass:     domainnotificationobserver.AppClassTransport,
		Magnitude:    domainnotificationobserver.MagnitudeSeveral, // Higher
		Horizon:      domainnotificationobserver.HorizonLater,
		PeriodKey:    "2026-01-08",
		EvidenceHash: "def456",
	}

	result := engine.MergeSignals(existing, newSignal)
	if result.Magnitude != domainnotificationobserver.MagnitudeSeveral {
		t.Error("Higher magnitude signal should win")
	}

	// Same magnitude, more urgent horizon wins
	newSignal.Magnitude = domainnotificationobserver.MagnitudeAFew
	newSignal.Horizon = domainnotificationobserver.HorizonNow
	result = engine.MergeSignals(existing, newSignal)
	if result.Horizon != domainnotificationobserver.HorizonNow {
		t.Error("More urgent horizon should win when magnitude is equal")
	}
}

func TestEngineConvertToPressureInput(t *testing.T) {
	engine := notificationobserver.NewEngine()

	signal := &domainnotificationobserver.NotificationPressureSignal{
		Source:       domainnotificationobserver.SourceMobileOS,
		AppClass:     domainnotificationobserver.AppClassTransport,
		Magnitude:    domainnotificationobserver.MagnitudeAFew,
		Horizon:      domainnotificationobserver.HorizonNow,
		PeriodKey:    "2026-01-08",
		EvidenceHash: "abc123",
	}

	pressureInput := engine.ConvertToPressureInput(signal, "sovereign_hash_123")
	if pressureInput == nil {
		t.Fatal("Expected pressure input, got nil")
	}
	if pressureInput.EvidenceHash != signal.EvidenceHash {
		t.Error("Evidence hash should be preserved")
	}

	// Nil signal returns nil
	pressureInput = engine.ConvertToPressureInput(nil, "sovereign_hash_123")
	if pressureInput != nil {
		t.Error("Expected nil for nil signal")
	}

	// Empty sovereign hash returns nil
	pressureInput = engine.ConvertToPressureInput(signal, "")
	if pressureInput != nil {
		t.Error("Expected nil for empty sovereign hash")
	}
}

// ============================================================================
// Section 3: Persistence Tests
// ============================================================================

func TestStoreAppendAndGetByPeriod(t *testing.T) {
	store := persist.NewNotificationObserverStore(persist.DefaultNotificationObserverStoreConfig())

	signal := &domainnotificationobserver.NotificationPressureSignal{
		Source:       domainnotificationobserver.SourceMobileOS,
		AppClass:     domainnotificationobserver.AppClassTransport,
		Magnitude:    domainnotificationobserver.MagnitudeAFew,
		Horizon:      domainnotificationobserver.HorizonNow,
		PeriodKey:    "2026-01-08",
		EvidenceHash: "abc123",
	}
	signal.StatusHash = signal.ComputeStatusHash()
	signal.SignalID = signal.ComputeSignalID()

	err := store.AppendSignal(signal)
	if err != nil {
		t.Fatalf("AppendSignal failed: %v", err)
	}

	signals := store.GetByPeriod("2026-01-08")
	if len(signals) != 1 {
		t.Fatalf("Expected 1 signal, got %d", len(signals))
	}
	if signals[0].AppClass != domainnotificationobserver.AppClassTransport {
		t.Error("Retrieved signal has wrong app class")
	}
}

func TestStoreDeduplicationMaxOnePerAppClassPerPeriod(t *testing.T) {
	store := persist.NewNotificationObserverStore(persist.DefaultNotificationObserverStoreConfig())

	// First signal
	signal1 := &domainnotificationobserver.NotificationPressureSignal{
		Source:       domainnotificationobserver.SourceMobileOS,
		AppClass:     domainnotificationobserver.AppClassTransport,
		Magnitude:    domainnotificationobserver.MagnitudeAFew,
		Horizon:      domainnotificationobserver.HorizonSoon,
		PeriodKey:    "2026-01-08",
		EvidenceHash: "abc123",
	}
	signal1.StatusHash = signal1.ComputeStatusHash()
	signal1.SignalID = signal1.ComputeSignalID()

	store.AppendSignal(signal1)

	// Second signal with same app class and period but higher magnitude
	signal2 := &domainnotificationobserver.NotificationPressureSignal{
		Source:       domainnotificationobserver.SourceMobileOS,
		AppClass:     domainnotificationobserver.AppClassTransport,
		Magnitude:    domainnotificationobserver.MagnitudeSeveral, // Higher
		Horizon:      domainnotificationobserver.HorizonNow,
		PeriodKey:    "2026-01-08",
		EvidenceHash: "def456",
	}
	signal2.StatusHash = signal2.ComputeStatusHash()
	signal2.SignalID = signal2.ComputeSignalID()

	store.AppendSignal(signal2)

	// Should still have only 1 signal (max 1 per app class per period)
	signals := store.GetByPeriod("2026-01-08")
	if len(signals) != 1 {
		t.Fatalf("Expected 1 signal (max 1 per app class per period), got %d", len(signals))
	}

	// Should have the higher magnitude signal
	if signals[0].Magnitude != domainnotificationobserver.MagnitudeSeveral {
		t.Error("Higher magnitude signal should replace lower")
	}
}

func TestStoreBoundedRetentionMaxRecords(t *testing.T) {
	cfg := persist.NotificationObserverStoreConfig{
		MaxRecords:       5, // Very small for testing
		MaxRetentionDays: 30,
	}
	store := persist.NewNotificationObserverStore(cfg)

	// Add 7 signals (different app classes to avoid dedup)
	classes := []domainnotificationobserver.NotificationAppClass{
		domainnotificationobserver.AppClassTransport,
		domainnotificationobserver.AppClassHealth,
		domainnotificationobserver.AppClassInstitution,
		domainnotificationobserver.AppClassCommerce,
		domainnotificationobserver.AppClassUnknown,
	}

	for i := 0; i < 7; i++ {
		signal := &domainnotificationobserver.NotificationPressureSignal{
			Source:       domainnotificationobserver.SourceMobileOS,
			AppClass:     classes[i%5],
			Magnitude:    domainnotificationobserver.MagnitudeAFew,
			Horizon:      domainnotificationobserver.HorizonNow,
			PeriodKey:    "2026-01-0" + string('1'+byte(i)),
			EvidenceHash: "hash" + string('0'+byte(i)),
		}
		signal.StatusHash = signal.ComputeStatusHash()
		signal.SignalID = signal.ComputeSignalID()
		store.AppendSignal(signal)
	}

	// Should have max 5 records
	total := store.TotalRecords()
	if total > 5 {
		t.Errorf("Expected max 5 records, got %d", total)
	}
}

func TestStoreEvictOldPeriods(t *testing.T) {
	cfg := persist.NotificationObserverStoreConfig{
		MaxRecords:       200,
		MaxRetentionDays: 7, // 7 days retention
	}
	store := persist.NewNotificationObserverStore(cfg)

	// Add signal from 10 days ago
	oldSignal := &domainnotificationobserver.NotificationPressureSignal{
		Source:       domainnotificationobserver.SourceMobileOS,
		AppClass:     domainnotificationobserver.AppClassTransport,
		Magnitude:    domainnotificationobserver.MagnitudeAFew,
		Horizon:      domainnotificationobserver.HorizonNow,
		PeriodKey:    "2025-12-28", // Old date
		EvidenceHash: "old123",
	}
	oldSignal.StatusHash = oldSignal.ComputeStatusHash()
	oldSignal.SignalID = oldSignal.ComputeSignalID()
	store.AppendSignal(oldSignal)

	// Add recent signal
	newSignal := &domainnotificationobserver.NotificationPressureSignal{
		Source:       domainnotificationobserver.SourceMobileOS,
		AppClass:     domainnotificationobserver.AppClassHealth,
		Magnitude:    domainnotificationobserver.MagnitudeAFew,
		Horizon:      domainnotificationobserver.HorizonNow,
		PeriodKey:    "2026-01-08",
		EvidenceHash: "new123",
	}
	newSignal.StatusHash = newSignal.ComputeStatusHash()
	newSignal.SignalID = newSignal.ComputeSignalID()
	store.AppendSignal(newSignal)

	// Evict with explicit clock
	now := time.Date(2026, 1, 8, 12, 0, 0, 0, time.UTC)
	store.EvictOldPeriods(now)

	// Old period should be gone
	oldSignals := store.GetByPeriod("2025-12-28")
	if len(oldSignals) != 0 {
		t.Error("Old signals should be evicted")
	}

	// New period should remain
	newSignals := store.GetByPeriod("2026-01-08")
	if len(newSignals) != 1 {
		t.Error("New signals should remain")
	}
}

// ============================================================================
// Section 4: Privacy Tests
// ============================================================================

func TestForbiddenContentCheck(t *testing.T) {
	// Email pattern
	err := domainnotificationobserver.CheckForbiddenContent("user@example.com")
	if err == nil {
		t.Error("Should reject email pattern")
	}

	// URL pattern
	err = domainnotificationobserver.CheckForbiddenContent("https://example.com")
	if err == nil {
		t.Error("Should reject URL pattern")
	}

	// Currency pattern
	err = domainnotificationobserver.CheckForbiddenContent("$50.00")
	if err == nil {
		t.Error("Should reject currency pattern")
	}

	// Time pattern
	err = domainnotificationobserver.CheckForbiddenContent("arriving at 15:30")
	if err == nil {
		t.Error("Should reject time pattern")
	}

	// Valid abstract bucket
	err = domainnotificationobserver.CheckForbiddenContent("transport")
	if err != nil {
		t.Errorf("Should accept abstract bucket: %v", err)
	}
}

func TestHashStringDeterminism(t *testing.T) {
	// Same input produces same hash
	hash1 := domainnotificationobserver.HashString("test_input")
	hash2 := domainnotificationobserver.HashString("test_input")
	if hash1 != hash2 {
		t.Error("Same input should produce same hash")
	}

	// Different input produces different hash
	hash3 := domainnotificationobserver.HashString("different_input")
	if hash1 == hash3 {
		t.Error("Different input should produce different hash")
	}
}

func TestSignalCanonicalStringDeterminism(t *testing.T) {
	signal := &domainnotificationobserver.NotificationPressureSignal{
		Source:       domainnotificationobserver.SourceMobileOS,
		AppClass:     domainnotificationobserver.AppClassTransport,
		Magnitude:    domainnotificationobserver.MagnitudeAFew,
		Horizon:      domainnotificationobserver.HorizonNow,
		PeriodKey:    "2026-01-08",
		EvidenceHash: "abc123",
	}

	cs1 := signal.CanonicalString()
	cs2 := signal.CanonicalString()
	if cs1 != cs2 {
		t.Error("CanonicalString should be deterministic")
	}

	// Should include version prefix
	if len(cs1) < 15 || cs1[:12] != "NOTIF_SIGNAL" {
		t.Error("CanonicalString should have NOTIF_SIGNAL prefix")
	}
}

func TestSignalIDUniqueness(t *testing.T) {
	signal1 := &domainnotificationobserver.NotificationPressureSignal{
		Source:    domainnotificationobserver.SourceMobileOS,
		AppClass:  domainnotificationobserver.AppClassTransport,
		PeriodKey: "2026-01-08",
	}

	signal2 := &domainnotificationobserver.NotificationPressureSignal{
		Source:    domainnotificationobserver.SourceMobileOS,
		AppClass:  domainnotificationobserver.AppClassHealth, // Different class
		PeriodKey: "2026-01-08",
	}

	signal3 := &domainnotificationobserver.NotificationPressureSignal{
		Source:    domainnotificationobserver.SourceMobileOS,
		AppClass:  domainnotificationobserver.AppClassTransport,
		PeriodKey: "2026-01-09", // Different period
	}

	id1 := signal1.ComputeSignalID()
	id2 := signal2.ComputeSignalID()
	id3 := signal3.ComputeSignalID()

	if id1 == id2 {
		t.Error("Different app classes should have different signal IDs")
	}
	if id1 == id3 {
		t.Error("Different periods should have different signal IDs")
	}
}

// ============================================================================
// Section 5: Constants Tests
// ============================================================================

func TestBoundedRetentionConstants(t *testing.T) {
	if domainnotificationobserver.MaxSignalRecords != 200 {
		t.Errorf("MaxSignalRecords = %d, want 200", domainnotificationobserver.MaxSignalRecords)
	}
	if domainnotificationobserver.MaxRetentionDays != 30 {
		t.Errorf("MaxRetentionDays = %d, want 30", domainnotificationobserver.MaxRetentionDays)
	}
	if domainnotificationobserver.MaxSignalsPerAppClassPerPeriod != 1 {
		t.Errorf("MaxSignalsPerAppClassPerPeriod = %d, want 1", domainnotificationobserver.MaxSignalsPerAppClassPerPeriod)
	}
}
