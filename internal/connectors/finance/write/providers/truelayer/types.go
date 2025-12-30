// Package truelayer provides TrueLayer payment connector for v9 Slice 3.
//
// CRITICAL: This is the FIRST slice where money may actually move.
// It must be minimal, constrained, auditable, interruptible, and boring.
//
// HARD SAFETY CONSTRAINTS:
// - TrueLayer Payments API only
// - Default cap £1.00 (100 pence)
// - Pre-defined payees only
// - Explicit per-action approval
// - Forced pause before execution
// - No retries
// - Full audit trail
//
// Subordinate to:
// - docs/CANON_ADDENDUM_V9_FINANCIAL_EXECUTION.md
// - docs/TECHNICAL_SPLIT_V9_EXECUTION.md
package truelayer

import (
	"time"
)

// TrueLayer Payments API URLs.
const (
	// Sandbox endpoints
	SandboxPaymentsURL = "https://payment.truelayer-sandbox.com"
	SandboxAuthURL     = "https://auth.truelayer-sandbox.com"

	// Production endpoints
	LivePaymentsURL = "https://payment.truelayer.com"
	LiveAuthURL     = "https://auth.truelayer.com"
)

// TrueLayer Payments API scopes.
// CRITICAL: Only minimal payment scopes are allowed.
const (
	// ScopePayments allows creating payments.
	ScopePayments = "payments"
)

// AllowedWriteScopes is the exhaustive list of permitted write scopes.
// CRITICAL: This is intentionally minimal.
var AllowedWriteScopes = []string{
	ScopePayments,
}

// ForbiddenWriteScopePatterns are patterns that MUST be rejected.
var ForbiddenWriteScopePatterns = []string{
	"standing_order",
	"direct_debit",
	"recurring",
	"mandate",
	"bulk",
	"batch",
	"schedule",
	"auto",
}

// PaymentRequest is the TrueLayer payment initiation request.
type PaymentRequest struct {
	// AmountInMinor is the amount in minor units (pence for GBP).
	AmountInMinor int64 `json:"amount_in_minor"`

	// Currency is the ISO 4217 currency code.
	Currency string `json:"currency"`

	// PaymentMethod specifies the payment method.
	PaymentMethod PaymentMethod `json:"payment_method"`

	// User contains user information.
	User PaymentUser `json:"user"`

	// Metadata contains additional metadata.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// PaymentMethod specifies the payment method configuration.
type PaymentMethod struct {
	// Type is the payment method type.
	Type string `json:"type"`

	// ProviderSelection specifies provider selection.
	ProviderSelection *ProviderSelection `json:"provider_selection,omitempty"`

	// Beneficiary specifies the payment beneficiary.
	Beneficiary *Beneficiary `json:"beneficiary,omitempty"`
}

// ProviderSelection specifies how the provider is selected.
type ProviderSelection struct {
	// Type is "user_selected" or "preselected".
	Type string `json:"type"`

	// Filter is the provider filter.
	Filter *ProviderFilter `json:"filter,omitempty"`

	// ProviderID is the preselected provider ID.
	ProviderID string `json:"provider_id,omitempty"`
}

// ProviderFilter filters available providers.
type ProviderFilter struct {
	// Countries limits to specific countries.
	Countries []string `json:"countries,omitempty"`

	// ReleaseChannel limits to specific release channel.
	ReleaseChannel string `json:"release_channel,omitempty"`
}

// Beneficiary specifies the payment recipient.
type Beneficiary struct {
	// Type is "external_account" or "merchant_account".
	Type string `json:"type"`

	// AccountHolderName is the beneficiary name.
	AccountHolderName string `json:"account_holder_name,omitempty"`

	// Reference is the payment reference.
	Reference string `json:"reference,omitempty"`

	// AccountIdentifier is the bank account details.
	AccountIdentifier *AccountIdentifier `json:"account_identifier,omitempty"`

	// MerchantAccountID is the merchant account ID.
	MerchantAccountID string `json:"merchant_account_id,omitempty"`
}

// AccountIdentifier contains bank account details.
type AccountIdentifier struct {
	// Type is "sort_code_account_number" or "iban".
	Type string `json:"type"`

	// SortCode is the UK sort code.
	SortCode string `json:"sort_code,omitempty"`

	// AccountNumber is the account number.
	AccountNumber string `json:"account_number,omitempty"`

	// IBAN is the international bank account number.
	IBAN string `json:"iban,omitempty"`
}

// PaymentUser contains user information for the payment.
type PaymentUser struct {
	// ID is the user ID.
	ID string `json:"id,omitempty"`

	// Name is the user name.
	Name string `json:"name,omitempty"`

	// Email is the user email.
	Email string `json:"email,omitempty"`

	// Phone is the user phone.
	Phone string `json:"phone,omitempty"`
}

// PaymentResponse is the TrueLayer payment creation response.
type PaymentResponse struct {
	// ID is the payment ID.
	ID string `json:"id"`

	// Status is the payment status.
	Status string `json:"status"`

	// ResourceToken is the resource token for status checks.
	ResourceToken string `json:"resource_token"`

	// User contains user information.
	User PaymentUserResponse `json:"user"`

	// CreatedAt is when the payment was created.
	CreatedAt time.Time `json:"created_at"`
}

// PaymentUserResponse contains user information from response.
type PaymentUserResponse struct {
	ID string `json:"id"`
}

// PaymentStatusResponse is the TrueLayer payment status response.
type PaymentStatusResponse struct {
	// ID is the payment ID.
	ID string `json:"id"`

	// Status is the current status.
	Status string `json:"status"`

	// PaymentMethod contains payment method details.
	PaymentMethod PaymentMethodResponse `json:"payment_method,omitempty"`

	// FailureReason explains failure if applicable.
	FailureReason string `json:"failure_reason,omitempty"`

	// CreatedAt is when the payment was created.
	CreatedAt time.Time `json:"created_at"`

	// AuthorizedAt is when the payment was authorized.
	AuthorizedAt *time.Time `json:"authorized_at,omitempty"`

	// ExecutedAt is when the payment was executed.
	ExecutedAt *time.Time `json:"executed_at,omitempty"`

	// SettledAt is when the payment was settled.
	SettledAt *time.Time `json:"settled_at,omitempty"`

	// FailedAt is when the payment failed.
	FailedAt *time.Time `json:"failed_at,omitempty"`
}

// PaymentMethodResponse contains payment method from response.
type PaymentMethodResponse struct {
	// Type is the payment method type.
	Type string `json:"type"`

	// ProviderID is the provider used.
	ProviderID string `json:"provider_id,omitempty"`
}

// TrueLayer payment statuses.
const (
	StatusAuthorizationRequired = "authorization_required"
	StatusAuthorizing           = "authorizing"
	StatusAuthorized            = "authorized"
	StatusExecuted              = "executed"
	StatusSettled               = "settled"
	StatusFailed                = "failed"
)

// ErrorResponse is the TrueLayer error response.
type ErrorResponse struct {
	// Type is the error type.
	Type string `json:"type"`

	// Title is a short error title.
	Title string `json:"title,omitempty"`

	// Detail is a detailed error description.
	Detail string `json:"detail,omitempty"`

	// TraceID is the request trace ID.
	TraceID string `json:"trace_id,omitempty"`
}

// TokenResponse is the OAuth token response.
type TokenResponse struct {
	// AccessToken is the access token.
	AccessToken string `json:"access_token"`

	// TokenType is the token type.
	TokenType string `json:"token_type"`

	// ExpiresIn is seconds until expiry.
	ExpiresIn int `json:"expires_in"`

	// Scope is the granted scope.
	Scope string `json:"scope,omitempty"`
}

// SandboxBeneficiary returns sandbox beneficiary details.
// CRITICAL: For v9 Slice 3, only sandbox beneficiaries are supported.
func SandboxBeneficiary() *Beneficiary {
	return &Beneficiary{
		Type:              "external_account",
		AccountHolderName: "TrueLayer Sandbox",
		Reference:         "QuantumLife-v9-Test",
		AccountIdentifier: &AccountIdentifier{
			Type:          "sort_code_account_number",
			SortCode:      "040668",
			AccountNumber: "00000871",
		},
	}
}
