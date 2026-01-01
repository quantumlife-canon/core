// Package todayquietly provides the "Today, quietly." projection engine.
//
// Phase 18.2: Recognition + Suppression + Preference
//
// CRITICAL: This is NOT an interruption list. No action buttons.
// CRITICAL: Observations are mirrors, not commands.
// CRITICAL: Deterministic output for same inputs + same clock.
//
// Reference: Phase 18.2 specification
package todayquietly

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

// TodayQuietlyPage represents the full page model.
type TodayQuietlyPage struct {
	// Title is always "Today, quietly."
	Title string

	// Subtitle is always "Nothing needs you — unless it truly does."
	Subtitle string

	// Recognition is a single calm, non-actionable sentence.
	// Must NOT include counts.
	Recognition string

	// Observations are exactly 3 quiet observations (non-actionable mirrors).
	Observations []QuietObservation

	// SuppressedInsight is exactly 1 suppressed insight (demonstrates restraint).
	SuppressedInsight SuppressedInsight

	// PermissionPivot contains the preference choice options.
	PermissionPivot PermissionPivot

	// PageHash is a deterministic hash of the page content.
	PageHash string

	// GeneratedAt is when this page was generated.
	GeneratedAt time.Time
}

// QuietObservation represents a single non-actionable mirror.
type QuietObservation struct {
	// ID is a deterministic hash for this observation.
	ID string

	// Text is the observation text. Non-actionable, no dates, no names,
	// no verbs like "do", "respond now", "pay now".
	Text string

	// Signal identifies what signal triggered this observation.
	// Used for deterministic ordering.
	Signal string
}

// SuppressedInsight represents something deliberately not surfaced.
type SuppressedInsight struct {
	// Title is always "There's one thing we chose not to surface yet."
	Title string

	// Reason is always "Because it doesn't need you today."
	Reason string
}

// PermissionPivot represents the preference capture options.
type PermissionPivot struct {
	// Prompt is the question shown on the page.
	Prompt string

	// Choices are the available options (exactly 2).
	Choices []PermissionChoice

	// DefaultChoice is the mode that should be pre-selected.
	DefaultChoice string
}

// PermissionChoice represents a single preference option.
type PermissionChoice struct {
	// Mode is either "quiet" or "show_all".
	Mode string

	// Label is the display text.
	Label string

	// IsDefault indicates if this is the default choice.
	IsDefault bool
}

// PreferenceRecord represents a stored preference.
type PreferenceRecord struct {
	// Mode is "quiet" or "show_all".
	Mode string

	// RecordedAt is when this preference was recorded.
	RecordedAt time.Time

	// Hash is the SHA256 hash of the canonical record string.
	Hash string

	// Source identifies where this preference came from.
	Source string
}

// ProjectionInput contains the signals used to generate the page.
type ProjectionInput struct {
	// HasWorkObligations indicates work-related obligations exist.
	HasWorkObligations bool

	// HasFamilyObligations indicates family-related obligations exist.
	HasFamilyObligations bool

	// HasFinanceObligations indicates finance-related obligations exist.
	HasFinanceObligations bool

	// HasCalendarCommitments indicates calendar commitments exist.
	HasCalendarCommitments bool

	// HasOpenConversations indicates open/pending conversations exist.
	HasOpenConversations bool

	// HasImportantNotTimeSensitive indicates important but non-urgent items exist.
	HasImportantNotTimeSensitive bool

	// CircleCount is the number of active circles.
	CircleCount int

	// Now is the current time (injected for determinism).
	Now time.Time
}

// Hash computes a deterministic hash for the projection input.
func (p *ProjectionInput) Hash() string {
	canonical := fmt.Sprintf(
		"work:%t|family:%t|finance:%t|calendar:%t|conversations:%t|important:%t|circles:%d|time:%s",
		p.HasWorkObligations,
		p.HasFamilyObligations,
		p.HasFinanceObligations,
		p.HasCalendarCommitments,
		p.HasOpenConversations,
		p.HasImportantNotTimeSensitive,
		p.CircleCount,
		p.Now.Format(time.RFC3339),
	)
	h := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(h[:])
}

// ComputePageHash computes a deterministic hash of the page content.
func (p *TodayQuietlyPage) ComputePageHash() string {
	var parts []string
	parts = append(parts, p.Title)
	parts = append(parts, p.Subtitle)
	parts = append(parts, p.Recognition)

	for _, obs := range p.Observations {
		parts = append(parts, obs.ID)
		parts = append(parts, obs.Text)
	}

	parts = append(parts, p.SuppressedInsight.Title)
	parts = append(parts, p.SuppressedInsight.Reason)
	parts = append(parts, p.GeneratedAt.Format(time.RFC3339))

	canonical := strings.Join(parts, "|")
	h := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(h[:])
}

// computeObservationID computes a deterministic ID for an observation.
func computeObservationID(text, signal string) string {
	canonical := fmt.Sprintf("obs|%s|%s", signal, text)
	h := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(h[:16]) // First 16 bytes = 32 hex chars
}

// sortObservations sorts observations deterministically by signal then text.
func sortObservations(obs []QuietObservation) {
	sort.Slice(obs, func(i, j int) bool {
		if obs[i].Signal == obs[j].Signal {
			return obs[i].Text < obs[j].Text
		}
		return obs[i].Signal < obs[j].Signal
	})
}
