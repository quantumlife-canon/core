// Package interruptions - deduplication logic.
//
// Dedup prevents the same interruption from being surfaced repeatedly
// within a time window. Uses deterministic bucket-based keys.
//
// CRITICAL: Deterministic. Same inputs = same dedup decisions.
// CRITICAL: Synchronous. No goroutines.
package interruptions

import (
	"quantumlife/pkg/domain/interrupt"
)

// DedupStore tracks seen dedup keys.
type DedupStore interface {
	// HasSeen returns true if the key was already seen.
	HasSeen(key string) bool

	// MarkSeen marks a key as seen.
	MarkSeen(key string)

	// Clear removes all entries.
	Clear()

	// Count returns number of tracked keys.
	Count() int
}

// InMemoryDeduper implements DedupStore with in-memory storage.
type InMemoryDeduper struct {
	seen map[string]bool
}

// NewInMemoryDeduper creates a new in-memory deduper.
func NewInMemoryDeduper() *InMemoryDeduper {
	return &InMemoryDeduper{
		seen: make(map[string]bool),
	}
}

// HasSeen checks if a key was already seen.
func (d *InMemoryDeduper) HasSeen(key string) bool {
	return d.seen[key]
}

// MarkSeen marks a key as seen.
func (d *InMemoryDeduper) MarkSeen(key string) {
	d.seen[key] = true
}

// Clear removes all entries.
func (d *InMemoryDeduper) Clear() {
	d.seen = make(map[string]bool)
}

// Count returns the number of tracked keys.
func (d *InMemoryDeduper) Count() int {
	return len(d.seen)
}

// Dedup filters out duplicate interruptions.
// Returns (kept, dropped count).
func Dedup(interruptions []*interrupt.Interruption, store DedupStore) ([]*interrupt.Interruption, int) {
	var kept []*interrupt.Interruption
	dropped := 0

	for _, i := range interruptions {
		if store.HasSeen(i.DedupKey) {
			dropped++
			continue
		}
		store.MarkSeen(i.DedupKey)
		kept = append(kept, i)
	}

	return kept, dropped
}

// Verify interface compliance.
var _ DedupStore = (*InMemoryDeduper)(nil)
