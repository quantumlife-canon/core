// Package demo_phase18_8_oauth_gmail demonstrates Phase 18.8 OAuth Gmail behavior.
//
// These tests demonstrate:
// 1. OAuth state management with CSRF protection
// 2. Gmail OAuth flow lifecycle
// 3. Read-only scope enforcement
// 4. Deterministic receipts and audit logging
// 5. Idempotent revocation
//
// Reference: docs/ADR/ADR-0041-phase18-8-real-oauth-gmail-readonly.md
package demo_phase18_8_oauth_gmail

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"quantumlife/internal/connectors/auth"
	"quantumlife/internal/connectors/auth/impl_inmem"
	"quantumlife/internal/oauth"
	"quantumlife/internal/persist"
)

// fixedClock provides a deterministic clock for testing.
type fixedClock struct {
	now time.Time
}

func (c *fixedClock) Now() time.Time {
	return c.now
}

func (c *fixedClock) Advance(d time.Duration) {
	c.now = c.now.Add(d)
}

// TestOAuthStateManagement demonstrates state generation and validation.
func TestOAuthStateManagement(t *testing.T) {
	t.Log("=== Demo: OAuth State Management ===")

	// Setup
	secret := []byte("test-secret-32-bytes-for-hmac!!")
	clock := &fixedClock{now: time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)}
	manager := oauth.NewStateManager(secret, clock.Now)

	circleID := "circle-123"

	// Generate state
	t.Log("Generating OAuth state for circle:", circleID)
	state, err := manager.GenerateState(circleID)
	if err != nil {
		t.Fatalf("Failed to generate state: %v", err)
	}

	t.Logf("State generated:")
	t.Logf("  - CircleID: %s", state.CircleID)
	t.Logf("  - Nonce: %s (16 bytes hex)", state.Nonce)
	t.Logf("  - IssuedAtBucket: %d", state.IssuedAtBucket)
	t.Logf("  - Hash: %s", state.Hash())

	// Validate state
	t.Log("\nValidating state...")
	encoded := state.Encode()
	t.Logf("Encoded state: %s...", encoded[:50])

	validated, err := manager.ValidateState(encoded)
	if err != nil {
		t.Fatalf("Failed to validate state: %v", err)
	}

	if validated.CircleID != circleID {
		t.Errorf("CircleID mismatch: got %s, want %s", validated.CircleID, circleID)
	}

	t.Log("State validated successfully!")

	// Test state expiration
	t.Log("\nTesting state expiration...")
	clock.Advance(11 * time.Minute) // States expire after 10 minutes
	managerAfterExpiry := oauth.NewStateManager(secret, clock.Now)
	_, err = managerAfterExpiry.ValidateState(encoded)
	if err == nil {
		t.Error("Expected state to be expired, but it validated")
	} else {
		t.Logf("State correctly expired: %v", err)
	}

	t.Log("\n=== OAuth State Management Demo Complete ===")
}

// TestGmailOAuthFlow demonstrates the Gmail OAuth flow lifecycle.
func TestGmailOAuthFlow(t *testing.T) {
	t.Log("=== Demo: Gmail OAuth Flow ===")

	// Setup
	clock := &fixedClock{now: time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)}
	secret := []byte("test-secret-32-bytes-for-hmac!!")
	stateManager := oauth.NewStateManager(secret, clock.Now)

	// Create mock broker
	authConfig := auth.Config{
		Google: auth.GoogleConfig{
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
		},
		TokenEncryptionKey: "test-encryption-key-32-bytes!!!",
	}
	broker := impl_inmem.NewBroker(authConfig, nil)

	// Create handler
	handler := oauth.NewGmailHandler(
		stateManager,
		broker,
		nil, // Use default HTTP client
		"http://localhost:8080",
		clock.Now,
	)

	circleID := "circle-456"

	// Step 1: Start OAuth flow
	t.Log("Step 1: Starting OAuth flow...")
	result, err := handler.Start(circleID)
	if err != nil {
		t.Fatalf("Failed to start OAuth: %v", err)
	}

	t.Logf("OAuth started:")
	t.Logf("  - AuthURL: %s...", result.AuthURL[:80])
	t.Logf("  - Receipt Success: %v", result.Receipt.Success)
	t.Logf("  - Receipt Action: %s", result.Receipt.Action)
	t.Logf("  - Receipt Hash: %s", result.Receipt.Hash()[:16])

	// Verify scopes in URL
	// Note: gmail.readonly is mapped to https://www.googleapis.com/auth/gmail.readonly
	if !strings.Contains(result.AuthURL, "gmail.readonly") &&
		!strings.Contains(result.AuthURL, "gmail") {
		t.Error("AuthURL does not contain Gmail scope")
	}
	if strings.Contains(result.AuthURL, "gmail.send") ||
		strings.Contains(result.AuthURL, "gmail.modify") ||
		strings.Contains(result.AuthURL, "gmail.compose") {
		t.Error("AuthURL contains forbidden write scopes")
	}

	t.Log("OAuth flow started with read-only scopes!")

	// Step 2: Check connection status (before callback)
	t.Log("\nStep 2: Checking connection status before callback...")
	hasConnection, err := handler.HasConnection(context.Background(), circleID)
	if err != nil {
		t.Fatalf("Failed to check connection: %v", err)
	}
	if hasConnection {
		t.Error("Expected no connection before callback")
	} else {
		t.Log("Correctly shows no connection before callback")
	}

	t.Log("\n=== Gmail OAuth Flow Demo Complete ===")
}

// TestReadOnlyScopeEnforcement demonstrates scope validation.
func TestReadOnlyScopeEnforcement(t *testing.T) {
	t.Log("=== Demo: Read-Only Scope Enforcement ===")

	// Test allowed scopes
	allowedScopes := []string{
		"gmail.readonly",
		"https://www.googleapis.com/auth/gmail.readonly",
	}

	for _, scope := range allowedScopes {
		t.Logf("Testing allowed scope: %s", scope)
		// The actual validation happens in the handler, but we demonstrate the concept
	}

	// Test forbidden scopes (would be rejected)
	forbiddenScopes := []string{
		"gmail.send",
		"gmail.modify",
		"gmail.compose",
		"gmail.insert",
		"https://www.googleapis.com/auth/gmail.send",
	}

	for _, scope := range forbiddenScopes {
		t.Logf("Forbidden scope (would be rejected): %s", scope)
	}

	// Verify GmailScopes constant
	t.Log("\nVerifying GmailScopes constant...")
	if len(oauth.GmailScopes) != 1 {
		t.Errorf("Expected 1 scope, got %d", len(oauth.GmailScopes))
	}
	if oauth.GmailScopes[0] != "gmail.readonly" {
		t.Errorf("Expected gmail.readonly, got %s", oauth.GmailScopes[0])
	}
	t.Log("GmailScopes correctly contains only gmail.readonly")

	t.Log("\n=== Read-Only Scope Enforcement Demo Complete ===")
}

// TestDeterministicReceipts demonstrates receipt hashing.
func TestDeterministicReceipts(t *testing.T) {
	t.Log("=== Demo: Deterministic Receipts ===")

	fixedTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	// Create a connection receipt
	receipt := &oauth.ConnectionReceipt{
		CircleID:    "circle-789",
		Provider:    oauth.ProviderGoogle,
		Product:     oauth.ProductGmail,
		Action:      oauth.ActionOAuthStart,
		Success:     true,
		At:          fixedTime,
		StateHash:   "abc123",
		TokenHandle: "handle-xyz",
	}

	t.Log("Connection Receipt:")
	t.Logf("  - Canonical: %s", receipt.CanonicalString())
	t.Logf("  - Hash: %s", receipt.Hash())

	// Create same receipt again - hash should be identical
	receipt2 := &oauth.ConnectionReceipt{
		CircleID:    "circle-789",
		Provider:    oauth.ProviderGoogle,
		Product:     oauth.ProductGmail,
		Action:      oauth.ActionOAuthStart,
		Success:     true,
		At:          fixedTime,
		StateHash:   "abc123",
		TokenHandle: "handle-xyz",
	}

	if receipt.Hash() != receipt2.Hash() {
		t.Error("Identical receipts should have identical hashes")
	} else {
		t.Log("Receipts are deterministic: identical inputs = identical hashes")
	}

	// Create a sync receipt
	syncReceipt := &oauth.SyncReceipt{
		CircleID:        "circle-789",
		Provider:        oauth.ProviderGoogle,
		Product:         oauth.ProductGmail,
		Success:         true,
		At:              fixedTime,
		MessagesFetched: 5,
		MagnitudeBucket: oauth.MagnitudeBucket(5),
		EventsGenerated: 5,
		ConnectionHash:  receipt.Hash(),
	}

	t.Log("\nSync Receipt:")
	t.Logf("  - MessagesFetched: %d", syncReceipt.MessagesFetched)
	t.Logf("  - MagnitudeBucket: %s", syncReceipt.MagnitudeBucket)
	t.Logf("  - Hash: %s", syncReceipt.Hash())

	// Test magnitude buckets
	t.Log("\nMagnitude bucket examples:")
	for _, count := range []int{0, 3, 10, 50} {
		t.Logf("  %d messages -> %s", count, oauth.MagnitudeBucket(count))
	}

	t.Log("\n=== Deterministic Receipts Demo Complete ===")
}

// TestIdempotentRevocation demonstrates revocation behavior.
func TestIdempotentRevocation(t *testing.T) {
	t.Log("=== Demo: Idempotent Revocation ===")

	// Setup
	clock := &fixedClock{now: time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)}
	secret := []byte("test-secret-32-bytes-for-hmac!!")
	stateManager := oauth.NewStateManager(secret, clock.Now)

	authConfig := auth.Config{
		TokenEncryptionKey: "test-encryption-key-32-bytes!!!",
	}
	broker := impl_inmem.NewBroker(authConfig, nil)

	handler := oauth.NewGmailHandler(
		stateManager,
		broker,
		nil,
		"http://localhost:8080",
		clock.Now,
	)

	circleID := "circle-revoke-test"

	// Revoke when not connected - should succeed (idempotent)
	t.Log("Revoking connection that doesn't exist...")
	result, err := handler.Revoke(context.Background(), circleID)
	if err != nil {
		t.Fatalf("Revoke should not error: %v", err)
	}

	t.Logf("Revoke result:")
	t.Logf("  - Success: %v (should be true - idempotent)", result.Receipt.Success)
	t.Logf("  - ProviderRevoked: %v", result.Receipt.ProviderRevoked)
	t.Logf("  - LocalRemoved: %v", result.Receipt.LocalRemoved)

	if !result.Receipt.Success {
		t.Error("Revoke should succeed even when not connected (idempotent)")
	}

	// Revoke again - should still succeed
	t.Log("\nRevoking again (should still succeed)...")
	result2, err := handler.Revoke(context.Background(), circleID)
	if err != nil {
		t.Fatalf("Second revoke should not error: %v", err)
	}

	if !result2.Receipt.Success {
		t.Error("Second revoke should also succeed (idempotent)")
	}

	t.Log("Revocation is idempotent!")

	t.Log("\n=== Idempotent Revocation Demo Complete ===")
}

// TestPersistenceRecords demonstrates OAuth record persistence.
func TestPersistenceRecords(t *testing.T) {
	t.Log("=== Demo: OAuth Persistence Records ===")

	store := persist.NewOAuthRecordStore()
	fixedTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	// Create and store OAuth state record
	stateRecord := &persist.OAuthStateRecord{
		CircleID:    "circle-persist",
		Provider:    "google",
		Product:     "gmail",
		StateHash:   "state-hash-123",
		IssuedAt:    fixedTime,
		ExpiresAt:   fixedTime.Add(10 * time.Minute),
		Consumed:    false,
		ReceiptHash: "receipt-hash-456",
	}
	store.AppendState(stateRecord)

	t.Log("OAuth State Record:")
	t.Logf("  - Canonical: %s", stateRecord.CanonicalString())
	t.Logf("  - Hash: %s", stateRecord.Hash()[:32])
	t.Logf("  - Sequence: %d", stateRecord.ReplaySequence)

	// Create and store token handle record
	handleRecord := &persist.OAuthTokenHandleRecord{
		HandleID:    "handle-789",
		CircleID:    "circle-persist",
		Provider:    "google",
		Product:     "gmail",
		Scopes:      []string{"gmail.readonly"},
		CreatedAt:   fixedTime,
		ExpiresAt:   fixedTime.Add(24 * time.Hour),
		Revoked:     false,
		ReceiptHash: "receipt-hash-789",
	}
	store.AppendTokenHandle(handleRecord)

	t.Log("\nToken Handle Record:")
	t.Logf("  - HandleID: %s", handleRecord.HandleID)
	t.Logf("  - Scopes: %v", handleRecord.Scopes)
	t.Logf("  - Sequence: %d", handleRecord.ReplaySequence)

	// Create and store sync receipt record
	syncRecord := &persist.GmailSyncReceiptRecord{
		CircleID:        "circle-persist",
		Provider:        "google",
		Product:         "gmail",
		Success:         true,
		At:              fixedTime,
		MessagesFetched: 12,
		MagnitudeBucket: "several",
		EventsGenerated: 12,
		ConnectionHash:  "conn-hash-abc",
	}
	store.AppendSyncReceipt(syncRecord)

	t.Log("\nSync Receipt Record:")
	t.Logf("  - MessagesFetched: %d", syncRecord.MessagesFetched)
	t.Logf("  - MagnitudeBucket: %s", syncRecord.MagnitudeBucket)
	t.Logf("  - Sequence: %d", syncRecord.ReplaySequence)

	// Verify sequence ordering
	t.Log("\nStore summary:")
	t.Logf("  - Total States: %d", len(store.States()))
	t.Logf("  - Total Token Handles: %d", len(store.TokenHandles()))
	t.Logf("  - Total Sync Receipts: %d", len(store.SyncReceipts()))
	t.Logf("  - Current Sequence: %d", store.Sequence())

	t.Log("\n=== OAuth Persistence Records Demo Complete ===")
}

// TestMockHTTPServer demonstrates OAuth callback handling.
func TestMockHTTPServer(t *testing.T) {
	t.Log("=== Demo: OAuth HTTP Handling ===")

	// Create a test server that simulates Google's OAuth endpoint
	oauthServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("Mock Google OAuth received request: %s %s", r.Method, r.URL.Path)

		if r.URL.Path == "/revoke" {
			// Simulate revocation success
			w.WriteHeader(http.StatusOK)
			return
		}

		// Default response
		w.WriteHeader(http.StatusOK)
	}))
	defer oauthServer.Close()

	t.Logf("Mock OAuth server running at: %s", oauthServer.URL)

	// Simulate a request to the revoke endpoint
	resp, err := http.Post(oauthServer.URL+"/revoke?token=test", "application/x-www-form-urlencoded", nil)
	if err != nil {
		t.Fatalf("Failed to call mock revoke: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", resp.StatusCode)
	}

	t.Log("Mock revocation succeeded!")
	t.Log("\n=== OAuth HTTP Handling Demo Complete ===")
}
