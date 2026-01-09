// Package transparencylog provides domain types for Phase 51: Transparency Log / Claim Ledger.
//
// CRITICAL INVARIANTS:
// - NO POWER: This package is observation/proof only. It MUST NOT change any
//   runtime behavior (no wiring into vendor caps, packs, pressure, interrupts).
// - HASH-ONLY: Never store or render raw vendor/pack identifiers, names, urls,
//   emails, merchant strings. Only hashes, buckets, fingerprints.
// - APPEND-ONLY: Ledger records are append-only with dedup/idempotence but no
//   mutation/deletion.
// - DETERMINISTIC: Same stored claims + same clock period = same ledger output.
// - PIPE-DELIMITED: All canonical strings use pipe-delimited format, NOT JSON.
//
// Reference: docs/ADR/ADR-0089-phase51-transparency-log-claim-ledger.md
package transparencylog

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ============================================================================
// Enums
// ============================================================================

// LogKind identifies what kind of entry this is in the transparency log.
type LogKind string

const (
	// LogSignedVendorClaim is a logged signed vendor claim.
	LogSignedVendorClaim LogKind = "log_signed_vendor_claim"
	// LogSignedPackManifest is a logged signed pack manifest.
	LogSignedPackManifest LogKind = "log_signed_pack_manifest"
)

// Validate checks if the LogKind is valid.
func (k LogKind) Validate() error {
	switch k {
	case LogSignedVendorClaim, LogSignedPackManifest:
		return nil
	default:
		return fmt.Errorf("invalid LogKind: %q", k)
	}
}

// String returns the string representation.
func (k LogKind) String() string {
	return string(k)
}

// ============================================================================

// LogProvenanceBucket identifies the provenance bucket for a log entry.
// Mirrors Phase 50 provenance buckets but keeps this package independent.
type LogProvenanceBucket string

const (
	// ProvUserSupplied means the entry came from user-supplied source.
	ProvUserSupplied LogProvenanceBucket = "prov_user_supplied"
	// ProvMarketplace means the entry came from marketplace flow.
	ProvMarketplace LogProvenanceBucket = "prov_marketplace"
	// ProvUnknown means provenance is unknown or not specified.
	ProvUnknown LogProvenanceBucket = "prov_unknown"
)

// Validate checks if the LogProvenanceBucket is valid.
func (p LogProvenanceBucket) Validate() error {
	switch p {
	case ProvUserSupplied, ProvMarketplace, ProvUnknown:
		return nil
	default:
		return fmt.Errorf("invalid LogProvenanceBucket: %q", p)
	}
}

// String returns the string representation.
func (p LogProvenanceBucket) String() string {
	return string(p)
}

// ============================================================================

// MagnitudeBucket represents a coarse count bucket for summaries.
type MagnitudeBucket string

const (
	// MagnitudeNothing means 0 entries.
	MagnitudeNothing MagnitudeBucket = "nothing"
	// MagnitudeAFew means 1-3 entries.
	MagnitudeAFew MagnitudeBucket = "a_few"
	// MagnitudeSeveral means 4-12 entries.
	MagnitudeSeveral MagnitudeBucket = "several"
)

// Validate checks if the MagnitudeBucket is valid.
func (m MagnitudeBucket) Validate() error {
	switch m {
	case MagnitudeNothing, MagnitudeAFew, MagnitudeSeveral:
		return nil
	default:
		return fmt.Errorf("invalid MagnitudeBucket: %q", m)
	}
}

// String returns the string representation.
func (m MagnitudeBucket) String() string {
	return string(m)
}

// ComputeMagnitudeBucket returns the magnitude bucket for a count.
func ComputeMagnitudeBucket(count int) MagnitudeBucket {
	switch {
	case count == 0:
		return MagnitudeNothing
	case count <= 3:
		return MagnitudeAFew
	default:
		return MagnitudeSeveral
	}
}

// ============================================================================
// Value Types
// ============================================================================

// KeyFingerprint is the SHA256 fingerprint of a public key.
// Used for display and dedup, never the raw key.
type KeyFingerprint string

// Validate checks if the KeyFingerprint is valid (64 hex characters).
func (f KeyFingerprint) Validate() error {
	if len(f) != 64 {
		return fmt.Errorf("KeyFingerprint must be 64 hex characters, got %d", len(f))
	}
	for _, c := range f {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return fmt.Errorf("KeyFingerprint must be lowercase hex, got invalid char %q", c)
		}
	}
	return nil
}

// String returns the string representation.
func (f KeyFingerprint) String() string {
	return string(f)
}

// Short returns a shortened fingerprint for display (first 16 chars).
func (f KeyFingerprint) Short() string {
	if len(f) >= 16 {
		return string(f)[:16]
	}
	return string(f)
}

// ============================================================================

// PeriodKey is the time period bucket (e.g., "2025-W03" for weekly).
// Must be pipe-safe (no pipe characters).
type PeriodKey string

// Validate checks if the PeriodKey is valid.
func (p PeriodKey) Validate() error {
	if p == "" {
		return errors.New("PeriodKey cannot be empty")
	}
	if strings.Contains(string(p), "|") {
		return errors.New("PeriodKey cannot contain pipe character")
	}
	if strings.Contains(string(p), "=") {
		return errors.New("PeriodKey cannot contain equals character")
	}
	return nil
}

// String returns the string representation.
func (p PeriodKey) String() string {
	return string(p)
}

// ============================================================================

// LogLineHash is the SHA256 hash of a canonical log line.
type LogLineHash string

// Validate checks if the LogLineHash is valid (64 hex characters).
func (h LogLineHash) Validate() error {
	if len(h) != 64 {
		return fmt.Errorf("LogLineHash must be 64 hex characters, got %d", len(h))
	}
	for _, c := range h {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return fmt.Errorf("LogLineHash must be lowercase hex, got invalid char %q", c)
		}
	}
	return nil
}

// String returns the string representation.
func (h LogLineHash) String() string {
	return string(h)
}

// Short returns a shortened hash for display (first 16 chars).
func (h LogLineHash) Short() string {
	if len(h) >= 16 {
		return string(h)[:16]
	}
	return string(h)
}

// ============================================================================

// LogEntryID is a unique identifier for a log entry.
type LogEntryID string

// Validate checks if the LogEntryID is valid.
func (id LogEntryID) Validate() error {
	if id == "" {
		return errors.New("LogEntryID cannot be empty")
	}
	if strings.Contains(string(id), "|") {
		return errors.New("LogEntryID cannot contain pipe character")
	}
	return nil
}

// String returns the string representation.
func (id LogEntryID) String() string {
	return string(id)
}

// ============================================================================

// RefHash is a hash reference (claim hash or manifest hash).
type RefHash string

// Validate checks if the RefHash is valid (64 hex characters).
func (h RefHash) Validate() error {
	if len(h) != 64 {
		return fmt.Errorf("RefHash must be 64 hex characters, got %d", len(h))
	}
	for _, c := range h {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return fmt.Errorf("RefHash must be lowercase hex, got invalid char %q", c)
		}
	}
	return nil
}

// String returns the string representation.
func (h RefHash) String() string {
	return string(h)
}

// Short returns a shortened hash for display (first 16 chars).
func (h RefHash) Short() string {
	if len(h) >= 16 {
		return string(h)[:16]
	}
	return string(h)
}

// ============================================================================
// Core Structures
// ============================================================================

// TransparencyLogEntry is a single entry in the transparency log.
// This is what gets persisted (hash-only, no raw identifiers).
type TransparencyLogEntry struct {
	// EntryID is a unique identifier for this entry.
	EntryID LogEntryID
	// Period is the time period bucket (e.g., "2025-W03").
	Period PeriodKey
	// Kind identifies what kind of entry this is.
	Kind LogKind
	// Provenance is the provenance bucket.
	Provenance LogProvenanceBucket
	// KeyFP is the key fingerprint of the signer.
	KeyFP KeyFingerprint
	// RefHash is the claim hash or manifest hash (already hash-only).
	RefHash RefHash
	// LineHash is the hash of CanonicalLine().
	LineHash LogLineHash
	// CreatedBucket is a coarse bucket only (e.g., "na" for no timestamp).
	CreatedBucket string
}

// Validate checks all fields of the entry.
func (e TransparencyLogEntry) Validate() error {
	if err := e.EntryID.Validate(); err != nil {
		return fmt.Errorf("entry EntryID: %w", err)
	}
	if err := e.Period.Validate(); err != nil {
		return fmt.Errorf("entry Period: %w", err)
	}
	if err := e.Kind.Validate(); err != nil {
		return fmt.Errorf("entry Kind: %w", err)
	}
	if err := e.Provenance.Validate(); err != nil {
		return fmt.Errorf("entry Provenance: %w", err)
	}
	if err := e.KeyFP.Validate(); err != nil {
		return fmt.Errorf("entry KeyFP: %w", err)
	}
	if err := e.RefHash.Validate(); err != nil {
		return fmt.Errorf("entry RefHash: %w", err)
	}
	if e.LineHash != "" {
		if err := e.LineHash.Validate(); err != nil {
			return fmt.Errorf("entry LineHash: %w", err)
		}
	}
	if e.CreatedBucket == "" {
		return errors.New("entry CreatedBucket cannot be empty")
	}
	return nil
}

// CanonicalLine returns the strict pipe-delimited format for this entry.
// Format: v1|period=<P>|kind=<K>|prov=<PR>|keyfp=<FP>|ref=<REF>|created=<B>
func (e TransparencyLogEntry) CanonicalLine() string {
	parts := []string{
		"v1",
		"period=" + string(e.Period),
		"kind=" + string(e.Kind),
		"prov=" + string(e.Provenance),
		"keyfp=" + string(e.KeyFP),
		"ref=" + string(e.RefHash),
		"created=" + e.CreatedBucket,
	}
	return strings.Join(parts, "|")
}

// ComputeLineHash computes the SHA256 hash of the canonical line.
func (e TransparencyLogEntry) ComputeLineHash() LogLineHash {
	hash := sha256.Sum256([]byte(e.CanonicalLine()))
	return LogLineHash(hex.EncodeToString(hash[:]))
}

// ============================================================================

// TransparencyLogLineView is a render-safe view of a log entry for UI display.
// Contains only what should be shown, with prefixes for hashes.
type TransparencyLogLineView struct {
	// Kind identifies what kind of entry this is.
	Kind LogKind
	// Provenance is the provenance bucket.
	Provenance LogProvenanceBucket
	// KeyFP is the key fingerprint (full or short).
	KeyFP KeyFingerprint
	// RefHash is the claim/manifest hash (full or short).
	RefHash RefHash
	// LineHash is the line hash (full or short).
	LineHash LogLineHash
}

// ============================================================================

// TransparencyLogSummary provides a summary of the transparency log page.
type TransparencyLogSummary struct {
	// TotalBucket is the magnitude bucket (nothing|a_few|several).
	TotalBucket MagnitudeBucket
	// KindsPresent lists the kinds present (max 2).
	KindsPresent []LogKind
	// ProvenancePresent lists the provenances present (max 3).
	ProvenancePresent []LogProvenanceBucket
}

// Validate checks the summary fields.
func (s TransparencyLogSummary) Validate() error {
	if err := s.TotalBucket.Validate(); err != nil {
		return fmt.Errorf("summary TotalBucket: %w", err)
	}
	for _, k := range s.KindsPresent {
		if err := k.Validate(); err != nil {
			return fmt.Errorf("summary KindsPresent: %w", err)
		}
	}
	for _, p := range s.ProvenancePresent {
		if err := p.Validate(); err != nil {
			return fmt.Errorf("summary ProvenancePresent: %w", err)
		}
	}
	return nil
}

// CanonicalString returns the canonical string for hashing.
func (s TransparencyLogSummary) CanonicalString() string {
	kinds := make([]string, len(s.KindsPresent))
	for i, k := range s.KindsPresent {
		kinds[i] = string(k)
	}
	sort.Strings(kinds)

	provs := make([]string, len(s.ProvenancePresent))
	for i, p := range s.ProvenancePresent {
		provs[i] = string(p)
	}
	sort.Strings(provs)

	return fmt.Sprintf("summary|total=%s|kinds=%s|provs=%s",
		s.TotalBucket,
		strings.Join(kinds, ","),
		strings.Join(provs, ","))
}

// ============================================================================

// TransparencyLogPage represents a rendered page of the transparency log.
type TransparencyLogPage struct {
	// Title is the page title.
	Title string
	// Lines are the visible log line views (max 12 for UI).
	Lines []TransparencyLogLineView
	// Summary is the summary of the page.
	Summary TransparencyLogSummary
	// StatusHash is the hash of the page state.
	StatusHash string
	// Period is the period for this page.
	Period PeriodKey
	// TotalCount is the total number of entries (may exceed displayed lines).
	TotalCount int
}

// ComputeStatusHash computes the status hash for the page.
// Hash of canonical concatenation of line hashes + summary.
func (p TransparencyLogPage) ComputeStatusHash() string {
	var parts []string
	for _, line := range p.Lines {
		parts = append(parts, string(line.LineHash))
	}
	parts = append(parts, p.Summary.CanonicalString())
	combined := strings.Join(parts, "|")
	hash := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(hash[:])
}

// ============================================================================

// TransparencyLogExportBundle is a bundle for exporting/importing log entries.
type TransparencyLogExportBundle struct {
	// Version is the bundle format version (e.g., "v1").
	Version string
	// Period is the period for this bundle.
	Period PeriodKey
	// Lines are the canonical log lines (pipe-delimited).
	Lines []string
	// BundleHash is the hash of the bundle content.
	BundleHash string
}

// Validate checks the bundle fields.
func (b TransparencyLogExportBundle) Validate() error {
	if b.Version == "" {
		return errors.New("bundle Version cannot be empty")
	}
	if b.Version != "v1" {
		return fmt.Errorf("unsupported bundle version: %q", b.Version)
	}
	if err := b.Period.Validate(); err != nil {
		return fmt.Errorf("bundle Period: %w", err)
	}
	// Validate each line doesn't contain forbidden patterns
	for i, line := range b.Lines {
		if err := ValidateNoForbiddenPatterns(line); err != nil {
			return fmt.Errorf("bundle line %d: %w", i, err)
		}
	}
	return nil
}

// ComputeBundleHash computes the hash of the bundle content.
func (b TransparencyLogExportBundle) ComputeBundleHash() string {
	content := fmt.Sprintf("bundle|v1|period=%s|lines=%d|", b.Period, len(b.Lines))
	for _, line := range b.Lines {
		content += line + "\n"
	}
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// ToCanonicalFormat returns the bundle as canonical text format for export.
// First line: bundle|v1|period=<P>|bundle_hash=<H>
// Following lines: each canonical log line
func (b TransparencyLogExportBundle) ToCanonicalFormat() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("bundle|v1|period=%s|bundle_hash=%s\n", b.Period, b.BundleHash))
	for _, line := range b.Lines {
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	return sb.String()
}

// ============================================================================
// Validation Helpers
// ============================================================================

// ForbiddenPatterns are patterns that must never appear in log entries.
var ForbiddenPatterns = []string{
	"@",           // email indicator
	"http://",     // URL
	"https://",    // URL
	"vendorID",    // raw identifier
	"vendor_id",   // raw identifier
	"packID",      // raw identifier
	"pack_id",     // raw identifier
	"merchant",    // raw identifier
	"periodKey",   // client-supplied period
	"period_key",  // client-supplied period
	".com",        // domain
	".org",        // domain
	".net",        // domain
}

// ValidateNoForbiddenPatterns checks that a string doesn't contain forbidden patterns.
func ValidateNoForbiddenPatterns(s string) error {
	lower := strings.ToLower(s)
	for _, pattern := range ForbiddenPatterns {
		if strings.Contains(lower, strings.ToLower(pattern)) {
			return fmt.Errorf("forbidden pattern found: %q", pattern)
		}
	}
	return nil
}

// ============================================================================
// Parse Helpers
// ============================================================================

// ParseCanonicalLine parses a canonical line back into an entry (partial).
// Returns entry without EntryID (that's assigned on insert).
func ParseCanonicalLine(line string) (TransparencyLogEntry, error) {
	parts := strings.Split(line, "|")
	if len(parts) < 7 {
		return TransparencyLogEntry{}, fmt.Errorf("invalid canonical line: expected 7 parts, got %d", len(parts))
	}

	if parts[0] != "v1" {
		return TransparencyLogEntry{}, fmt.Errorf("unsupported line version: %q", parts[0])
	}

	entry := TransparencyLogEntry{
		CreatedBucket: "na", // default
	}

	for _, part := range parts[1:] {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key, val := kv[0], kv[1]
		switch key {
		case "period":
			entry.Period = PeriodKey(val)
		case "kind":
			entry.Kind = LogKind(val)
		case "prov":
			entry.Provenance = LogProvenanceBucket(val)
		case "keyfp":
			entry.KeyFP = KeyFingerprint(val)
		case "ref":
			entry.RefHash = RefHash(val)
		case "created":
			entry.CreatedBucket = val
		}
	}

	// Compute line hash
	entry.LineHash = entry.ComputeLineHash()

	return entry, nil
}

// ParseExportBundle parses a canonical export bundle from text.
func ParseExportBundle(text string) (TransparencyLogExportBundle, error) {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) < 1 {
		return TransparencyLogExportBundle{}, errors.New("empty bundle")
	}

	// Parse header: bundle|v1|period=<P>|bundle_hash=<H>
	header := lines[0]
	if !strings.HasPrefix(header, "bundle|v1|") {
		return TransparencyLogExportBundle{}, fmt.Errorf("invalid bundle header: %q", header)
	}

	bundle := TransparencyLogExportBundle{
		Version: "v1",
	}

	headerParts := strings.Split(header, "|")
	for _, part := range headerParts[2:] {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "period":
			bundle.Period = PeriodKey(kv[1])
		case "bundle_hash":
			bundle.BundleHash = kv[1]
		}
	}

	// Parse entry lines
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "v1|") {
			return TransparencyLogExportBundle{}, fmt.Errorf("invalid entry line: %q", line)
		}
		bundle.Lines = append(bundle.Lines, line)
	}

	return bundle, nil
}
