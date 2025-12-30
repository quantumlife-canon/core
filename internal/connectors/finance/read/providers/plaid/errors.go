// Package plaid provides a Plaid read-only connector for US/Canada/UK Open Banking.
//
// CRITICAL: This package is READ-ONLY by design. No payment initiation,
// no transfers, no write products. Execution is architecturally impossible.
//
// Reference: docs/TECHNOLOGY_SELECTION_V8_FINANCIAL_READ.md
package plaid

import (
	"errors"
	"fmt"
)

// Provider-specific errors.
var (
	// ErrNotConfigured is returned when Plaid credentials are not set.
	ErrNotConfigured = errors.New("plaid: provider not configured")

	// ErrInvalidToken is returned when the access token is invalid or expired.
	ErrInvalidToken = errors.New("plaid: invalid or expired access token")

	// ErrRateLimited is returned when rate limits are exceeded.
	ErrRateLimited = errors.New("plaid: rate limit exceeded")

	// ErrProviderError is returned for upstream Plaid errors.
	ErrProviderError = errors.New("plaid: provider returned an error")

	// ErrForbiddenProduct is returned when a forbidden product is detected.
	// CRITICAL: Any product related to payments/transfers MUST be rejected.
	ErrForbiddenProduct = errors.New("plaid: forbidden product detected (payments/transfers not allowed)")

	// ErrItemLoginRequired is returned when the user needs to re-authenticate.
	ErrItemLoginRequired = errors.New("plaid: item requires user re-authentication")

	// ErrPartialData is returned when only partial data is available.
	ErrPartialData = errors.New("plaid: partial data returned")

	// ErrInstitutionNotSupported is returned when the institution is not supported.
	ErrInstitutionNotSupported = errors.New("plaid: institution not supported")
)

// APIError represents an error from the Plaid API.
type APIError struct {
	StatusCode   int
	ErrorType    string
	ErrorCode    string
	ErrorMessage string
	RequestID    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("plaid API error: %s - %s (type=%s, code=%s, request_id=%s)",
		e.ErrorMessage, e.ErrorCode, e.ErrorType, e.ErrorCode, e.RequestID)
}

// IsRetryable returns true if the error might succeed on retry.
func (e *APIError) IsRetryable() bool {
	return e.StatusCode >= 500 || e.StatusCode == 429
}

// IsAuthError returns true if the error is an authentication failure.
func (e *APIError) IsAuthError() bool {
	return e.ErrorType == "INVALID_INPUT" || e.ErrorType == "INVALID_ACCESS_TOKEN"
}

// IsItemError returns true if the error is related to the Plaid Item.
func (e *APIError) IsItemError() bool {
	return e.ErrorType == "ITEM_ERROR"
}
