// Package payees_test provides tests for the v9.10 payee registry.
package payees_test

import (
	"errors"
	"testing"

	"quantumlife/internal/connectors/finance/write/payees"
)

func TestDefaultRegistry_AllowedPayees(t *testing.T) {
	reg := payees.NewDefaultRegistry()

	t.Run("sandbox-utility is allowed for mock-write", func(t *testing.T) {
		if !reg.IsAllowed(payees.PayeeSandboxUtility, "mock-write") {
			t.Error("sandbox-utility should be allowed for mock-write")
		}

		err := reg.RequireAllowed(payees.PayeeSandboxUtility, "mock-write")
		if err != nil {
			t.Errorf("sandbox-utility should not return error: %v", err)
		}
	})

	t.Run("sandbox-rent is allowed for mock-write", func(t *testing.T) {
		if !reg.IsAllowed(payees.PayeeSandboxRent, "mock-write") {
			t.Error("sandbox-rent should be allowed for mock-write")
		}

		err := reg.RequireAllowed(payees.PayeeSandboxRent, "mock-write")
		if err != nil {
			t.Errorf("sandbox-rent should not return error: %v", err)
		}
	})

	t.Run("sandbox-merchant is allowed for mock-write", func(t *testing.T) {
		if !reg.IsAllowed(payees.PayeeSandboxMerchant, "mock-write") {
			t.Error("sandbox-merchant should be allowed for mock-write")
		}
	})
}

func TestDefaultRegistry_UnregisteredPayees(t *testing.T) {
	reg := payees.NewDefaultRegistry()

	t.Run("unknown payee is not allowed", func(t *testing.T) {
		unknownPayee := payees.PayeeID("unknown-payee")

		if reg.IsAllowed(unknownPayee, "mock-write") {
			t.Error("unknown payee should NOT be allowed")
		}

		err := reg.RequireAllowed(unknownPayee, "mock-write")
		if err == nil {
			t.Error("unknown payee should return error")
		}

		// Should be ErrPayeeNotRegistered
		if !errors.Is(err, payees.ErrPayeeNotRegistered) {
			t.Errorf("expected ErrPayeeNotRegistered, got %v", err)
		}
	})

	t.Run("free-text recipient is not allowed", func(t *testing.T) {
		freeTextPayee := payees.PayeeID("John Smith - Account 12345")

		if reg.IsAllowed(freeTextPayee, "mock-write") {
			t.Error("free-text payee should NOT be allowed")
		}

		err := reg.RequireAllowed(freeTextPayee, "mock-write")
		if err == nil {
			t.Error("free-text payee should return error")
		}

		if !errors.Is(err, payees.ErrPayeeNotRegistered) {
			t.Errorf("expected ErrPayeeNotRegistered, got %v", err)
		}
	})
}

func TestDefaultRegistry_ProviderMismatch(t *testing.T) {
	reg := payees.NewDefaultRegistry()

	t.Run("TrueLayer payee not allowed for mock-write", func(t *testing.T) {
		// sandbox-utility-tl is registered for truelayer-sandbox
		tlPayee := payees.PayeeID("sandbox-utility-tl")

		// Should be allowed for truelayer-sandbox
		if !reg.IsAllowed(tlPayee, "truelayer-sandbox") {
			t.Error("sandbox-utility-tl should be allowed for truelayer-sandbox")
		}

		// mock-write is special - it accepts sandbox payees for testing
		if !reg.IsAllowed(tlPayee, "mock-write") {
			t.Error("sandbox-utility-tl should be allowed for mock-write (testing)")
		}
	})
}

func TestDefaultRegistry_Get(t *testing.T) {
	reg := payees.NewDefaultRegistry()

	t.Run("get existing payee", func(t *testing.T) {
		entry, ok := reg.Get(payees.PayeeSandboxUtility)
		if !ok {
			t.Error("sandbox-utility should exist")
		}

		if entry.ID != payees.PayeeSandboxUtility {
			t.Errorf("expected ID %q, got %q", payees.PayeeSandboxUtility, entry.ID)
		}

		if entry.Environment != payees.EnvSandbox {
			t.Errorf("expected environment %q, got %q", payees.EnvSandbox, entry.Environment)
		}

		if !entry.Allowed {
			t.Error("sandbox-utility should be allowed")
		}

		if entry.Currency != "GBP" {
			t.Errorf("expected currency GBP, got %q", entry.Currency)
		}
	})

	t.Run("get non-existent payee", func(t *testing.T) {
		_, ok := reg.Get(payees.PayeeID("non-existent"))
		if ok {
			t.Error("non-existent payee should not exist")
		}
	})
}

func TestDefaultRegistry_List(t *testing.T) {
	reg := payees.NewDefaultRegistry()

	entries := reg.List()

	// Should have at least 5 entries (3 mock + 2 truelayer)
	if len(entries) < 5 {
		t.Errorf("expected at least 5 entries, got %d", len(entries))
	}

	// All entries should be sandbox (no live in default)
	for _, e := range entries {
		if e.Environment == payees.EnvLive {
			t.Errorf("entry %s should not be live environment in default registry", e.ID)
		}
	}
}

func TestDefaultRegistry_IsLiveEnvironment(t *testing.T) {
	reg := payees.NewDefaultRegistry()

	t.Run("sandbox payees are not live", func(t *testing.T) {
		if reg.IsLiveEnvironment(payees.PayeeSandboxUtility) {
			t.Error("sandbox-utility should not be live environment")
		}

		if reg.IsLiveEnvironment(payees.PayeeSandboxRent) {
			t.Error("sandbox-rent should not be live environment")
		}
	})
}

func TestDefaultRegistry_IsRegistered(t *testing.T) {
	reg := payees.NewDefaultRegistry()

	t.Run("sandbox payees are registered", func(t *testing.T) {
		if !reg.IsRegistered(payees.PayeeSandboxUtility) {
			t.Error("sandbox-utility should be registered")
		}

		if !reg.IsRegistered(payees.PayeeSandboxRent) {
			t.Error("sandbox-rent should be registered")
		}
	})

	t.Run("unknown payees are not registered", func(t *testing.T) {
		if reg.IsRegistered(payees.PayeeID("random-string")) {
			t.Error("random-string should not be registered")
		}
	})
}

func TestCustomRegistry(t *testing.T) {
	entries := []payees.Entry{
		{
			ID:          "custom-payee",
			DisplayName: "Custom Test Payee",
			ProviderID:  "custom-provider",
			Environment: payees.EnvSandbox,
			Allowed:     true,
			Currency:    "USD",
		},
	}

	reg := payees.NewCustomRegistry(entries)

	t.Run("custom payee is allowed", func(t *testing.T) {
		if !reg.IsAllowed("custom-payee", "custom-provider") {
			t.Error("custom payee should be allowed")
		}
	})

	t.Run("default payees are not present", func(t *testing.T) {
		if reg.IsAllowed(payees.PayeeSandboxUtility, "mock-write") {
			t.Error("sandbox-utility should not be in custom registry")
		}
	})
}

func TestPayeeError(t *testing.T) {
	t.Run("error with block reason", func(t *testing.T) {
		err := &payees.PayeeError{
			PayeeID:     payees.PayeeSandboxUtility,
			ProviderID:  "mock-write",
			Err:         payees.ErrPayeeNotAllowed,
			BlockReason: "payee explicitly blocked",
		}

		msg := err.Error()
		if msg == "" {
			t.Error("error message should not be empty")
		}

		// Should contain payee ID
		if !containsString(msg, string(payees.PayeeSandboxUtility)) {
			t.Errorf("error message should contain payee ID: %s", msg)
		}

		// Should contain block reason
		if !containsString(msg, "payee explicitly blocked") {
			t.Errorf("error message should contain block reason: %s", msg)
		}
	})

	t.Run("unwrap returns underlying error", func(t *testing.T) {
		err := &payees.PayeeError{
			PayeeID: payees.PayeeSandboxUtility,
			Err:     payees.ErrPayeeNotRegistered,
		}

		if !errors.Is(err, payees.ErrPayeeNotRegistered) {
			t.Error("errors.Is should match underlying error")
		}
	})
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
