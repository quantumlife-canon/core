// Package impl_inmem provides an in-memory implementation of the intersection interfaces.
// This is for demo and testing purposes only.
//
// CRITICAL: This implementation is NOT for production use.
// Production requires persistent storage with proper contract versioning.
package impl_inmem

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"quantumlife/internal/intersection"
	"quantumlife/pkg/primitives"
)

// Runtime implements the intersection.Runtime and LoopDiscoverer interfaces.
type Runtime struct {
	mu            sync.RWMutex
	intersections map[string]*intersection.Intersection
	contracts     map[string][]intersection.Contract
	idCounter     int
}

// NewRuntime creates a new in-memory intersection runtime.
func NewRuntime() *Runtime {
	return &Runtime{
		intersections: make(map[string]*intersection.Intersection),
		contracts:     make(map[string][]intersection.Contract),
	}
}

// Create initializes a new intersection from an accepted proposal.
func (r *Runtime) Create(ctx context.Context, req intersection.CreateRequest) (*intersection.Intersection, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.idCounter++
	intersectionID := fmt.Sprintf("int-%d", r.idCounter)

	now := time.Now()
	i := &intersection.Intersection{
		ID:        intersectionID,
		TenantID:  req.TenantID,
		State:     intersection.StateActive,
		Version:   "1.0.0",
		CreatedAt: now,
		UpdatedAt: now,
	}

	r.intersections[intersectionID] = i

	// Store the initial contract
	contract := req.Contract
	contract.IntersectionID = intersectionID
	contract.Version = "1.0.0"
	contract.CreatedAt = now
	r.contracts[intersectionID] = []intersection.Contract{contract}

	return i, nil
}

// Get retrieves an intersection by ID.
func (r *Runtime) Get(ctx context.Context, intersectionID string) (*intersection.Intersection, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if i, ok := r.intersections[intersectionID]; ok {
		return i, nil
	}
	return nil, fmt.Errorf("intersection not found: %s", intersectionID)
}

// Amend modifies an intersection (requires all-party consent).
func (r *Runtime) Amend(ctx context.Context, req intersection.AmendRequest) (*intersection.Intersection, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	i, ok := r.intersections[req.IntersectionID]
	if !ok {
		return nil, fmt.Errorf("intersection not found: %s", req.IntersectionID)
	}

	if i.State != intersection.StateActive {
		return nil, fmt.Errorf("intersection is not active: %s", req.IntersectionID)
	}

	// Increment version
	contracts := r.contracts[req.IntersectionID]
	newVersion := fmt.Sprintf("%d.0.0", len(contracts)+1)

	newContract := req.NewContract
	newContract.IntersectionID = req.IntersectionID
	newContract.Version = newVersion
	newContract.PreviousVersion = i.Version
	newContract.CreatedAt = time.Now()

	r.contracts[req.IntersectionID] = append(contracts, newContract)

	i.Version = newVersion
	i.UpdatedAt = time.Now()

	return i, nil
}

// Dissolve ends an intersection (any party can initiate).
func (r *Runtime) Dissolve(ctx context.Context, intersectionID string, initiatorCircleID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	i, ok := r.intersections[intersectionID]
	if !ok {
		return fmt.Errorf("intersection not found: %s", intersectionID)
	}

	i.State = intersection.StateDissolved
	i.UpdatedAt = time.Now()
	return nil
}

// GetContract retrieves the current contract version.
func (r *Runtime) GetContract(ctx context.Context, intersectionID string) (*intersection.Contract, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	contracts, ok := r.contracts[intersectionID]
	if !ok || len(contracts) == 0 {
		return nil, fmt.Errorf("no contract found for intersection: %s", intersectionID)
	}

	// Return the latest contract
	return &contracts[len(contracts)-1], nil
}

// GetContractHistory retrieves all contract versions.
func (r *Runtime) GetContractHistory(ctx context.Context, intersectionID string) ([]intersection.Contract, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	contracts, ok := r.contracts[intersectionID]
	if !ok {
		return []intersection.Contract{}, nil
	}

	result := make([]intersection.Contract, len(contracts))
	copy(result, contracts)
	return result, nil
}

// ListParties returns all circles in this intersection.
func (r *Runtime) ListParties(ctx context.Context, intersectionID string) ([]intersection.Party, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	contracts, ok := r.contracts[intersectionID]
	if !ok || len(contracts) == 0 {
		return []intersection.Party{}, nil
	}

	return contracts[len(contracts)-1].Parties, nil
}

// IsParty checks if a circle is a party to this intersection.
func (r *Runtime) IsParty(ctx context.Context, intersectionID string, circleID string) (bool, error) {
	parties, err := r.ListParties(ctx, intersectionID)
	if err != nil {
		return false, err
	}

	for _, p := range parties {
		if p.CircleID == circleID {
			return true, nil
		}
	}
	return false, nil
}

// DiscoverForLoop finds or creates an intersection for a loop.
func (r *Runtime) DiscoverForLoop(ctx context.Context, loopCtx primitives.LoopContext, criteria intersection.DiscoveryCriteria) (*intersection.DiscoveryResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// For demo: if PreferExisting, try to find an existing intersection
	if criteria.PreferExisting {
		for id, i := range r.intersections {
			if i.State == intersection.StateActive {
				contracts := r.contracts[id]
				if len(contracts) > 0 {
					contract := contracts[len(contracts)-1]
					for _, party := range contract.Parties {
						if party.CircleID == criteria.IssuerCircleID {
							return &intersection.DiscoveryResult{
								IntersectionID:  id,
								IsNew:           false,
								ContractVersion: i.Version,
								AvailableScopes: criteria.RequiredScopes,
							}, nil
						}
					}
				}
			}
		}
	}

	// Create a new intersection for the demo
	r.idCounter++
	intersectionID := fmt.Sprintf("int-%d", r.idCounter)

	now := time.Now()
	i := &intersection.Intersection{
		ID:        intersectionID,
		TenantID:  "demo-tenant",
		State:     intersection.StateActive,
		Version:   "1.0.0",
		CreatedAt: now,
		UpdatedAt: now,
	}

	r.intersections[intersectionID] = i

	// Create a minimal contract
	parties := []intersection.Party{
		{
			CircleID:      criteria.IssuerCircleID,
			PartyType:     "initiator",
			JoinedAt:      now,
			GrantedScopes: criteria.RequiredScopes,
		},
	}

	if criteria.TargetCircleID != "" {
		parties = append(parties, intersection.Party{
			CircleID:  criteria.TargetCircleID,
			PartyType: "acceptor",
			JoinedAt:  now,
		})
	}

	contract := intersection.Contract{
		IntersectionID: intersectionID,
		Version:        "1.0.0",
		Parties:        parties,
		CreatedAt:      now,
	}

	if criteria.ContractTerms != nil {
		contract.Scopes = criteria.ContractTerms.Scopes
		contract.Ceilings = criteria.ContractTerms.Ceilings
		contract.Governance = criteria.ContractTerms.Governance
	}

	r.contracts[intersectionID] = []intersection.Contract{contract}

	return &intersection.DiscoveryResult{
		IntersectionID:  intersectionID,
		IsNew:           true,
		ContractVersion: "1.0.0",
		AvailableScopes: criteria.RequiredScopes,
	}, nil
}

// generateID generates a random ID with the given prefix.
func generateID(prefix string) string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(bytes))
}

// Verify interface compliance at compile time.
var (
	_ intersection.Runtime       = (*Runtime)(nil)
	_ intersection.LoopDiscoverer = (*Runtime)(nil)
)
