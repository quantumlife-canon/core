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
