package execution

import (
	"sync"

	"quantumlife/pkg/domain/draft"
)

// Store provides envelope storage operations.
type Store interface {
	// Put stores an envelope.
	Put(env Envelope) error

	// Get retrieves an envelope by ID.
	Get(id string) (Envelope, bool)

	// GetByDraftID retrieves an envelope by draft ID.
	GetByDraftID(draftID draft.DraftID) (Envelope, bool)

	// GetByIdempotencyKey retrieves an envelope by idempotency key.
	GetByIdempotencyKey(key string) (Envelope, bool)

	// Update updates an existing envelope.
	Update(env Envelope) error

	// List lists envelopes matching a filter.
	List(filter ListFilter) []Envelope
}

// ListFilter specifies criteria for listing envelopes.
type ListFilter struct {
	// Status filters by envelope status.
	Status EnvelopeStatus

	// IncludeAll includes all statuses if true.
	IncludeAll bool
}

// MemoryStore is an in-memory envelope store for testing.
type MemoryStore struct {
	mu        sync.RWMutex
	envelopes map[string]Envelope
}

// NewMemoryStore creates a new in-memory envelope store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		envelopes: make(map[string]Envelope),
	}
}

// Put stores an envelope.
func (s *MemoryStore) Put(env Envelope) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.envelopes[env.EnvelopeID]; exists {
		return ErrEnvelopeExists
	}

	s.envelopes[env.EnvelopeID] = env
	return nil
}

// Get retrieves an envelope by ID.
func (s *MemoryStore) Get(id string) (Envelope, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	env, exists := s.envelopes[id]
	return env, exists
}

// GetByDraftID retrieves an envelope by draft ID.
func (s *MemoryStore) GetByDraftID(draftID draft.DraftID) (Envelope, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, env := range s.envelopes {
		if env.DraftID == draftID {
			return env, true
		}
	}
	return Envelope{}, false
}

// GetByIdempotencyKey retrieves an envelope by idempotency key.
func (s *MemoryStore) GetByIdempotencyKey(key string) (Envelope, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, env := range s.envelopes {
		if env.IdempotencyKey == key {
			return env, true
		}
	}
	return Envelope{}, false
}

// Update updates an existing envelope.
func (s *MemoryStore) Update(env Envelope) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.envelopes[env.EnvelopeID]; !exists {
		return ErrEnvelopeNotFound
	}

	s.envelopes[env.EnvelopeID] = env
	return nil
}

// List lists envelopes matching a filter.
func (s *MemoryStore) List(filter ListFilter) []Envelope {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Envelope
	for _, env := range s.envelopes {
		if filter.IncludeAll {
			result = append(result, env)
			continue
		}
		if filter.Status != "" && env.Status == filter.Status {
			result = append(result, env)
		}
	}
	return result
}

// Store errors.
var (
	ErrEnvelopeExists   = storeError("envelope already exists")
	ErrEnvelopeNotFound = storeError("envelope not found")
)

type storeError string

func (e storeError) Error() string { return string(e) }

// Verify interface compliance.
var _ Store = (*MemoryStore)(nil)
