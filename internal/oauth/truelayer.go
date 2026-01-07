// Package oauth provides TrueLayer OAuth flow handling.
//
// Phase 29: TrueLayer Read-Only Connect (UK Sandbox) + Finance Mirror Proof
// Reference: docs/ADR/ADR-0060-phase29-truelayer-readonly-finance-mirror.md
//
// CRITICAL: Read-only scopes only (accounts, balance, transactions).
// CRITICAL: No goroutines. All operations synchronous.
// CRITICAL: Stdlib only (net/http).
// CRITICAL: No payment scopes allowed - hard blocked.
package oauth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TrueLayer environment constants.
const (
	TrueLayerEnvSandbox = "uk_sandbox"
	TrueLayerEnvLive    = "uk_live"

	// TrueLayer sandbox URLs
	trueLayerSandboxAuthURL = "https://auth.truelayer-sandbox.com"
	trueLayerSandboxAPIURL  = "https://api.truelayer-sandbox.com"

	// TrueLayer live URLs (not used in Phase 29)
	trueLayerLiveAuthURL = "https://auth.truelayer.com"
	trueLayerLiveAPIURL  = "https://api.truelayer.com"
)

// TrueLayerScopes defines the only allowed scopes for TrueLayer OAuth.
// CRITICAL: Read-only only. No payment scopes.
var TrueLayerScopes = []string{
	"accounts",
	"balance",
	"transactions",
	"offline_access",
}

// TrueLayerForbiddenScopePatterns are patterns that MUST be rejected.
var TrueLayerForbiddenScopePatterns = []string{
	"payment",
	"payments",
	"pay",
	"transfer",
	"write",
	"initiate",
	"standing_order",
	"direct_debit",
	"beneficiar",
	"mandate",
}

// TrueLayerHandler handles TrueLayer OAuth flows.
type TrueLayerHandler struct {
	stateManager *StateManager
	httpClient   *http.Client
	redirectBase string
	clock        func() time.Time

	// TrueLayer configuration
	clientID     string
	clientSecret string
	environment  string
	authURL      string
	apiURL       string
}

// TrueLayerConfig configures the TrueLayer OAuth handler.
type TrueLayerConfig struct {
	StateManager *StateManager
	HTTPClient   *http.Client
	RedirectBase string
	Clock        func() time.Time
	ClientID     string
	ClientSecret string
	Environment  string // "uk_sandbox" or "uk_live"
}

// NewTrueLayerHandler creates a new TrueLayer OAuth handler.
func NewTrueLayerHandler(config TrueLayerConfig) (*TrueLayerHandler, error) {
	if config.ClientID == "" {
		return nil, errors.New("truelayer client_id required")
	}
	if config.ClientSecret == "" {
		return nil, errors.New("truelayer client_secret required")
	}

	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	// Determine URLs based on environment
	authURL := trueLayerSandboxAuthURL
	apiURL := trueLayerSandboxAPIURL
	if config.Environment == TrueLayerEnvLive {
		authURL = trueLayerLiveAuthURL
		apiURL = trueLayerLiveAPIURL
	}

	return &TrueLayerHandler{
		stateManager: config.StateManager,
		httpClient:   httpClient,
		redirectBase: strings.TrimSuffix(config.RedirectBase, "/"),
		clock:        config.Clock,
		clientID:     config.ClientID,
		clientSecret: config.ClientSecret,
		environment:  config.Environment,
		authURL:      authURL,
		apiURL:       apiURL,
	}, nil
}

// TrueLayerStartResult contains the result of starting an OAuth flow.
type TrueLayerStartResult struct {
	AuthURL string
	State   *State
	Receipt *TrueLayerConnectionReceipt
}

// Start initiates the TrueLayer OAuth flow for a circle.
func (h *TrueLayerHandler) Start(circleID string) (*TrueLayerStartResult, error) {
	// Generate state
	state, err := h.stateManager.GenerateState(circleID)
	if err != nil {
		return nil, fmt.Errorf("generate state: %w", err)
	}

	// Validate scopes are read-only
	for _, scope := range TrueLayerScopes {
		if isForbiddenTrueLayerScope(scope) {
			return nil, fmt.Errorf("forbidden scope: %s", scope)
		}
	}

	// Build redirect URI
	redirectURI := h.redirectBase + "/connect/truelayer/callback"

	// Build auth URL
	params := url.Values{
		"response_type": {"code"},
		"client_id":     {h.clientID},
		"redirect_uri":  {redirectURI},
		"state":         {state.Encode()},
		"scope":         {strings.Join(TrueLayerScopes, " ")},
		"providers":     {"uk-ob-all uk-oauth-all"},
	}
	authURL := h.authURL + "/?" + params.Encode()

	// Create receipt
	receipt := &TrueLayerConnectionReceipt{
		CircleID:  circleID,
		Action:    TrueLayerActionOAuthStart,
		Success:   true,
		At:        h.clock(),
		StateHash: state.Hash(),
	}

	return &TrueLayerStartResult{
		AuthURL: authURL,
		State:   state,
		Receipt: receipt,
	}, nil
}

// TrueLayerCallbackResult contains the result of handling an OAuth callback.
type TrueLayerCallbackResult struct {
	CircleID     string
	AccessToken  string // SENSITIVE: Never log
	RefreshToken string // SENSITIVE: Never log
	ExpiresIn    int
	Receipt      *TrueLayerConnectionReceipt
}

// Callback handles the OAuth callback from TrueLayer.
func (h *TrueLayerHandler) Callback(ctx context.Context, code, stateParam string) (*TrueLayerCallbackResult, error) {
	// Validate state
	state, err := h.stateManager.ValidateState(stateParam)
	if err != nil {
		return nil, fmt.Errorf("validate state: %w", err)
	}

	// Build redirect URI
	redirectURI := h.redirectBase + "/connect/truelayer/callback"

	// Exchange code for tokens
	tokenResp, err := h.exchangeCode(ctx, code, redirectURI)
	if err != nil {
		return &TrueLayerCallbackResult{
			CircleID: state.CircleID,
			Receipt: &TrueLayerConnectionReceipt{
				CircleID:   state.CircleID,
				Action:     TrueLayerActionOAuthCallback,
				Success:    false,
				FailReason: "token_exchange_failed",
				At:         h.clock(),
				StateHash:  state.Hash(),
			},
		}, fmt.Errorf("exchange code: %w", err)
	}

	// Verify scopes are read-only
	if err := validateTrueLayerScopes(tokenResp.Scope); err != nil {
		return nil, fmt.Errorf("invalid scopes: %w", err)
	}

	// Create success receipt
	receipt := &TrueLayerConnectionReceipt{
		CircleID:  state.CircleID,
		Action:    TrueLayerActionOAuthCallback,
		Success:   true,
		At:        h.clock(),
		StateHash: state.Hash(),
	}

	return &TrueLayerCallbackResult{
		CircleID:     state.CircleID,
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresIn:    tokenResp.ExpiresIn,
		Receipt:      receipt,
	}, nil
}

// trueLayerTokenResponse is the OAuth token endpoint response.
type trueLayerTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// exchangeCode exchanges an authorization code for tokens.
func (h *TrueLayerHandler) exchangeCode(ctx context.Context, code, redirectURI string) (*trueLayerTokenResponse, error) {
	tokenURL := h.authURL + "/connect/token"

	data := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {h.clientID},
		"client_secret": {h.clientSecret},
		"code":          {code},
		"redirect_uri":  {redirectURI},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	// Limit response size
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024)) // 1MB limit
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed: %s", string(body))
	}

	var tokenResp trueLayerTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &tokenResp, nil
}

// TrueLayerRevokeResult contains the result of a revocation.
type TrueLayerRevokeResult struct {
	Receipt *TrueLayerRevokeReceipt
}

// Revoke revokes the TrueLayer connection for a circle.
// This is idempotent - returns success even if already disconnected.
func (h *TrueLayerHandler) Revoke(ctx context.Context, circleID string) (*TrueLayerRevokeResult, error) {
	now := h.clock()

	// Note: TrueLayer doesn't have a revocation endpoint like Google.
	// We just remove local tokens and mark as disconnected.
	// The access token will expire naturally.

	return &TrueLayerRevokeResult{
		Receipt: &TrueLayerRevokeReceipt{
			CircleID:     circleID,
			Success:      true,
			At:           now,
			LocalRemoved: true,
		},
	}, nil
}

// validateTrueLayerScopes ensures only read-only scopes are present.
func validateTrueLayerScopes(scopeStr string) error {
	if scopeStr == "" {
		return nil // No scope returned is OK
	}

	scopes := strings.Split(scopeStr, " ")
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			continue
		}
		if isForbiddenTrueLayerScope(scope) {
			return fmt.Errorf("forbidden scope: %s", scope)
		}
	}
	return nil
}

// isForbiddenTrueLayerScope checks if a scope is forbidden.
func isForbiddenTrueLayerScope(scope string) bool {
	scopeLower := strings.ToLower(scope)
	for _, pattern := range TrueLayerForbiddenScopePatterns {
		if strings.Contains(scopeLower, pattern) {
			return true
		}
	}
	return false
}

// TrueLayerConnectionReceipt records what happened during a connect operation.
type TrueLayerConnectionReceipt struct {
	CircleID   string
	Action     TrueLayerAction
	Success    bool
	FailReason string // Only set if Success=false, no PII
	At         time.Time
	StateHash  string // Hash of OAuthState used (for correlation)
}

// TrueLayerAction describes what action was taken.
type TrueLayerAction string

const (
	TrueLayerActionOAuthStart    TrueLayerAction = "oauth_start"
	TrueLayerActionOAuthCallback TrueLayerAction = "oauth_callback"
	TrueLayerActionSync          TrueLayerAction = "sync"
	TrueLayerActionRevoke        TrueLayerAction = "revoke"
)

// CanonicalString returns the canonical string representation.
func (r *TrueLayerConnectionReceipt) CanonicalString() string {
	return fmt.Sprintf("TRUELAYER_CONN_RECEIPT|v1|%s|%s|%t|%s|%s|%s",
		r.CircleID,
		r.Action,
		r.Success,
		r.FailReason,
		r.At.UTC().Format(time.RFC3339),
		r.StateHash,
	)
}

// Hash returns the SHA256 hash of the receipt.
func (r *TrueLayerConnectionReceipt) Hash() string {
	return trueLayerHashString(r.CanonicalString())
}

// TrueLayerRevokeReceipt records what happened during a revocation.
type TrueLayerRevokeReceipt struct {
	CircleID     string
	Success      bool
	FailReason   string
	At           time.Time
	LocalRemoved bool
}

// CanonicalString returns the canonical string representation.
func (r *TrueLayerRevokeReceipt) CanonicalString() string {
	return fmt.Sprintf("TRUELAYER_REVOKE_RECEIPT|v1|%s|%t|%s|%s|%t",
		r.CircleID,
		r.Success,
		r.FailReason,
		r.At.UTC().Format(time.RFC3339),
		r.LocalRemoved,
	)
}

// Hash returns the SHA256 hash of the receipt.
func (r *TrueLayerRevokeReceipt) Hash() string {
	return trueLayerHashString(r.CanonicalString())
}

// trueLayerHashString computes a SHA256 hash of the input string.
func trueLayerHashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
