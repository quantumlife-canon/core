// Package interrupt - hashing utilities.
//
// CRITICAL: All hashing uses canonical strings (NOT JSON).
// CRITICAL: Deterministic: same input = same output.
package interrupt

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// HashCanonical computes a deterministic hash from a prefix and canonical string.
// Returns first 16 hex chars of SHA256.
func HashCanonical(prefix, canonical string) string {
	input := fmt.Sprintf("%s|%s", prefix, canonical)
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:])[:16]
}

// ComputeInterruptionsHash computes a deterministic hash over sorted interruptions.
func ComputeInterruptionsHash(interruptions []*Interruption) string {
	if len(interruptions) == 0 {
		return "empty"
	}

	// Sort before hashing
	sorted := make([]*Interruption, len(interruptions))
	copy(sorted, interruptions)
	SortInterruptions(sorted)

	var parts []string
	for _, i := range sorted {
		parts = append(parts, i.CanonicalString())
	}

	canonical := strings.Join(parts, "\n")
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:])
}

// ComputeDedupKeyHash computes a hash of a dedup key for storage.
func ComputeDedupKeyHash(dedupKey string) string {
	hash := sha256.Sum256([]byte(dedupKey))
	return hex.EncodeToString(hash[:])[:16]
}

// DedupKeyCanonical generates canonical dedup key payload.
func DedupKeyCanonical(circleID, trigger, sourceRef, bucket string) string {
	return fmt.Sprintf("dedup|%s|%s|%s|%s", circleID, trigger, sourceRef, bucket)
}

// SortedMapKeys returns sorted keys from a string-keyed map.
// Used for deterministic iteration.
func SortedMapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
