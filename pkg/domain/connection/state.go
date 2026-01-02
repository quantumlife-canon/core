package connection

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// ConnectionState represents the computed state for a single connection kind.
type ConnectionState struct {
	// Kind identifies what type of source.
	Kind ConnectionKind

	// Status is the current computed status.
	Status ConnectionStatus

	// LastChangedAt is when the status last changed.
	LastChangedAt time.Time

	// LastIntentHash is the hash of the last intent that changed state.
	LastIntentHash string
}

// CanonicalString returns the pipe-delimited canonical representation.
func (s *ConnectionState) CanonicalString() string {
	var b strings.Builder
	b.WriteString("CONN_STATE")
	b.WriteString("|v1|")
	b.WriteString(string(s.Kind))
	b.WriteString("|")
	b.WriteString(string(s.Status))
	b.WriteString("|")
	b.WriteString(s.LastChangedAt.UTC().Format(time.RFC3339))
	b.WriteString("|")
	b.WriteString(s.LastIntentHash)
	return b.String()
}

// ConnectionStateSet represents the full set of connection states.
type ConnectionStateSet struct {
	// States maps kind to state.
	States map[ConnectionKind]*ConnectionState

	// Hash is the computed hash of the entire state set.
	Hash string
}

// NewConnectionStateSet creates an empty state set with all kinds initialized.
func NewConnectionStateSet() *ConnectionStateSet {
	set := &ConnectionStateSet{
		States: make(map[ConnectionKind]*ConnectionState),
	}
	// Initialize all kinds to NotConnected
	for _, kind := range AllKinds() {
		set.States[kind] = &ConnectionState{
			Kind:   kind,
			Status: StatusNotConnected,
		}
	}
	set.ComputeHash()
	return set
}

// Get returns the state for a kind.
func (s *ConnectionStateSet) Get(kind ConnectionKind) *ConnectionState {
	if state, ok := s.States[kind]; ok {
		return state
	}
	return nil
}

// List returns all states in deterministic order (email, calendar, finance).
func (s *ConnectionStateSet) List() []*ConnectionState {
	result := make([]*ConnectionState, 0, len(AllKinds()))
	for _, kind := range AllKinds() {
		if state, ok := s.States[kind]; ok {
			result = append(result, state)
		}
	}
	return result
}

// CanonicalString returns the canonical representation of the entire state set.
func (s *ConnectionStateSet) CanonicalString() string {
	var b strings.Builder
	b.WriteString("CONN_STATE_SET|v1")
	for _, kind := range AllKinds() {
		b.WriteString("|")
		if state, ok := s.States[kind]; ok {
			b.WriteString(state.CanonicalString())
		}
	}
	return b.String()
}

// ComputeHash computes and stores the hash of the state set.
func (s *ConnectionStateSet) ComputeHash() {
	h := sha256.Sum256([]byte(s.CanonicalString()))
	s.Hash = hex.EncodeToString(h[:])
}

// ComputeState computes the connection state set from a list of intents.
// Deterministic: same intents => same state.
//
// Rules:
// - Last-write-wins by timestamp, tie-break by hash lexical
// - if latest action=disconnect → NotConnected
// - if connect + mode=mock → ConnectedMock
// - if connect + mode=real + config missing → NeedsConfig
// - if connect + mode=real + config present → ConnectedReal
//
// The configPresent map indicates which kinds have real configuration.
// In Phase 18.6, we can treat all as NeedsConfig unless explicitly configured.
func ComputeState(intents IntentList, configPresent map[ConnectionKind]bool) *ConnectionStateSet {
	// Sort intents deterministically
	sorted := make(IntentList, len(intents))
	copy(sorted, intents)
	sorted.Sort()

	// Start with empty state set
	result := NewConnectionStateSet()

	// Apply each intent in order
	for _, intent := range sorted {
		state := result.States[intent.Kind]
		if state == nil {
			state = &ConnectionState{Kind: intent.Kind}
			result.States[intent.Kind] = state
		}

		// Update state based on intent
		state.LastChangedAt = intent.At
		state.LastIntentHash = intent.ID

		switch intent.Action {
		case ActionDisconnect:
			state.Status = StatusNotConnected
		case ActionConnect:
			switch intent.Mode {
			case ModeMock:
				state.Status = StatusConnectedMock
			case ModeReal:
				// Check if config is present
				if configPresent != nil && configPresent[intent.Kind] {
					state.Status = StatusConnectedReal
				} else {
					state.Status = StatusNeedsConfig
				}
			}
		}
	}

	// Compute final hash
	result.ComputeHash()
	return result
}

// ComputeStateFromIntents is a convenience function that computes state
// assuming no real configuration is present (all real connections → NeedsConfig).
func ComputeStateFromIntents(intents IntentList) *ConnectionStateSet {
	return ComputeState(intents, nil)
}
