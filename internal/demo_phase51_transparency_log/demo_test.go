// Package demo_phase51_transparency_log contains tests for Phase 51: Transparency Log / Claim Ledger.
//
// These tests verify the Phase 51 invariants:
// - NO POWER: Observation/proof only, no runtime behavior changes
// - HASH-ONLY: No raw identifiers stored or rendered
// - APPEND-ONLY: Dedup/idempotence but no mutation/deletion
// - DETERMINISTIC: Same inputs = same outputs
// - PIPE-DELIMITED: Canonical strings use pipe format, not JSON
//
// Reference: docs/ADR/ADR-0089-phase51-transparency-log-claim-ledger.md
package demo_phase51_transparency_log

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"quantumlife/internal/persist"
	"quantumlife/internal/transparencylog"
	domain "quantumlife/pkg/domain/transparencylog"
)

// ============================================================================
// Test Data
// ============================================================================

const (
	testPeriod    = domain.PeriodKey("2025-W03")
	testKeyFP     = domain.KeyFingerprint("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	testKeyFP2    = domain.KeyFingerprint("fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210")
	testRefHash   = domain.RefHash("abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789")
	testRefHash2  = domain.RefHash("9876543210abcdef9876543210abcdef9876543210abcdef9876543210abcdef")
)

func testClock() time.Time {
	return time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
}

func makeTestEntry(kind domain.LogKind, prov domain.LogProvenanceBucket, keyfp domain.KeyFingerprint, ref domain.RefHash) domain.TransparencyLogEntry {
	entry := domain.TransparencyLogEntry{
		EntryID:       domain.LogEntryID("test-entry"),
		Period:        testPeriod,
		Kind:          kind,
		Provenance:    prov,
		KeyFP:         keyfp,
		RefHash:       ref,
		CreatedBucket: "na",
	}
	entry.LineHash = entry.ComputeLineHash()
	entry.EntryID = domain.LogEntryID(string(entry.LineHash)[:32])
	return entry
}

// ============================================================================
// Canonical Line Format Tests
// ============================================================================

func TestCanonicalLineFormat_PipeDelimited(t *testing.T) {
	entry := makeTestEntry(domain.LogSignedVendorClaim, domain.ProvUserSupplied, testKeyFP, testRefHash)
	line := entry.CanonicalLine()

	if !strings.Contains(line, "|") {
		t.Error("canonical line must be pipe-delimited")
	}
	if strings.Contains(line, "{") || strings.Contains(line, "}") {
		t.Error("canonical line must NOT use JSON format")
	}
	if strings.Contains(line, ":") && strings.Contains(line, "\"") {
		t.Error("canonical line must NOT use JSON-like colon-quote patterns")
	}
}

func TestCanonicalLineFormat_StartsWithV1(t *testing.T) {
	entry := makeTestEntry(domain.LogSignedVendorClaim, domain.ProvUserSupplied, testKeyFP, testRefHash)
	line := entry.CanonicalLine()

	if !strings.HasPrefix(line, "v1|") {
		t.Errorf("canonical line must start with 'v1|', got: %s", line)
	}
}

func TestCanonicalLineFormat_ContainsAllFields(t *testing.T) {
	entry := makeTestEntry(domain.LogSignedVendorClaim, domain.ProvUserSupplied, testKeyFP, testRefHash)
	line := entry.CanonicalLine()

	requiredParts := []string{
		"period=" + string(testPeriod),
		"kind=" + string(domain.LogSignedVendorClaim),
		"prov=" + string(domain.ProvUserSupplied),
		"keyfp=" + string(testKeyFP),
		"ref=" + string(testRefHash),
		"created=na",
	}

	for _, part := range requiredParts {
		if !strings.Contains(line, part) {
			t.Errorf("canonical line missing required part %q, line: %s", part, line)
		}
	}
}

func TestCanonicalLineFormat_StableAcrossCalls(t *testing.T) {
	entry := makeTestEntry(domain.LogSignedVendorClaim, domain.ProvUserSupplied, testKeyFP, testRefHash)

	line1 := entry.CanonicalLine()
	line2 := entry.CanonicalLine()
	line3 := entry.CanonicalLine()

	if line1 != line2 || line2 != line3 {
		t.Errorf("canonical line not stable: %q, %q, %q", line1, line2, line3)
	}
}

// ============================================================================
// Line Hash Tests
// ============================================================================

func TestLineHash_StableGivenSameInput(t *testing.T) {
	entry1 := makeTestEntry(domain.LogSignedVendorClaim, domain.ProvUserSupplied, testKeyFP, testRefHash)
	entry2 := makeTestEntry(domain.LogSignedVendorClaim, domain.ProvUserSupplied, testKeyFP, testRefHash)

	hash1 := entry1.ComputeLineHash()
	hash2 := entry2.ComputeLineHash()

	if hash1 != hash2 {
		t.Errorf("line hash not stable for same input: %s != %s", hash1, hash2)
	}
}

func TestLineHash_DifferentInputsDifferentHash(t *testing.T) {
	entry1 := makeTestEntry(domain.LogSignedVendorClaim, domain.ProvUserSupplied, testKeyFP, testRefHash)
	entry2 := makeTestEntry(domain.LogSignedVendorClaim, domain.ProvUserSupplied, testKeyFP, testRefHash2)

	hash1 := entry1.ComputeLineHash()
	hash2 := entry2.ComputeLineHash()

	if hash1 == hash2 {
		t.Error("different inputs should produce different hashes")
	}
}

func TestLineHash_Is64HexChars(t *testing.T) {
	entry := makeTestEntry(domain.LogSignedVendorClaim, domain.ProvUserSupplied, testKeyFP, testRefHash)
	hash := entry.ComputeLineHash()

	if len(hash) != 64 {
		t.Errorf("line hash must be 64 chars, got %d", len(hash))
	}
	if err := hash.Validate(); err != nil {
		t.Errorf("line hash validation failed: %v", err)
	}
}

func TestLineHash_LowercaseHex(t *testing.T) {
	entry := makeTestEntry(domain.LogSignedVendorClaim, domain.ProvUserSupplied, testKeyFP, testRefHash)
	hash := string(entry.ComputeLineHash())

	for _, c := range hash {
		if c >= 'A' && c <= 'F' {
			t.Errorf("line hash must be lowercase hex, found uppercase: %c", c)
		}
	}
}

// ============================================================================
// Ordering Tests
// ============================================================================

func TestOrdering_DeterministicRegardlessOfInputOrder(t *testing.T) {
	engine := transparencylog.NewEngine()

	// Create inputs in one order
	inputs1 := transparencylog.TransparencyInputs{
		SignedClaimRefs: []transparencylog.SignedRef{
			{RefHash: string(testRefHash2), KeyFP: string(testKeyFP), Provenance: "user_supplied"},
			{RefHash: string(testRefHash), KeyFP: string(testKeyFP2), Provenance: "marketplace"},
		},
	}

	// Same inputs in reverse order
	inputs2 := transparencylog.TransparencyInputs{
		SignedClaimRefs: []transparencylog.SignedRef{
			{RefHash: string(testRefHash), KeyFP: string(testKeyFP2), Provenance: "marketplace"},
			{RefHash: string(testRefHash2), KeyFP: string(testKeyFP), Provenance: "user_supplied"},
		},
	}

	entries1, err := engine.BuildEntries(testPeriod, inputs1)
	if err != nil {
		t.Fatalf("BuildEntries failed: %v", err)
	}

	entries2, err := engine.BuildEntries(testPeriod, inputs2)
	if err != nil {
		t.Fatalf("BuildEntries failed: %v", err)
	}

	if len(entries1) != len(entries2) {
		t.Fatalf("entry counts differ: %d != %d", len(entries1), len(entries2))
	}

	for i := range entries1 {
		if entries1[i].LineHash != entries2[i].LineHash {
			t.Errorf("entry %d hash differs despite deterministic ordering: %s != %s",
				i, entries1[i].LineHash, entries2[i].LineHash)
		}
	}
}

func TestOrdering_SortsByKindThenProvenanceThenKeyFP(t *testing.T) {
	engine := transparencylog.NewEngine()

	inputs := transparencylog.TransparencyInputs{
		SignedClaimRefs: []transparencylog.SignedRef{
			{RefHash: string(testRefHash), KeyFP: string(testKeyFP2), Provenance: "marketplace"},
			{RefHash: string(testRefHash), KeyFP: string(testKeyFP), Provenance: "user_supplied"},
		},
		SignedManifestRefs: []transparencylog.SignedRef{
			{RefHash: string(testRefHash), KeyFP: string(testKeyFP), Provenance: "unknown"},
		},
	}

	entries, err := engine.BuildEntries(testPeriod, inputs)
	if err != nil {
		t.Fatalf("BuildEntries failed: %v", err)
	}

	// Verify sorted by kind first (pack manifest < vendor claim alphabetically)
	for i := 1; i < len(entries); i++ {
		prev, curr := entries[i-1], entries[i]
		if string(prev.Kind) > string(curr.Kind) {
			t.Errorf("entries not sorted by kind at %d", i)
		}
		if prev.Kind == curr.Kind && string(prev.Provenance) > string(curr.Provenance) {
			t.Errorf("entries with same kind not sorted by provenance at %d", i)
		}
	}
}

// ============================================================================
// Export Bundle Tests
// ============================================================================

func TestExportBundle_HashDeterministic(t *testing.T) {
	engine := transparencylog.NewEngine()

	inputs := transparencylog.TransparencyInputs{
		SignedClaimRefs: []transparencylog.SignedRef{
			{RefHash: string(testRefHash), KeyFP: string(testKeyFP), Provenance: "user_supplied"},
		},
	}

	entries, _ := engine.BuildEntries(testPeriod, inputs)

	bundle1 := engine.BuildExportBundle(testPeriod, entries)
	bundle2 := engine.BuildExportBundle(testPeriod, entries)

	if bundle1.BundleHash != bundle2.BundleHash {
		t.Errorf("bundle hash not deterministic: %s != %s", bundle1.BundleHash, bundle2.BundleHash)
	}
}

func TestExportBundle_ToCanonicalFormat_TextPlain(t *testing.T) {
	engine := transparencylog.NewEngine()

	inputs := transparencylog.TransparencyInputs{
		SignedClaimRefs: []transparencylog.SignedRef{
			{RefHash: string(testRefHash), KeyFP: string(testKeyFP), Provenance: "user_supplied"},
		},
	}

	entries, _ := engine.BuildEntries(testPeriod, inputs)
	bundle := engine.BuildExportBundle(testPeriod, entries)
	text := bundle.ToCanonicalFormat()

	// Should be text/plain, not JSON
	if strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[") {
		t.Error("export bundle must be text/plain, not JSON")
	}
	if !strings.HasPrefix(text, "bundle|v1|") {
		t.Errorf("export bundle must start with 'bundle|v1|', got: %s", text[:min(50, len(text))])
	}
}

func TestExportBundle_ContainsPeriod(t *testing.T) {
	engine := transparencylog.NewEngine()

	entries := []domain.TransparencyLogEntry{
		makeTestEntry(domain.LogSignedVendorClaim, domain.ProvUserSupplied, testKeyFP, testRefHash),
	}

	bundle := engine.BuildExportBundle(testPeriod, entries)

	if bundle.Period != testPeriod {
		t.Errorf("bundle period mismatch: %s != %s", bundle.Period, testPeriod)
	}
}

func TestExportBundle_RoundTrip(t *testing.T) {
	engine := transparencylog.NewEngine()

	inputs := transparencylog.TransparencyInputs{
		SignedClaimRefs: []transparencylog.SignedRef{
			{RefHash: string(testRefHash), KeyFP: string(testKeyFP), Provenance: "user_supplied"},
		},
	}

	entries, _ := engine.BuildEntries(testPeriod, inputs)
	bundle := engine.BuildExportBundle(testPeriod, entries)
	text := bundle.ToCanonicalFormat()

	parsedBundle, err := domain.ParseExportBundle(text)
	if err != nil {
		t.Fatalf("ParseExportBundle failed: %v", err)
	}

	if parsedBundle.Period != bundle.Period {
		t.Errorf("round-trip period mismatch: %s != %s", parsedBundle.Period, bundle.Period)
	}
	if len(parsedBundle.Lines) != len(bundle.Lines) {
		t.Errorf("round-trip lines count mismatch: %d != %d", len(parsedBundle.Lines), len(bundle.Lines))
	}
}

// ============================================================================
// Import Tests
// ============================================================================

func TestImport_RejectsForbiddenPatterns(t *testing.T) {
	forbiddenInputs := []string{
		"v1|period=2025-W03|kind=log_signed_vendor_claim|prov=prov_user_supplied|keyfp=" + string(testKeyFP) + "|ref=" + string(testRefHash) + "|created=test@example.com",
		"v1|period=2025-W03|kind=log_signed_vendor_claim|prov=prov_user_supplied|keyfp=" + string(testKeyFP) + "|ref=" + string(testRefHash) + "|created=https://evil.com",
		"v1|period=2025-W03|kind=log_signed_vendor_claim|prov=prov_user_supplied|keyfp=" + string(testKeyFP) + "|ref=" + string(testRefHash) + "|created=vendorID:123",
	}

	for _, line := range forbiddenInputs {
		err := domain.ValidateNoForbiddenPatterns(line)
		if err == nil {
			t.Errorf("should reject forbidden pattern in: %s", line)
		}
	}
}

func TestImport_RejectsInvalidBundleHash(t *testing.T) {
	engine := transparencylog.NewEngine()

	bundle := domain.TransparencyLogExportBundle{
		Version:    "v1",
		Period:     testPeriod,
		Lines:      []string{"v1|period=2025-W03|kind=log_signed_vendor_claim|prov=prov_user_supplied|keyfp=" + string(testKeyFP) + "|ref=" + string(testRefHash) + "|created=na"},
		BundleHash: "invalid_hash",
	}

	err := engine.ValidateExportBundle(bundle)
	if err == nil {
		t.Error("should reject bundle with invalid hash")
	}
}

func TestImport_IdempotentByLineHash(t *testing.T) {
	store := persist.NewTransparencyLogStore(testClock)

	entry := makeTestEntry(domain.LogSignedVendorClaim, domain.ProvUserSupplied, testKeyFP, testRefHash)

	// First append should succeed
	wasNew1, err := store.Append(entry)
	if err != nil {
		t.Fatalf("first append failed: %v", err)
	}
	if !wasNew1 {
		t.Error("first append should report as new")
	}

	// Second append of same entry should be idempotent
	wasNew2, err := store.Append(entry)
	if err != nil {
		t.Fatalf("second append failed: %v", err)
	}
	if wasNew2 {
		t.Error("second append of same entry should not report as new")
	}

	// Count should be 1, not 2
	if store.Count() != 1 {
		t.Errorf("store should have 1 entry, got %d", store.Count())
	}
}

func TestImport_DedupByPeriodAndLineHash(t *testing.T) {
	store := persist.NewTransparencyLogStore(testClock)

	entry1 := makeTestEntry(domain.LogSignedVendorClaim, domain.ProvUserSupplied, testKeyFP, testRefHash)

	// Same content but different period
	entry2 := makeTestEntry(domain.LogSignedVendorClaim, domain.ProvUserSupplied, testKeyFP, testRefHash)
	entry2.Period = domain.PeriodKey("2025-W04")
	entry2.LineHash = entry2.ComputeLineHash()

	store.Append(entry1)
	store.Append(entry2)

	// Should have 2 entries (different periods)
	if store.Count() != 2 {
		t.Errorf("store should have 2 entries (different periods), got %d", store.Count())
	}
}

// ============================================================================
// Retention Tests
// ============================================================================

func TestRetention_EvictsWhenAtCapacity(t *testing.T) {
	store := persist.NewTransparencyLogStore(testClock)

	// Add MaxEntries + 1 entries
	for i := 0; i <= persist.TransparencyLogMaxEntries; i++ {
		// Generate unique hash based on i using fmt.Sprintf
		hashStr := fmt.Sprintf("%064x", i)
		refHash := domain.RefHash(hashStr)

		entry := makeTestEntry(domain.LogSignedVendorClaim, domain.ProvUserSupplied, testKeyFP, refHash)
		store.Append(entry)
	}

	// Should not exceed max
	if store.Count() > persist.TransparencyLogMaxEntries {
		t.Errorf("store should not exceed max entries (%d), got %d",
			persist.TransparencyLogMaxEntries, store.Count())
	}
}

// ============================================================================
// Page Tests
// ============================================================================

func TestPage_ShowsNothingSummaryWhenEmpty(t *testing.T) {
	engine := transparencylog.NewEngine()

	page := engine.BuildPage(testPeriod, []domain.TransparencyLogEntry{})

	if page.Summary.TotalBucket != domain.MagnitudeNothing {
		t.Errorf("empty page should show 'nothing' magnitude, got %s", page.Summary.TotalBucket)
	}
}

func TestPage_ShowsAFewFor1To3Entries(t *testing.T) {
	engine := transparencylog.NewEngine()

	entries := []domain.TransparencyLogEntry{
		makeTestEntry(domain.LogSignedVendorClaim, domain.ProvUserSupplied, testKeyFP, testRefHash),
	}

	page := engine.BuildPage(testPeriod, entries)

	if page.Summary.TotalBucket != domain.MagnitudeAFew {
		t.Errorf("1 entry should show 'a_few' magnitude, got %s", page.Summary.TotalBucket)
	}
}

func TestPage_ShowsSeveralFor4Plus(t *testing.T) {
	engine := transparencylog.NewEngine()

	var entries []domain.TransparencyLogEntry
	for i := 0; i < 5; i++ {
		hashStr := fmt.Sprintf("%064x", i)
		refHash := domain.RefHash(hashStr)
		entries = append(entries, makeTestEntry(domain.LogSignedVendorClaim, domain.ProvUserSupplied, testKeyFP, refHash))
	}

	page := engine.BuildPage(testPeriod, entries)

	if page.Summary.TotalBucket != domain.MagnitudeSeveral {
		t.Errorf("5 entries should show 'several' magnitude, got %s", page.Summary.TotalBucket)
	}
}

func TestPage_Max12LinesDisplayed(t *testing.T) {
	engine := transparencylog.NewEngine()

	var entries []domain.TransparencyLogEntry
	for i := 0; i < 20; i++ {
		hashStr := fmt.Sprintf("%064x", i)
		refHash := domain.RefHash(hashStr)
		entries = append(entries, makeTestEntry(domain.LogSignedVendorClaim, domain.ProvUserSupplied, testKeyFP, refHash))
	}

	page := engine.BuildPage(testPeriod, entries)

	if len(page.Lines) > transparencylog.MaxDisplayLines {
		t.Errorf("page should show max %d lines, got %d", transparencylog.MaxDisplayLines, len(page.Lines))
	}
	if page.TotalCount != 20 {
		t.Errorf("total count should be 20, got %d", page.TotalCount)
	}
}

func TestPage_HasStatusHash(t *testing.T) {
	engine := transparencylog.NewEngine()

	entries := []domain.TransparencyLogEntry{
		makeTestEntry(domain.LogSignedVendorClaim, domain.ProvUserSupplied, testKeyFP, testRefHash),
	}

	page := engine.BuildPage(testPeriod, entries)

	if page.StatusHash == "" {
		t.Error("page should have a status hash")
	}
	if len(page.StatusHash) != 64 {
		t.Errorf("status hash should be 64 chars, got %d", len(page.StatusHash))
	}
}

func TestPage_StatusHashStable(t *testing.T) {
	engine := transparencylog.NewEngine()

	entries := []domain.TransparencyLogEntry{
		makeTestEntry(domain.LogSignedVendorClaim, domain.ProvUserSupplied, testKeyFP, testRefHash),
	}

	page1 := engine.BuildPage(testPeriod, entries)
	page2 := engine.BuildPage(testPeriod, entries)

	if page1.StatusHash != page2.StatusHash {
		t.Errorf("status hash not stable: %s != %s", page1.StatusHash, page2.StatusHash)
	}
}

// ============================================================================
// Store Tests
// ============================================================================

func TestStore_AppendOnlyNeverMutates(t *testing.T) {
	store := persist.NewTransparencyLogStore(testClock)

	entry := makeTestEntry(domain.LogSignedVendorClaim, domain.ProvUserSupplied, testKeyFP, testRefHash)
	store.Append(entry)

	// Get all entries
	entries1, _ := store.ReplayAll()

	// Append another
	entry2 := makeTestEntry(domain.LogSignedVendorClaim, domain.ProvUserSupplied, testKeyFP, testRefHash2)
	store.Append(entry2)

	entries2, _ := store.ReplayAll()

	// First entry should still be the same
	if len(entries2) < 1 || entries2[0].LineHash != entries1[0].LineHash {
		t.Error("existing entries should not be mutated")
	}
}

func TestStore_ListByPeriodFilters(t *testing.T) {
	store := persist.NewTransparencyLogStore(testClock)

	entry1 := makeTestEntry(domain.LogSignedVendorClaim, domain.ProvUserSupplied, testKeyFP, testRefHash)
	entry1.Period = domain.PeriodKey("2025-W03")
	entry1.LineHash = entry1.ComputeLineHash()

	entry2 := makeTestEntry(domain.LogSignedVendorClaim, domain.ProvUserSupplied, testKeyFP, testRefHash2)
	entry2.Period = domain.PeriodKey("2025-W04")
	entry2.LineHash = entry2.ComputeLineHash()

	store.Append(entry1)
	store.Append(entry2)

	w03Entries, _ := store.ListByPeriod(domain.PeriodKey("2025-W03"))
	w04Entries, _ := store.ListByPeriod(domain.PeriodKey("2025-W04"))

	if len(w03Entries) != 1 {
		t.Errorf("W03 should have 1 entry, got %d", len(w03Entries))
	}
	if len(w04Entries) != 1 {
		t.Errorf("W04 should have 1 entry, got %d", len(w04Entries))
	}
}

func TestStore_IsEntrySeenWorks(t *testing.T) {
	store := persist.NewTransparencyLogStore(testClock)

	entry := makeTestEntry(domain.LogSignedVendorClaim, domain.ProvUserSupplied, testKeyFP, testRefHash)

	// Before append
	if store.IsEntrySeen(entry.Period, entry.LineHash) {
		t.Error("entry should not be seen before append")
	}

	store.Append(entry)

	// After append
	if !store.IsEntrySeen(entry.Period, entry.LineHash) {
		t.Error("entry should be seen after append")
	}
}

func TestStore_AppendBatchReturnsNewCount(t *testing.T) {
	store := persist.NewTransparencyLogStore(testClock)

	entry1 := makeTestEntry(domain.LogSignedVendorClaim, domain.ProvUserSupplied, testKeyFP, testRefHash)
	entry2 := makeTestEntry(domain.LogSignedVendorClaim, domain.ProvUserSupplied, testKeyFP, testRefHash2)

	newCount, err := store.AppendBatch([]domain.TransparencyLogEntry{entry1, entry2})
	if err != nil {
		t.Fatalf("AppendBatch failed: %v", err)
	}
	if newCount != 2 {
		t.Errorf("AppendBatch should return 2 new entries, got %d", newCount)
	}

	// Append same batch again
	newCount2, _ := store.AppendBatch([]domain.TransparencyLogEntry{entry1, entry2})
	if newCount2 != 0 {
		t.Errorf("AppendBatch of duplicates should return 0 new, got %d", newCount2)
	}
}

// ============================================================================
// Validation Tests
// ============================================================================

func TestValidation_EntryRequiresAllFields(t *testing.T) {
	invalid := domain.TransparencyLogEntry{}
	if err := invalid.Validate(); err == nil {
		t.Error("empty entry should fail validation")
	}
}

func TestValidation_KeyFingerprintMustBe64Hex(t *testing.T) {
	shortFP := domain.KeyFingerprint("abc")
	if err := shortFP.Validate(); err == nil {
		t.Error("short fingerprint should fail validation")
	}

	uppercaseFP := domain.KeyFingerprint("0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF")
	if err := uppercaseFP.Validate(); err == nil {
		t.Error("uppercase fingerprint should fail validation")
	}
}

func TestValidation_PeriodKeyCannotContainPipe(t *testing.T) {
	badPeriod := domain.PeriodKey("2025|W03")
	if err := badPeriod.Validate(); err == nil {
		t.Error("period with pipe should fail validation")
	}
}

func TestValidation_RefHashMustBe64Hex(t *testing.T) {
	shortHash := domain.RefHash("abc")
	if err := shortHash.Validate(); err == nil {
		t.Error("short hash should fail validation")
	}
}

// ============================================================================
// Enum Tests
// ============================================================================

func TestEnum_LogKindValidation(t *testing.T) {
	if err := domain.LogSignedVendorClaim.Validate(); err != nil {
		t.Errorf("LogSignedVendorClaim should be valid: %v", err)
	}
	if err := domain.LogSignedPackManifest.Validate(); err != nil {
		t.Errorf("LogSignedPackManifest should be valid: %v", err)
	}

	invalid := domain.LogKind("invalid_kind")
	if err := invalid.Validate(); err == nil {
		t.Error("invalid kind should fail validation")
	}
}

func TestEnum_LogProvenanceBucketValidation(t *testing.T) {
	if err := domain.ProvUserSupplied.Validate(); err != nil {
		t.Errorf("ProvUserSupplied should be valid: %v", err)
	}
	if err := domain.ProvMarketplace.Validate(); err != nil {
		t.Errorf("ProvMarketplace should be valid: %v", err)
	}
	if err := domain.ProvUnknown.Validate(); err != nil {
		t.Errorf("ProvUnknown should be valid: %v", err)
	}

	invalid := domain.LogProvenanceBucket("invalid_prov")
	if err := invalid.Validate(); err == nil {
		t.Error("invalid provenance should fail validation")
	}
}

func TestEnum_MagnitudeBucketValidation(t *testing.T) {
	if err := domain.MagnitudeNothing.Validate(); err != nil {
		t.Errorf("MagnitudeNothing should be valid: %v", err)
	}
	if err := domain.MagnitudeAFew.Validate(); err != nil {
		t.Errorf("MagnitudeAFew should be valid: %v", err)
	}
	if err := domain.MagnitudeSeveral.Validate(); err != nil {
		t.Errorf("MagnitudeSeveral should be valid: %v", err)
	}
}

func TestEnum_ComputeMagnitudeBucket(t *testing.T) {
	tests := []struct {
		count    int
		expected domain.MagnitudeBucket
	}{
		{0, domain.MagnitudeNothing},
		{1, domain.MagnitudeAFew},
		{2, domain.MagnitudeAFew},
		{3, domain.MagnitudeAFew},
		{4, domain.MagnitudeSeveral},
		{12, domain.MagnitudeSeveral},
		{100, domain.MagnitudeSeveral},
	}

	for _, tc := range tests {
		got := domain.ComputeMagnitudeBucket(tc.count)
		if got != tc.expected {
			t.Errorf("ComputeMagnitudeBucket(%d) = %s, want %s", tc.count, got, tc.expected)
		}
	}
}

// ============================================================================
// Parse Tests
// ============================================================================

func TestParse_CanonicalLineRoundTrip(t *testing.T) {
	original := makeTestEntry(domain.LogSignedVendorClaim, domain.ProvUserSupplied, testKeyFP, testRefHash)
	line := original.CanonicalLine()

	parsed, err := domain.ParseCanonicalLine(line)
	if err != nil {
		t.Fatalf("ParseCanonicalLine failed: %v", err)
	}

	if parsed.Period != original.Period {
		t.Errorf("period mismatch: %s != %s", parsed.Period, original.Period)
	}
	if parsed.Kind != original.Kind {
		t.Errorf("kind mismatch: %s != %s", parsed.Kind, original.Kind)
	}
	if parsed.Provenance != original.Provenance {
		t.Errorf("provenance mismatch: %s != %s", parsed.Provenance, original.Provenance)
	}
	if parsed.KeyFP != original.KeyFP {
		t.Errorf("keyfp mismatch: %s != %s", parsed.KeyFP, original.KeyFP)
	}
	if parsed.RefHash != original.RefHash {
		t.Errorf("refhash mismatch: %s != %s", parsed.RefHash, original.RefHash)
	}
}

func TestParse_RejectsInvalidVersion(t *testing.T) {
	_, err := domain.ParseCanonicalLine("v2|period=2025-W03|kind=log_signed_vendor_claim|prov=prov_user_supplied|keyfp=" + string(testKeyFP) + "|ref=" + string(testRefHash) + "|created=na")
	if err == nil {
		t.Error("should reject v2 version")
	}
}

func TestParse_RejectsInsufficientParts(t *testing.T) {
	_, err := domain.ParseCanonicalLine("v1|period=2025-W03")
	if err == nil {
		t.Error("should reject line with insufficient parts")
	}
}

// ============================================================================
// Summary Tests
// ============================================================================

func TestSummary_CanonicalStringStable(t *testing.T) {
	summary := domain.TransparencyLogSummary{
		TotalBucket:       domain.MagnitudeAFew,
		KindsPresent:      []domain.LogKind{domain.LogSignedVendorClaim, domain.LogSignedPackManifest},
		ProvenancePresent: []domain.LogProvenanceBucket{domain.ProvUserSupplied, domain.ProvMarketplace},
	}

	str1 := summary.CanonicalString()
	str2 := summary.CanonicalString()

	if str1 != str2 {
		t.Errorf("summary canonical string not stable: %s != %s", str1, str2)
	}
}

func TestSummary_SortsKindsAndProvs(t *testing.T) {
	summary := domain.TransparencyLogSummary{
		TotalBucket:       domain.MagnitudeAFew,
		KindsPresent:      []domain.LogKind{domain.LogSignedVendorClaim, domain.LogSignedPackManifest},
		ProvenancePresent: []domain.LogProvenanceBucket{domain.ProvMarketplace, domain.ProvUserSupplied},
	}

	str := summary.CanonicalString()

	// Should be sorted alphabetically
	if !strings.Contains(str, "kinds=log_signed_pack_manifest,log_signed_vendor_claim") {
		t.Errorf("kinds not sorted in canonical string: %s", str)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
