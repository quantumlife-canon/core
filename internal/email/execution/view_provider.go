package execution

import (
	"context"
	"time"

	"quantumlife/pkg/domain/identity"
)

// ViewProvider provides email thread views.
type ViewProvider interface {
	// GetThreadView retrieves the current view of an email thread.
	GetThreadView(ctx context.Context, req GetThreadViewRequest) (ViewSnapshot, error)
}

// GetThreadViewRequest contains parameters for getting a thread view.
type GetThreadViewRequest struct {
	// Provider identifies the email provider.
	Provider string

	// AccountID identifies the email account.
	AccountID string

	// CircleID identifies the circle.
	CircleID identity.EntityID

	// IntersectionID is optional for shared contexts.
	IntersectionID identity.EntityID

	// ThreadID identifies the email thread.
	ThreadID string
}

// MemoryViewProvider is an in-memory view provider for testing.
type MemoryViewProvider struct {
	// views stores pre-configured views by thread ID.
	views map[string]ViewSnapshot

	// clock provides current time.
	clock func() time.Time
}

// NewMemoryViewProvider creates a new in-memory view provider.
func NewMemoryViewProvider(clock func() time.Time) *MemoryViewProvider {
	return &MemoryViewProvider{
		views: make(map[string]ViewSnapshot),
		clock: clock,
	}
}

// SetView sets a view for a thread.
func (p *MemoryViewProvider) SetView(threadID string, view ViewSnapshot) {
	p.views[threadID] = view
}

// GetThreadView retrieves a thread view.
func (p *MemoryViewProvider) GetThreadView(ctx context.Context, req GetThreadViewRequest) (ViewSnapshot, error) {
	if view, found := p.views[req.ThreadID]; found {
		return view, nil
	}

	// Return a fresh view with the request parameters
	return NewViewSnapshot(ViewSnapshotParams{
		Provider:       req.Provider,
		AccountID:      req.AccountID,
		CircleID:       req.CircleID,
		IntersectionID: req.IntersectionID,
		ThreadID:       req.ThreadID,
	}, p.clock()), nil
}

// Ensure MemoryViewProvider implements ViewProvider.
var _ ViewProvider = (*MemoryViewProvider)(nil)
