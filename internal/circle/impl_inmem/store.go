// Package impl_inmem provides an in-memory implementation of the circle interfaces.
// This is for demo and testing purposes only.
//
// CRITICAL: This implementation is NOT for production use.
// Production requires persistent storage with proper identity management.
package impl_inmem

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"quantumlife/internal/circle"
)

// Runtime implements the circle.Runtime interface.
type Runtime struct {
	mu        sync.RWMutex
	circles   map[string]*circle.Circle
	policies  map[string]*circle.Policy
	grants    map[string][]circle.AuthorityGrant
	idCounter int
}

// NewRuntime creates a new in-memory circle runtime.
func NewRuntime() *Runtime {
	return &Runtime{
		circles:  make(map[string]*circle.Circle),
		policies: make(map[string]*circle.Policy),
		grants:   make(map[string][]circle.AuthorityGrant),
	}
}

// Create initializes a new circle with the given identity.
func (r *Runtime) Create(ctx context.Context, req circle.CreateRequest) (*circle.Circle, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.idCounter++
	circleID := fmt.Sprintf("circle-%d", r.idCounter)

	now := time.Now()
	c := &circle.Circle{
		ID:        circleID,
		TenantID:  req.TenantID,
		State:     circle.StateActive,
		CreatedAt: now,
		UpdatedAt: now,
	}

	r.circles[circleID] = c

	// Set default policy if provided
	if req.Policy != nil {
		policy := *req.Policy
		policy.CircleID = circleID
		policy.Version = 1
		policy.UpdatedAt = now
		r.policies[circleID] = &policy
	} else {
		r.policies[circleID] = &circle.Policy{
			CircleID:   circleID,
			Version:    1,
			Boundaries: []circle.Boundary{},
			UpdatedAt:  now,
		}
	}

	return c, nil
}

// Get retrieves a circle by ID.
func (r *Runtime) Get(ctx context.Context, circleID string) (*circle.Circle, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if c, ok := r.circles[circleID]; ok {
		return c, nil
	}
	return nil, fmt.Errorf("circle not found: %s", circleID)
}

// Suspend pauses a circle's operations.
func (r *Runtime) Suspend(ctx context.Context, circleID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, ok := r.circles[circleID]
	if !ok {
		return fmt.Errorf("circle not found: %s", circleID)
	}
	if c.State == circle.StateTerminated {
		return fmt.Errorf("cannot suspend terminated circle: %s", circleID)
	}

	c.State = circle.StateSuspended
	c.UpdatedAt = time.Now()
	return nil
}

// Resume restarts a suspended circle.
func (r *Runtime) Resume(ctx context.Context, circleID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, ok := r.circles[circleID]
	if !ok {
		return fmt.Errorf("circle not found: %s", circleID)
	}
	if c.State != circle.StateSuspended {
		return fmt.Errorf("circle is not suspended: %s", circleID)
	}

	c.State = circle.StateActive
	c.UpdatedAt = time.Now()
	return nil
}

// Terminate permanently ends a circle.
func (r *Runtime) Terminate(ctx context.Context, circleID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, ok := r.circles[circleID]
	if !ok {
		return fmt.Errorf("circle not found: %s", circleID)
	}

	c.State = circle.StateTerminated
	c.UpdatedAt = time.Now()
	return nil
}

// GetPolicy retrieves the circle's current policy.
func (r *Runtime) GetPolicy(ctx context.Context, circleID string) (*circle.Policy, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, ok := r.circles[circleID]; !ok {
		return nil, fmt.Errorf("circle not found: %s", circleID)
	}

	if policy, ok := r.policies[circleID]; ok {
		return policy, nil
	}
	return nil, fmt.Errorf("policy not found for circle: %s", circleID)
}

// UpdatePolicy modifies the circle's policy.
func (r *Runtime) UpdatePolicy(ctx context.Context, circleID string, policy *circle.Policy) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.circles[circleID]; !ok {
		return fmt.Errorf("circle not found: %s", circleID)
	}

	existing := r.policies[circleID]
	newVersion := 1
	if existing != nil {
		newVersion = existing.Version + 1
	}

	policy.CircleID = circleID
	policy.Version = newVersion
	policy.UpdatedAt = time.Now()
	r.policies[circleID] = policy

	return nil
}

// GrantAuthority creates a new authority grant.
func (r *Runtime) GrantAuthority(ctx context.Context, req circle.GrantRequest) (*circle.AuthorityGrant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.circles[req.CircleID]; !ok {
		return nil, fmt.Errorf("circle not found: %s", req.CircleID)
	}

	grantID := generateID("grant")
	grant := circle.AuthorityGrant{
		ID:             grantID,
		CircleID:       req.CircleID,
		IntersectionID: req.IntersectionID,
		Scopes:         req.Scopes,
		Ceilings:       req.Ceilings,
		GrantedAt:      time.Now(),
		ExpiresAt:      req.ExpiresAt,
	}

	r.grants[req.CircleID] = append(r.grants[req.CircleID], grant)
	return &grant, nil
}

// RevokeAuthority revokes an existing authority grant.
func (r *Runtime) RevokeAuthority(ctx context.Context, grantID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	for circleID, grants := range r.grants {
		for i, g := range grants {
			if g.ID == grantID {
				r.grants[circleID][i].RevokedAt = &now
				return nil
			}
		}
	}
	return fmt.Errorf("grant not found: %s", grantID)
}

// ListGrants returns all authority grants for this circle.
func (r *Runtime) ListGrants(ctx context.Context, circleID string) ([]circle.AuthorityGrant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, ok := r.circles[circleID]; !ok {
		return nil, fmt.Errorf("circle not found: %s", circleID)
	}

	grants := r.grants[circleID]
	if grants == nil {
		return []circle.AuthorityGrant{}, nil
	}

	// Return only active grants
	var active []circle.AuthorityGrant
	now := time.Now()
	for _, g := range grants {
		if g.RevokedAt == nil && g.ExpiresAt.After(now) {
			active = append(active, g)
		}
	}
	return active, nil
}

// generateID generates a random ID with the given prefix.
func generateID(prefix string) string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(bytes))
}

// Verify interface compliance at compile time.
var _ circle.Runtime = (*Runtime)(nil)
