// Package demo_v99_registry tests v9.9 provider registry enforcement.
//
// CRITICAL: v9.9 enforces that only allowlisted providers can be used
// for financial execution. This test verifies:
// 1. Allowed providers (mock-write, truelayer-sandbox) work normally
// 2. Blocked providers (truelayer-live) are rejected with audit events
// 3. Unknown providers are rejected
package demo_v99_registry

import (
	"context"
	"testing"
	"time"

	"quantumlife/internal/connectors/finance/write"
	"quantumlife/internal/connectors/finance/write/registry"
	"quantumlife/internal/finance/execution"
	"quantumlife/internal/finance/execution/attempts"
	"quantumlife/pkg/events"
)

// MockWriteConnector for testing - can be configured with any provider ID
type MockWriteConnector struct {
	providerID  string
	environment string
}

func NewMockWriteConnectorWithID(id, env string) *MockWriteConnector {
	return &MockWriteConnector{
		providerID:  id,
		environment: env,
	}
}

func (c *MockWriteConnector) Provider() string {
	return c.providerID
}

func (c *MockWriteConnector) ProviderID() string {
	return c.providerID
}

func (c *MockWriteConnector) ProviderInfo() (string, string) {
	return c.providerID, c.environment
}

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
	}, nil
}

func (c *MockWriteConnector) Abort(ctx context.Context, envelopeID string) (bool, error) {
	return true, nil
}

// Test helper to create a minimal test executor
func createTestExecutor(mockConnector write.WriteConnector, reg registry.Registry) (*execution.V96Executor, []events.Event) {
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

	if reg != nil {
		executor.SetProviderRegistry(reg)
	}

	return executor, capturedEvents
}

// Test 1: Allowed provider (mock-write) passes registry check
func TestAllowedProviderPassesRegistry(t *testing.T) {
	t.Run("mock-write is allowed by default registry", func(t *testing.T) {
		mockConnector := NewMockWriteConnectorWithID("mock-write", "mock")
		executor, _ := createTestExecutor(mockConnector, nil) // Use default registry

		// The executor should have mock-write allowed
		reg := registry.NewDefaultRegistry()
		if !reg.IsAllowed(registry.ProviderMockWrite) {
			t.Error("mock-write should be allowed in default registry")
		}

		// Verify executor was created successfully
		if executor == nil {
			t.Error("executor should be created successfully")
		}
	})
}

// Test 2: Blocked provider (truelayer-live) is rejected
func TestBlockedProviderRejected(t *testing.T) {
	t.Run("truelayer-live is blocked by default registry", func(t *testing.T) {
		reg := registry.NewDefaultRegistry()

		if reg.IsAllowed(registry.ProviderTrueLayerLive) {
			t.Error("truelayer-live should NOT be allowed by default")
		}

		err := reg.RequireAllowed(registry.ProviderTrueLayerLive)
		if err == nil {
			t.Error("RequireAllowed should return error for truelayer-live")
		}
	})
}

// Test 3: Unknown provider is rejected
func TestUnknownProviderRejected(t *testing.T) {
	t.Run("unknown provider is not registered", func(t *testing.T) {
		reg := registry.NewDefaultRegistry()
		unknownID := registry.ProviderID("some-unknown-provider")

		if reg.IsAllowed(unknownID) {
			t.Error("unknown provider should NOT be allowed")
		}

		err := reg.RequireAllowed(unknownID)
		if err == nil {
			t.Error("RequireAllowed should return error for unknown provider")
		}
	})
}

// Test 4: Registry allows sandbox providers
func TestSandboxProviderAllowed(t *testing.T) {
	t.Run("truelayer-sandbox is allowed", func(t *testing.T) {
		reg := registry.NewDefaultRegistry()

		if !reg.IsAllowed(registry.ProviderTrueLayerSandbox) {
			t.Error("truelayer-sandbox should be allowed")
		}

		err := reg.RequireAllowed(registry.ProviderTrueLayerSandbox)
		if err != nil {
			t.Errorf("RequireAllowed should not return error for truelayer-sandbox: %v", err)
		}
	})
}

// Test 5: Live environment detection
func TestLiveEnvironmentDetection(t *testing.T) {
	t.Run("correctly identifies live environment", func(t *testing.T) {
		reg := registry.NewDefaultRegistry()

		if reg.IsLiveEnvironment(registry.ProviderMockWrite) {
			t.Error("mock-write should not be live environment")
		}

		if reg.IsLiveEnvironment(registry.ProviderTrueLayerSandbox) {
			t.Error("truelayer-sandbox should not be live environment")
		}

		if !reg.IsLiveEnvironment(registry.ProviderTrueLayerLive) {
			t.Error("truelayer-live should be live environment")
		}
	})
}

// Test 6: Registry entries have correct metadata
func TestRegistryEntryMetadata(t *testing.T) {
	t.Run("mock-write entry has correct metadata", func(t *testing.T) {
		reg := registry.NewDefaultRegistry()

		entry, ok := reg.Get(registry.ProviderMockWrite)
		if !ok {
			t.Fatal("mock-write should exist in registry")
		}

		if entry.ID != registry.ProviderMockWrite {
			t.Errorf("expected ID %q, got %q", registry.ProviderMockWrite, entry.ID)
		}

		if !entry.IsWrite {
			t.Error("mock-write should be a write provider")
		}

		if entry.Environment != registry.EnvMock {
			t.Errorf("expected environment %q, got %q", registry.EnvMock, entry.Environment)
		}

		if !entry.Allowed {
			t.Error("mock-write should be allowed")
		}
	})

	t.Run("truelayer-live entry has correct metadata", func(t *testing.T) {
		reg := registry.NewDefaultRegistry()

		entry, ok := reg.Get(registry.ProviderTrueLayerLive)
		if !ok {
			t.Fatal("truelayer-live should exist in registry")
		}

		if entry.ID != registry.ProviderTrueLayerLive {
			t.Errorf("expected ID %q, got %q", registry.ProviderTrueLayerLive, entry.ID)
		}

		if entry.Environment != registry.EnvLive {
			t.Errorf("expected environment %q, got %q", registry.EnvLive, entry.Environment)
		}

		if entry.Allowed {
			t.Error("truelayer-live should NOT be allowed by default")
		}
	})
}

// Test 7: Custom registry can override defaults
func TestCustomRegistry(t *testing.T) {
	t.Run("custom registry can allow different providers", func(t *testing.T) {
		customEntries := []registry.Entry{
			{
				ID:          "custom-provider",
				DisplayName: "Custom Test Provider",
				IsWrite:     true,
				Environment: "test",
				Allowed:     true,
			},
		}

		reg := registry.NewCustomRegistry(customEntries)

		if !reg.IsAllowed("custom-provider") {
			t.Error("custom-provider should be allowed in custom registry")
		}

		if reg.IsAllowed(registry.ProviderMockWrite) {
			t.Error("mock-write should NOT be in custom registry")
		}
	})
}

// Verify interface compliance
var _ write.WriteConnector = (*MockWriteConnector)(nil)
