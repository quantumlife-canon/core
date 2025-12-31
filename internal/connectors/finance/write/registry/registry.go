// Package registry provides write provider registration and allowlisting for v9.9 financial execution.
//
// CRITICAL: v9.9 Provider Registry Lock + Write Allowlist Enforcement
//
// This registry makes it structurally difficult/impossible for new or unapproved
// write providers to be used for financial execution. It enforces TWO constraints:
// 1. Runtime: executors MUST consult this registry before invoking any WriteConnector
// 2. CI: guardrails scan for unregistered/unallowed providers
//
// DEFAULT ALLOWLIST:
// - mock-write: Always allowed (simulated, never moves money)
// - truelayer-sandbox: Allowed (sandbox environment)
// - truelayer-live: EXISTS but BLOCKED by default (requires explicit enablement)
//
// Subordinate to:
// - docs/QUANTUMLIFE_CANON_V1.md
// - docs/CANON_ADDENDUM_V9_FINANCIAL_EXECUTION.md
// - docs/TECHNICAL_SPLIT_V9_EXECUTION.md
package registry

import (
	"errors"
	"fmt"
	"sort"
	"sync"
)

// ProviderID uniquely identifies a write provider.
type ProviderID string

// Known provider IDs.
const (
	// ProviderMockWrite is the mock/simulated write provider.
	// CRITICAL: This provider NEVER moves real money.
	ProviderMockWrite ProviderID = "mock-write"

	// ProviderTrueLayerSandbox is the TrueLayer sandbox write provider.
	// This uses TrueLayer's sandbox environment for testing.
	ProviderTrueLayerSandbox ProviderID = "truelayer-sandbox"

	// ProviderTrueLayerLive is the TrueLayer live/production write provider.
	// CRITICAL: This is BLOCKED by default. Real money can move.
	ProviderTrueLayerLive ProviderID = "truelayer-live"
)

// Environment constants.
const (
	EnvSandbox = "sandbox"
	EnvLive    = "live"
	EnvMock    = "mock"
)

// Capability constants.
const (
	CapabilityPaymentCreate = "payment:create"
)

// Entry represents a registered write provider.
type Entry struct {
	// ID uniquely identifies the provider.
	ID ProviderID

	// DisplayName is the human-readable name.
	DisplayName string

	// IsWrite indicates this is a write provider (always true in this registry).
	IsWrite bool

	// Environment identifies the provider environment.
	// Values: "sandbox", "live", "mock"
	Environment string

	// Capabilities lists what operations the provider supports.
	// Example: ["payment:create"]
	Capabilities []string

	// Allowed indicates if this provider can be used.
	// Providers may be registered but NOT allowed.
	Allowed bool

	// BlockReason explains why the provider is blocked (if not allowed).
	BlockReason string
}

// Registry manages write provider registration and allowlisting.
type Registry interface {
	// Get retrieves a provider entry by ID.
	// Returns (entry, true) if found, (Entry{}, false) if not registered.
	Get(id ProviderID) (Entry, bool)

	// List returns all registered provider entries (sorted by ID).
	List() []Entry

	// IsAllowed checks if a provider is registered AND allowed.
	IsAllowed(id ProviderID) bool

	// RequireAllowed validates that a provider can be used.
	// Returns nil if allowed, error if not registered or not allowed.
	RequireAllowed(id ProviderID) error

	// IsLiveEnvironment checks if a provider targets live/production.
	IsLiveEnvironment(id ProviderID) bool

	// AllowedProviderIDs returns a sorted list of allowed provider IDs.
	// v9.12: Used for policy snapshot computation.
	AllowedProviderIDs() []string

	// BlockedProviderIDs returns a sorted list of blocked (but registered) provider IDs.
	// v9.12: Used for policy snapshot computation.
	BlockedProviderIDs() []string
}

// Errors.
var (
	// ErrProviderNotRegistered is returned when a provider is not in the registry.
	ErrProviderNotRegistered = errors.New("provider not registered in write provider registry")

	// ErrProviderNotAllowed is returned when a provider is registered but not allowed.
	ErrProviderNotAllowed = errors.New("provider is registered but not on the allowlist")

	// ErrProviderLiveBlocked is returned when a live/production provider is blocked.
	ErrProviderLiveBlocked = errors.New("live/production provider is blocked by default - explicit enablement required")
)

// ProviderError provides detailed error information for registry failures.
type ProviderError struct {
	ProviderID  ProviderID
	Err         error
	BlockReason string
}

func (e *ProviderError) Error() string {
	if e.BlockReason != "" {
		return fmt.Sprintf("provider %q: %s (reason: %s)", e.ProviderID, e.Err.Error(), e.BlockReason)
	}
	return fmt.Sprintf("provider %q: %s", e.ProviderID, e.Err.Error())
}

func (e *ProviderError) Unwrap() error {
	return e.Err
}

// inMemoryRegistry is the immutable in-memory implementation of Registry.
type inMemoryRegistry struct {
	mu      sync.RWMutex
	entries map[ProviderID]Entry
}

// NewDefaultRegistry creates a new registry with the default allowlist.
//
// DEFAULT ALLOWLIST:
// - mock-write: ALLOWED (simulated only, never moves money)
// - truelayer-sandbox: ALLOWED (TrueLayer sandbox environment)
// - truelayer-live: EXISTS but BLOCKED (live environment requires explicit enablement)
//
// CRITICAL: Adding new providers to the allowlist requires code changes AND
// passing the CI guardrail. This is intentional friction.
func NewDefaultRegistry() Registry {
	r := &inMemoryRegistry{
		entries: make(map[ProviderID]Entry),
	}

	// Register mock-write: ALLOWED
	// This is the default provider for demos and testing.
	// CRITICAL: Must never report MoneyMoved=true.
	r.entries[ProviderMockWrite] = Entry{
		ID:           ProviderMockWrite,
		DisplayName:  "Mock Write Provider",
		IsWrite:      true,
		Environment:  EnvMock,
		Capabilities: []string{CapabilityPaymentCreate},
		Allowed:      true,
		BlockReason:  "",
	}

	// Register truelayer-sandbox: ALLOWED
	// TrueLayer sandbox is safe for testing with test money.
	r.entries[ProviderTrueLayerSandbox] = Entry{
		ID:           ProviderTrueLayerSandbox,
		DisplayName:  "TrueLayer Sandbox",
		IsWrite:      true,
		Environment:  EnvSandbox,
		Capabilities: []string{CapabilityPaymentCreate},
		Allowed:      true,
		BlockReason:  "",
	}

	// Register truelayer-live: EXISTS but BLOCKED
	// CRITICAL: Live/production provider is blocked by default.
	// Real money can move through this provider.
	r.entries[ProviderTrueLayerLive] = Entry{
		ID:           ProviderTrueLayerLive,
		DisplayName:  "TrueLayer Live",
		IsWrite:      true,
		Environment:  EnvLive,
		Capabilities: []string{CapabilityPaymentCreate},
		Allowed:      false,
		BlockReason:  "live/production environment blocked by default",
	}

	return r
}

// NewCustomRegistry creates a registry with a custom set of entries.
// Use for testing only - production code should use NewDefaultRegistry.
func NewCustomRegistry(entries []Entry) Registry {
	r := &inMemoryRegistry{
		entries: make(map[ProviderID]Entry),
	}
	for _, e := range entries {
		r.entries[e.ID] = e
	}
	return r
}

// Get retrieves a provider entry by ID.
func (r *inMemoryRegistry) Get(id ProviderID) (Entry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, ok := r.entries[id]
	return entry, ok
}

// List returns all registered provider entries (sorted by ID for determinism).
func (r *inMemoryRegistry) List() []Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entries := make([]Entry, 0, len(r.entries))
	for _, e := range r.entries {
		entries = append(entries, e)
	}

	// Sort by ID for deterministic output
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ID < entries[j].ID
	})

	return entries
}

// IsAllowed checks if a provider is registered AND allowed.
func (r *inMemoryRegistry) IsAllowed(id ProviderID) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, ok := r.entries[id]
	if !ok {
		return false
	}
	return entry.Allowed
}

// RequireAllowed validates that a provider can be used.
// Returns nil if allowed, error if not registered or not allowed.
func (r *inMemoryRegistry) RequireAllowed(id ProviderID) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, ok := r.entries[id]
	if !ok {
		return &ProviderError{
			ProviderID: id,
			Err:        ErrProviderNotRegistered,
		}
	}

	if !entry.Allowed {
		// Distinguish between live-blocked and generally not allowed
		if entry.Environment == EnvLive {
			return &ProviderError{
				ProviderID:  id,
				Err:         ErrProviderLiveBlocked,
				BlockReason: entry.BlockReason,
			}
		}
		return &ProviderError{
			ProviderID:  id,
			Err:         ErrProviderNotAllowed,
			BlockReason: entry.BlockReason,
		}
	}

	return nil
}

// IsLiveEnvironment checks if a provider targets live/production.
func (r *inMemoryRegistry) IsLiveEnvironment(id ProviderID) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, ok := r.entries[id]
	if !ok {
		return false
	}
	return entry.Environment == EnvLive
}

// AllowedProviderIDs returns a sorted list of allowed provider IDs.
// v9.12: Used for policy snapshot computation.
func (r *inMemoryRegistry) AllowedProviderIDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]string, 0)
	for id, entry := range r.entries {
		if entry.Allowed {
			result = append(result, string(id))
		}
	}
	sort.Strings(result)
	return result
}

// BlockedProviderIDs returns a sorted list of blocked (but registered) provider IDs.
// v9.12: Used for policy snapshot computation.
func (r *inMemoryRegistry) BlockedProviderIDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]string, 0)
	for id, entry := range r.entries {
		if !entry.Allowed {
			result = append(result, string(id))
		}
	}
	sort.Strings(result)
	return result
}

// Verify interface compliance.
var _ Registry = (*inMemoryRegistry)(nil)
