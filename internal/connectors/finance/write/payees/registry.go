// Package payees provides v9.10 payee registry enforcement.
//
// CRITICAL: v9.10 Payee Registry Lock eliminates free-text recipients.
// ALL executions MUST reference a registered PayeeID.
//
// This mirrors v9.9 provider registry enforcement but for recipients.
//
// NON-NEGOTIABLE INVARIANTS:
// - No free-text recipients in any write execution path
// - No runtime-supplied payment destinations
// - No payee creation during execution
// - No implicit or inferred recipients
// - ALL payees must be pre-registered and explicitly allowed
//
// Subordinate to:
// - docs/QUANTUMLIFE_CANON_V1.md
// - docs/CANON_ADDENDUM_V9_FINANCIAL_EXECUTION.md
// - docs/TECHNICAL_SPLIT_V9_EXECUTION.md
// - docs/ADR/ADR-0012-write-provider-registry-lock.md
// - docs/ADR/ADR-0013-payee-registry-lock.md
package payees

import (
	"errors"
	"fmt"
	"sort"
)

// PayeeID is the canonical payee identifier.
// CRITICAL: This is the ONLY way to reference a payment recipient.
// Free-text recipients are architecturally impossible.
type PayeeID string

// Known payee IDs.
const (
	// PayeeSandboxUtility is a sandbox utility provider for testing.
	PayeeSandboxUtility PayeeID = "sandbox-utility"

	// PayeeSandboxRent is a sandbox rent recipient for testing.
	PayeeSandboxRent PayeeID = "sandbox-rent"

	// PayeeSandboxMerchant is a sandbox merchant for testing.
	PayeeSandboxMerchant PayeeID = "sandbox-merchant"
)

// Environment represents the payee environment.
type Environment string

const (
	// EnvSandbox is the sandbox/testing environment.
	EnvSandbox Environment = "sandbox"

	// EnvLive is the live/production environment.
	EnvLive Environment = "live"

	// EnvMock is the mock/simulated environment.
	EnvMock Environment = "mock"
)

// Entry represents a registered payee in the registry.
type Entry struct {
	// ID is the unique payee identifier.
	ID PayeeID

	// DisplayName is the human-readable name for audit purposes.
	DisplayName string

	// ProviderID is the provider this payee is registered with.
	// CRITICAL: Payee must match the write provider being used.
	ProviderID string

	// Environment is sandbox, live, or mock.
	Environment Environment

	// Allowed indicates if this payee can be used for execution.
	// Live payees are NOT allowed by default.
	Allowed bool

	// AccountIdentifier is the provider-specific destination identifier.
	// For TrueLayer sandbox, this is a sandbox beneficiary ID.
	AccountIdentifier string

	// Currency is the supported currency for this payee.
	Currency string

	// BlockReason explains why payee is blocked (if Allowed=false).
	BlockReason string
}

// Registry provides payee lookup and validation.
//
// CRITICAL: All execution paths MUST consult this registry.
// No free-text recipients allowed.
type Registry interface {
	// Get returns a payee entry by ID.
	Get(id PayeeID) (Entry, bool)

	// List returns all registered payees.
	List() []Entry

	// IsRegistered returns true if the payee exists in the registry.
	IsRegistered(id PayeeID) bool

	// IsAllowed returns true if the payee is allowed for the given provider.
	// CRITICAL: Returns false if:
	// - Payee is not registered
	// - Payee provider doesn't match
	// - Payee is explicitly blocked
	IsAllowed(payeeID PayeeID, providerID string) bool

	// RequireAllowed returns an error if the payee cannot be used.
	// Use this in execution paths - it provides detailed error information.
	RequireAllowed(payeeID PayeeID, providerID string) error

	// IsLiveEnvironment returns true if the payee is in live environment.
	IsLiveEnvironment(id PayeeID) bool
}

// Sentinel errors for payee registry.
var (
	// ErrPayeeNotRegistered indicates the payee is not in the registry.
	ErrPayeeNotRegistered = errors.New("payee not registered")

	// ErrPayeeNotAllowed indicates the payee exists but is not allowed.
	ErrPayeeNotAllowed = errors.New("payee not on allowlist")

	// ErrPayeeLiveBlocked indicates a live payee is blocked by default.
	ErrPayeeLiveBlocked = errors.New("live payee blocked by default")

	// ErrPayeeProviderMismatch indicates the payee is for a different provider.
	ErrPayeeProviderMismatch = errors.New("payee registered for different provider")
)

// PayeeError provides detailed error information for payee validation failures.
type PayeeError struct {
	PayeeID     PayeeID
	ProviderID  string
	Err         error
	BlockReason string
}

func (e *PayeeError) Error() string {
	msg := fmt.Sprintf("payee %q: %v", e.PayeeID, e.Err)
	if e.BlockReason != "" {
		msg += fmt.Sprintf(" (%s)", e.BlockReason)
	}
	return msg
}

func (e *PayeeError) Unwrap() error {
	return e.Err
}

// defaultRegistry implements Registry with an immutable payee list.
type defaultRegistry struct {
	entries map[PayeeID]Entry
}

// NewDefaultRegistry creates the default payee registry.
//
// Default allowlist:
// - sandbox-utility: ALLOWED (sandbox testing)
// - sandbox-rent: ALLOWED (sandbox testing)
// - sandbox-merchant: ALLOWED (sandbox testing)
//
// Any unknown PayeeID: BLOCKED
// Live payees: BLOCKED by default
func NewDefaultRegistry() Registry {
	entries := map[PayeeID]Entry{
		PayeeSandboxUtility: {
			ID:                PayeeSandboxUtility,
			DisplayName:       "Sandbox Utility Provider",
			ProviderID:        "mock-write",
			Environment:       EnvSandbox,
			Allowed:           true,
			AccountIdentifier: "sandbox-beneficiary-utility",
			Currency:          "GBP",
		},
		PayeeSandboxRent: {
			ID:                PayeeSandboxRent,
			DisplayName:       "Sandbox Rent Recipient",
			ProviderID:        "mock-write",
			Environment:       EnvSandbox,
			Allowed:           true,
			AccountIdentifier: "sandbox-beneficiary-rent",
			Currency:          "GBP",
		},
		PayeeSandboxMerchant: {
			ID:                PayeeSandboxMerchant,
			DisplayName:       "Sandbox Test Merchant",
			ProviderID:        "mock-write",
			Environment:       EnvSandbox,
			Allowed:           true,
			AccountIdentifier: "sandbox-beneficiary-merchant",
			Currency:          "GBP",
		},
	}

	// Also register sandbox payees for truelayer-sandbox provider
	entries["sandbox-utility-tl"] = Entry{
		ID:                "sandbox-utility-tl",
		DisplayName:       "TrueLayer Sandbox Utility",
		ProviderID:        "truelayer-sandbox",
		Environment:       EnvSandbox,
		Allowed:           true,
		AccountIdentifier: "truelayer-sandbox-utility",
		Currency:          "GBP",
	}

	entries["sandbox-rent-tl"] = Entry{
		ID:                "sandbox-rent-tl",
		DisplayName:       "TrueLayer Sandbox Rent",
		ProviderID:        "truelayer-sandbox",
		Environment:       EnvSandbox,
		Allowed:           true,
		AccountIdentifier: "truelayer-sandbox-rent",
		Currency:          "GBP",
	}

	return &defaultRegistry{entries: entries}
}

// NewCustomRegistry creates a registry with custom entries.
// Use for testing only - production should use NewDefaultRegistry.
func NewCustomRegistry(customEntries []Entry) Registry {
	entries := make(map[PayeeID]Entry)
	for _, e := range customEntries {
		entries[e.ID] = e
	}
	return &defaultRegistry{entries: entries}
}

func (r *defaultRegistry) Get(id PayeeID) (Entry, bool) {
	entry, ok := r.entries[id]
	return entry, ok
}

func (r *defaultRegistry) List() []Entry {
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

func (r *defaultRegistry) IsRegistered(id PayeeID) bool {
	_, ok := r.entries[id]
	return ok
}

func (r *defaultRegistry) IsAllowed(payeeID PayeeID, providerID string) bool {
	entry, ok := r.entries[payeeID]
	if !ok {
		return false
	}
	if !entry.Allowed {
		return false
	}
	// Payee must match provider (or be mock-write which accepts any mock payee)
	if entry.ProviderID != providerID && providerID != "mock-write" {
		return false
	}
	return true
}

func (r *defaultRegistry) RequireAllowed(payeeID PayeeID, providerID string) error {
	entry, ok := r.entries[payeeID]
	if !ok {
		return &PayeeError{
			PayeeID:     payeeID,
			ProviderID:  providerID,
			Err:         ErrPayeeNotRegistered,
			BlockReason: "payee ID not found in registry",
		}
	}

	// Check if live environment is blocked
	if entry.Environment == EnvLive && !entry.Allowed {
		return &PayeeError{
			PayeeID:     payeeID,
			ProviderID:  providerID,
			Err:         ErrPayeeLiveBlocked,
			BlockReason: "live payees blocked by default",
		}
	}

	// Check if payee is explicitly blocked
	if !entry.Allowed {
		return &PayeeError{
			PayeeID:     payeeID,
			ProviderID:  providerID,
			Err:         ErrPayeeNotAllowed,
			BlockReason: entry.BlockReason,
		}
	}

	// Check provider match (mock-write accepts sandbox payees for testing)
	if entry.ProviderID != providerID && providerID != "mock-write" {
		return &PayeeError{
			PayeeID:     payeeID,
			ProviderID:  providerID,
			Err:         ErrPayeeProviderMismatch,
			BlockReason: fmt.Sprintf("payee registered for %s, not %s", entry.ProviderID, providerID),
		}
	}

	return nil
}

func (r *defaultRegistry) IsLiveEnvironment(id PayeeID) bool {
	entry, ok := r.entries[id]
	if !ok {
		return false
	}
	return entry.Environment == EnvLive
}
