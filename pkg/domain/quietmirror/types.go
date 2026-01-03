// Package quietmirror provides types for the Quiet Inbox Mirror.
//
// Phase 22: Quiet Inbox Mirror (First Real Value Moment)
//
// This package defines the abstract reflection of Gmail activity.
// It proves the system is working WITHOUT showing content, urgency, or actions.
//
// CRITICAL INVARIANTS:
//   - Abstraction over explanation
//   - No email subjects, senders, timestamps, or counts
//   - Magnitude buckets only (nothing | a_few | several)
//   - Category buckets only (work | time | money | people | home)
//   - One calm, ignorable statement
//   - Deterministic output
//   - No LLM usage
//   - Pipe-delimited canonical strings
//   - SHA256 hashing
//
// Reference: docs/ADR/ADR-0052-phase22-quiet-inbox-mirror.md
package quietmirror

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
)

// MirrorMagnitude represents the abstract magnitude of observed activity.
// Never a count - only abstract buckets.
type MirrorMagnitude string

const (
	// MagnitudeNothing indicates no notable patterns observed.
	MagnitudeNothing MirrorMagnitude = "nothing"

	// MagnitudeAFew indicates a small amount of patterns observed.
	MagnitudeAFew MirrorMagnitude = "a_few"

	// MagnitudeSeveral indicates a moderate amount of patterns observed.
	MagnitudeSeveral MirrorMagnitude = "several"
)

// DisplayText returns human-readable text for the magnitude.
func (m MirrorMagnitude) DisplayText() string {
	switch m {
	case MagnitudeNothing:
		return "Nothing"
	case MagnitudeAFew:
		return "A few"
	case MagnitudeSeveral:
		return "Several"
	default:
		return ""
	}
}

// MirrorCategory represents an abstract category of observed patterns.
type MirrorCategory string

const (
	// CategoryWork represents work-related patterns.
	CategoryWork MirrorCategory = "work"

	// CategoryTime represents time-sensitive patterns.
	CategoryTime MirrorCategory = "time"

	// CategoryMoney represents financial patterns.
	CategoryMoney MirrorCategory = "money"

	// CategoryPeople represents people-related patterns.
	CategoryPeople MirrorCategory = "people"

	// CategoryHome represents home/personal patterns.
	CategoryHome MirrorCategory = "home"
)

// DisplayText returns human-readable text for the category.
func (c MirrorCategory) DisplayText() string {
	switch c {
	case CategoryWork:
		return "Work"
	case CategoryTime:
		return "Time"
	case CategoryMoney:
		return "Money"
	case CategoryPeople:
		return "People"
	case CategoryHome:
		return "Home"
	default:
		return ""
	}
}

// MirrorStatement represents a single calm statement about the mirror.
// This is the ONLY text shown to the circle - no explanations, no details.
type MirrorStatement struct {
	// Text is the calm statement text.
	Text string

	// StatementKind identifies the type of statement for determinism.
	StatementKind StatementKind
}

// StatementKind identifies the type of calm statement.
type StatementKind string

const (
	// StatementKindNothing indicates nothing needs attention.
	StatementKindNothing StatementKind = "nothing"

	// StatementKindWatching indicates things are being watched.
	StatementKindWatching StatementKind = "watching"

	// StatementKindPatterns indicates patterns are being observed.
	StatementKindPatterns StatementKind = "patterns"
)

// QuietMirrorSummary is the abstract reflection of Gmail activity.
//
// CRITICAL: This contains NO identifiable information:
//   - No email subjects
//   - No senders
//   - No timestamps
//   - No counts
//   - No actions
type QuietMirrorSummary struct {
	// CircleID identifies the circle this summary belongs to.
	CircleID string

	// Period is the time period bucket (e.g., "2024-01-15").
	// This is a bucket, not a precise timestamp.
	Period string

	// Magnitude is the abstract activity level.
	Magnitude MirrorMagnitude

	// Categories are the observed category patterns (max 3).
	// Sorted alphabetically for determinism.
	Categories []MirrorCategory

	// Statement is the single calm statement.
	Statement MirrorStatement

	// HasMirror indicates if there's anything to show.
	// False means "Nothing needs you" - which is still valuable.
	HasMirror bool

	// SourceHash is a hash of the input data for replay verification.
	// Never contains identifiable information.
	SourceHash string
}

// Hash computes a deterministic SHA256 hash of the summary.
// Uses pipe-delimited canonical string format.
func (s *QuietMirrorSummary) Hash() string {
	canonical := s.CanonicalString()
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:])
}

// CanonicalString returns a pipe-delimited canonical representation.
// Format: QUIET_MIRROR|v1|circle|period|magnitude|categories|statement_kind|has_mirror|source_hash
func (s *QuietMirrorSummary) CanonicalString() string {
	// Sort categories for determinism
	cats := make([]string, len(s.Categories))
	for i, c := range s.Categories {
		cats[i] = string(c)
	}
	sort.Strings(cats)

	catStr := ""
	for i, c := range cats {
		if i > 0 {
			catStr += ","
		}
		catStr += c
	}

	hasMirror := "false"
	if s.HasMirror {
		hasMirror = "true"
	}

	return "QUIET_MIRROR|v1|" +
		s.CircleID + "|" +
		s.Period + "|" +
		string(s.Magnitude) + "|" +
		catStr + "|" +
		string(s.Statement.StatementKind) + "|" +
		hasMirror + "|" +
		s.SourceHash
}

// QuietMirrorInput contains the abstract inputs for computing a mirror.
//
// CRITICAL: This must NEVER contain raw Gmail fields.
// Only magnitude buckets and category presence are allowed.
type QuietMirrorInput struct {
	// CircleID identifies the circle.
	CircleID string

	// Period is the time period bucket.
	Period string

	// HasConnection indicates if Gmail is connected.
	HasConnection bool

	// HasSyncReceipt indicates if a sync has occurred.
	HasSyncReceipt bool

	// SyncReceiptHash is the hash of the sync receipt (for source tracking).
	SyncReceiptHash string

	// ObligationMagnitude is the abstract count of obligations.
	ObligationMagnitude MirrorMagnitude

	// CategoryPresence indicates which categories have activity.
	// This is boolean presence, not counts.
	CategoryPresence map[MirrorCategory]bool
}

// SourceHash computes a hash of the input for tracking.
func (i *QuietMirrorInput) SourceHash() string {
	// Sort categories for determinism
	var cats []string
	for cat, present := range i.CategoryPresence {
		if present {
			cats = append(cats, string(cat))
		}
	}
	sort.Strings(cats)

	catStr := ""
	for j, c := range cats {
		if j > 0 {
			catStr += ","
		}
		catStr += c
	}

	hasConn := "false"
	if i.HasConnection {
		hasConn = "true"
	}

	hasSync := "false"
	if i.HasSyncReceipt {
		hasSync = "true"
	}

	canonical := "MIRROR_INPUT|v1|" +
		i.CircleID + "|" +
		i.Period + "|" +
		hasConn + "|" +
		hasSync + "|" +
		i.SyncReceiptHash + "|" +
		string(i.ObligationMagnitude) + "|" +
		catStr

	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:])
}

// QuietMirrorPage contains the UI display data for /mirror/inbox.
//
// CRITICAL: No actions, no buttons, no lists, no counts.
// This is proof of care, not a dashboard.
type QuietMirrorPage struct {
	// Title is the page title ("Seen, quietly.")
	Title string

	// Statement is the single calm statement.
	Statement string

	// Categories are the abstract category chips (max 3).
	Categories []string

	// Footer is the reassurance text.
	Footer string

	// HasContent indicates if there's anything to show.
	HasContent bool

	// SummaryHash is the hash for audit/replay.
	SummaryHash string
}

// NewEmptyPage creates an empty page for when there's no mirror.
func NewEmptyPage() *QuietMirrorPage {
	return &QuietMirrorPage{
		Title:      "Seen, quietly.",
		Statement:  "Nothing here needs you today.",
		Categories: nil,
		Footer:     "We're watching so you don't have to.",
		HasContent: false,
	}
}

// NewMirrorPage creates a page from a summary.
func NewMirrorPage(summary *QuietMirrorSummary) *QuietMirrorPage {
	cats := make([]string, len(summary.Categories))
	for i, c := range summary.Categories {
		cats[i] = c.DisplayText()
	}

	return &QuietMirrorPage{
		Title:       "Seen, quietly.",
		Statement:   summary.Statement.Text,
		Categories:  cats,
		Footer:      "We're watching so you don't have to.",
		HasContent:  summary.HasMirror,
		SummaryHash: summary.Hash(),
	}
}
