// Package rulepack provides types for Rule Pack Export.
//
// Phase 19.6: Rule Pack Export (Promotion Pipeline)
//
// This file provides the stable text export format for RulePacks.
// The format is pipe-delimited, NOT JSON, for deterministic output.
//
// CRITICAL: Export contains NO raw identifiers.
//
// Reference: docs/ADR/ADR-0047-phase19-6-rulepack-export.md
package rulepack

import (
	"strings"

	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/shadowgate"
	"quantumlife/pkg/domain/shadowllm"
)

// =============================================================================
// Export Format
// =============================================================================

// ExportHeader is the header line for exported packs.
const ExportHeader = "# RULEPACK EXPORT FORMAT " + ExportFormatVersion

// ToText exports the RulePack to a stable text format.
//
// The format is:
//   # RULEPACK EXPORT FORMAT v1
//   # PACK_ID|period|circle|created_bucket|change_count|pack_hash
//   PACK|<pack_id>|<period>|<circle>|<created_bucket>|<change_count>|<pack_hash>
//   # CHANGES (sorted deterministically)
//   CHANGE|<change_id>|<candidate_hash>|<intent_hash>|<circle>|<kind>|<scope>|<target_hash>|<category>|<delta>|<usefulness>|<confidence>|<novelty>|<agreement>
//   ...
//   # END
//
// CRITICAL: No raw identifiers. Only hashes and buckets.
func (p *RulePack) ToText() string {
	var b strings.Builder

	// Header
	b.WriteString(ExportHeader)
	b.WriteString("\n")
	b.WriteString("# PACK_ID|period|circle|created_bucket|change_count|pack_hash\n")

	// Pack line
	b.WriteString("PACK|")
	b.WriteString(p.PackID)
	b.WriteString("|")
	b.WriteString(p.PeriodKey)
	b.WriteString("|")
	if p.CircleID == "" {
		b.WriteString("all")
	} else {
		b.WriteString(string(p.CircleID))
	}
	b.WriteString("|")
	b.WriteString(p.CreatedAtBucket)
	b.WriteString("|")
	b.WriteString(itoa(len(p.Changes)))
	b.WriteString("|")
	b.WriteString(p.PackHash)
	b.WriteString("\n")

	// Changes header
	b.WriteString("# CHANGES (sorted deterministically)\n")
	b.WriteString("# change_id|candidate_hash|intent_hash|circle|kind|scope|target_hash|category|delta|usefulness|confidence|novelty|agreement\n")

	// Change lines
	for _, c := range p.Changes {
		b.WriteString("CHANGE|")
		b.WriteString(c.ChangeID)
		b.WriteString("|")
		b.WriteString(c.CandidateHash)
		b.WriteString("|")
		b.WriteString(c.IntentHash)
		b.WriteString("|")
		if c.CircleID == "" {
			b.WriteString("all")
		} else {
			b.WriteString(string(c.CircleID))
		}
		b.WriteString("|")
		b.WriteString(string(c.ChangeKind))
		b.WriteString("|")
		b.WriteString(string(c.TargetScope))
		b.WriteString("|")
		if c.TargetHash == "" {
			b.WriteString("none")
		} else {
			b.WriteString(c.TargetHash)
		}
		b.WriteString("|")
		b.WriteString(string(c.Category))
		b.WriteString("|")
		b.WriteString(string(c.SuggestedDelta))
		b.WriteString("|")
		b.WriteString(string(c.UsefulnessBucket))
		b.WriteString("|")
		b.WriteString(string(c.VoteConfidenceBucket))
		b.WriteString("|")
		b.WriteString(string(c.NoveltyBucket))
		b.WriteString("|")
		b.WriteString(string(c.AgreementBucket))
		b.WriteString("\n")
	}

	// Footer
	b.WriteString("# END\n")

	return b.String()
}

// ParseText parses a RulePack from the text export format.
// Returns an error if the format is invalid.
//
// NOTE: This is for testing/validation only. Production systems
// should use the structured types directly.
func ParseText(text string) (*RulePack, error) {
	lines := strings.Split(text, "\n")
	if len(lines) < 3 {
		return nil, ErrInvalidExportFormat
	}

	// Validate header
	if !strings.HasPrefix(lines[0], "# RULEPACK EXPORT FORMAT") {
		return nil, ErrInvalidExportFormat
	}

	pack := &RulePack{
		ExportFormatVersion: ExportFormatVersion,
	}

	// Find and parse PACK line
	for _, line := range lines {
		if strings.HasPrefix(line, "PACK|") {
			parts := strings.Split(line, "|")
			if len(parts) < 7 {
				return nil, ErrInvalidExportFormat
			}
			pack.PackID = parts[1]
			pack.PeriodKey = parts[2]
			if parts[3] != "all" {
				pack.CircleID = identity.EntityID(parts[3])
			}
			pack.CreatedAtBucket = parts[4]
			// parts[5] is change count (derived)
			pack.PackHash = parts[6]
		} else if strings.HasPrefix(line, "CHANGE|") {
			parts := strings.Split(line, "|")
			if len(parts) < 14 {
				continue // Skip malformed change lines
			}
			change := RuleChange{
				ChangeID:             parts[1],
				CandidateHash:        parts[2],
				IntentHash:           parts[3],
				ChangeKind:           ChangeKind(parts[5]),
				TargetScope:          TargetScope(parts[6]),
				SuggestedDelta:       SuggestedDelta(parts[9]),
				UsefulnessBucket:     shadowgate.UsefulnessBucket(parts[10]),
				VoteConfidenceBucket: shadowgate.VoteConfidenceBucket(parts[11]),
				NoveltyBucket:        NoveltyBucket(parts[12]),
				AgreementBucket:      AgreementBucket(parts[13]),
			}
			if parts[4] != "all" {
				change.CircleID = identity.EntityID(parts[4])
			}
			if parts[7] != "none" {
				change.TargetHash = parts[7]
			}
			change.Category = shadowllm.AbstractCategory(parts[8])
			pack.Changes = append(pack.Changes, change)
		}
	}

	return pack, nil
}

// =============================================================================
// Export Errors
// =============================================================================

const (
	ErrInvalidExportFormat packError = "invalid export format"
)

// =============================================================================
// Privacy Validation
// =============================================================================

// ForbiddenPatterns are patterns that must never appear in exports.
var ForbiddenPatterns = []string{
	"@",       // Email addresses
	"http://", // URLs
	"https://",
	"www.",
	".com",
	".org",
	".net",
	".io",
	"$",  // Currency
	"£",
	"€",
	"¥",
	"₹",
	// Common vendor names (lowercase check)
	"amazon",
	"google",
	"apple",
	"microsoft",
	"netflix",
	"uber",
	"lyft",
	"venmo",
	"paypal",
	"chase",
	"wells fargo",
	"bank of america",
	"citibank",
}

// ValidateExportPrivacy checks that the export contains no forbidden patterns.
func ValidateExportPrivacy(text string) error {
	lower := strings.ToLower(text)
	for _, pattern := range ForbiddenPatterns {
		if strings.Contains(lower, strings.ToLower(pattern)) {
			return ErrPrivacyViolation
		}
	}
	return nil
}

const (
	ErrPrivacyViolation packError = "privacy violation in export"
)
