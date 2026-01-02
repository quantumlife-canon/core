// Package auth provides the Token Broker pattern for OAuth credential management.
// Credentials are owned by Circles; usage is authorized via Intersections.
//
// Reference: docs/TECHNICAL_SPLIT_V1.md §3.5 Action Execution Layer
//
// CRITICAL: v5 is READ-ONLY. Write scopes cannot be minted.
package auth

import (
	"context"
	"errors"
	"time"

	"quantumlife/pkg/primitives"
)

// ProviderID identifies an OAuth provider.
type ProviderID string

// Supported providers.
const (
	// ProviderGoogle is Google Calendar API.
	ProviderGoogle ProviderID = "google"

	// ProviderMicrosoft is Microsoft Graph API.
	ProviderMicrosoft ProviderID = "microsoft"

	// ProviderTrueLayer is TrueLayer Open Banking API (v8.2).
	// CRITICAL: This provider is READ-ONLY. No payment scopes are allowed.
	ProviderTrueLayer ProviderID = "truelayer"

	// ProviderPlaid is Plaid financial data API (v8.3).
	// CRITICAL: This provider is READ-ONLY. No payment products are allowed.
	ProviderPlaid ProviderID = "plaid"
)

// ValidProviders returns all valid provider IDs.
func ValidProviders() []ProviderID {
	return []ProviderID{ProviderGoogle, ProviderMicrosoft, ProviderTrueLayer, ProviderPlaid}
}

// IsValidProvider checks if the provider ID is valid.
func IsValidProvider(p ProviderID) bool {
	switch p {
	case ProviderGoogle, ProviderMicrosoft, ProviderTrueLayer, ProviderPlaid:
		return true
	default:
		return false
	}
}

// TokenBroker manages OAuth tokens for calendar providers.
// Tokens are owned by Circles and usage is authorized via Intersections.
type TokenBroker interface {
	// BeginOAuth generates an OAuth authorization URL.
	// The user should be redirected to this URL to authorize access.
	//
	// Parameters:
	//   - provider: The OAuth provider (google, microsoft)
	//   - redirectURI: Where to redirect after authorization
	//   - state: CSRF protection state (should be random and verified)
	//   - scopes: QuantumLife scopes to request (e.g., "calendar:read")
	//
	// Returns the authorization URL.
	BeginOAuth(provider ProviderID, redirectURI string, state string, scopes []string) (authURL string, err error)

	// ExchangeCode exchanges an authorization code for tokens.
	// This stores the refresh token (encrypted) and returns a handle.
	//
	// NOTE: In v5, this is called via CLI, not HTTP callback.
	//
	// Parameters:
	//   - provider: The OAuth provider
	//   - code: The authorization code from the OAuth callback
	//   - redirectURI: Must match the original request
	//
	// Returns a TokenHandle that can be used to mint access tokens.
	ExchangeCode(ctx context.Context, provider ProviderID, code string, redirectURI string) (TokenHandle, error)

	// ExchangeCodeForCircle exchanges an authorization code for a specific circle.
	// This is the web callback version that binds the token to a circle.
	//
	// Parameters:
	//   - circleID: The circle to bind the token to
	//   - provider: The OAuth provider
	//   - code: The authorization code from the OAuth callback
	//   - redirectURI: Must match the original request
	//
	// Returns a TokenHandle that can be used to mint access tokens.
	ExchangeCodeForCircle(ctx context.Context, circleID string, provider ProviderID, code string, redirectURI string) (TokenHandle, error)

	// MintAccessToken mints an access token for a specific operation.
	// The token is minted from the stored refresh token.
	//
	// CRITICAL: This requires an AuthorizationProof. The proof must exist
	// and authorize the requested scopes.
	//
	// Parameters:
	//   - envelope: The execution envelope with traceability context
	//   - provider: The OAuth provider
	//   - requiredScopes: QuantumLife scopes needed (e.g., "calendar:read")
	//
	// Returns an AccessToken with the actual provider token.
	MintAccessToken(ctx context.Context, envelope primitives.ExecutionEnvelope, provider ProviderID, requiredScopes []string) (AccessToken, error)

	// MintReadOnlyAccessToken mints an access token for read-only operations.
	// This is a simpler path for ingestion pipelines that don't require full authorization.
	//
	// CRITICAL: Only read scopes are allowed. Write scopes are rejected.
	//
	// Parameters:
	//   - circleID: The circle that owns the token
	//   - provider: The OAuth provider
	//   - requiredScopes: QuantumLife scopes needed (e.g., "email:read")
	//
	// Returns an AccessToken with the actual provider token.
	MintReadOnlyAccessToken(ctx context.Context, circleID string, provider ProviderID, requiredScopes []string) (AccessToken, error)

	// RevokeToken revokes the stored refresh token for a circle/provider.
	RevokeToken(ctx context.Context, circleID string, provider ProviderID) error

	// HasToken checks if a circle has a stored token for a provider.
	HasToken(ctx context.Context, circleID string, provider ProviderID) (bool, error)
}

// TokenHandle is an opaque identifier for a stored refresh token.
// The actual token is stored encrypted; this handle references it.
type TokenHandle struct {
	// ID is the unique identifier for this token handle.
	ID string

	// CircleID is the circle that owns this token.
	CircleID string

	// Provider is the OAuth provider.
	Provider ProviderID

	// Scopes are the granted scopes (QuantumLife format).
	Scopes []string

	// CreatedAt is when the token was created.
	CreatedAt time.Time

	// ExpiresAt is when the refresh token expires (if known).
	ExpiresAt time.Time
}

// AccessToken is a short-lived access token for provider API calls.
// The token string should be treated as sensitive and redacted in logs.
type AccessToken struct {
	// Token is the actual access token string.
	// SENSITIVE: Must be redacted in logs.
	Token string

	// Expiry is when this access token expires.
	Expiry time.Time

	// Provider is the OAuth provider this token is for.
	Provider ProviderID

	// Scopes are the granted scopes (provider format).
	ProviderScopes []string
}

// IsExpired returns true if the access token has expired.
func (t AccessToken) IsExpired() bool {
	return time.Now().After(t.Expiry)
}

// RedactedToken returns a redacted version of the token for logging.
func (t AccessToken) RedactedToken() string {
	if len(t.Token) <= 8 {
		return "***"
	}
	return t.Token[:4] + "..." + t.Token[len(t.Token)-4:]
}

// Token broker errors.
var (
	// ErrInvalidProvider is returned for unknown providers.
	ErrInvalidProvider = errors.New("invalid provider")

	// ErrNoToken is returned when no token exists for the circle/provider.
	ErrNoToken = errors.New("no token found for circle/provider")

	// ErrTokenExpired is returned when the refresh token has expired.
	ErrTokenExpired = errors.New("refresh token has expired")

	// ErrWriteScopeNotAllowed is returned when write scopes are requested in v5.
	ErrWriteScopeNotAllowed = errors.New("write scopes are not allowed in v5 (read-only mode)")

	// ErrAuthorizationRequired is returned when no authorization proof is provided.
	ErrAuthorizationRequired = errors.New("authorization proof is required to mint tokens")

	// ErrScopeNotGranted is returned when requested scope is not in the authorization.
	ErrScopeNotGranted = errors.New("requested scope is not granted by authorization")

	// ErrProviderNotConfigured is returned when provider credentials are not set.
	ErrProviderNotConfigured = errors.New("provider credentials not configured")

	// ErrInvalidCode is returned when OAuth code exchange fails.
	ErrInvalidCode = errors.New("invalid or expired authorization code")
)

// AuthorityChecker verifies that an authorization proof exists and is valid.
// This is injected to avoid circular dependencies with the authority package.
type AuthorityChecker interface {
	// GetProof retrieves an authorization proof by ID.
	GetProof(ctx context.Context, proofID string) (AuthProofSummary, error)
}

// AuthProofSummary contains the minimal authorization proof information
// needed by the token broker. This avoids importing authority types directly.
type AuthProofSummary struct {
	// ID is the proof identifier.
	ID string

	// Authorized indicates if the proof grants authorization.
	Authorized bool

	// ScopesGranted lists the scopes granted by this proof.
	ScopesGranted []string

	// IntersectionID is the intersection that provided the authorization.
	IntersectionID string

	// ContractVersion is the contract version used.
	ContractVersion string
}
