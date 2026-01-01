package notifyexec

import (
	"time"

	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/interrupt"
	"quantumlife/pkg/domain/notify"
)

// Badge represents a web UI badge.
type Badge struct {
	NotificationID string
	CircleID       identity.EntityID
	Level          interrupt.Level
	Summary        string
	CreatedAt      time.Time
	ExpiresAt      time.Time
}

// BadgeStore stores web badges.
type BadgeStore interface {
	// Add adds a badge.
	Add(badge Badge) error

	// Remove removes a badge by notification ID.
	Remove(notificationID string) error

	// GetBadges returns current badge counts.
	GetBadges(now time.Time) *notify.BadgeCounts

	// GetByCircle returns badges for a circle.
	GetByCircle(circleID identity.EntityID, now time.Time) []Badge

	// Clear removes all badges.
	Clear() error
}

// EmailDraft represents an email draft for notification.
type EmailDraft struct {
	DraftID        string
	NotificationID string
	CircleID       identity.EntityID
	PersonIDs      []identity.EntityID
	Subject        string
	Body           string
	Channel        notify.Channel
	CreatedAt      time.Time
}

// EmailDraftCreator creates email drafts for notifications.
type EmailDraftCreator interface {
	// CreateDraft creates an email draft (not auto-sent).
	CreateDraft(draft EmailDraft) error
}

// PushRequest represents a push notification request.
type PushRequest struct {
	NotificationID string
	PersonIDs      []identity.EntityID
	Title          string
	Body           string
	Level          interrupt.Level
}

// PushProvider delivers push notifications.
type PushProvider interface {
	// Send sends a push notification.
	Send(req PushRequest) error
}

// SMSRequest represents an SMS request.
type SMSRequest struct {
	NotificationID string
	PersonIDs      []identity.EntityID
	Message        string
}

// SMSProvider delivers SMS notifications.
type SMSProvider interface {
	// Send sends an SMS.
	Send(req SMSRequest) error
}

// PolicyVerifier verifies policy snapshots.
type PolicyVerifier interface {
	// Verify checks if a policy hash is still valid.
	Verify(hash string) error
}

// ViewVerifier verifies view snapshots.
type ViewVerifier interface {
	// Verify checks if a view hash is still valid.
	Verify(hash string, capturedAt time.Time) error
}

// EnvelopeStore stores notification envelopes.
type EnvelopeStore interface {
	// Put stores an envelope.
	Put(env NotificationEnvelope) error

	// Get retrieves an envelope by ID.
	Get(id string) (NotificationEnvelope, bool)

	// GetByIdempotencyKey retrieves by idempotency key.
	GetByIdempotencyKey(key string) (NotificationEnvelope, bool)

	// List returns all envelopes.
	List() []NotificationEnvelope

	// ListByStatus returns envelopes by status.
	ListByStatus(status EnvelopeStatus) []NotificationEnvelope
}

// MemoryBadgeStore is an in-memory badge store.
type MemoryBadgeStore struct {
	badges map[string]Badge
}

// NewMemoryBadgeStore creates a new in-memory badge store.
func NewMemoryBadgeStore() *MemoryBadgeStore {
	return &MemoryBadgeStore{
		badges: make(map[string]Badge),
	}
}

// Add adds a badge.
func (s *MemoryBadgeStore) Add(badge Badge) error {
	s.badges[badge.NotificationID] = badge
	return nil
}

// Remove removes a badge.
func (s *MemoryBadgeStore) Remove(notificationID string) error {
	delete(s.badges, notificationID)
	return nil
}

// GetBadges returns current badge counts.
func (s *MemoryBadgeStore) GetBadges(now time.Time) *notify.BadgeCounts {
	counts := notify.NewBadgeCounts()

	for _, badge := range s.badges {
		if now.After(badge.ExpiresAt) {
			continue // Skip expired badges
		}

		// Create a minimal notification for counting
		n := &notify.Notification{
			NotificationID: badge.NotificationID,
			CircleID:       badge.CircleID,
			Level:          badge.Level,
			Channel:        notify.ChannelWebBadge,
			Status:         notify.StatusPending,
		}
		counts.Add(n)
	}

	return counts
}

// GetByCircle returns badges for a circle.
func (s *MemoryBadgeStore) GetByCircle(circleID identity.EntityID, now time.Time) []Badge {
	var result []Badge
	for _, badge := range s.badges {
		if badge.CircleID == circleID && !now.After(badge.ExpiresAt) {
			result = append(result, badge)
		}
	}
	return result
}

// Clear removes all badges.
func (s *MemoryBadgeStore) Clear() error {
	s.badges = make(map[string]Badge)
	return nil
}

// MemoryEnvelopeStore is an in-memory envelope store.
type MemoryEnvelopeStore struct {
	envelopes     map[string]NotificationEnvelope
	byIdempotency map[string]string // idempotency key -> envelope ID
}

// NewMemoryEnvelopeStore creates a new in-memory envelope store.
func NewMemoryEnvelopeStore() *MemoryEnvelopeStore {
	return &MemoryEnvelopeStore{
		envelopes:     make(map[string]NotificationEnvelope),
		byIdempotency: make(map[string]string),
	}
}

// Put stores an envelope.
func (s *MemoryEnvelopeStore) Put(env NotificationEnvelope) error {
	s.envelopes[env.EnvelopeID] = env
	s.byIdempotency[env.IdempotencyKey] = env.EnvelopeID
	return nil
}

// Get retrieves an envelope by ID.
func (s *MemoryEnvelopeStore) Get(id string) (NotificationEnvelope, bool) {
	env, ok := s.envelopes[id]
	return env, ok
}

// GetByIdempotencyKey retrieves by idempotency key.
func (s *MemoryEnvelopeStore) GetByIdempotencyKey(key string) (NotificationEnvelope, bool) {
	id, ok := s.byIdempotency[key]
	if !ok {
		return NotificationEnvelope{}, false
	}
	return s.Get(id)
}

// List returns all envelopes.
func (s *MemoryEnvelopeStore) List() []NotificationEnvelope {
	result := make([]NotificationEnvelope, 0, len(s.envelopes))
	for _, env := range s.envelopes {
		result = append(result, env)
	}
	return result
}

// ListByStatus returns envelopes by status.
func (s *MemoryEnvelopeStore) ListByStatus(status EnvelopeStatus) []NotificationEnvelope {
	var result []NotificationEnvelope
	for _, env := range s.envelopes {
		if env.Status == status {
			result = append(result, env)
		}
	}
	return result
}

// MockPushProvider is a mock push provider for testing.
type MockPushProvider struct {
	Sent []PushRequest
}

// NewMockPushProvider creates a new mock push provider.
func NewMockPushProvider() *MockPushProvider {
	return &MockPushProvider{
		Sent: make([]PushRequest, 0),
	}
}

// Send records the push request.
func (p *MockPushProvider) Send(req PushRequest) error {
	p.Sent = append(p.Sent, req)
	return nil
}

// MockSMSProvider is a mock SMS provider for testing.
type MockSMSProvider struct {
	Sent []SMSRequest
}

// NewMockSMSProvider creates a new mock SMS provider.
func NewMockSMSProvider() *MockSMSProvider {
	return &MockSMSProvider{
		Sent: make([]SMSRequest, 0),
	}
}

// Send records the SMS request.
func (p *MockSMSProvider) Send(req SMSRequest) error {
	p.Sent = append(p.Sent, req)
	return nil
}

// MockEmailDraftCreator is a mock email draft creator.
type MockEmailDraftCreator struct {
	Drafts []EmailDraft
}

// NewMockEmailDraftCreator creates a new mock email draft creator.
func NewMockEmailDraftCreator() *MockEmailDraftCreator {
	return &MockEmailDraftCreator{
		Drafts: make([]EmailDraft, 0),
	}
}

// CreateDraft records the draft.
func (c *MockEmailDraftCreator) CreateDraft(draft EmailDraft) error {
	c.Drafts = append(c.Drafts, draft)
	return nil
}
