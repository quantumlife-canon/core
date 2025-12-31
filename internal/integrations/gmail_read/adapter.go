// Package gmail_read provides a read-only adapter for Gmail integration.
//
// CRITICAL: This adapter is READ-ONLY. It NEVER writes to Gmail.
// All data is transformed to canonical EmailMessageEvent format.
//
// Reference: docs/INTEGRATIONS_MATRIX_V1.md
package gmail_read

import (
	"time"

	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/events"
	"quantumlife/pkg/domain/identity"
)

// Adapter defines the interface for Gmail read operations.
type Adapter interface {
	// FetchMessages retrieves messages from Gmail and returns canonical events.
	// This is a synchronous operation - no background polling.
	FetchMessages(accountEmail string, since time.Time, limit int) ([]*events.EmailMessageEvent, error)

	// FetchUnreadCount returns the count of unread messages.
	FetchUnreadCount(accountEmail string) (int, error)

	// Name returns the adapter name.
	Name() string
}

// MockAdapter is a mock implementation for testing and demos.
type MockAdapter struct {
	clock    clock.Clock
	messages []*MockMessage
}

// MockMessage represents a mock email message.
type MockMessage struct {
	MessageID       string
	ThreadID        string
	AccountEmail    string
	From            events.EmailAddress
	To              []events.EmailAddress
	Subject         string
	BodyPreview     string
	SentAt          time.Time
	IsRead          bool
	IsImportant     bool
	IsAutomated     bool
	IsTransactional bool
	Labels          []string
	CircleID        identity.EntityID
}

// NewMockAdapter creates a new mock Gmail adapter.
func NewMockAdapter(clk clock.Clock) *MockAdapter {
	return &MockAdapter{
		clock:    clk,
		messages: make([]*MockMessage, 0),
	}
}

// AddMockMessage adds a message to the mock adapter.
func (a *MockAdapter) AddMockMessage(msg *MockMessage) {
	a.messages = append(a.messages, msg)
}

func (a *MockAdapter) Name() string {
	return "gmail_mock"
}

func (a *MockAdapter) FetchMessages(accountEmail string, since time.Time, limit int) ([]*events.EmailMessageEvent, error) {
	now := a.clock.Now()
	var result []*events.EmailMessageEvent

	for _, msg := range a.messages {
		if msg.AccountEmail != accountEmail {
			continue
		}
		if !since.IsZero() && msg.SentAt.Before(since) {
			continue
		}

		event := events.NewEmailMessageEvent(
			"gmail",
			msg.MessageID,
			msg.AccountEmail,
			now,
			msg.SentAt,
		)

		event.ThreadID = msg.ThreadID
		event.From = msg.From
		event.To = msg.To
		event.Subject = msg.Subject
		event.BodyPreview = msg.BodyPreview
		event.IsRead = msg.IsRead
		event.IsImportant = msg.IsImportant
		event.IsAutomated = msg.IsAutomated
		event.IsTransactional = msg.IsTransactional
		event.Labels = msg.Labels
		event.Folder = "INBOX"

		// Extract sender domain
		if len(msg.From.Address) > 0 {
			for i := len(msg.From.Address) - 1; i >= 0; i-- {
				if msg.From.Address[i] == '@' {
					event.SenderDomain = msg.From.Address[i+1:]
					break
				}
			}
		}

		// Set circle
		event.Circle = msg.CircleID

		result = append(result, event)

		if limit > 0 && len(result) >= limit {
			break
		}
	}

	return result, nil
}

func (a *MockAdapter) FetchUnreadCount(accountEmail string) (int, error) {
	count := 0
	for _, msg := range a.messages {
		if msg.AccountEmail == accountEmail && !msg.IsRead {
			count++
		}
	}
	return count, nil
}

// Verify interface compliance.
var _ Adapter = (*MockAdapter)(nil)
