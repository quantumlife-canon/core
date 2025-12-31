package events

import (
	"errors"
	"sort"
	"sync"
	"time"

	"quantumlife/pkg/domain/identity"
)

// Store errors.
var (
	ErrEventNotFound = errors.New("event not found")
	ErrEventExists   = errors.New("event already exists")
)

// InMemoryEventStore is a thread-safe in-memory implementation of EventStore.
type InMemoryEventStore struct {
	mu     sync.RWMutex
	events map[string]CanonicalEvent

	// Indexes
	byCircle map[identity.EntityID][]string // circle ID -> event IDs
	byType   map[EventType][]string         // event type -> event IDs
}

// NewInMemoryEventStore creates a new in-memory event store.
func NewInMemoryEventStore() *InMemoryEventStore {
	return &InMemoryEventStore{
		events:   make(map[string]CanonicalEvent),
		byCircle: make(map[identity.EntityID][]string),
		byType:   make(map[EventType][]string),
	}
}

func (s *InMemoryEventStore) Store(event CanonicalEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := event.EventID()
	if _, exists := s.events[id]; exists {
		return ErrEventExists
	}

	s.events[id] = event

	// Update indexes
	circleID := event.CircleID()
	if circleID != "" {
		s.byCircle[circleID] = append(s.byCircle[circleID], id)
	}

	eventType := event.EventType()
	s.byType[eventType] = append(s.byType[eventType], id)

	return nil
}

func (s *InMemoryEventStore) GetByID(id string) (CanonicalEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	event, exists := s.events[id]
	if !exists {
		return nil, ErrEventNotFound
	}
	return event, nil
}

func (s *InMemoryEventStore) GetByCircle(circleID identity.EntityID, eventType *EventType, limit int) ([]CanonicalEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	eventIDs := s.byCircle[circleID]
	if len(eventIDs) == 0 {
		return nil, nil
	}

	var result []CanonicalEvent
	for _, id := range eventIDs {
		event := s.events[id]
		if eventType != nil && event.EventType() != *eventType {
			continue
		}
		result = append(result, event)
	}

	// Sort by occurred time (newest first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].OccurredAt().After(result[j].OccurredAt())
	})

	// Apply limit
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result, nil
}

func (s *InMemoryEventStore) GetByTimeRange(start, end time.Time, eventType *EventType) ([]CanonicalEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []CanonicalEvent
	for _, event := range s.events {
		occurred := event.OccurredAt()
		if occurred.Before(start) || occurred.After(end) {
			continue
		}
		if eventType != nil && event.EventType() != *eventType {
			continue
		}
		result = append(result, event)
	}

	// Sort by occurred time
	sort.Slice(result, func(i, j int) bool {
		return result[i].OccurredAt().Before(result[j].OccurredAt())
	})

	return result, nil
}

func (s *InMemoryEventStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.events)
}

func (s *InMemoryEventStore) CountByType(eventType EventType) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.byType[eventType])
}

// CountByCircle returns event count for a specific circle.
func (s *InMemoryEventStore) CountByCircle(circleID identity.EntityID) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.byCircle[circleID])
}

// GetRecentByCircle returns the most recent events for a circle.
func (s *InMemoryEventStore) GetRecentByCircle(circleID identity.EntityID, count int) []CanonicalEvent {
	events, _ := s.GetByCircle(circleID, nil, count)
	return events
}

// Verify interface compliance.
var _ EventStore = (*InMemoryEventStore)(nil)
