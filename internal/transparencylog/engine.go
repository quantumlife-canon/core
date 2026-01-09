// Package transparencylog provides the engine for Phase 51: Transparency Log.
//
// CRITICAL INVARIANTS:
// - NO POWER: This engine is observation/proof only. It MUST NOT change any
//   runtime behavior (no wiring into vendor caps, packs, pressure, interrupts).
// - PURE FUNCTIONS: All operations are deterministic.
// - CLOCK INJECTION: Time buckets are passed in, never uses time.Now() directly.
// - HASH-ONLY: Only produces hash-safe outputs.
// - NO NETWORK: No external calls. No storage. Pure transformation.
//
// Reference: docs/ADR/ADR-0089-phase51-transparency-log-claim-ledger.md
package transparencylog

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	domain "quantumlife/pkg/domain/transparencylog"
)

// MaxDisplayLines is the maximum number of lines to display in UI.
const MaxDisplayLines = 12

// ============================================================================
// Input Types
// ============================================================================

// SignedRef is a hash-only reference to a signed claim or manifest.
// This is the input format from Phase 50 stores.
type SignedRef struct {
	// RefHash is the claim hash or manifest hash.
	RefHash string
	// KeyFP is the key fingerprint of the signer.
	KeyFP string
	// Provenance is the provenance bucket string.
	Provenance string
}

// TransparencyInputs contains the inputs for building transparency log entries.
type TransparencyInputs struct {
	// SignedClaimRefs are references to signed vendor claims.
	SignedClaimRefs []SignedRef
	// SignedManifestRefs are references to signed pack manifests.
	SignedManifestRefs []SignedRef
}

// ============================================================================
// Engine
// ============================================================================

// Engine builds transparency log entries and pages.
// It is stateless and produces deterministic output.
type Engine struct{}

// NewEngine creates a new transparency log engine.
func NewEngine() *Engine {
	return &Engine{}
}

// ============================================================================
// Entry Building
// ============================================================================

// BuildEntries builds transparency log entries from inputs.
// Returns entries sorted in deterministic order.
func (e *Engine) BuildEntries(period domain.PeriodKey, inputs TransparencyInputs) ([]domain.TransparencyLogEntry, error) {
	if err := period.Validate(); err != nil {
		return nil, fmt.Errorf("invalid period: %w", err)
	}

	var entries []domain.TransparencyLogEntry

	// Build entries from signed claim refs
	for _, ref := range inputs.SignedClaimRefs {
		entry, err := e.buildEntryFromRef(period, domain.LogSignedVendorClaim, ref)
		if err != nil {
			continue // Skip invalid refs
		}
		entries = append(entries, entry)
	}

	// Build entries from signed manifest refs
	for _, ref := range inputs.SignedManifestRefs {
		entry, err := e.buildEntryFromRef(period, domain.LogSignedPackManifest, ref)
		if err != nil {
			continue // Skip invalid refs
		}
		entries = append(entries, entry)
	}

	// Sort deterministically
	sortEntries(entries)

	return entries, nil
}

// buildEntryFromRef builds a single entry from a signed reference.
func (e *Engine) buildEntryFromRef(period domain.PeriodKey, kind domain.LogKind, ref SignedRef) (domain.TransparencyLogEntry, error) {
	// Map provenance string to bucket
	provBucket := mapProvenanceToBucket(ref.Provenance)

	entry := domain.TransparencyLogEntry{
		Period:        period,
		Kind:          kind,
		Provenance:    provBucket,
		KeyFP:         domain.KeyFingerprint(ref.KeyFP),
		RefHash:       domain.RefHash(ref.RefHash),
		CreatedBucket: "na", // No timestamps in engine layer
	}

	// Validate the entry fields
	if err := entry.RefHash.Validate(); err != nil {
		return domain.TransparencyLogEntry{}, err
	}
	if err := entry.KeyFP.Validate(); err != nil {
		return domain.TransparencyLogEntry{}, err
	}

	// Compute line hash
	entry.LineHash = entry.ComputeLineHash()

	// Generate entry ID from line hash
	entry.EntryID = domain.LogEntryID(string(entry.LineHash)[:32])

	return entry, nil
}

// mapProvenanceToBucket maps a provenance string to a LogProvenanceBucket.
func mapProvenanceToBucket(prov string) domain.LogProvenanceBucket {
	lower := strings.ToLower(prov)
	switch {
	case strings.Contains(lower, "user"):
		return domain.ProvUserSupplied
	case strings.Contains(lower, "marketplace"):
		return domain.ProvMarketplace
	default:
		return domain.ProvUnknown
	}
}

// sortEntries sorts entries in deterministic order:
// Kind, then Provenance, then KeyFP, then RefHash, then LineHash
func sortEntries(entries []domain.TransparencyLogEntry) {
	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]

		// Sort by Kind first
		if a.Kind != b.Kind {
			return string(a.Kind) < string(b.Kind)
		}

		// Then by Provenance
		if a.Provenance != b.Provenance {
			return string(a.Provenance) < string(b.Provenance)
		}

		// Then by KeyFP
		if a.KeyFP != b.KeyFP {
			return string(a.KeyFP) < string(b.KeyFP)
		}

		// Then by RefHash
		if a.RefHash != b.RefHash {
			return string(a.RefHash) < string(b.RefHash)
		}

		// Finally by LineHash
		return string(a.LineHash) < string(b.LineHash)
	})
}

// ============================================================================
// Page Building
// ============================================================================

// BuildPage builds a transparency log page from entries.
func (e *Engine) BuildPage(period domain.PeriodKey, entries []domain.TransparencyLogEntry) domain.TransparencyLogPage {
	// Build line views (max 12 for UI)
	var lines []domain.TransparencyLogLineView
	for i, entry := range entries {
		if i >= MaxDisplayLines {
			break
		}
		lines = append(lines, domain.TransparencyLogLineView{
			Kind:       entry.Kind,
			Provenance: entry.Provenance,
			KeyFP:      entry.KeyFP,
			RefHash:    entry.RefHash,
			LineHash:   entry.LineHash,
		})
	}

	// Build summary
	summary := e.buildSummary(entries)

	// Build page
	page := domain.TransparencyLogPage{
		Title:      "Transparency, quietly.",
		Lines:      lines,
		Summary:    summary,
		Period:     period,
		TotalCount: len(entries),
	}

	// Compute status hash
	page.StatusHash = page.ComputeStatusHash()

	return page
}

// buildSummary builds a summary from entries.
func (e *Engine) buildSummary(entries []domain.TransparencyLogEntry) domain.TransparencyLogSummary {
	// Compute magnitude bucket
	totalBucket := domain.ComputeMagnitudeBucket(len(entries))

	// Collect unique kinds (max 2)
	kindSet := make(map[domain.LogKind]bool)
	for _, entry := range entries {
		kindSet[entry.Kind] = true
	}
	var kinds []domain.LogKind
	for k := range kindSet {
		kinds = append(kinds, k)
		if len(kinds) >= 2 {
			break
		}
	}
	// Sort for determinism
	sort.Slice(kinds, func(i, j int) bool {
		return string(kinds[i]) < string(kinds[j])
	})

	// Collect unique provenances (max 3)
	provSet := make(map[domain.LogProvenanceBucket]bool)
	for _, entry := range entries {
		provSet[entry.Provenance] = true
	}
	var provs []domain.LogProvenanceBucket
	for p := range provSet {
		provs = append(provs, p)
		if len(provs) >= 3 {
			break
		}
	}
	// Sort for determinism
	sort.Slice(provs, func(i, j int) bool {
		return string(provs[i]) < string(provs[j])
	})

	return domain.TransparencyLogSummary{
		TotalBucket:       totalBucket,
		KindsPresent:      kinds,
		ProvenancePresent: provs,
	}
}

// ============================================================================
// Export/Import
// ============================================================================

// BuildExportBundle builds an export bundle from entries.
func (e *Engine) BuildExportBundle(period domain.PeriodKey, entries []domain.TransparencyLogEntry) domain.TransparencyLogExportBundle {
	// Sort entries for deterministic export
	sortedEntries := make([]domain.TransparencyLogEntry, len(entries))
	copy(sortedEntries, entries)
	sortEntries(sortedEntries)

	// Build canonical lines
	var lines []string
	for _, entry := range sortedEntries {
		lines = append(lines, entry.CanonicalLine())
	}

	bundle := domain.TransparencyLogExportBundle{
		Version: "v1",
		Period:  period,
		Lines:   lines,
	}

	// Compute bundle hash
	bundle.BundleHash = bundle.ComputeBundleHash()

	return bundle
}

// ValidateExportBundle validates an export bundle.
func (e *Engine) ValidateExportBundle(bundle domain.TransparencyLogExportBundle) error {
	if err := bundle.Validate(); err != nil {
		return err
	}

	// Verify bundle hash
	expectedHash := bundle.ComputeBundleHash()
	if bundle.BundleHash != expectedHash {
		return fmt.Errorf("bundle hash mismatch: expected %s, got %s", expectedHash, bundle.BundleHash)
	}

	// Validate each line can be parsed
	for i, line := range bundle.Lines {
		_, err := domain.ParseCanonicalLine(line)
		if err != nil {
			return fmt.Errorf("invalid line %d: %w", i, err)
		}
	}

	return nil
}

// ImportBundle parses an export bundle and returns entries.
// Does not persist - that's the store layer's job.
func (e *Engine) ImportBundle(bundle domain.TransparencyLogExportBundle) ([]domain.TransparencyLogEntry, error) {
	if err := e.ValidateExportBundle(bundle); err != nil {
		return nil, err
	}

	var entries []domain.TransparencyLogEntry
	for _, line := range bundle.Lines {
		entry, err := domain.ParseCanonicalLine(line)
		if err != nil {
			return nil, err
		}
		// Generate entry ID
		entry.EntryID = domain.LogEntryID(string(entry.LineHash)[:32])
		entries = append(entries, entry)
	}

	return entries, nil
}

// ============================================================================
// Helper Functions
// ============================================================================

// HashCanonical computes SHA256 hex of a canonical string.
func HashCanonical(line string) string {
	hash := sha256.Sum256([]byte(line))
	return hex.EncodeToString(hash[:])
}

// DedupKey returns a deduplication key for an entry.
// Uses period + line hash for uniqueness.
func DedupKey(entry domain.TransparencyLogEntry) string {
	return string(entry.Period) + "|" + string(entry.LineHash)
}
