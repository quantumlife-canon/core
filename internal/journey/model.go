// Package journey provides the guided journey flow for first-time setup.
//
// Phase 26A: Guided Journey (Product/UX)
//
// A single guided journey that makes the system feel coherent in <5 minutes:
// Start → Connect → Sync → Today → (optional proof/mirror) → One reversible action → Done
//
// CRITICAL INVARIANTS:
//   - stdlib only (no external deps)
//   - no goroutines
//   - no time.Now() - clock injection only
//   - no auto-retries, no background polling
//   - deterministic: same inputs + same clock => same outputs
//   - privacy: no identifiable info (no subjects, senders, vendors, amounts)
//   - magnitude buckets only
//   - hash-only persistence
//   - ONE whisper max on Today page (single whisper rule)
//
// Reference: docs/ADR/ADR-0056-phase26A-guided-journey.md
package journey

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"quantumlife/internal/persist"
)

// StepKind identifies which step in the journey.
type StepKind string

const (
	StepConnect StepKind = "step_connect"
	StepSync    StepKind = "step_sync"
	StepMirror  StepKind = "step_mirror"
	StepToday   StepKind = "step_today"
	StepAction  StepKind = "step_action"
	StepDone    StepKind = "step_done"
)

// String returns the string representation.
func (s StepKind) String() string {
	return string(s)
}

// AllSteps returns all steps in order.
func AllSteps() []StepKind {
	return []StepKind{StepConnect, StepSync, StepMirror, StepToday, StepAction, StepDone}
}

// StepIndex returns the 1-based index of a step, or 0 if done.
func StepIndex(s StepKind) int {
	switch s {
	case StepConnect:
		return 1
	case StepSync:
		return 2
	case StepMirror:
		return 3
	case StepToday:
		return 4
	case StepAction:
		return 5
	case StepDone:
		return 0
	default:
		return 0
	}
}

// TotalSteps returns the total number of steps (excluding done).
const TotalSteps = 5

// JourneyAction represents a button action on the journey page.
type JourneyAction struct {
	// Label is the button text.
	Label string

	// Method is "GET" or "POST".
	Method string

	// Path is the target URL path.
	Path string

	// FormFields are hidden form fields for POST actions.
	FormFields map[string]string
}

// JourneyPage represents the rendered journey page.
type JourneyPage struct {
	// Title is the main heading.
	Title string

	// Subtitle is the secondary heading.
	Subtitle string

	// Lines are body text lines.
	Lines []string

	// PrimaryAction is the main CTA button.
	PrimaryAction JourneyAction

	// SecondaryAction is the "Not now" or "Skip" action (optional).
	SecondaryAction *JourneyAction

	// StepLabel describes progress (e.g., "Step 2 of 5").
	StepLabel string

	// CurrentStep is the step kind.
	CurrentStep StepKind

	// StatusHash is a deterministic hash of this page state.
	StatusHash string

	// IsDone indicates the journey is complete or dismissed.
	IsDone bool
}

// JourneyInputs contains all inputs needed to compute the journey state.
// All inputs are abstract (no identifiable info).
type JourneyInputs struct {
	// CircleID is the circle being configured.
	CircleID string

	// HasGmail indicates if Gmail is connected.
	HasGmail bool

	// GmailMode is "mock" or "real" if connected.
	GmailMode string

	// HasSyncReceipt indicates if any sync has been done.
	HasSyncReceipt bool

	// LastSyncMagnitude is the magnitude bucket of the last sync.
	LastSyncMagnitude persist.MagnitudeBucket

	// MirrorViewed indicates if the inbox mirror was viewed this period.
	MirrorViewed bool

	// ActionEligible indicates if a Phase 25 undoable action is eligible.
	ActionEligible bool

	// ActionUsedThisPeriod indicates if an action was already used today.
	ActionUsedThisPeriod bool

	// UndoAvailable indicates if an undo is available.
	UndoAvailable bool

	// DismissedStatusHash is the status hash of the dismissed journey (if any).
	DismissedStatusHash string

	// Now is the current bucketed time (for period computation).
	Now time.Time
}

// PeriodKey returns the daily period key for journey dismissals.
// Format: YYYY-MM-DD
func (i *JourneyInputs) PeriodKey() string {
	return i.Now.UTC().Format("2006-01-02")
}

// ComputeStatusHash computes a deterministic hash of the inputs.
// This is used to detect material state changes.
func (i *JourneyInputs) ComputeStatusHash() string {
	var b strings.Builder
	b.WriteString("JOURNEY_STATUS|v1|")
	b.WriteString(i.CircleID)
	b.WriteString("|")
	if i.HasGmail {
		b.WriteString("gmail:")
		b.WriteString(i.GmailMode)
	} else {
		b.WriteString("no_gmail")
	}
	b.WriteString("|")
	if i.HasSyncReceipt {
		b.WriteString("synced:")
		b.WriteString(string(i.LastSyncMagnitude))
	} else {
		b.WriteString("no_sync")
	}
	b.WriteString("|")
	if i.MirrorViewed {
		b.WriteString("mirror_viewed")
	} else {
		b.WriteString("mirror_not_viewed")
	}
	b.WriteString("|")
	if i.ActionEligible {
		b.WriteString("action_eligible")
	} else {
		b.WriteString("action_not_eligible")
	}
	b.WriteString("|")
	if i.ActionUsedThisPeriod {
		b.WriteString("action_used")
	} else {
		b.WriteString("action_not_used")
	}
	b.WriteString("|")
	b.WriteString(i.PeriodKey())

	h := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(h[:16]) // 32 hex chars
}

// JourneyDismissal represents a journey dismissal record.
type JourneyDismissal struct {
	// CircleID is the circle this dismissal applies to.
	CircleID string

	// PeriodKey is the daily period (YYYY-MM-DD).
	PeriodKey string

	// StatusHash is the status hash at time of dismissal.
	StatusHash string

	// DismissedAt is the bucketed dismissal time.
	DismissedAt time.Time
}

// CanonicalString returns the pipe-delimited canonical representation.
func (d *JourneyDismissal) CanonicalString() string {
	var b strings.Builder
	b.WriteString("JOURNEY_DISMISS|v1|")
	b.WriteString(d.CircleID)
	b.WriteString("|")
	b.WriteString(d.PeriodKey)
	b.WriteString("|")
	b.WriteString(d.StatusHash)
	b.WriteString("|")
	// Bucket to 5-minute granularity
	bucket := d.DismissedAt.Truncate(5 * time.Minute)
	b.WriteString(bucket.UTC().Format(time.RFC3339))
	return b.String()
}

// Hash returns the SHA256 hash of the canonical string.
func (d *JourneyDismissal) Hash() string {
	h := sha256.Sum256([]byte(d.CanonicalString()))
	return hex.EncodeToString(h[:])
}

// NewJourneyDismissal creates a new dismissal record.
func NewJourneyDismissal(circleID, periodKey, statusHash string, now time.Time) *JourneyDismissal {
	return &JourneyDismissal{
		CircleID:    circleID,
		PeriodKey:   periodKey,
		StatusHash:  statusHash,
		DismissedAt: now,
	}
}
