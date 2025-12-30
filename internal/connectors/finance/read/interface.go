// Package read defines the finance read connector interface.
// This is a DATA PLANE component — deterministic only, NO LLM/SLM.
//
// CRITICAL: This package is READ-ONLY by design. No write operations exist.
// No write methods are defined. No write scopes are accepted.
// Execution is architecturally impossible.
//
// Reference: docs/TECHNOLOGY_SELECTION_V8_FINANCIAL_READ.md
package read

import (
	"context"
	"errors"

	"quantumlife/pkg/primitives"
)

// ReadConnector defines the finance read connector interface.
// CRITICAL: This interface contains ONLY read operations.
// No write methods exist by design — execution is impossible.
type ReadConnector interface {
	// ListAccounts returns financial accounts for the connected provider.
	// This is a read-only operation.
	ListAccounts(ctx context.Context, env primitives.ExecutionEnvelope, req ListAccountsRequest) (*AccountsReceipt, error)

	// ListTransactions returns transactions within the specified time window.
	// This is a read-only operation.
	ListTransactions(ctx context.Context, env primitives.ExecutionEnvelope, req ListTransactionsRequest) (*TransactionsReceipt, error)

	// Supports returns the connector's capabilities.
	// CRITICAL: Only Read capability exists — no Write capability is possible.
	Supports(ctx context.Context) Capabilities

	// ProviderInfo returns information about the connected provider.
	ProviderInfo() ProviderInfo
}

// Capabilities describes what the connector supports.
// CRITICAL: Write is intentionally absent — not false, but non-existent.
type Capabilities struct {
	// Read indicates the connector supports read operations.
	Read bool
}

// ProviderInfo contains metadata about a financial provider.
type ProviderInfo struct {
	// ID uniquely identifies this provider.
	ID string

	// Name is the human-readable provider name.
	Name string

	// Type identifies the provider type (e.g., "plaid", "mock").
	Type string

	// InstitutionID identifies the financial institution (if applicable).
	InstitutionID string

	// InstitutionName is the institution's display name.
	InstitutionName string
}

// Scope constants for finance read operations.
// CRITICAL: Only read scopes are defined. Write scopes do not exist.
const (
	// ScopeFinanceRead is the required scope for all finance read operations.
	ScopeFinanceRead = "finance:read"
)

// AllowedScopes is the exhaustive list of permitted scopes for finance read.
// Any scope not in this list MUST be rejected.
// CRITICAL: No write scopes exist in this list.
var AllowedScopes = []string{
	ScopeFinanceRead,
}

// ForbiddenScopePatterns are patterns that MUST be rejected.
// These patterns prevent any write capability from being requested.
var ForbiddenScopePatterns = []string{
	"finance:write",
	"finance:execute",
	"finance:transfer",
	"payment",
	"transfer",
	"initiate",
}

// IsAllowedScope checks if a scope is in the allowed list.
func IsAllowedScope(scope string) bool {
	for _, allowed := range AllowedScopes {
		if scope == allowed {
			return true
		}
	}
	return false
}

// IsForbiddenScope checks if a scope matches any forbidden pattern.
func IsForbiddenScope(scope string) bool {
	for _, pattern := range ForbiddenScopePatterns {
		if containsPattern(scope, pattern) {
			return true
		}
	}
	return false
}

// containsPattern checks if s contains pattern (case-insensitive).
func containsPattern(s, pattern string) bool {
	// Simple substring match
	for i := 0; i <= len(s)-len(pattern); i++ {
		match := true
		for j := 0; j < len(pattern); j++ {
			sc := s[i+j]
			pc := pattern[j]
			// Case-insensitive comparison
			if sc != pc && sc != pc+32 && sc != pc-32 {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// ValidateEnvelopeForFinanceRead validates an envelope for finance read operations.
// CRITICAL: This rejects Execute mode and any non-read scopes.
func ValidateEnvelopeForFinanceRead(env *primitives.ExecutionEnvelope) error {
	// CRITICAL: Execute mode is forbidden for finance read
	if env.Mode == primitives.ModeExecute {
		return ErrExecuteModeNotAllowed
	}

	// Mode must be suggest_only or simulate
	if env.Mode != primitives.ModeSuggestOnly && env.Mode != primitives.ModeSimulate {
		return ErrInvalidMode
	}

	// Validate required fields
	if env.TraceID == "" {
		return ErrTraceIDRequired
	}
	if env.ActorCircleID == "" {
		return ErrActorCircleIDRequired
	}
	if len(env.ScopesUsed) == 0 {
		return ErrScopesRequired
	}

	// CRITICAL: Validate all scopes are allowed (read-only)
	for _, scope := range env.ScopesUsed {
		if IsForbiddenScope(scope) {
			return ErrForbiddenScope
		}
		if !IsAllowedScope(scope) {
			return ErrScopeNotAllowed
		}
	}

	// Must have finance:read scope
	hasFinanceRead := false
	for _, scope := range env.ScopesUsed {
		if scope == ScopeFinanceRead {
			hasFinanceRead = true
			break
		}
	}
	if !hasFinanceRead {
		return ErrFinanceReadScopeRequired
	}

	return nil
}

// Errors for finance read operations.
var (
	// ErrExecuteModeNotAllowed is returned when execute mode is used.
	// CRITICAL: Finance read does not support execute mode.
	ErrExecuteModeNotAllowed = errors.New("execute mode is not allowed for finance read operations")

	// ErrInvalidMode is returned when an unsupported mode is used.
	ErrInvalidMode = errors.New("mode must be suggest_only or simulate for finance read")

	// ErrTraceIDRequired is returned when trace ID is missing.
	ErrTraceIDRequired = errors.New("trace ID is required for audit")

	// ErrActorCircleIDRequired is returned when actor circle ID is missing.
	ErrActorCircleIDRequired = errors.New("actor circle ID is required")

	// ErrScopesRequired is returned when scopes are empty.
	ErrScopesRequired = errors.New("scopes used must be non-empty")

	// ErrScopeNotAllowed is returned when a scope is not in the allowed list.
	ErrScopeNotAllowed = errors.New("scope is not in the allowed list for finance read")

	// ErrForbiddenScope is returned when a forbidden scope pattern is detected.
	ErrForbiddenScope = errors.New("forbidden scope pattern detected (write/transfer/payment)")

	// ErrFinanceReadScopeRequired is returned when finance:read scope is missing.
	ErrFinanceReadScopeRequired = errors.New("finance:read scope is required")

	// ErrProviderUnavailable is returned when the provider is not reachable.
	ErrProviderUnavailable = errors.New("financial provider is unavailable")

	// ErrPartialData is returned when only partial data is available.
	ErrPartialData = errors.New("partial data returned from provider")
)
