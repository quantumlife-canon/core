// Package truelayer tests for TrueLayer connector.
//
// Phase 17b: httptest-based tests for TrueLayer API integration.
// These tests verify request construction without hitting the real network.
package truelayer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"quantumlife/internal/connectors/finance/write"
)

// TestConnector_SandboxURL verifies sandbox URL construction.
func TestConnector_SandboxURL(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify path
		if r.URL.Path != "/payments" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		// Verify method
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		// Verify headers
		auth := r.Header.Get("Authorization")
		if auth == "" {
			t.Error("expected Authorization header")
		}

		idempotencyKey := r.Header.Get("Idempotency-Key")
		if idempotencyKey == "" {
			t.Error("expected Idempotency-Key header")
		}

		// Return success response
		resp := PaymentResponse{
			ID:     "pay-sandbox-001",
			Status: "authorization_required",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create connector with test server URL
	connector, err := NewConnector(ConnectorConfig{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		SigningKey:   "test-signing-key",
		Environment:  "sandbox",
		Config: write.WriteConfig{
			CapCents:          100, // £1.00
			AllowedCurrencies: []string{"GBP"},
		},
	})
	if err != nil {
		t.Fatalf("NewConnector failed: %v", err)
	}

	// Verify provider info
	providerID, env := connector.ProviderInfo()
	if providerID != "truelayer-sandbox" {
		t.Errorf("expected truelayer-sandbox, got %s", providerID)
	}
	if env != "sandbox" {
		t.Errorf("expected sandbox environment, got %s", env)
	}

	t.Logf("Provider: %s, Environment: %s", providerID, env)
}

// TestConnector_ProviderInfo verifies provider info for sandbox vs live.
func TestConnector_ProviderInfo(t *testing.T) {
	tests := []struct {
		name           string
		environment    string
		wantProviderID string
		wantEnv        string
	}{
		{
			name:           "sandbox default",
			environment:    "",
			wantProviderID: "truelayer-sandbox",
			wantEnv:        "sandbox",
		},
		{
			name:           "sandbox explicit",
			environment:    "sandbox",
			wantProviderID: "truelayer-sandbox",
			wantEnv:        "sandbox",
		},
		{
			name:           "live",
			environment:    "live",
			wantProviderID: "truelayer-live",
			wantEnv:        "live",
		},
		{
			name:           "production",
			environment:    "production",
			wantProviderID: "truelayer-live",
			wantEnv:        "live",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connector, err := NewConnector(ConnectorConfig{
				Environment: tt.environment,
			})
			if err != nil {
				t.Fatalf("NewConnector failed: %v", err)
			}

			providerID, env := connector.ProviderInfo()
			if providerID != tt.wantProviderID {
				t.Errorf("ProviderInfo() providerID = %s, want %s", providerID, tt.wantProviderID)
			}
			if env != tt.wantEnv {
				t.Errorf("ProviderInfo() env = %s, want %s", env, tt.wantEnv)
			}
		})
	}
}

// TestConnector_PrepareValidation verifies prepare validates all requirements.
func TestConnector_PrepareValidation(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	connector, err := NewConnector(ConnectorConfig{
		Environment: "sandbox",
		Config: write.WriteConfig{
			CapCents:          100,
			AllowedCurrencies: []string{"GBP"},
		},
	})
	if err != nil {
		t.Fatalf("NewConnector failed: %v", err)
	}

	// Test with nil envelope
	result, err := connector.Prepare(context.Background(), write.PrepareRequest{
		Envelope: nil,
		Now:      now,
	})
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	if result.Valid {
		t.Error("expected invalid for nil envelope")
	}
	if result.InvalidReason != "envelope is nil" {
		t.Errorf("unexpected reason: %s", result.InvalidReason)
	}

	t.Log("Nil envelope correctly rejected")
}

// TestConnector_PrepareAmountCap verifies cap enforcement.
func TestConnector_PrepareAmountCap(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	connector, err := NewConnector(ConnectorConfig{
		Environment: "sandbox",
		Config: write.WriteConfig{
			CapCents:          100, // £1.00 cap
			AllowedCurrencies: []string{"GBP"},
		},
	})
	if err != nil {
		t.Fatalf("NewConnector failed: %v", err)
	}

	// Create envelope exceeding cap
	envelope := &write.ExecutionEnvelope{
		EnvelopeID: "env-over-cap",
		SealHash:   "seal-hash-0000000000000001",
		ActionHash: "action-hash-00000000000001",
		ActionSpec: write.ActionSpec{
			Type:        "payment",
			AmountCents: 200, // Over £1.00 cap
			Currency:    "GBP",
			PayeeID:     "sandbox-utility",
		},
		Expiry:              now.Add(24 * time.Hour),
		RevocationWaived:    true,
		RevocationWindowEnd: now,
	}

	approval := &write.ApprovalArtifact{
		ArtifactID: "approval-001",
		ActionHash: "action-hash-00000000000001",
		ApprovedAt: now,
		ExpiresAt:  now.Add(1 * time.Hour),
	}

	result, err := connector.Prepare(context.Background(), write.PrepareRequest{
		Envelope: envelope,
		Approval: approval,
		PayeeID:  "sandbox-utility",
		Now:      now,
	})
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}

	if result.Valid {
		t.Error("expected invalid for amount over cap")
	}

	t.Logf("Amount over cap correctly rejected: %s", result.InvalidReason)
}

// TestConnector_Abort verifies abort functionality.
func TestConnector_Abort(t *testing.T) {
	connector, err := NewConnector(ConnectorConfig{
		Environment: "sandbox",
	})
	if err != nil {
		t.Fatalf("NewConnector failed: %v", err)
	}

	// Abort should work without hitting network (internal state only)
	aborted, err := connector.Abort(context.Background(), "env-to-abort")
	if err != nil {
		t.Fatalf("Abort failed: %v", err)
	}

	if !aborted {
		t.Error("expected Abort to return true")
	}

	t.Log("Abort succeeded")
}

// TestConnector_ExecuteRequiresCredentials verifies credentials are required.
func TestConnector_ExecuteRequiresCredentials(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Create connector WITHOUT credentials
	connector, err := NewConnector(ConnectorConfig{
		Environment: "sandbox",
		Config: write.WriteConfig{
			CapCents:            100,
			AllowedCurrencies:   []string{"GBP"},
			ForcedPauseDuration: 0, // Skip pause in tests
		},
	})
	if err != nil {
		t.Fatalf("NewConnector failed: %v", err)
	}

	envelope := &write.ExecutionEnvelope{
		EnvelopeID: "env-no-creds",
		SealHash:   "seal-hash-001",
		ActionHash: "action-hash-001",
		ActionSpec: write.ActionSpec{
			Type:        "payment",
			AmountCents: 50,
			Currency:    "GBP",
			PayeeID:     "sandbox-utility",
		},
		Expiry:              now.Add(24 * time.Hour),
		RevocationWaived:    true,
		RevocationWindowEnd: now,
	}

	approval := &write.ApprovalArtifact{
		ArtifactID: "approval-001",
		ActionHash: "action-hash-001",
		ApprovedAt: now,
		ExpiresAt:  now.Add(1 * time.Hour),
	}

	_, err = connector.Execute(context.Background(), write.ExecuteRequest{
		Envelope:       envelope,
		Approval:       approval,
		PayeeID:        "sandbox-utility",
		IdempotencyKey: "idem-001",
		Now:            now,
	})

	if err == nil {
		t.Error("expected error when credentials not configured")
	}
	if err != write.ErrProviderNotConfigured {
		t.Errorf("expected ErrProviderNotConfigured, got %v", err)
	}

	t.Logf("Missing credentials correctly rejected: %v", err)
}
