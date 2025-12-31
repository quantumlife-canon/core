// Package execution provides v9.13 View Provider interface.
//
// CRITICAL: ViewProvider enables read-before-write verification:
// 1. Provider fetches current view state before execution
// 2. View is checked for freshness (MaxStaleness)
// 3. View hash is verified against envelope binding
//
// This prevents executing based on stale or drifted view data.
//
// Reference: ADR-0014, Canon Addendum v9
package execution

import (
	"context"
	"time"

	"quantumlife/pkg/clock"
)

// ViewProvider is the interface for obtaining view snapshots.
// Implementations must return deterministic, auditable snapshots.
type ViewProvider interface {
	// ProviderID returns the provider identifier.
	ProviderID() string

	// GetViewSnapshot fetches the current view state.
	// CRITICAL: Must use injected clock for CapturedAt timestamp.
	GetViewSnapshot(ctx context.Context, req ViewSnapshotRequest) (ViewSnapshot, error)
}

// ViewSnapshotRequest contains parameters for fetching a view snapshot.
type ViewSnapshotRequest struct {
	// CircleID is the circle requesting the view.
	CircleID string

	// IntersectionID is the shared context (optional).
	IntersectionID string

	// PayeeID is the payee for the execution.
	PayeeID string

	// Currency is the currency for the execution.
	Currency string

	// AmountCents is the amount being executed.
	AmountCents int64

	// ProviderID is the write provider that will be used.
	ProviderID string

	// Clock provides deterministic time.
	Clock clock.Clock

	// TraceID for audit correlation.
	TraceID string
}

// MockViewProvider is an in-memory ViewProvider for testing.
// Returns deterministic snapshots based on configuration.
type MockViewProvider struct {
	providerID string
	clock      clock.Clock
	idGen      func() string

	// Configuration for mock behavior
	payeeAllowed    bool
	providerAllowed bool
	balanceOK       bool
	accounts        []string
	sharedViewHash  string

	// Optional override for CapturedAt (for staleness testing)
	capturedAtOverride *time.Time

	// Optional override for SnapshotID (for deterministic hash testing)
	snapshotIDOverride *string
}

// MockViewProviderConfig contains configuration for MockViewProvider.
type MockViewProviderConfig struct {
	ProviderID      string
	Clock           clock.Clock
	IDGenerator     func() string
	PayeeAllowed    bool
	ProviderAllowed bool
	BalanceOK       bool
	Accounts        []string
	SharedViewHash  string
}

// NewMockViewProvider creates a new mock view provider.
func NewMockViewProvider(cfg MockViewProviderConfig) *MockViewProvider {
	return &MockViewProvider{
		providerID:      cfg.ProviderID,
		clock:           cfg.Clock,
		idGen:           cfg.IDGenerator,
		payeeAllowed:    cfg.PayeeAllowed,
		providerAllowed: cfg.ProviderAllowed,
		balanceOK:       cfg.BalanceOK,
		accounts:        cfg.Accounts,
		sharedViewHash:  cfg.SharedViewHash,
	}
}

// ProviderID returns the provider identifier.
func (p *MockViewProvider) ProviderID() string {
	return p.providerID
}

// GetViewSnapshot returns a deterministic mock snapshot.
func (p *MockViewProvider) GetViewSnapshot(ctx context.Context, req ViewSnapshotRequest) (ViewSnapshot, error) {
	capturedAt := p.clock.Now()
	if p.capturedAtOverride != nil {
		capturedAt = *p.capturedAtOverride
	}

	snapshotID := p.idGen()
	if p.snapshotIDOverride != nil {
		snapshotID = *p.snapshotIDOverride
	}

	return ViewSnapshot{
		SnapshotID:         snapshotID,
		CapturedAt:         capturedAt,
		CircleID:           req.CircleID,
		IntersectionID:     req.IntersectionID,
		PayeeID:            req.PayeeID,
		Currency:           req.Currency,
		AmountCents:        req.AmountCents,
		PayeeAllowed:       p.payeeAllowed,
		ProviderID:         req.ProviderID,
		ProviderAllowed:    p.providerAllowed,
		AccountVisibility:  p.accounts,
		SharedViewHash:     p.sharedViewHash,
		BalanceCheckPassed: p.balanceOK,
	}, nil
}

// SetCapturedAtOverride sets a specific CapturedAt time for testing staleness.
func (p *MockViewProvider) SetCapturedAtOverride(t time.Time) {
	p.capturedAtOverride = &t
}

// ClearCapturedAtOverride clears the CapturedAt override.
func (p *MockViewProvider) ClearCapturedAtOverride() {
	p.capturedAtOverride = nil
}

// SetSnapshotIDOverride sets a specific SnapshotID for deterministic hash testing.
func (p *MockViewProvider) SetSnapshotIDOverride(id string) {
	p.snapshotIDOverride = &id
}

// ClearSnapshotIDOverride clears the SnapshotID override.
func (p *MockViewProvider) ClearSnapshotIDOverride() {
	p.snapshotIDOverride = nil
}

// SetPayeeAllowed updates the payee allowed flag.
func (p *MockViewProvider) SetPayeeAllowed(allowed bool) {
	p.payeeAllowed = allowed
}

// SetProviderAllowed updates the provider allowed flag.
func (p *MockViewProvider) SetProviderAllowed(allowed bool) {
	p.providerAllowed = allowed
}

// SetBalanceOK updates the balance check result.
func (p *MockViewProvider) SetBalanceOK(ok bool) {
	p.balanceOK = ok
}

// SetAccounts updates the visible accounts.
func (p *MockViewProvider) SetAccounts(accounts []string) {
	p.accounts = accounts
}

// SetSharedViewHash updates the shared view hash.
func (p *MockViewProvider) SetSharedViewHash(hash string) {
	p.sharedViewHash = hash
}

// Verify interface compliance
var _ ViewProvider = (*MockViewProvider)(nil)
