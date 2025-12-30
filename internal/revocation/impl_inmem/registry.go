// Package impl_inmem provides an in-memory implementation of the revocation registry.
// This is for demo and testing purposes.
//
// CRITICAL: In production, revocation signals must be persisted and distributed.
//
// Reference: docs/TECHNICAL_SPLIT_V1.md §3.3 Authority & Policy Engine
package impl_inmem

import (
	"context"
	"fmt"
	"sync"
	"time"

	"quantumlife/internal/revocation"
)

// Registry implements revocation.Registry with in-memory storage.
type Registry struct {
	mu        sync.RWMutex
	signals   map[string]*revocation.Signal // key: targetID
	clockFunc func() time.Time
	idCounter int
}

// NewRegistry creates a new in-memory revocation registry.
func NewRegistry() *Registry {
	return &Registry{
		signals:   make(map[string]*revocation.Signal),
		clockFunc: time.Now,
	}
}

// NewRegistryWithClock creates a registry with an injected clock for determinism.
func NewRegistryWithClock(clockFunc func() time.Time) *Registry {
	return &Registry{
		signals:   make(map[string]*revocation.Signal),
		clockFunc: clockFunc,
	}
}

// IsRevoked checks if a target has been revoked.
func (r *Registry) IsRevoked(ctx context.Context, targetID string) (*revocation.Signal, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if signal, ok := r.signals[targetID]; ok {
		return signal, nil
	}
	return nil, nil
}

// IsActionRevoked checks if an action or its authority is revoked.
// This checks multiple targets that could block the action.
func (r *Registry) IsActionRevoked(ctx context.Context, actionID, intersectionID, authorityProofID string) (*revocation.Signal, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check action itself
	if signal, ok := r.signals[actionID]; ok {
		return signal, nil
	}

	// Check intersection
	if signal, ok := r.signals[intersectionID]; ok {
		return signal, nil
	}

	// Check authority proof
	if signal, ok := r.signals[authorityProofID]; ok {
		return signal, nil
	}

	return nil, nil
}

// CheckBeforeWrite is the final safety check before an external write.
// CRITICAL: This MUST return nil to proceed with the write.
func (r *Registry) CheckBeforeWrite(ctx context.Context, actionID, intersectionID, authorityProofID string) error {
	signal, err := r.IsActionRevoked(ctx, actionID, intersectionID, authorityProofID)
	if err != nil {
		return fmt.Errorf("failed to check revocation: %w", err)
	}

	if signal != nil {
		// Convert signal type to appropriate error
		switch signal.Type {
		case revocation.SignalActionCancelled:
			return fmt.Errorf("%w: %s (by %s)", revocation.ErrActionRevoked, signal.Reason, signal.RevokedBy)
		case revocation.SignalAuthorityRevoked:
			return fmt.Errorf("%w: %s", revocation.ErrAuthorityRevoked, signal.Reason)
		case revocation.SignalIntersectionDissolved:
			return fmt.Errorf("%w: %s", revocation.ErrIntersectionDissolved, signal.Reason)
		case revocation.SignalCircleSuspended:
			return fmt.Errorf("%w: %s", revocation.ErrCircleSuspended, signal.Reason)
		case revocation.SignalCircleRevoked:
			return fmt.Errorf("%w: %s", revocation.ErrCircleRevoked, signal.Reason)
		default:
			return fmt.Errorf("revoked: %s", signal.Reason)
		}
	}

	return nil
}

// Revoke emits a revocation signal.
func (r *Registry) Revoke(ctx context.Context, signal revocation.Signal) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Generate ID if not provided
	if signal.ID == "" {
		r.idCounter++
		signal.ID = fmt.Sprintf("revoke-%d", r.idCounter)
	}

	// Set revocation time if not provided
	if signal.RevokedAt.IsZero() {
		signal.RevokedAt = r.clockFunc()
	}

	// Store by target ID for fast lookup
	r.signals[signal.TargetID] = &signal

	return nil
}

// RevokeAction emits a revocation signal for a specific action.
func (r *Registry) RevokeAction(ctx context.Context, actionID, reason, revokedBy string) error {
	return r.Revoke(ctx, revocation.Signal{
		Type:       revocation.SignalActionCancelled,
		TargetID:   actionID,
		TargetType: "action",
		Reason:     reason,
		RevokedBy:  revokedBy,
	})
}

// RevokeAuthority emits a revocation signal for an authority grant.
func (r *Registry) RevokeAuthority(ctx context.Context, grantID, reason, revokedBy string) error {
	return r.Revoke(ctx, revocation.Signal{
		Type:       revocation.SignalAuthorityRevoked,
		TargetID:   grantID,
		TargetType: "authority_grant",
		Reason:     reason,
		RevokedBy:  revokedBy,
	})
}

// RevokeIntersection emits a revocation signal for an intersection.
func (r *Registry) RevokeIntersection(ctx context.Context, intersectionID, reason, revokedBy string) error {
	return r.Revoke(ctx, revocation.Signal{
		Type:       revocation.SignalIntersectionDissolved,
		TargetID:   intersectionID,
		TargetType: "intersection",
		Reason:     reason,
		RevokedBy:  revokedBy,
	})
}

// ClearRevocation removes a revocation signal (for testing/recovery).
func (r *Registry) ClearRevocation(ctx context.Context, targetID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.signals, targetID)
}

// GetAllSignals returns all revocation signals (for testing/audit).
func (r *Registry) GetAllSignals() []*revocation.Signal {
	r.mu.RLock()
	defer r.mu.RUnlock()

	signals := make([]*revocation.Signal, 0, len(r.signals))
	for _, s := range r.signals {
		signals = append(signals, s)
	}
	return signals
}

// Verify interface compliance at compile time.
var _ revocation.Registry = (*Registry)(nil)
