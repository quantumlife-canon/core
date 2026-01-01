// Package proof implements the Quiet Proof (Restraint Ledger) for Phase 18.5.
// It provides abstract proof that QuantumLife withheld interruptions.
//
// Core principles:
// - No raw counts, no dates, no vendors, no names, no IDs
// - Magnitude buckets only: nothing / a_few / several
// - Categories limited: money, time, work, people, home
// - Deterministic output ordering and hashing
// - stdlib only
package proof

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
)

// Category represents abstract life domains.
type Category string

const (
	CategoryMoney  Category = "money"
	CategoryTime   Category = "time"
	CategoryWork   Category = "work"
	CategoryPeople Category = "people"
	CategoryHome   Category = "home"
)

// Magnitude represents abstract quantity buckets.
// NEVER raw counts - only these three buckets.
type Magnitude string

const (
	MagnitudeNothing Magnitude = "nothing"
	MagnitudeAFew    Magnitude = "a_few"
	MagnitudeSeveral Magnitude = "several"
)

// ProofSummary is the abstract proof of restraint.
// Contains no identifiers, no counts, no dates.
type ProofSummary struct {
	Magnitude  Magnitude  // nothing / a_few / several
	Categories []Category // sorted lexicographically
	Statement  string     // calm, abstract copy
	WhyLine    string     // optional short reassurance
	Hash       string     // SHA256 of canonical string
}

// ProofInput provides the data needed to compute proof.
// Counts are used internally for bucketing only - never exposed.
type ProofInput struct {
	// SuppressedByCategory maps category to suppressed count.
	// Engine will bucket these; counts are never shown.
	SuppressedByCategory map[Category]int

	// PreferenceQuiet indicates if user prefers quiet mode.
	PreferenceQuiet bool

	// Period is the time window (e.g., "week").
	// No dates are ever shown - just the abstract period.
	Period string
}

// CanonicalString returns the deterministic string representation
// used for hashing. Format: PROOF|v1|<magnitude>|<cat1,cat2,...>|<statement>
func (p ProofSummary) CanonicalString() string {
	cats := make([]string, len(p.Categories))
	for i, c := range p.Categories {
		cats[i] = string(c)
	}
	return fmt.Sprintf("PROOF|v1|%s|%s|%s",
		p.Magnitude,
		strings.Join(cats, ","),
		p.Statement,
	)
}

// ComputeHash calculates SHA256 hash of the canonical string.
func (p ProofSummary) ComputeHash() string {
	h := sha256.Sum256([]byte(p.CanonicalString()))
	return fmt.Sprintf("%x", h)
}

// categoryOrder defines deterministic sort order for categories.
var categoryOrder = map[Category]int{
	CategoryHome:   0,
	CategoryMoney:  1,
	CategoryPeople: 2,
	CategoryTime:   3,
	CategoryWork:   4,
}

// SortCategories returns categories sorted lexicographically.
func SortCategories(cats []Category) []Category {
	sorted := make([]Category, len(cats))
	copy(sorted, cats)
	sort.Slice(sorted, func(i, j int) bool {
		return categoryOrder[sorted[i]] < categoryOrder[sorted[j]]
	})
	return sorted
}

// AckAction represents acknowledgement actions.
type AckAction string

const (
	AckDismissed AckAction = "dismissed"
	AckViewed    AckAction = "viewed"
)

// AckRecord represents an acknowledgement record.
// Only hashes are stored, never raw content.
type AckRecord struct {
	Action    AckAction
	ProofHash string
	TSHash    string // Hash of timestamp, not raw timestamp
}

// CanonicalString returns the deterministic string for hashing.
func (r AckRecord) CanonicalString() string {
	return fmt.Sprintf("ACK|v1|%s|%s|%s", r.Action, r.ProofHash, r.TSHash)
}

// ComputeRecordHash calculates SHA256 of the record.
func (r AckRecord) ComputeRecordHash() string {
	h := sha256.Sum256([]byte(r.CanonicalString()))
	return fmt.Sprintf("%x", h)
}
