// Package truelayer provides a TrueLayer read-only connector for UK/EU Open Banking.
//
// CRITICAL: This package is READ-ONLY by design. No payment initiation,
// no transfers, no write scopes. Execution is architecturally impossible.
//
// Reference: docs/TECHNOLOGY_SELECTION_V8_FINANCIAL_READ.md
package truelayer

import (
	"errors"
	"fmt"
)

// Provider-specific errors.
var (
	// ErrNotConfigured is returned when TrueLayer credentials are not set.
	ErrNotConfigured = errors.New("truelayer: provider not configured")

	// ErrInvalidToken is returned when the access token is invalid or expired.
	ErrInvalidToken = errors.New("truelayer: invalid or expired access token")

	// ErrRateLimited is returned when rate limits are exceeded.
	ErrRateLimited = errors.New("truelayer: rate limit exceeded")

	// ErrProviderError is returned for upstream TrueLayer errors.
	ErrProviderError = errors.New("truelayer: provider returned an error")

	// ErrForbiddenScope is returned when a forbidden scope is detected.
	// CRITICAL: Any scope containing "payment", "transfer", or "write" MUST be rejected.
	ErrForbiddenScope = errors.New("truelayer: forbidden scope detected (payments/transfers not allowed)")

	// ErrConsentRequired is returned when the user hasn't consented.
	ErrConsentRequired = errors.New("truelayer: user consent required")

	// ErrPartialData is returned when only partial data is available.
	ErrPartialData = errors.New("truelayer: partial data returned")
)

// APIError represents an error from the TrueLayer API.
type APIError struct {
	StatusCode int
	ErrorType  string
	Message    string
	RequestID  string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("truelayer API error: %s (status=%d, type=%s, request_id=%s)",
		e.Message, e.StatusCode, e.ErrorType, e.RequestID)
}

// IsRetryable returns true if the error might succeed on retry.
func (e *APIError) IsRetryable() bool {
	return e.StatusCode >= 500 || e.StatusCode == 429
}

// IsAuthError returns true if the error is an authentication failure.
func (e *APIError) IsAuthError() bool {
	return e.StatusCode == 401 || e.StatusCode == 403
}
