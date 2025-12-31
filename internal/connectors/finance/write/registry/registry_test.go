// Package registry_test provides tests for the v9.9 provider registry.
package registry_test

import (
	"errors"
	"testing"

	"quantumlife/internal/connectors/finance/write/registry"
)

func TestDefaultRegistry_AllowedProviders(t *testing.T) {
	reg := registry.NewDefaultRegistry()

	t.Run("mock-write is allowed", func(t *testing.T) {
		if !reg.IsAllowed(registry.ProviderMockWrite) {
			t.Error("mock-write should be allowed")
		}

		err := reg.RequireAllowed(registry.ProviderMockWrite)
		if err != nil {
			t.Errorf("mock-write should not return error: %v", err)
		}
	})

	t.Run("truelayer-sandbox is allowed", func(t *testing.T) {
		if !reg.IsAllowed(registry.ProviderTrueLayerSandbox) {
			t.Error("truelayer-sandbox should be allowed")
		}

		err := reg.RequireAllowed(registry.ProviderTrueLayerSandbox)
		if err != nil {
			t.Errorf("truelayer-sandbox should not return error: %v", err)
		}
	})
}

func TestDefaultRegistry_BlockedProviders(t *testing.T) {
	reg := registry.NewDefaultRegistry()

	t.Run("truelayer-live is blocked by default", func(t *testing.T) {
		if reg.IsAllowed(registry.ProviderTrueLayerLive) {
			t.Error("truelayer-live should NOT be allowed by default")
		}

		err := reg.RequireAllowed(registry.ProviderTrueLayerLive)
		if err == nil {
			t.Error("truelayer-live should return error")
		}

		// Should be ErrProviderLiveBlocked
		if !errors.Is(err, registry.ErrProviderLiveBlocked) {
			t.Errorf("expected ErrProviderLiveBlocked, got %v", err)
		}

		// Check provider error details
		var provErr *registry.ProviderError
		if errors.As(err, &provErr) {
			if provErr.ProviderID != registry.ProviderTrueLayerLive {
				t.Errorf("expected provider ID %q, got %q", registry.ProviderTrueLayerLive, provErr.ProviderID)
			}
		} else {
			t.Error("expected ProviderError type")
		}
	})
}

func TestDefaultRegistry_UnregisteredProviders(t *testing.T) {
	reg := registry.NewDefaultRegistry()

	t.Run("unknown provider is not allowed", func(t *testing.T) {
		unknownProvider := registry.ProviderID("unknown-provider")

		if reg.IsAllowed(unknownProvider) {
			t.Error("unknown provider should NOT be allowed")
		}

		err := reg.RequireAllowed(unknownProvider)
		if err == nil {
			t.Error("unknown provider should return error")
		}

		// Should be ErrProviderNotRegistered
		if !errors.Is(err, registry.ErrProviderNotRegistered) {
			t.Errorf("expected ErrProviderNotRegistered, got %v", err)
		}
	})
}

func TestDefaultRegistry_Get(t *testing.T) {
	reg := registry.NewDefaultRegistry()

	t.Run("get existing provider", func(t *testing.T) {
		entry, ok := reg.Get(registry.ProviderMockWrite)
		if !ok {
			t.Error("mock-write should exist")
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

	t.Run("get non-existent provider", func(t *testing.T) {
		_, ok := reg.Get(registry.ProviderID("non-existent"))
		if ok {
			t.Error("non-existent provider should not exist")
		}
	})
}

func TestDefaultRegistry_List(t *testing.T) {
	reg := registry.NewDefaultRegistry()

	entries := reg.List()

	// Should have exactly 3 entries: mock-write, truelayer-sandbox, truelayer-live
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}

	// Verify entries are sorted by ID (deterministic order)
	if len(entries) >= 1 && entries[0].ID != registry.ProviderMockWrite {
		t.Errorf("first entry should be mock-write, got %s", entries[0].ID)
	}

	// All entries should be write providers
	for _, e := range entries {
		if !e.IsWrite {
			t.Errorf("entry %s should be a write provider", e.ID)
		}
	}
}

func TestDefaultRegistry_IsLiveEnvironment(t *testing.T) {
	reg := registry.NewDefaultRegistry()

	t.Run("mock-write is not live", func(t *testing.T) {
		if reg.IsLiveEnvironment(registry.ProviderMockWrite) {
			t.Error("mock-write should not be live environment")
		}
	})

	t.Run("truelayer-sandbox is not live", func(t *testing.T) {
		if reg.IsLiveEnvironment(registry.ProviderTrueLayerSandbox) {
			t.Error("truelayer-sandbox should not be live environment")
		}
	})

	t.Run("truelayer-live is live environment", func(t *testing.T) {
		if !reg.IsLiveEnvironment(registry.ProviderTrueLayerLive) {
			t.Error("truelayer-live should be live environment")
		}
	})
}

func TestCustomRegistry(t *testing.T) {
	entries := []registry.Entry{
		{
			ID:          "custom-provider",
			DisplayName: "Custom Test Provider",
			IsWrite:     true,
			Environment: "test",
			Allowed:     true,
		},
	}

	reg := registry.NewCustomRegistry(entries)

	t.Run("custom provider is allowed", func(t *testing.T) {
		if !reg.IsAllowed("custom-provider") {
			t.Error("custom provider should be allowed")
		}
	})

	t.Run("default providers are not present", func(t *testing.T) {
		if reg.IsAllowed(registry.ProviderMockWrite) {
			t.Error("mock-write should not be in custom registry")
		}
	})
}

func TestProviderError(t *testing.T) {
	t.Run("error with block reason", func(t *testing.T) {
		err := &registry.ProviderError{
			ProviderID:  registry.ProviderTrueLayerLive,
			Err:         registry.ErrProviderLiveBlocked,
			BlockReason: "live environment blocked by default",
		}

		msg := err.Error()
		if msg == "" {
			t.Error("error message should not be empty")
		}

		// Should contain provider ID
		if !containsString(msg, string(registry.ProviderTrueLayerLive)) {
			t.Errorf("error message should contain provider ID: %s", msg)
		}

		// Should contain block reason
		if !containsString(msg, "live environment blocked") {
			t.Errorf("error message should contain block reason: %s", msg)
		}
	})

	t.Run("error without block reason", func(t *testing.T) {
		err := &registry.ProviderError{
			ProviderID: "unknown",
			Err:        registry.ErrProviderNotRegistered,
		}

		msg := err.Error()
		if msg == "" {
			t.Error("error message should not be empty")
		}
	})

	t.Run("unwrap returns underlying error", func(t *testing.T) {
		err := &registry.ProviderError{
			ProviderID: registry.ProviderTrueLayerLive,
			Err:        registry.ErrProviderLiveBlocked,
		}

		if !errors.Is(err, registry.ErrProviderLiveBlocked) {
			t.Error("errors.Is should match underlying error")
		}
	})
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStringHelper(s, substr))
}

func containsStringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
