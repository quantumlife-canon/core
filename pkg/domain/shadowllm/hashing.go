// Package shadowllm provides canonical hashing for shadow-mode types.
//
// CRITICAL: All canonical strings are pipe-delimited, NOT JSON.
// Pattern: "PREFIX|version|field1|field2|...|fieldN"
//
// This ensures deterministic hashing across Go versions and platforms.
package shadowllm

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
)

// Hash returns the canonical SHA256 hash of the ShadowRun.
// This hash is deterministic given the same inputs.
func (r *ShadowRun) Hash() string {
	if r.hash != "" {
		return r.hash
	}
	r.hash = computeHash(r.CanonicalString())
	return r.hash
}

// CanonicalString returns the pipe-delimited canonical representation.
// Format: SHADOW_RUN|v1|run_id|circle_id|inputs_hash|model_spec|seed|created_at|signals_hash
func (r *ShadowRun) CanonicalString() string {
	var b strings.Builder
	b.WriteString("SHADOW_RUN|v1|")
	b.WriteString(r.RunID)
	b.WriteString("|")
	b.WriteString(string(r.CircleID))
	b.WriteString("|")
	b.WriteString(r.InputsHash)
	b.WriteString("|")
	b.WriteString(r.ModelSpec)
	b.WriteString("|")
	b.WriteString(strconv.FormatInt(r.Seed, 10))
	b.WriteString("|")
	b.WriteString(r.CreatedAt.UTC().Format(time.RFC3339Nano))
	b.WriteString("|")
	b.WriteString(r.computeSignalsHash())
	return b.String()
}

// computeSignalsHash computes a hash of all signals in deterministic order.
func (r *ShadowRun) computeSignalsHash() string {
	if len(r.Signals) == 0 {
		return "empty"
	}

	var b strings.Builder
	for i, sig := range r.Signals {
		if i > 0 {
			b.WriteString(";")
		}
		b.WriteString(sig.CanonicalString())
	}
	return computeHash(b.String())
}

// CanonicalString returns the pipe-delimited canonical representation.
// Format: SHADOW_SIGNAL|v1|kind|circle_id|item_key_hash|category|value|confidence|notes_hash|created_at
func (s *ShadowSignal) CanonicalString() string {
	var b strings.Builder
	b.WriteString("SHADOW_SIGNAL|v1|")
	b.WriteString(string(s.Kind))
	b.WriteString("|")
	b.WriteString(string(s.CircleID))
	b.WriteString("|")
	b.WriteString(s.ItemKeyHash)
	b.WriteString("|")
	b.WriteString(string(s.Category))
	b.WriteString("|")
	b.WriteString(formatFloat(s.ValueFloat))
	b.WriteString("|")
	b.WriteString(formatFloat(s.ConfidenceFloat))
	b.WriteString("|")
	b.WriteString(s.NotesHash)
	b.WriteString("|")
	b.WriteString(s.CreatedAt.UTC().Format(time.RFC3339Nano))
	return b.String()
}

// Hash returns the canonical SHA256 hash of the ShadowSignal.
func (s *ShadowSignal) Hash() string {
	return computeHash(s.CanonicalString())
}

// computeHash computes SHA256 hash of a string.
func computeHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// formatFloat formats a float64 with fixed precision for determinism.
// Uses 6 decimal places to ensure consistent representation.
func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', 6, 64)
}

// HashItemKey creates a SHA256 hash of an item key.
// This is used to obscure the actual item identifier.
func HashItemKey(itemKey string) string {
	return computeHash("ITEM_KEY|" + itemKey)
}

// HashNotes creates a SHA256 hash of notes content.
// This ensures notes content never leaks through signals.
func HashNotes(notes string) string {
	if notes == "" {
		return "empty"
	}
	return computeHash("NOTES|" + notes)
}

// HashInputs creates a SHA256 hash of abstract inputs.
// The inputs must already be abstract (buckets, counts) - never raw content.
func HashInputs(abstractInputs string) string {
	return computeHash("INPUTS|" + abstractInputs)
}

// =============================================================================
// Phase 19.2: ShadowReceipt and ShadowSuggestion Hashing
// =============================================================================

// Hash returns the canonical SHA256 hash of the ShadowReceipt.
// This hash is deterministic given the same inputs.
func (r *ShadowReceipt) Hash() string {
	if r.hash != "" {
		return r.hash
	}
	r.hash = computeHash(r.CanonicalString())
	return r.hash
}

// CanonicalString returns the pipe-delimited canonical representation.
// Format: SHADOW_RECEIPT|v1|receipt_id|circle_id|window_bucket|input_digest_hash|model_spec|created_at|suggestions_hash
func (r *ShadowReceipt) CanonicalString() string {
	var b strings.Builder
	b.WriteString("SHADOW_RECEIPT|v1|")
	b.WriteString(r.ReceiptID)
	b.WriteString("|")
	b.WriteString(string(r.CircleID))
	b.WriteString("|")
	b.WriteString(r.WindowBucket)
	b.WriteString("|")
	b.WriteString(r.InputDigestHash)
	b.WriteString("|")
	b.WriteString(r.ModelSpec)
	b.WriteString("|")
	b.WriteString(r.CreatedAt.UTC().Format(time.RFC3339Nano))
	b.WriteString("|")
	b.WriteString(r.computeSuggestionsHash())
	return b.String()
}

// computeSuggestionsHash computes a hash of all suggestions in deterministic order.
func (r *ShadowReceipt) computeSuggestionsHash() string {
	if len(r.Suggestions) == 0 {
		return "empty"
	}

	var b strings.Builder
	for i, sug := range r.Suggestions {
		if i > 0 {
			b.WriteString(";")
		}
		b.WriteString(sug.CanonicalString())
	}
	return computeHash(b.String())
}

// CanonicalString returns the pipe-delimited canonical representation.
// Format: SHADOW_SUGGESTION|v1|category|horizon|magnitude|confidence|suggestion_type|item_key_hash
func (s *ShadowSuggestion) CanonicalString() string {
	var b strings.Builder
	b.WriteString("SHADOW_SUGGESTION|v1|")
	b.WriteString(string(s.Category))
	b.WriteString("|")
	b.WriteString(string(s.Horizon))
	b.WriteString("|")
	b.WriteString(string(s.Magnitude))
	b.WriteString("|")
	b.WriteString(string(s.Confidence))
	b.WriteString("|")
	b.WriteString(string(s.SuggestionType))
	b.WriteString("|")
	b.WriteString(s.ItemKeyHash)
	return b.String()
}

// Hash returns the canonical SHA256 hash of the ShadowSuggestion.
func (s *ShadowSuggestion) Hash() string {
	return computeHash(s.CanonicalString())
}

// CanonicalString returns the pipe-delimited canonical representation of input digest.
// Format: SHADOW_INPUT_DIGEST|v1|circle_id|obl_buckets|held_buckets|surface|draft|triggers|mirror
func (d *ShadowInputDigest) CanonicalString() string {
	var b strings.Builder
	b.WriteString("SHADOW_INPUT_DIGEST|v1|")
	b.WriteString(string(d.CircleID))
	b.WriteString("|")

	// Obligation counts by category (sorted)
	for _, cat := range AllCategories() {
		mag := d.ObligationCountByCategory[cat]
		if mag == "" {
			mag = MagnitudeNothing
		}
		b.WriteString(string(cat))
		b.WriteString(":obl:")
		b.WriteString(string(mag))
		b.WriteString(",")
	}

	b.WriteString("|")

	// Held counts by category (sorted)
	for _, cat := range AllCategories() {
		mag := d.HeldCountByCategory[cat]
		if mag == "" {
			mag = MagnitudeNothing
		}
		b.WriteString(string(cat))
		b.WriteString(":held:")
		b.WriteString(string(mag))
		b.WriteString(",")
	}

	b.WriteString("|surf:")
	b.WriteString(string(d.SurfaceCandidateCount))
	b.WriteString("|draft:")
	b.WriteString(string(d.DraftCandidateCount))
	b.WriteString("|triggers:")
	if d.TriggersSeen {
		b.WriteString("true")
	} else {
		b.WriteString("false")
	}
	b.WriteString("|mirror:")
	b.WriteString(string(d.MirrorBucket))

	return b.String()
}

// Hash returns the canonical SHA256 hash of the ShadowInputDigest.
func (d *ShadowInputDigest) Hash() string {
	return computeHash(d.CanonicalString())
}
