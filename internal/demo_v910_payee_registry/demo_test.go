// Package demo_v910_payee_registry tests v9.10 payee registry enforcement.
//
// CRITICAL: v9.10 enforces that:
// 1. All executions must use registered PayeeID
// 2. Free-text recipients are rejected
// 3. Unknown payees are blocked with audit events
// 4. Live payees are blocked by default
// 5. Payee must match provider
package demo_v910_payee_registry

import (
	"context"
	"testing"
	"time"

	"quantumlife/internal/connectors/finance/write"
	"quantumlife/internal/connectors/finance/write/payees"
	"quantumlife/internal/connectors/finance/write/registry"
	"quantumlife/internal/finance/execution"
	"quantumlife/internal/finance/execution/attempts"
	"quantumlife/pkg/events"
)

// MockWriteConnector for testing
type MockWriteConnector struct {
	providerID  string
	environment string
}

func NewMockWriteConnector() *MockWriteConnector {
	return &MockWriteConnector{
		providerID:  "mock-write",
		environment: "mock",
	}
}

func (c *MockWriteConnector) Provider() string               { return c.providerID }
func (c *MockWriteConnector) ProviderID() string             { return c.providerID }
func (c *MockWriteConnector) ProviderInfo() (string, string) { return c.providerID, c.environment }

func (c *MockWriteConnector) Prepare(ctx context.Context, req write.PrepareRequest) (*write.PrepareResult, error) {
	return &write.PrepareResult{Valid: true}, nil
}

func (c *MockWriteConnector) Execute(ctx context.Context, req write.ExecuteRequest) (*write.PaymentReceipt, error) {
	return &write.PaymentReceipt{
		ReceiptID:   "test-receipt",
		EnvelopeID:  req.Envelope.EnvelopeID,
		Status:      write.PaymentSimulated,
		Simulated:   true,
		AmountCents: req.Envelope.ActionSpec.AmountCents,
		Currency:    req.Envelope.ActionSpec.Currency,
		PayeeID:     req.PayeeID,
	}, nil
}

func (c *MockWriteConnector) Abort(ctx context.Context, envelopeID string) (bool, error) {
	return true, nil
}

// Helper to create a test executor
func createTestExecutor(mockConnector write.WriteConnector) (*execution.V96Executor, []events.Event) {
	counter := 0
	idGen := func() string {
		counter++
		return "test-id-" + string(rune('a'+counter))
	}

	var capturedEvents []events.Event
	emitter := func(e events.Event) {
		capturedEvents = append(capturedEvents, e)
	}

	signingKey := []byte("test-signing-key-32-bytes-long!!")
	presentationStore := execution.NewPresentationStore(idGen, emitter)
	revocationChecker := execution.NewRevocationChecker(idGen)
	presentationGate := execution.NewPresentationGate(presentationStore, idGen, emitter)
	multiPartyGate := execution.NewMultiPartyGate(idGen, emitter)
	approvalVerifier := execution.NewApprovalVerifier(signingKey)
	attemptLedger := attempts.NewInMemoryLedger(attempts.DefaultLedgerConfig(), idGen, emitter)

	config := execution.DefaultV96ExecutorConfig()
	config.ForcedPauseDuration = 10 * time.Millisecond
	config.RevocationPollInterval = 5 * time.Millisecond
	config.TrueLayerConfigured = false

	executor := execution.NewV96Executor(
		nil,
		mockConnector,
		presentationGate,
		multiPartyGate,
		approvalVerifier,
		revocationChecker,
		attemptLedger,
		config,
		idGen,
		emitter,
	)

	return executor, capturedEvents
}

// Test 1: Registered sandbox payee passes validation
func TestRegisteredPayeeAllowed(t *testing.T) {
	t.Run("sandbox-utility is allowed", func(t *testing.T) {
		reg := payees.NewDefaultRegistry()

		err := reg.RequireAllowed(payees.PayeeSandboxUtility, "mock-write")
		if err != nil {
			t.Errorf("sandbox-utility should be allowed: %v", err)
		}
	})

	t.Run("sandbox-rent is allowed", func(t *testing.T) {
		reg := payees.NewDefaultRegistry()

		err := reg.RequireAllowed(payees.PayeeSandboxRent, "mock-write")
		if err != nil {
			t.Errorf("sandbox-rent should be allowed: %v", err)
		}
	})
}

// Test 2: Unknown PayeeID blocks execution
func TestUnknownPayeeBlocked(t *testing.T) {
	t.Run("unknown payee is rejected", func(t *testing.T) {
		reg := payees.NewDefaultRegistry()
		unknownPayee := payees.PayeeID("unknown-random-payee")

		if reg.IsAllowed(unknownPayee, "mock-write") {
			t.Error("unknown payee should NOT be allowed")
		}

		err := reg.RequireAllowed(unknownPayee, "mock-write")
		if err == nil {
			t.Error("RequireAllowed should return error for unknown payee")
		}
	})
}

// Test 3: Free-text recipient is rejected
func TestFreeTextRecipientBlocked(t *testing.T) {
	t.Run("free-text recipient string is rejected", func(t *testing.T) {
		reg := payees.NewDefaultRegistry()

		// Try various free-text patterns
		freeTextRecipients := []string{
			"John Smith",
			"Sort Code 12345",
			"john@example.com",
			"IBAN: GB82 WEST 1234 5698 7654 32",
			"Utility Company Ltd",
		}

		for _, recipient := range freeTextRecipients {
			payeeID := payees.PayeeID(recipient)
			if reg.IsAllowed(payeeID, "mock-write") {
				t.Errorf("free-text recipient %q should NOT be allowed", recipient)
			}
		}
	})
}

// Test 4: Provider registry still enforced (v9.9)
func TestProviderRegistryStillEnforced(t *testing.T) {
	t.Run("provider allowlist is checked", func(t *testing.T) {
		reg := registry.NewDefaultRegistry()

		// mock-write should be allowed
		if !reg.IsAllowed(registry.ProviderMockWrite) {
			t.Error("mock-write should be allowed")
		}

		// truelayer-live should be blocked
		if reg.IsAllowed(registry.ProviderTrueLayerLive) {
			t.Error("truelayer-live should NOT be allowed by default")
		}
	})
}

// Test 5: Payee metadata is correct
func TestPayeeMetadata(t *testing.T) {
	t.Run("sandbox-utility has correct metadata", func(t *testing.T) {
		reg := payees.NewDefaultRegistry()

		entry, ok := reg.Get(payees.PayeeSandboxUtility)
		if !ok {
			t.Fatal("sandbox-utility should exist")
		}

		if entry.ID != payees.PayeeSandboxUtility {
			t.Errorf("expected ID %q, got %q", payees.PayeeSandboxUtility, entry.ID)
		}

		if entry.ProviderID != "mock-write" {
			t.Errorf("expected provider mock-write, got %q", entry.ProviderID)
		}

		if entry.Environment != payees.EnvSandbox {
			t.Errorf("expected sandbox environment, got %q", entry.Environment)
		}

		if !entry.Allowed {
			t.Error("sandbox-utility should be allowed")
		}

		if entry.Currency != "GBP" {
			t.Errorf("expected GBP currency, got %q", entry.Currency)
		}
	})
}

// Test 6: TrueLayer sandbox payees
func TestTrueLayerSandboxPayees(t *testing.T) {
	t.Run("TrueLayer sandbox payees are registered", func(t *testing.T) {
		reg := payees.NewDefaultRegistry()

		tlUtility := payees.PayeeID("sandbox-utility-tl")
		if !reg.IsRegistered(tlUtility) {
			t.Error("sandbox-utility-tl should be registered")
		}

		if !reg.IsAllowed(tlUtility, "truelayer-sandbox") {
			t.Error("sandbox-utility-tl should be allowed for truelayer-sandbox")
		}
	})
}

// Test 7: Custom registry overrides defaults
func TestCustomRegistry(t *testing.T) {
	t.Run("custom registry with different payees", func(t *testing.T) {
		customEntries := []payees.Entry{
			{
				ID:          "custom-payee",
				DisplayName: "Custom Test Payee",
				ProviderID:  "custom-provider",
				Environment: payees.EnvSandbox,
				Allowed:     true,
				Currency:    "EUR",
			},
		}

		reg := payees.NewCustomRegistry(customEntries)

		// Custom payee should work
		if !reg.IsAllowed("custom-payee", "custom-provider") {
			t.Error("custom-payee should be allowed in custom registry")
		}

		// Default payees should NOT be present
		if reg.IsAllowed(payees.PayeeSandboxUtility, "mock-write") {
			t.Error("sandbox-utility should NOT be in custom registry")
		}
	})
}

// Verify interface compliance
var _ write.WriteConnector = (*MockWriteConnector)(nil)
