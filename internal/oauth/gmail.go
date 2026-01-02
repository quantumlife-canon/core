// Package oauth provides Gmail OAuth flow handling.
//
// Phase 18.8: Real OAuth (Gmail Read-Only)
// Reference: docs/ADR/ADR-0041-phase18-8-real-oauth-gmail-readonly.md
//
// CRITICAL: Read-only scopes only (gmail.readonly).
// CRITICAL: No goroutines. All operations synchronous.
// CRITICAL: Stdlib only (net/http).
package oauth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"quantumlife/internal/connectors/auth"
)

// GmailScopes defines the only allowed scopes for Gmail OAuth.
// CRITICAL: Read-only only. No write scopes.
var GmailScopes = []string{"gmail.readonly"}

// GmailHandler handles Gmail OAuth flows.
type GmailHandler struct {
	stateManager *StateManager
	broker       auth.TokenBroker
	httpClient   *http.Client
	redirectBase string
	clock        func() time.Time
}

// NewGmailHandler creates a new Gmail OAuth handler.
func NewGmailHandler(
	stateManager *StateManager,
	broker auth.TokenBroker,
	httpClient *http.Client,
	redirectBase string,
	clock func() time.Time,
) *GmailHandler {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &GmailHandler{
		stateManager: stateManager,
		broker:       broker,
		httpClient:   httpClient,
		redirectBase: strings.TrimSuffix(redirectBase, "/"),
		clock:        clock,
	}
}

// StartResult contains the result of starting an OAuth flow.
type StartResult struct {
	AuthURL string
	State   *State
	Receipt *ConnectionReceipt
}

// Start initiates the Gmail OAuth flow for a circle.
func (h *GmailHandler) Start(circleID string) (*StartResult, error) {
	// Generate state
	state, err := h.stateManager.GenerateState(circleID)
	if err != nil {
		return nil, fmt.Errorf("generate state: %w", err)
	}

	// Build redirect URI
	redirectURI := h.redirectBase + "/connect/gmail/callback"

	// Get auth URL from broker
	authURL, err := h.broker.BeginOAuth(
		auth.ProviderGoogle,
		redirectURI,
		state.Encode(),
		GmailScopes,
	)
	if err != nil {
		return nil, fmt.Errorf("begin oauth: %w", err)
	}

	// Create receipt
	receipt := &ConnectionReceipt{
		CircleID:  circleID,
		Provider:  ProviderGoogle,
		Product:   ProductGmail,
		Action:    ActionOAuthStart,
		Success:   true,
		At:        h.clock(),
		StateHash: state.Hash(),
	}

	return &StartResult{
		AuthURL: authURL,
		State:   state,
		Receipt: receipt,
	}, nil
}

// CallbackResult contains the result of handling an OAuth callback.
type CallbackResult struct {
	CircleID    string
	TokenHandle *auth.TokenHandle
	Receipt     *ConnectionReceipt
}

// Callback handles the OAuth callback from Google.
func (h *GmailHandler) Callback(ctx context.Context, code, stateParam string) (*CallbackResult, error) {
	// Validate state
	state, err := h.stateManager.ValidateState(stateParam)
	if err != nil {
		return nil, fmt.Errorf("validate state: %w", err)
	}

	// Build redirect URI
	redirectURI := h.redirectBase + "/connect/gmail/callback"

	// Exchange code for tokens
	handle, err := h.broker.ExchangeCodeForCircle(ctx, state.CircleID, auth.ProviderGoogle, code, redirectURI)
	if err != nil {
		return &CallbackResult{
			CircleID: state.CircleID,
			Receipt: &ConnectionReceipt{
				CircleID:   state.CircleID,
				Provider:   ProviderGoogle,
				Product:    ProductGmail,
				Action:     ActionOAuthCallback,
				Success:    false,
				FailReason: "token_exchange_failed",
				At:         h.clock(),
				StateHash:  state.Hash(),
			},
		}, fmt.Errorf("exchange code: %w", err)
	}

	// Verify scopes are read-only
	if err := validateReadOnlyScopes(handle.Scopes); err != nil {
		// Revoke immediately if we got write scopes
		_ = h.broker.RevokeToken(ctx, state.CircleID, auth.ProviderGoogle)
		return nil, fmt.Errorf("invalid scopes: %w", err)
	}

	// Create success receipt
	receipt := &ConnectionReceipt{
		CircleID:    state.CircleID,
		Provider:    ProviderGoogle,
		Product:     ProductGmail,
		Action:      ActionOAuthCallback,
		Success:     true,
		At:          h.clock(),
		StateHash:   state.Hash(),
		TokenHandle: handle.ID,
	}

	return &CallbackResult{
		CircleID:    state.CircleID,
		TokenHandle: &handle,
		Receipt:     receipt,
	}, nil
}

// validateReadOnlyScopes ensures only read-only scopes are present.
func validateReadOnlyScopes(scopes []string) error {
	for _, scope := range scopes {
		// Only gmail.readonly is allowed
		if scope != "gmail.readonly" &&
			scope != "https://www.googleapis.com/auth/gmail.readonly" {
			return fmt.Errorf("forbidden scope: %s", scope)
		}
	}
	return nil
}

// RevokeResult contains the result of a revocation.
type RevokeResult struct {
	Receipt *RevokeReceipt
}

// Revoke revokes the Gmail connection for a circle.
// This is idempotent - returns success even if already disconnected.
func (h *GmailHandler) Revoke(ctx context.Context, circleID string) (*RevokeResult, error) {
	now := h.clock()

	// Check if token exists
	hasToken, err := h.broker.HasToken(ctx, circleID, auth.ProviderGoogle)
	if err != nil {
		return &RevokeResult{
			Receipt: &RevokeReceipt{
				CircleID:        circleID,
				Provider:        ProviderGoogle,
				Product:         ProductGmail,
				Success:         true, // Idempotent - treat as already disconnected
				At:              now,
				ProviderRevoked: false,
				LocalRemoved:    false,
			},
		}, nil
	}

	if !hasToken {
		// Already disconnected
		return &RevokeResult{
			Receipt: &RevokeReceipt{
				CircleID:        circleID,
				Provider:        ProviderGoogle,
				Product:         ProductGmail,
				Success:         true,
				At:              now,
				ProviderRevoked: false,
				LocalRemoved:    false,
			},
		}, nil
	}

	// Try to revoke with Google
	providerRevoked := false
	if err := h.revokeWithGoogle(ctx, circleID); err == nil {
		providerRevoked = true
	}
	// Continue even if Google revoke fails - still remove local token

	// Remove local token
	localRemoved := false
	if err := h.broker.RevokeToken(ctx, circleID, auth.ProviderGoogle); err == nil {
		localRemoved = true
	}

	return &RevokeResult{
		Receipt: &RevokeReceipt{
			CircleID:        circleID,
			Provider:        ProviderGoogle,
			Product:         ProductGmail,
			Success:         true,
			At:              now,
			ProviderRevoked: providerRevoked,
			LocalRemoved:    localRemoved,
		},
	}, nil
}

// revokeWithGoogle calls Google's revocation endpoint.
func (h *GmailHandler) revokeWithGoogle(ctx context.Context, circleID string) error {
	// First mint a token to get something to revoke
	token, err := h.broker.MintReadOnlyAccessToken(ctx, circleID, auth.ProviderGoogle, []string{"email:read"})
	if err != nil {
		return fmt.Errorf("mint token for revoke: %w", err)
	}

	// Call Google revocation endpoint
	revokeURL := "https://oauth2.googleapis.com/revoke"
	data := url.Values{}
	data.Set("token", token.Token)

	req, err := http.NewRequestWithContext(ctx, "POST", revokeURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("create revoke request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("revoke request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("revoke failed: %s", string(body))
	}

	return nil
}

// HasConnection checks if a circle has a Gmail connection.
func (h *GmailHandler) HasConnection(ctx context.Context, circleID string) (bool, error) {
	return h.broker.HasToken(ctx, circleID, auth.ProviderGoogle)
}

// ErrNoConnection indicates no connection exists for the circle.
var ErrNoConnection = errors.New("no gmail connection")

// SyncInput contains parameters for a Gmail sync.
type SyncInput struct {
	CircleID string
	Since    time.Time
	Limit    int
}

// SyncOutput contains the result of a Gmail sync.
type SyncOutput struct {
	MessageCount int
	EventCount   int
	Receipt      *SyncReceipt
}
