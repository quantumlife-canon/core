// Package shadowdiff provides hashing for shadow diff types.
//
// Phase 19.4: Shadow Diff + Calibration (Truth Harness)
//
// CRITICAL: All hashing uses pipe-delimited canonical strings.
// Never JSON. Never content. Only abstract buckets.
//
// Reference: docs/ADR/ADR-0045-phase19-4-shadow-diff-calibration.md
package shadowdiff

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// =============================================================================
// Comparison Key Hashing
// =============================================================================

// CanonicalString returns the pipe-delimited canonical representation.
func (k *ComparisonKey) CanonicalString() string {
	return fmt.Sprintf("COMPARISON_KEY|v1|%s|%s|%s",
		k.CircleID,
		k.Category,
		k.ItemKeyHash,
	)
}

// Hash returns the SHA256 hash of the canonical string.
func (k *ComparisonKey) Hash() string {
	return computeHash(k.CanonicalString())
}

// =============================================================================
// Canon Signal Hashing
// =============================================================================

// CanonicalString returns the pipe-delimited canonical representation.
func (s *CanonSignal) CanonicalString() string {
	surfaceStr := "false"
	if s.SurfaceDecision {
		surfaceStr = "true"
	}
	holdStr := "false"
	if s.HoldDecision {
		holdStr = "true"
	}
	return fmt.Sprintf("CANON_SIGNAL|v1|%s|%s|%s|%s|%s",
		s.Key.CanonicalString(),
		s.Horizon,
		s.Magnitude,
		surfaceStr,
		holdStr,
	)
}

// Hash returns the SHA256 hash of the canonical string.
func (s *CanonSignal) Hash() string {
	return computeHash(s.CanonicalString())
}

// =============================================================================
// Shadow Signal Hashing
// =============================================================================

// CanonicalString returns the pipe-delimited canonical representation.
func (s *ShadowSignal) CanonicalString() string {
	return fmt.Sprintf("SHADOW_SIGNAL|v1|%s|%s|%s|%s|%s",
		s.Key.CanonicalString(),
		s.Horizon,
		s.Magnitude,
		s.Confidence,
		s.SuggestionType,
	)
}

// Hash returns the SHA256 hash of the canonical string.
func (s *ShadowSignal) Hash() string {
	return computeHash(s.CanonicalString())
}

// =============================================================================
// Diff Result Hashing
// =============================================================================

// CanonicalString returns the pipe-delimited canonical representation.
func (d *DiffResult) CanonicalString() string {
	var b strings.Builder

	b.WriteString("DIFF_RESULT|v1|")
	b.WriteString(d.DiffID)
	b.WriteString("|")
	b.WriteString(string(d.CircleID))
	b.WriteString("|")
	b.WriteString(d.Key.CanonicalString())
	b.WriteString("|")

	// Canon signal (may be nil)
	if d.CanonSignal != nil {
		b.WriteString(d.CanonSignal.CanonicalString())
	} else {
		b.WriteString("nil")
	}
	b.WriteString("|")

	// Shadow signal (may be nil)
	if d.ShadowSignal != nil {
		b.WriteString(d.ShadowSignal.CanonicalString())
	} else {
		b.WriteString("nil")
	}
	b.WriteString("|")

	b.WriteString(string(d.Agreement))
	b.WriteString("|")
	b.WriteString(string(d.NoveltyType))
	b.WriteString("|")
	b.WriteString(d.PeriodBucket)
	b.WriteString("|")
	b.WriteString(d.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"))

	return b.String()
}

// Hash returns the SHA256 hash of the canonical string.
// Result is cached after first computation.
func (d *DiffResult) Hash() string {
	if d.hash != "" {
		return d.hash
	}
	d.hash = computeHash(d.CanonicalString())
	return d.hash
}

// =============================================================================
// Calibration Record Hashing
// =============================================================================

// CanonicalString returns the pipe-delimited canonical representation.
func (r *CalibrationRecord) CanonicalString() string {
	return fmt.Sprintf("CALIBRATION_RECORD|v1|%s|%s|%s|%s|%s|%s",
		r.RecordID,
		r.DiffID,
		r.DiffHash,
		r.Vote,
		r.PeriodBucket,
		r.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	)
}

// Hash returns the SHA256 hash of the canonical string.
func (r *CalibrationRecord) Hash() string {
	return computeHash(r.CanonicalString())
}

// =============================================================================
// Calibration Stats Hashing
// =============================================================================

// CanonicalString returns the pipe-delimited canonical representation.
func (s *CalibrationStats) CanonicalString() string {
	var b strings.Builder

	b.WriteString("CALIBRATION_STATS|v1|")
	b.WriteString(s.PeriodBucket)
	b.WriteString("|")
	b.WriteString(fmt.Sprintf("%d", s.TotalDiffs))
	b.WriteString("|")
	b.WriteString(fmt.Sprintf("%.6f", s.AgreementRate))
	b.WriteString("|")
	b.WriteString(fmt.Sprintf("%.6f", s.NoveltyRate))
	b.WriteString("|")
	b.WriteString(fmt.Sprintf("%.6f", s.ConflictRate))
	b.WriteString("|")
	b.WriteString(fmt.Sprintf("%.6f", s.UsefulnessScore))
	b.WriteString("|")
	b.WriteString(fmt.Sprintf("%d", s.VotedCount))
	b.WriteString("|")

	// Agreement counts in sorted order
	b.WriteString("agreements:")
	agreementKeys := make([]AgreementKind, 0, len(s.AgreementCounts))
	for k := range s.AgreementCounts {
		agreementKeys = append(agreementKeys, k)
	}
	sort.Slice(agreementKeys, func(i, j int) bool {
		return agreementKeys[i] < agreementKeys[j]
	})
	for i, k := range agreementKeys {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(fmt.Sprintf("%s=%d", k, s.AgreementCounts[k]))
	}
	b.WriteString("|")

	// Novelty counts in sorted order
	b.WriteString("novelties:")
	noveltyKeys := make([]Novelty, 0, len(s.NoveltyCounts))
	for k := range s.NoveltyCounts {
		noveltyKeys = append(noveltyKeys, k)
	}
	sort.Slice(noveltyKeys, func(i, j int) bool {
		return noveltyKeys[i] < noveltyKeys[j]
	})
	for i, k := range noveltyKeys {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(fmt.Sprintf("%s=%d", k, s.NoveltyCounts[k]))
	}

	return b.String()
}

// Hash returns the SHA256 hash of the canonical string.
func (s *CalibrationStats) Hash() string {
	return computeHash(s.CanonicalString())
}

// =============================================================================
// Helper Functions
// =============================================================================

// computeHash computes SHA256 hash and returns hex string.
func computeHash(input string) string {
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:])
}
