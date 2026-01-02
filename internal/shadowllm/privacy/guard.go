// Package privacy provides privacy validation for shadow LLM inputs.
//
// Phase 19.3: Azure OpenAI Shadow Provider
//
// CRITICAL INVARIANTS:
//   - Input MUST contain ONLY abstract data (buckets, hashes, counts)
//   - Input MUST NOT contain: email addresses, URLs, vendor names, amounts, raw text
//   - Validation is STRICT - any suspect pattern blocks the request
//   - No goroutines. No time.Now() - clock injection only.
//   - Stdlib only. No external dependencies.
//
// Reference: docs/ADR/ADR-0044-phase19-3-azure-openai-shadow-provider.md
package privacy

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"

	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/shadowllm"
)

// ShadowInput is the privacy-safe input for shadow LLM providers.
//
// CRITICAL: Contains ONLY abstract data.
// CRITICAL: No raw text, emails, amounts, or identifiable information.
type ShadowInput struct {
	// CircleID is the circle being analyzed.
	CircleID identity.EntityID

	// TimeBucket is the time window (day bucket, e.g., "2024-01-15").
	TimeBucket string

	// ObligationMagnitudes maps category => magnitude bucket.
	ObligationMagnitudes map[shadowllm.AbstractCategory]shadowllm.MagnitudeBucket

	// HeldMagnitudes maps category => magnitude bucket of held items.
	HeldMagnitudes map[shadowllm.AbstractCategory]shadowllm.MagnitudeBucket

	// SurfaceCandidateMagnitude is the magnitude bucket of surface candidates.
	SurfaceCandidateMagnitude shadowllm.MagnitudeBucket

	// DraftCandidateMagnitude is the magnitude bucket of draft candidates.
	DraftCandidateMagnitude shadowllm.MagnitudeBucket

	// TriggersSeen indicates if any triggers were seen.
	TriggersSeen bool

	// MirrorMagnitude indicates the mirror summary magnitude.
	MirrorMagnitude shadowllm.MagnitudeBucket

	// CategoryPresence indicates which categories have items.
	// Value is true if category has any items.
	CategoryPresence map[shadowllm.AbstractCategory]bool

	// StateSnapshotHash is a hash of the view/state snapshot.
	// Never the actual snapshot content.
	StateSnapshotHash string

	// InputDigestHash is the hash of the full input digest.
	InputDigestHash string
}

// PolicyVersion identifies the privacy policy version.
// Incremented when validation rules change.
const PolicyVersion = "v1.0.0"

// PolicyHash returns a hash of the current policy version.
func PolicyHash() string {
	h := sha256.Sum256([]byte("PRIVACY_POLICY|" + PolicyVersion))
	return hex.EncodeToString(h[:16])
}

// Guard provides privacy validation for shadow inputs.
type Guard struct {
	forbiddenPatterns []*regexp.Regexp
}

// NewGuard creates a new privacy guard.
func NewGuard() *Guard {
	return &Guard{
		forbiddenPatterns: compileForbiddenPatterns(),
	}
}

// compileForbiddenPatterns compiles regex patterns for forbidden content.
func compileForbiddenPatterns() []*regexp.Regexp {
	patterns := []string{
		// Email addresses
		`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`,
		// URLs
		`https?://[^\s]+`,
		// Currency amounts with symbols
		`[$£€¥]\s*\d+`,
		`\d+\s*[$£€¥]`,
		// Large numbers that could be amounts
		`\d{4,}`,
		// Phone numbers
		`\d{3}[-.\s]?\d{3}[-.\s]?\d{4}`,
		// Dates in common formats
		`\d{1,2}/\d{1,2}/\d{2,4}`,
		`\d{4}-\d{2}-\d{2}`,
		// Common vendor/company patterns (case insensitive checked separately)
		`(?i)(amazon|google|apple|microsoft|netflix|uber|lyft|venmo|paypal|chase|wells\s*fargo|bank\s*of\s*america)`,
		// IP addresses
		`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`,
		// Credit card patterns
		`\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}`,
		// SSN patterns
		`\d{3}[-\s]?\d{2}[-\s]?\d{4}`,
	}

	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		if re, err := regexp.Compile(p); err == nil {
			compiled = append(compiled, re)
		}
	}
	return compiled
}

// ValidateInput validates a ShadowInput for privacy compliance.
//
// CRITICAL: Returns error if any forbidden patterns detected.
// CRITICAL: This is a BLOCKING check - no requests proceed if this fails.
func (g *Guard) ValidateInput(input *ShadowInput) error {
	// Check CircleID for forbidden patterns
	if err := g.validateString(string(input.CircleID), "circle_id"); err != nil {
		return err
	}

	// Check TimeBucket (should be YYYY-MM-DD format only)
	if err := g.validateTimeBucket(input.TimeBucket); err != nil {
		return err
	}

	// Check state snapshot hash (should be hex only)
	if err := g.validateHash(input.StateSnapshotHash, "state_snapshot_hash"); err != nil {
		return err
	}

	// Check input digest hash (should be hex only)
	if err := g.validateHash(input.InputDigestHash, "input_digest_hash"); err != nil {
		return err
	}

	return nil
}

// validateString checks a string for forbidden patterns.
func (g *Guard) validateString(s, field string) error {
	if s == "" {
		return nil
	}

	// Check length - reject long strings that could contain content
	if len(s) > 256 {
		return &ValidationError{
			Field:   field,
			Message: "string too long (max 256 chars)",
		}
	}

	// Check forbidden patterns
	for _, re := range g.forbiddenPatterns {
		if re.MatchString(s) {
			return &ValidationError{
				Field:   field,
				Message: "contains forbidden pattern",
			}
		}
	}

	return nil
}

// validateTimeBucket checks that time bucket is in expected format.
func (g *Guard) validateTimeBucket(tb string) error {
	if tb == "" {
		return nil
	}

	// Must be YYYY-MM-DD format
	if len(tb) != 10 {
		return &ValidationError{
			Field:   "time_bucket",
			Message: "invalid format (must be YYYY-MM-DD)",
		}
	}

	// Check pattern
	if tb[4] != '-' || tb[7] != '-' {
		return &ValidationError{
			Field:   "time_bucket",
			Message: "invalid format (must be YYYY-MM-DD)",
		}
	}

	return nil
}

// validateHash checks that a hash is valid hex.
func (g *Guard) validateHash(h, field string) error {
	if h == "" || h == "empty" {
		return nil
	}

	// Hash should be hex only
	for _, c := range h {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return &ValidationError{
				Field:   field,
				Message: "invalid hash format (must be hex)",
			}
		}
	}

	return nil
}

// BuildPrivacySafeInput constructs a privacy-safe input from raw sources.
//
// CRITICAL: This function extracts ONLY abstract data.
// CRITICAL: All counts are bucketed. All content is hashed.
func BuildPrivacySafeInput(
	circleID identity.EntityID,
	timeBucket string,
	digest shadowllm.ShadowInputDigest,
	stateSnapshotHash string,
) (*ShadowInput, error) {
	input := &ShadowInput{
		CircleID:                  circleID,
		TimeBucket:                timeBucket,
		ObligationMagnitudes:      make(map[shadowllm.AbstractCategory]shadowllm.MagnitudeBucket),
		HeldMagnitudes:            make(map[shadowllm.AbstractCategory]shadowllm.MagnitudeBucket),
		SurfaceCandidateMagnitude: digest.SurfaceCandidateCount,
		DraftCandidateMagnitude:   digest.DraftCandidateCount,
		TriggersSeen:              digest.TriggersSeen,
		MirrorMagnitude:           digest.MirrorBucket,
		CategoryPresence:          make(map[shadowllm.AbstractCategory]bool),
		StateSnapshotHash:         stateSnapshotHash,
		InputDigestHash:           digest.Hash(),
	}

	// Copy magnitude buckets
	for cat, mag := range digest.ObligationCountByCategory {
		input.ObligationMagnitudes[cat] = mag
		if mag != shadowllm.MagnitudeNothing {
			input.CategoryPresence[cat] = true
		}
	}

	for cat, mag := range digest.HeldCountByCategory {
		input.HeldMagnitudes[cat] = mag
		if mag != shadowllm.MagnitudeNothing {
			input.CategoryPresence[cat] = true
		}
	}

	return input, nil
}

// CanonicalString returns a deterministic string representation.
func (i *ShadowInput) CanonicalString() string {
	var b strings.Builder
	b.WriteString("SHADOW_INPUT|v1|")
	b.WriteString(string(i.CircleID))
	b.WriteString("|")
	b.WriteString(i.TimeBucket)
	b.WriteString("|")

	// Categories in sorted order
	for _, cat := range shadowllm.AllCategories() {
		mag := i.ObligationMagnitudes[cat]
		if mag == "" {
			mag = shadowllm.MagnitudeNothing
		}
		b.WriteString(string(cat))
		b.WriteString(":obl:")
		b.WriteString(string(mag))
		b.WriteString(",")
	}

	b.WriteString("|")

	for _, cat := range shadowllm.AllCategories() {
		mag := i.HeldMagnitudes[cat]
		if mag == "" {
			mag = shadowllm.MagnitudeNothing
		}
		b.WriteString(string(cat))
		b.WriteString(":held:")
		b.WriteString(string(mag))
		b.WriteString(",")
	}

	b.WriteString("|surf:")
	b.WriteString(string(i.SurfaceCandidateMagnitude))
	b.WriteString("|draft:")
	b.WriteString(string(i.DraftCandidateMagnitude))
	b.WriteString("|triggers:")
	if i.TriggersSeen {
		b.WriteString("true")
	} else {
		b.WriteString("false")
	}
	b.WriteString("|mirror:")
	b.WriteString(string(i.MirrorMagnitude))
	b.WriteString("|state_hash:")
	b.WriteString(i.StateSnapshotHash)
	b.WriteString("|digest_hash:")
	b.WriteString(i.InputDigestHash)

	return b.String()
}

// Hash returns the SHA256 hash of the canonical string.
func (i *ShadowInput) Hash() string {
	h := sha256.Sum256([]byte(i.CanonicalString()))
	return hex.EncodeToString(h[:])
}

// ValidationError represents a privacy validation error.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return "privacy validation failed: " + e.Field + ": " + e.Message
}
