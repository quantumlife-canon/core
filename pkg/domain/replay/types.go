// Package replay provides domain types for Phase 30A: Identity + Replay.
// This package implements deterministic replay bundle export/import.
//
// CRITICAL INVARIANTS:
// - stdlib only
// - No time.Now() - clock injection only
// - No goroutines
// - Pipe-delimited canonical format (NOT JSON)
// - No raw identifiers: no emails, URLs, amounts, subjects, message IDs
// - Deterministic output: same storelog + same clock bucket => same bundle hash
// - Bounded retention: default 30 days
//
// Reference: docs/ADR/ADR-0061-phase30A-identity-and-replay.md
package replay

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// BundleVersion is the current bundle format version.
const BundleVersion = "v1"

// ReplayBundleHeader contains metadata for the replay bundle.
// All fields are hashes or abstract values - no identifiers.
type ReplayBundleHeader struct {
	// Version is the bundle format version.
	Version string

	// CircleIDHash is SHA256 of the circle ID (hash-only, not raw).
	CircleIDHash string

	// PeriodKey is the export time bucket (15-min granularity).
	PeriodKey string

	// RecordCount is the number of records in the bundle.
	RecordCount int

	// EarliestPeriod is the earliest record period (day bucket, not timestamp).
	EarliestPeriod string

	// LatestPeriod is the latest record period (day bucket, not timestamp).
	LatestPeriod string

	// BundleHash is SHA256 of the canonical bundle content.
	BundleHash string
}

// CanonicalString returns a deterministic, pipe-delimited representation.
func (h *ReplayBundleHeader) CanonicalString() string {
	return fmt.Sprintf("%s|%s|%s|%d|%s|%s",
		h.Version,
		h.CircleIDHash,
		h.PeriodKey,
		h.RecordCount,
		h.EarliestPeriod,
		h.LatestPeriod,
	)
}

// Validate checks if the header is valid.
func (h *ReplayBundleHeader) Validate() error {
	if h.Version == "" {
		return errors.New("version is required")
	}
	if h.Version != BundleVersion {
		return fmt.Errorf("unsupported version: %s", h.Version)
	}
	if h.CircleIDHash == "" {
		return errors.New("circle ID hash is required")
	}
	if len(h.CircleIDHash) != 32 {
		return fmt.Errorf("invalid circle ID hash length: got %d, want 32", len(h.CircleIDHash))
	}
	if h.PeriodKey == "" {
		return errors.New("period key is required")
	}
	if h.RecordCount < 0 {
		return errors.New("record count cannot be negative")
	}
	if h.BundleHash == "" {
		return errors.New("bundle hash is required")
	}
	if len(h.BundleHash) != 64 {
		return fmt.Errorf("invalid bundle hash length: got %d, want 64", len(h.BundleHash))
	}
	return nil
}

// CanonicalRecordLine is a single record in the replay bundle.
// Format: RECORD_TYPE|RECORD_HASH|PERIOD_BUCKET|PAYLOAD_HASH
// We store only hashes and abstract data - never raw content.
type CanonicalRecordLine struct {
	// RecordType is the storelog record type (e.g., SHADOW_RECEIPT_ACK).
	RecordType string

	// RecordHash is the hash from the original storelog record.
	RecordHash string

	// PeriodBucket is the day bucket (YYYY-MM-DD) for ordering.
	PeriodBucket string

	// PayloadHash is SHA256 of the original payload (we don't store payload).
	PayloadHash string
}

// CanonicalString returns a deterministic, pipe-delimited representation.
func (r *CanonicalRecordLine) CanonicalString() string {
	return fmt.Sprintf("%s|%s|%s|%s",
		r.RecordType,
		r.RecordHash,
		r.PeriodBucket,
		r.PayloadHash,
	)
}

// ParseCanonicalRecordLine parses a line into a CanonicalRecordLine.
func ParseCanonicalRecordLine(line string) (*CanonicalRecordLine, error) {
	parts := strings.Split(line, "|")
	if len(parts) != 4 {
		return nil, fmt.Errorf("invalid record line format: expected 4 parts, got %d", len(parts))
	}
	return &CanonicalRecordLine{
		RecordType:   parts[0],
		RecordHash:   parts[1],
		PeriodBucket: parts[2],
		PayloadHash:  parts[3],
	}, nil
}

// Validate checks if the record line is valid.
func (r *CanonicalRecordLine) Validate() error {
	if r.RecordType == "" {
		return errors.New("record type is required")
	}
	if r.RecordHash == "" {
		return errors.New("record hash is required")
	}
	if r.PeriodBucket == "" {
		return errors.New("period bucket is required")
	}
	// Validate period bucket format (YYYY-MM-DD)
	if _, err := time.Parse("2006-01-02", r.PeriodBucket); err != nil {
		return fmt.Errorf("invalid period bucket format: %w", err)
	}
	if r.PayloadHash == "" {
		return errors.New("payload hash is required")
	}
	return nil
}

// ReplayBundle is the complete replay bundle for export/import.
type ReplayBundle struct {
	Header  ReplayBundleHeader
	Records []CanonicalRecordLine
}

// CanonicalString returns the full canonical representation.
// Format:
// HEADER_LINE
// ---
// RECORD_LINE_1
// RECORD_LINE_2
// ...
func (b *ReplayBundle) CanonicalString() string {
	var sb strings.Builder

	// Header line
	sb.WriteString(b.Header.CanonicalString())
	sb.WriteString("\n---\n")

	// Record lines (sorted for determinism)
	sortedRecords := make([]CanonicalRecordLine, len(b.Records))
	copy(sortedRecords, b.Records)
	sort.Slice(sortedRecords, func(i, j int) bool {
		// Sort by period bucket first, then by record hash for determinism
		if sortedRecords[i].PeriodBucket != sortedRecords[j].PeriodBucket {
			return sortedRecords[i].PeriodBucket < sortedRecords[j].PeriodBucket
		}
		return sortedRecords[i].RecordHash < sortedRecords[j].RecordHash
	})

	for _, record := range sortedRecords {
		sb.WriteString(record.CanonicalString())
		sb.WriteString("\n")
	}

	return sb.String()
}

// ComputeBundleHash computes the SHA256 hash of the bundle content.
func (b *ReplayBundle) ComputeBundleHash() string {
	// Hash header (without bundle hash) + sorted records
	content := b.Header.CanonicalString() + "\n"

	sortedRecords := make([]CanonicalRecordLine, len(b.Records))
	copy(sortedRecords, b.Records)
	sort.Slice(sortedRecords, func(i, j int) bool {
		if sortedRecords[i].PeriodBucket != sortedRecords[j].PeriodBucket {
			return sortedRecords[i].PeriodBucket < sortedRecords[j].PeriodBucket
		}
		return sortedRecords[i].RecordHash < sortedRecords[j].RecordHash
	})

	for _, record := range sortedRecords {
		content += record.CanonicalString() + "\n"
	}

	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// Validate checks if the bundle is valid including hash verification.
func (b *ReplayBundle) Validate() error {
	if err := b.Header.Validate(); err != nil {
		return fmt.Errorf("invalid header: %w", err)
	}

	if len(b.Records) != b.Header.RecordCount {
		return fmt.Errorf("record count mismatch: header says %d, found %d", b.Header.RecordCount, len(b.Records))
	}

	for i, record := range b.Records {
		if err := record.Validate(); err != nil {
			return fmt.Errorf("invalid record %d: %w", i, err)
		}
	}

	// Verify bundle hash
	computed := b.ComputeBundleHash()
	if computed != b.Header.BundleHash {
		return fmt.Errorf("bundle hash mismatch: expected %s, computed %s", b.Header.BundleHash, computed)
	}

	return nil
}

// ParseReplayBundle parses a bundle from its canonical string representation.
func ParseReplayBundle(content string) (*ReplayBundle, error) {
	lines := strings.Split(content, "\n")
	if len(lines) < 2 {
		return nil, errors.New("bundle too short")
	}

	// Find separator
	separatorIdx := -1
	for i, line := range lines {
		if line == "---" {
			separatorIdx = i
			break
		}
	}
	if separatorIdx == -1 {
		return nil, errors.New("missing separator")
	}
	if separatorIdx != 1 {
		return nil, errors.New("separator must be on line 2")
	}

	// Parse header
	headerParts := strings.Split(lines[0], "|")
	if len(headerParts) < 6 {
		return nil, fmt.Errorf("invalid header format: expected 6+ parts, got %d", len(headerParts))
	}

	recordCount := 0
	if _, err := fmt.Sscanf(headerParts[3], "%d", &recordCount); err != nil {
		return nil, fmt.Errorf("invalid record count: %w", err)
	}

	// Extract bundle hash from the end of content (if present)
	// The bundle hash is typically computed and added after initial parsing
	bundleHash := ""
	if len(headerParts) > 6 {
		bundleHash = headerParts[6]
	}

	header := ReplayBundleHeader{
		Version:        headerParts[0],
		CircleIDHash:   headerParts[1],
		PeriodKey:      headerParts[2],
		RecordCount:    recordCount,
		EarliestPeriod: headerParts[4],
		LatestPeriod:   headerParts[5],
		BundleHash:     bundleHash,
	}

	// Parse records
	var records []CanonicalRecordLine
	for i := separatorIdx + 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		record, err := ParseCanonicalRecordLine(line)
		if err != nil {
			return nil, fmt.Errorf("invalid record line %d: %w", i, err)
		}
		records = append(records, *record)
	}

	bundle := &ReplayBundle{
		Header:  header,
		Records: records,
	}

	// If no bundle hash was in header, compute it
	if bundle.Header.BundleHash == "" {
		bundle.Header.BundleHash = bundle.ComputeBundleHash()
	}

	return bundle, nil
}

// ExportResult contains the result of bundle export.
type ExportResult struct {
	Success    bool
	Error      string
	Bundle     *ReplayBundle
	BundleText string
}

// ImportResult contains the result of bundle import.
type ImportResult struct {
	Success       bool
	Error         string
	RecordsAdded  int
	RecordsExists int
	StatusHash    string
}

// NewImportResult creates an import result.
func NewImportResult(added, exists int) *ImportResult {
	result := &ImportResult{
		Success:       true,
		RecordsAdded:  added,
		RecordsExists: exists,
	}
	result.StatusHash = result.computeStatusHash()
	return result
}

func (r *ImportResult) computeStatusHash() string {
	canonical := fmt.Sprintf("v1|import_result|%d|%d", r.RecordsAdded, r.RecordsExists)
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:16])
}

// SafeRecordTypes lists the storelog record types that are safe for replay export.
// These contain only hashes, buckets, and abstract data - never identifiers.
var SafeRecordTypes = map[string]bool{
	// Phase 19: Shadow receipts (hash-only)
	"SHADOWLLM_RECEIPT": true,

	// Phase 19.4: Calibration (hash-only)
	"SHADOW_DIFF":        true,
	"SHADOW_CALIBRATION": true,

	// Phase 25: Undoable execution (hash-only)
	"UNDO_EXEC_RECORD": true,
	"UNDO_EXEC_ACK":    true,

	// Phase 26A: Journey (hash-only)
	"JOURNEY_DISMISSAL": true,

	// Phase 26B: First minutes (hash-only, abstract signals)
	"FIRST_MINUTES_SUMMARY":   true,
	"FIRST_MINUTES_DISMISSAL": true,

	// Phase 26C: Reality check (hash-only)
	"REALITY_ACK": true,

	// Phase 27: Shadow receipt proof (hash-only)
	"SHADOW_RECEIPT_ACK":  true,
	"SHADOW_RECEIPT_VOTE": true,

	// Phase 28: Trust action (hash-only)
	"TRUST_ACTION_RECEIPT": true,
	"TRUST_ACTION_UPDATE":  true,

	// Phase 29: Finance mirror (hash-only, abstract buckets)
	"TRUELAYER_CONNECTION": true,
	"FINANCE_SYNC_RECEIPT": true,
	"FINANCE_MIRROR_ACK":   true,

	// Phase 30A: Circle binding (hash-only)
	"CIRCLE_BINDING": true,
}

// IsSafeForExport checks if a record type is safe for replay export.
func IsSafeForExport(recordType string) bool {
	return SafeRecordTypes[recordType]
}

// ForbiddenPatterns are patterns that must NEVER appear in exported bundles.
var ForbiddenPatterns = []string{
	"@",             // Email addresses
	"http://",       // URLs
	"https://",      // URLs
	"£",             // Currency
	"$",             // Currency (but careful with regex)
	"€",             // Currency
	"Subject:",      // Email subjects
	"msg-",          // Message IDs
	"evt-",          // Event IDs
	"amazon",        // Merchant names
	"netflix",       // Merchant names
	"spotify",       // Merchant names
	"google.com",    // Domains
	"microsoft.com", // Domains
}

// ContainsForbiddenPattern checks if content contains any forbidden patterns.
func ContainsForbiddenPattern(content string) (bool, string) {
	lower := strings.ToLower(content)
	for _, pattern := range ForbiddenPatterns {
		if strings.Contains(lower, strings.ToLower(pattern)) {
			return true, pattern
		}
	}
	return false, ""
}

// HashString computes SHA256 of input and returns first 32 hex chars.
func HashString(s string) string {
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:16])
}
