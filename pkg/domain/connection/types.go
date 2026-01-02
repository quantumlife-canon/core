// Package connection provides domain types for connection management.
//
// Phase 18.6: First Connect (Consent-first Onboarding)
//
// CRITICAL: No OAuth libs, no third-party auth libs.
// CRITICAL: No goroutines. No time.Now(). stdlib-only.
// CRITICAL: Canonical strings are pipe-delimited, not JSON.
// CRITICAL: No storing of raw emails/secrets in connection intents.
//
// Reference: docs/ADR/ADR-0038-phase18-6-first-connect.md
package connection

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"
)

// ConnectionKind identifies what type of source can be connected.
type ConnectionKind string

const (
	KindEmail    ConnectionKind = "email"
	KindCalendar ConnectionKind = "calendar"
	KindFinance  ConnectionKind = "finance"
)

// AllKinds returns all connection kinds in deterministic order.
func AllKinds() []ConnectionKind {
	return []ConnectionKind{KindEmail, KindCalendar, KindFinance}
}

// Valid returns true if the kind is a recognized connection kind.
func (k ConnectionKind) Valid() bool {
	switch k {
	case KindEmail, KindCalendar, KindFinance:
		return true
	default:
		return false
	}
}

// String returns the string representation of the kind.
func (k ConnectionKind) String() string {
	return string(k)
}

// ConnectionStatus represents the current state of a connection.
type ConnectionStatus string

const (
	StatusNotConnected  ConnectionStatus = "not_connected"
	StatusConnectedMock ConnectionStatus = "connected_mock"
	StatusConnectedReal ConnectionStatus = "connected_real"
	StatusNeedsConfig   ConnectionStatus = "needs_config"
)

// String returns the string representation of the status.
func (s ConnectionStatus) String() string {
	return string(s)
}

// DisplayText returns human-readable text for the status.
func (s ConnectionStatus) DisplayText() string {
	switch s {
	case StatusNotConnected:
		return "Not connected"
	case StatusConnectedMock:
		return "Connected (mock)"
	case StatusConnectedReal:
		return "Connected"
	case StatusNeedsConfig:
		return "Needs configuration"
	default:
		return "Unknown"
	}
}

// IntentAction identifies what action an intent represents.
type IntentAction string

const (
	ActionConnect    IntentAction = "connect"
	ActionDisconnect IntentAction = "disconnect"
)

// String returns the string representation of the action.
func (a IntentAction) String() string {
	return string(a)
}

// IntentMode identifies whether the intent is for mock or real mode.
type IntentMode string

const (
	ModeMock IntentMode = "mock"
	ModeReal IntentMode = "real"
)

// String returns the string representation of the mode.
func (m IntentMode) String() string {
	return string(m)
}

// IntentNote is a bounded, fixed set of values for intent notes.
// No free text allowed - only predefined values.
type IntentNote string

const (
	NoteUserInitiated  IntentNote = "user_initiated"
	NoteSystemReset    IntentNote = "system_reset"
	NoteConfigRequired IntentNote = "config_required"
)

// String returns the string representation of the note.
func (n IntentNote) String() string {
	return string(n)
}

// ConnectionIntent represents an intent to connect or disconnect a source.
// Intents are append-only and form a replayable log.
type ConnectionIntent struct {
	// ID is a deterministic hash of the canonical string.
	ID string

	// Kind identifies what type of source.
	Kind ConnectionKind

	// Action is connect or disconnect.
	Action IntentAction

	// Mode is mock or real.
	Mode IntentMode

	// At is when this intent was created (from injected clock).
	At time.Time

	// Note is a bounded note value (no free text).
	Note IntentNote
}

// CanonicalString returns the pipe-delimited canonical representation.
// Format: CONN_INTENT|v1|kind|action|mode|atRFC3339|note
func (i *ConnectionIntent) CanonicalString() string {
	var b strings.Builder
	b.WriteString("CONN_INTENT")
	b.WriteString("|v1|")
	b.WriteString(string(i.Kind))
	b.WriteString("|")
	b.WriteString(string(i.Action))
	b.WriteString("|")
	b.WriteString(string(i.Mode))
	b.WriteString("|")
	b.WriteString(i.At.UTC().Format(time.RFC3339))
	b.WriteString("|")
	b.WriteString(string(i.Note))
	return b.String()
}

// Hash returns the SHA256 hash of the canonical string.
func (i *ConnectionIntent) Hash() string {
	h := sha256.Sum256([]byte(i.CanonicalString()))
	return hex.EncodeToString(h[:])
}

// ComputeID computes and sets the ID from the canonical string hash.
func (i *ConnectionIntent) ComputeID() {
	i.ID = i.Hash()
}

// NewConnectIntent creates a new connect intent.
func NewConnectIntent(kind ConnectionKind, mode IntentMode, at time.Time, note IntentNote) *ConnectionIntent {
	intent := &ConnectionIntent{
		Kind:   kind,
		Action: ActionConnect,
		Mode:   mode,
		At:     at,
		Note:   note,
	}
	intent.ComputeID()
	return intent
}

// NewDisconnectIntent creates a new disconnect intent.
func NewDisconnectIntent(kind ConnectionKind, mode IntentMode, at time.Time, note IntentNote) *ConnectionIntent {
	intent := &ConnectionIntent{
		Kind:   kind,
		Action: ActionDisconnect,
		Mode:   mode,
		At:     at,
		Note:   note,
	}
	intent.ComputeID()
	return intent
}

// ParseCanonicalIntent parses a canonical string into a ConnectionIntent.
func ParseCanonicalIntent(s string) (*ConnectionIntent, error) {
	parts := strings.Split(s, "|")
	if len(parts) != 7 {
		return nil, ErrInvalidCanonical
	}
	if parts[0] != "CONN_INTENT" || parts[1] != "v1" {
		return nil, ErrInvalidCanonical
	}

	at, err := time.Parse(time.RFC3339, parts[5])
	if err != nil {
		return nil, ErrInvalidCanonical
	}

	intent := &ConnectionIntent{
		Kind:   ConnectionKind(parts[2]),
		Action: IntentAction(parts[3]),
		Mode:   IntentMode(parts[4]),
		At:     at,
		Note:   IntentNote(parts[6]),
	}
	intent.ComputeID()
	return intent, nil
}

// IntentError represents an error in intent processing.
type IntentError string

func (e IntentError) Error() string { return string(e) }

// Intent errors.
var (
	ErrInvalidCanonical = IntentError("invalid canonical string format")
	ErrInvalidKind      = IntentError("invalid connection kind")
)

// IntentList is a list of intents with deterministic sorting.
type IntentList []*ConnectionIntent

// Sort sorts intents deterministically by timestamp, then by hash for tie-break.
func (l IntentList) Sort() {
	sort.Slice(l, func(i, j int) bool {
		if l[i].At.Equal(l[j].At) {
			return l[i].ID < l[j].ID // Lexical tie-break
		}
		return l[i].At.Before(l[j].At)
	})
}

// ByKind returns intents filtered by kind, sorted.
func (l IntentList) ByKind(kind ConnectionKind) IntentList {
	var result IntentList
	for _, intent := range l {
		if intent.Kind == kind {
			result = append(result, intent)
		}
	}
	result.Sort()
	return result
}

// Last returns the last intent for a given kind, or nil if none.
func (l IntentList) Last(kind ConnectionKind) *ConnectionIntent {
	filtered := l.ByKind(kind)
	if len(filtered) == 0 {
		return nil
	}
	return filtered[len(filtered)-1]
}
