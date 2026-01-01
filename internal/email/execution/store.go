package execution

import (
	"sort"
	"sync"

	"quantumlife/pkg/domain/draft"
)

// Store persists email execution envelopes.
type Store interface {
	// Put stores an envelope.
	Put(envelope Envelope) error

	// Get retrieves an envelope by ID.
	Get(id string) (Envelope, bool)

	// GetByDraftID retrieves an envelope by draft ID.
	GetByDraftID(draftID draft.DraftID) (Envelope, bool)

	// GetByIdempotencyKey retrieves an envelope by idempotency key.
	GetByIdempotencyKey(key string) (Envelope, bool)

	// List returns envelopes matching the filter.
	List(filter ListFilter) []Envelope
}

// ListFilter defines filtering options for List.
type ListFilter struct {
	// Status filters by envelope status.
	Status EnvelopeStatus

	// IncludeAll includes all statuses.
	IncludeAll bool
}

// MemoryStore is an in-memory envelope store.
type MemoryStore struct {
	mu sync.RWMutex

	// envelopes stores envelopes by ID.
	envelopes map[string]Envelope

	// byDraftID indexes by draft ID.
	byDraftID map[draft.DraftID]string

	// byIdempotencyKey indexes by idempotency key.
	byIdempotencyKey map[string]string
}

// NewMemoryStore creates a new in-memory envelope store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		envelopes:        make(map[string]Envelope),
		byDraftID:        make(map[draft.DraftID]string),
		byIdempotencyKey: make(map[string]string),
	}
}

// Put stores an envelope.
func (s *MemoryStore) Put(envelope Envelope) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.envelopes[envelope.EnvelopeID] = envelope
	s.byDraftID[envelope.DraftID] = envelope.EnvelopeID
	s.byIdempotencyKey[envelope.IdempotencyKey] = envelope.EnvelopeID

	return nil
}

// Get retrieves an envelope by ID.
func (s *MemoryStore) Get(id string) (Envelope, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	envelope, found := s.envelopes[id]
	return envelope, found
}

// GetByDraftID retrieves an envelope by draft ID.
func (s *MemoryStore) GetByDraftID(draftID draft.DraftID) (Envelope, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	envelopeID, found := s.byDraftID[draftID]
	if !found {
		return Envelope{}, false
	}

	return s.envelopes[envelopeID], true
}

// GetByIdempotencyKey retrieves an envelope by idempotency key.
func (s *MemoryStore) GetByIdempotencyKey(key string) (Envelope, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	envelopeID, found := s.byIdempotencyKey[key]
	if !found {
		return Envelope{}, false
	}

	return s.envelopes[envelopeID], true
}

// List returns envelopes matching the filter.
// Results are sorted deterministically by EnvelopeID.
func (s *MemoryStore) List(filter ListFilter) []Envelope {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Envelope

	for _, envelope := range s.envelopes {
		if filter.IncludeAll {
			result = append(result, envelope)
		} else if filter.Status == "" || envelope.Status == filter.Status {
			result = append(result, envelope)
		}
	}

	// Sort deterministically by EnvelopeID
	sort.Slice(result, func(i, j int) bool {
		return result[i].EnvelopeID < result[j].EnvelopeID
	})

	return result
}

// Ensure MemoryStore implements Store.
var _ Store = (*MemoryStore)(nil)
