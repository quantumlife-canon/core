// Package normalize provides provider-specific normalizers for v8.4.
//
// Normalizers convert raw provider data into canonical finance types,
// computing canonical IDs for deduplication and reconciliation.
//
// CRITICAL: Normalization is deterministic. No randomness allowed.
// The same raw input must always produce the same canonical output.
//
// Reference: docs/TECHNOLOGY_SELECTION_V8_FINANCIAL_READ.md
package normalize

import (
	"time"

	"quantumlife/pkg/primitives/finance"
)

// Normalizer converts raw provider data into canonical finance types.
// Implementations must be deterministic — same input produces same output.
type Normalizer interface {
	// NormalizeAccounts converts raw provider accounts to canonical form.
	// Returns canonical NormalizedAccount with computed CanonicalAccountID.
	NormalizeAccounts(provider string, raw any) ([]NormalizedAccountResult, error)

	// NormalizeTransactions converts raw provider transactions to canonical form.
	// Returns canonical TransactionRecord with computed CanonicalTransactionID.
	NormalizeTransactions(provider string, raw any, accountMapping map[string]string) ([]NormalizedTransactionResult, error)

	// Provider returns the provider this normalizer handles.
	Provider() string
}

// NormalizedAccountResult wraps a normalized account with identity metadata.
type NormalizedAccountResult struct {
	// Account is the canonical account representation.
	Account finance.NormalizedAccount

	// CanonicalID is the computed canonical account ID.
	CanonicalID string

	// ProviderAccountID is the original provider's account ID.
	ProviderAccountID string
}

// NormalizedTransactionResult wraps a normalized transaction with identity metadata.
type NormalizedTransactionResult struct {
	// Transaction is the canonical transaction representation.
	Transaction finance.TransactionRecord

	// CanonicalID is the computed canonical transaction ID.
	CanonicalID string

	// MatchKey is the pending→posted match key.
	MatchKey string

	// ProviderTransactionID is the original provider's transaction ID.
	ProviderTransactionID string

	// IsPending indicates if this is a pending transaction.
	IsPending bool
}

// NormalizationContext provides context for normalization operations.
type NormalizationContext struct {
	// OwnerType is "circle" or "intersection".
	OwnerType string

	// OwnerID is the circle or intersection ID.
	OwnerID string

	// SourceProvider is the provider name.
	SourceProvider string

	// TraceID links to the sync operation.
	TraceID string

	// NormalizerVersion is the version of normalization rules.
	NormalizerVersion string

	// SchemaVersion is the schema version.
	SchemaVersion string

	// NormalizedAt is when normalization occurred.
	NormalizedAt time.Time
}

// Registry holds registered normalizers by provider.
type Registry struct {
	normalizers map[string]Normalizer
}

// NewRegistry creates a new normalizer registry.
func NewRegistry() *Registry {
	return &Registry{
		normalizers: make(map[string]Normalizer),
	}
}

// Register adds a normalizer to the registry.
func (r *Registry) Register(n Normalizer) {
	r.normalizers[n.Provider()] = n
}

// Get retrieves a normalizer by provider.
func (r *Registry) Get(provider string) (Normalizer, bool) {
	n, ok := r.normalizers[provider]
	return n, ok
}

// Providers returns all registered provider names.
func (r *Registry) Providers() []string {
	providers := make([]string, 0, len(r.normalizers))
	for p := range r.normalizers {
		providers = append(providers, p)
	}
	return providers
}

// DefaultRegistry returns a registry with all standard normalizers.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(&PlaidNormalizer{})
	r.Register(&TrueLayerNormalizer{})
	r.Register(&MockNormalizer{})
	return r
}

// Error types for normalization.
type NormalizationError struct {
	Provider string
	Message  string
	Cause    error
}

func (e *NormalizationError) Error() string {
	if e.Cause != nil {
		return "normalize: " + e.Provider + ": " + e.Message + ": " + e.Cause.Error()
	}
	return "normalize: " + e.Provider + ": " + e.Message
}

func (e *NormalizationError) Unwrap() error {
	return e.Cause
}

// ErrUnknownProvider indicates an unsupported provider.
var ErrUnknownProvider = &NormalizationError{Message: "unknown provider"}

// ErrInvalidInput indicates the raw input was not the expected type.
var ErrInvalidInput = &NormalizationError{Message: "invalid input type"}
