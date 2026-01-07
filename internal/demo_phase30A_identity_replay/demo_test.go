// Package demo_phase30A_identity_replay provides demo tests for Phase 30A: Identity + Replay.
//
// These tests verify the critical safety invariants:
//   - Ed25519 device-rooted identity (stdlib only)
//   - Device fingerprint bound to Circle (hash-only, max 5 devices)
//   - Signed requests for replay export/import
//   - Deterministic replay bundle (pipe-delimited, NOT JSON)
//   - Bounded retention (30 days)
//   - No raw identifiers in bundles
//   - Deterministic hashing
//
// Reference: docs/ADR/ADR-0061-phase30A-identity-and-replay.md
package demo_phase30A_identity_replay

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"quantumlife/internal/deviceidentity"
	"quantumlife/internal/persist"
	"quantumlife/internal/replay"
	domaindeviceidentity "quantumlife/pkg/domain/deviceidentity"
	"quantumlife/pkg/domain/identity"
	domainreplay "quantumlife/pkg/domain/replay"
	"quantumlife/pkg/domain/storelog"
)

// testClock returns a deterministic clock for testing.
func testClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

// TestDeviceIdentityTypes verifies domain model types are correctly defined.
func TestDeviceIdentityTypes(t *testing.T) {
	// Verify MaxDevicesPerCircle constant
	if domaindeviceidentity.MaxDevicesPerCircle != 5 {
		t.Errorf("MaxDevicesPerCircle = %d, want 5", domaindeviceidentity.MaxDevicesPerCircle)
	}

	// Verify DefaultRetentionDays constant
	if domaindeviceidentity.DefaultRetentionDays != 30 {
		t.Errorf("DefaultRetentionDays = %d, want 30", domaindeviceidentity.DefaultRetentionDays)
	}

	// Verify MagnitudeBucket values
	magnitudes := []domaindeviceidentity.MagnitudeBucket{
		domaindeviceidentity.MagnitudeNothing,
		domaindeviceidentity.MagnitudeAFew,
		domaindeviceidentity.MagnitudeSeveral,
		domaindeviceidentity.MagnitudeMany,
	}
	for _, m := range magnitudes {
		if m == "" {
			t.Errorf("MagnitudeBucket value is empty")
		}
	}
}

// TestFingerprintDeterminism verifies fingerprint is deterministic.
func TestFingerprintDeterminism(t *testing.T) {
	// Same public key should always produce same fingerprint
	pubKey := domaindeviceidentity.DevicePublicKey("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")

	fp1 := pubKey.Fingerprint()
	fp2 := pubKey.Fingerprint()

	if fp1 != fp2 {
		t.Errorf("Fingerprint not deterministic: %s != %s", fp1, fp2)
	}

	// Fingerprint should be 64 hex chars
	if len(fp1) != 64 {
		t.Errorf("Fingerprint length = %d, want 64", len(fp1))
	}

	// Fingerprint should be lowercase hex
	for _, c := range fp1 {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("Fingerprint contains non-hex char: %c", c)
		}
	}
}

// TestPeriodKeyFormat verifies period key 15-minute bucket format.
func TestPeriodKeyFormat(t *testing.T) {
	testCases := []struct {
		name           string
		inputTime      time.Time
		expectedSuffix string
	}{
		{"At :00", time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC), "10:00"},
		{"At :07", time.Date(2025, 1, 15, 10, 7, 30, 0, time.UTC), "10:00"},
		{"At :14", time.Date(2025, 1, 15, 10, 14, 59, 0, time.UTC), "10:00"},
		{"At :15", time.Date(2025, 1, 15, 10, 15, 0, 0, time.UTC), "10:15"},
		{"At :22", time.Date(2025, 1, 15, 10, 22, 0, 0, time.UTC), "10:15"},
		{"At :30", time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC), "10:30"},
		{"At :45", time.Date(2025, 1, 15, 10, 45, 0, 0, time.UTC), "10:45"},
		{"At :59", time.Date(2025, 1, 15, 10, 59, 59, 0, time.UTC), "10:45"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pk := domaindeviceidentity.NewPeriodKey(tc.inputTime)

			// Period key format: YYYY-MM-DDTHH:MM
			if !strings.HasSuffix(string(pk), tc.expectedSuffix) {
				t.Errorf("PeriodKey = %q, want suffix %q", pk, tc.expectedSuffix)
			}

			// Should start with date
			if !strings.HasPrefix(string(pk), "2025-01-15T") {
				t.Errorf("PeriodKey = %q, want prefix 2025-01-15T", pk)
			}
		})
	}
}

// TestCircleBindingCanonicalString verifies canonical string format.
func TestCircleBindingCanonicalString(t *testing.T) {
	binding := domaindeviceidentity.NewCircleBinding(
		"test-circle",
		domaindeviceidentity.Fingerprint("abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"),
		time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
	)

	canonical := binding.CanonicalString()

	// Should be pipe-delimited
	if !strings.Contains(canonical, "|") {
		t.Errorf("CanonicalString should use pipe delimiter")
	}

	// Should start with version
	if !strings.HasPrefix(canonical, "v1|") {
		t.Errorf("CanonicalString should start with v1|, got %q", canonical)
	}

	// Should contain circle_binding type
	if !strings.Contains(canonical, "circle_binding") {
		t.Errorf("CanonicalString should contain circle_binding")
	}

	// Should NOT contain raw circle ID
	if strings.Contains(canonical, "test-circle") {
		t.Errorf("CanonicalString should not contain raw circle ID")
	}

	// BindingHash should be set
	if binding.BindingHash == "" {
		t.Errorf("BindingHash should be set")
	}
}

// TestCircleBindingHashDeterminism verifies binding hash is deterministic.
func TestCircleBindingHashDeterminism(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	fingerprint := domaindeviceidentity.Fingerprint("abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890")

	binding1 := domaindeviceidentity.NewCircleBinding("test-circle", fingerprint, now)
	binding2 := domaindeviceidentity.NewCircleBinding("test-circle", fingerprint, now)

	if binding1.BindingHash != binding2.BindingHash {
		t.Errorf("BindingHash not deterministic: %s != %s", binding1.BindingHash, binding2.BindingHash)
	}

	// Different circle should produce different hash
	binding3 := domaindeviceidentity.NewCircleBinding("other-circle", fingerprint, now)
	if binding1.BindingHash == binding3.BindingHash {
		t.Errorf("Different circles should produce different hashes")
	}
}

// TestMagnitudeBucketComputation verifies magnitude bucket abstraction.
func TestMagnitudeBucketComputation(t *testing.T) {
	testCases := []struct {
		count    int
		expected domaindeviceidentity.MagnitudeBucket
	}{
		{0, domaindeviceidentity.MagnitudeNothing},
		{1, domaindeviceidentity.MagnitudeAFew},
		{2, domaindeviceidentity.MagnitudeAFew},
		{3, domaindeviceidentity.MagnitudeAFew},
		{4, domaindeviceidentity.MagnitudeSeveral},
		{5, domaindeviceidentity.MagnitudeSeveral},
		{6, domaindeviceidentity.MagnitudeMany},
		{100, domaindeviceidentity.MagnitudeMany},
	}

	for _, tc := range testCases {
		result := domaindeviceidentity.ComputeMagnitude(tc.count)
		if result != tc.expected {
			t.Errorf("ComputeMagnitude(%d) = %q, want %q", tc.count, result, tc.expected)
		}
	}
}

// TestDeviceKeyStoreCreation verifies key store creates keys correctly.
func TestDeviceKeyStoreCreation(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "test-device-key")

	store := persist.NewDeviceKeyStore(keyPath)

	// First call should create keypair
	pubKey1, fp1, err := store.EnsureKeypair()
	if err != nil {
		t.Fatalf("EnsureKeypair failed: %v", err)
	}

	// Verify key and fingerprint are non-empty
	if pubKey1 == "" {
		t.Errorf("PublicKey should not be empty")
	}
	if fp1 == "" {
		t.Errorf("Fingerprint should not be empty")
	}

	// Second call should return same values (idempotent)
	pubKey2, fp2, err := store.EnsureKeypair()
	if err != nil {
		t.Fatalf("EnsureKeypair (2nd) failed: %v", err)
	}

	if pubKey1 != pubKey2 {
		t.Errorf("PublicKey should be same on repeated calls")
	}
	if fp1 != fp2 {
		t.Errorf("Fingerprint should be same on repeated calls")
	}

	// Verify key file exists with correct permissions
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("Key file stat failed: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("Key file permissions = %o, want 0600", info.Mode().Perm())
	}
}

// TestCircleBindingStoreMaxDevices verifies max devices per circle enforcement.
func TestCircleBindingStoreMaxDevices(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	store := persist.NewCircleBindingStore(testClock(now), nil)

	circleID := "test-circle"

	// Bind 5 devices (max allowed)
	for i := 0; i < domaindeviceidentity.MaxDevicesPerCircle; i++ {
		fp := domaindeviceidentity.Fingerprint(strings.Repeat("a", 60) + strings.Repeat(string('0'+byte(i)), 4))
		result, err := store.Bind(circleID, fp)
		if err != nil {
			t.Fatalf("Bind %d failed: %v", i, err)
		}
		if !result.Success {
			t.Errorf("Bind %d should succeed: %s", i, result.Error)
		}
	}

	// Verify count is at max
	count := store.GetBoundCount(circleID)
	if count != domaindeviceidentity.MaxDevicesPerCircle {
		t.Errorf("BoundCount = %d, want %d", count, domaindeviceidentity.MaxDevicesPerCircle)
	}

	// 6th device should fail
	fp6 := domaindeviceidentity.Fingerprint(strings.Repeat("b", 64))
	result, err := store.Bind(circleID, fp6)
	if err != nil {
		t.Fatalf("Bind 6 error: %v", err)
	}
	if result.Success {
		t.Errorf("Bind 6 should fail (max devices reached)")
	}
	if !result.AtMaxLimit {
		t.Errorf("AtMaxLimit should be true")
	}
}

// TestCircleBindingIdempotent verifies binding is idempotent.
func TestCircleBindingIdempotent(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	store := persist.NewCircleBindingStore(testClock(now), nil)

	circleID := "test-circle"
	fp := domaindeviceidentity.Fingerprint(strings.Repeat("a", 64))

	// First bind
	result1, _ := store.Bind(circleID, fp)
	if !result1.Success {
		t.Fatalf("First bind should succeed")
	}

	// Second bind (same fingerprint) should be idempotent
	result2, _ := store.Bind(circleID, fp)
	if !result2.Success {
		t.Fatalf("Second bind should succeed (idempotent)")
	}

	// Count should still be 1
	count := store.GetBoundCount(circleID)
	if count != 1 {
		t.Errorf("BoundCount = %d, want 1 (idempotent)", count)
	}
}

// TestReplayBundleFormat verifies replay bundle format constraints.
func TestReplayBundleFormat(t *testing.T) {
	header := domainreplay.ReplayBundleHeader{
		Version:        domainreplay.BundleVersion,
		CircleIDHash:   "abc123",
		PeriodKey:      "2025-01-15T10:30",
		RecordCount:    3,
		EarliestPeriod: "2025-01-01",
		LatestPeriod:   "2025-01-15",
	}

	canonical := header.CanonicalString()

	// Should be pipe-delimited
	parts := strings.Split(canonical, "|")
	if len(parts) != 6 {
		t.Errorf("Header should have 6 pipe-delimited parts, got %d", len(parts))
	}

	// Should start with version
	if parts[0] != domainreplay.BundleVersion {
		t.Errorf("First part should be version %q, got %q", domainreplay.BundleVersion, parts[0])
	}

	// Should NOT contain JSON
	if strings.Contains(canonical, "{") || strings.Contains(canonical, "}") {
		t.Errorf("CanonicalString should not contain JSON")
	}
}

// TestReplayBundleHashDeterminism verifies bundle hash is deterministic.
func TestReplayBundleHashDeterminism(t *testing.T) {
	bundle1 := &domainreplay.ReplayBundle{
		Header: domainreplay.ReplayBundleHeader{
			Version:        domainreplay.BundleVersion,
			CircleIDHash:   "abc123",
			PeriodKey:      "2025-01-15T10:30",
			RecordCount:    1,
			EarliestPeriod: "2025-01-15",
			LatestPeriod:   "2025-01-15",
		},
		Records: []domainreplay.CanonicalRecordLine{
			{RecordType: "TEST", RecordHash: "hash1", PeriodBucket: "2025-01-15", PayloadHash: "payload1"},
		},
	}

	bundle2 := &domainreplay.ReplayBundle{
		Header: domainreplay.ReplayBundleHeader{
			Version:        domainreplay.BundleVersion,
			CircleIDHash:   "abc123",
			PeriodKey:      "2025-01-15T10:30",
			RecordCount:    1,
			EarliestPeriod: "2025-01-15",
			LatestPeriod:   "2025-01-15",
		},
		Records: []domainreplay.CanonicalRecordLine{
			{RecordType: "TEST", RecordHash: "hash1", PeriodBucket: "2025-01-15", PayloadHash: "payload1"},
		},
	}

	hash1 := bundle1.ComputeBundleHash()
	hash2 := bundle2.ComputeBundleHash()

	if hash1 != hash2 {
		t.Errorf("BundleHash not deterministic: %s != %s", hash1, hash2)
	}

	// Different content should produce different hash
	bundle2.Records[0].RecordHash = "different"
	hash3 := bundle2.ComputeBundleHash()
	if hash1 == hash3 {
		t.Errorf("Different content should produce different hash")
	}
}

// TestSafeRecordTypes verifies safe record type whitelist.
func TestSafeRecordTypes(t *testing.T) {
	// These types should be safe
	safeTypes := []string{
		"SHADOWLLM_RECEIPT",
		"SHADOW_DIFF",
		"SHADOW_CALIBRATION",
		"REALITY_ACK",
		"JOURNEY_DISMISSAL",
		"FIRST_MINUTES_SUMMARY",
	}

	for _, rt := range safeTypes {
		if !domainreplay.IsSafeForExport(rt) {
			t.Errorf("Record type %q should be safe for export", rt)
		}
	}

	// These types should NOT be safe (contain raw data)
	unsafeTypes := []string{
		"EVENT",
		"DRAFT",
		"APPROVAL",
		"RANDOM_TYPE",
	}

	for _, rt := range unsafeTypes {
		if domainreplay.IsSafeForExport(rt) {
			t.Errorf("Record type %q should NOT be safe for export", rt)
		}
	}
}

// TestForbiddenPatternCheck verifies forbidden pattern detection.
func TestForbiddenPatternCheck(t *testing.T) {
	// These should be detected as forbidden
	forbiddenInputs := []string{
		"test@example.com",
		"https://example.com",
		"user@domain.org",
	}

	for _, input := range forbiddenInputs {
		found, _ := domainreplay.ContainsForbiddenPattern(input)
		if !found {
			t.Errorf("Should detect forbidden pattern in %q", input)
		}
	}

	// These should be allowed
	allowedInputs := []string{
		"v1|record|abc123|def456",
		"2025-01-15T10:30",
		"hash:abcdef123456",
	}

	for _, input := range allowedInputs {
		found, pattern := domainreplay.ContainsForbiddenPattern(input)
		if found {
			t.Errorf("Should not detect forbidden pattern in %q, found %q", input, pattern)
		}
	}
}

// TestIdentityEngineFlow verifies identity engine flow.
func TestIdentityEngineFlow(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "test-device-key")

	keyStore := persist.NewDeviceKeyStore(keyPath)
	bindingStore := persist.NewCircleBindingStore(testClock(now), nil)
	engine := deviceidentity.NewEngine(testClock(now), keyStore, bindingStore)

	circleID := "test-circle"

	// First ensure device identity (must be called before other operations)
	_, fp, err := engine.EnsureDeviceIdentity()
	if err != nil {
		t.Fatalf("EnsureDeviceIdentity failed: %v", err)
	}
	if fp == "" {
		t.Errorf("Fingerprint should not be empty")
	}

	// Initially not bound
	isBound, err := engine.IsBoundToCircle(circleID)
	if err != nil {
		t.Fatalf("IsBoundToCircle failed: %v", err)
	}
	if isBound {
		t.Errorf("Should not be bound initially")
	}

	// Bind to circle
	result, err := engine.BindToCircle(circleID)
	if err != nil {
		t.Fatalf("BindToCircle failed: %v", err)
	}
	if !result.Success {
		t.Errorf("BindToCircle should succeed: %s", result.Error)
	}

	// Now should be bound
	isBound, _ = engine.IsBoundToCircle(circleID)
	if !isBound {
		t.Errorf("Should be bound after binding")
	}

	// Build identity page
	page, err := engine.BuildIdentityPage(circleID)
	if err != nil {
		t.Fatalf("BuildIdentityPage failed: %v", err)
	}
	if !page.IsBound {
		t.Errorf("Page.IsBound should be true")
	}
	if page.FingerprintShort == "" {
		t.Errorf("Page.FingerprintShort should not be empty")
	}
}

// TestReplayEngineEmptyBundle verifies empty bundle generation.
func TestReplayEngineEmptyBundle(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	// Create mock storelog that returns empty
	mockLog := &mockStorelog{}
	engine := replay.NewEngine(testClock(now), mockLog)

	result, err := engine.BuildBundle("test-circle", 30)
	if err != nil {
		t.Fatalf("BuildBundle failed: %v", err)
	}

	if !result.Success {
		t.Errorf("BuildBundle should succeed for empty log: %s", result.Error)
	}

	if result.Bundle == nil {
		t.Fatalf("Bundle should not be nil")
	}

	if result.Bundle.Header.RecordCount != 0 {
		t.Errorf("RecordCount = %d, want 0", result.Bundle.Header.RecordCount)
	}

	if result.BundleText == "" {
		t.Errorf("BundleText should not be empty")
	}
}

// mockStorelog is a minimal storelog for testing.
type mockStorelog struct{}

func (m *mockStorelog) Append(record *storelog.LogRecord) error { return nil }
func (m *mockStorelog) Contains(hash string) bool               { return false }
func (m *mockStorelog) Get(hash string) (*storelog.LogRecord, error) {
	return nil, nil
}
func (m *mockStorelog) List() ([]*storelog.LogRecord, error) { return nil, nil }
func (m *mockStorelog) ListByType(recordType string) ([]*storelog.LogRecord, error) {
	return nil, nil
}
func (m *mockStorelog) ListByCircle(circleID identity.EntityID) ([]*storelog.LogRecord, error) {
	return nil, nil
}
func (m *mockStorelog) Count() int    { return 0 }
func (m *mockStorelog) Verify() error { return nil }
func (m *mockStorelog) Flush() error  { return nil }
