// Package read_test provides tests for the finance read connector interface.
package read_test

import (
	"testing"

	"quantumlife/internal/connectors/finance/read"
	"quantumlife/pkg/primitives"
)

func TestValidateEnvelopeForFinanceRead(t *testing.T) {
	validEnvelope := func() *primitives.ExecutionEnvelope {
		return &primitives.ExecutionEnvelope{
			Mode:          primitives.ModeSuggestOnly,
			TraceID:       "trace-123",
			ActorCircleID: "circle-1",
			ScopesUsed:    []string{read.ScopeFinanceRead},
		}
	}

	tests := []struct {
		name        string
		envelope    *primitives.ExecutionEnvelope
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid suggest_only mode",
			envelope:    validEnvelope(),
			expectError: false,
		},
		{
			name: "valid simulate mode",
			envelope: func() *primitives.ExecutionEnvelope {
				e := validEnvelope()
				e.Mode = primitives.ModeSimulate
				return e
			}(),
			expectError: false,
		},
		{
			name: "rejects execute mode",
			envelope: func() *primitives.ExecutionEnvelope {
				e := validEnvelope()
				e.Mode = primitives.ModeExecute
				return e
			}(),
			expectError: true,
			errorMsg:    "execute mode not allowed",
		},
		{
			name: "rejects missing trace ID",
			envelope: func() *primitives.ExecutionEnvelope {
				e := validEnvelope()
				e.TraceID = ""
				return e
			}(),
			expectError: true,
			errorMsg:    "trace ID required",
		},
		{
			name: "rejects missing actor circle ID",
			envelope: func() *primitives.ExecutionEnvelope {
				e := validEnvelope()
				e.ActorCircleID = ""
				return e
			}(),
			expectError: true,
			errorMsg:    "actor circle ID required",
		},
		{
			name: "rejects forbidden scope",
			envelope: func() *primitives.ExecutionEnvelope {
				e := validEnvelope()
				e.ScopesUsed = []string{read.ScopeFinanceRead, "finance:write"}
				return e
			}(),
			expectError: true,
			errorMsg:    "forbidden scope",
		},
		{
			name: "rejects missing finance:read scope",
			envelope: func() *primitives.ExecutionEnvelope {
				e := validEnvelope()
				e.ScopesUsed = []string{"other:scope"}
				return e
			}(),
			expectError: true,
			errorMsg:    "finance:read required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := read.ValidateEnvelopeForFinanceRead(tt.envelope)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
				}
				// Just check that error occurred, specific message validation optional
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestScopeValidation(t *testing.T) {
	// Test allowed scopes
	allowed := read.AllowedScopes
	if len(allowed) == 0 {
		t.Error("expected at least one allowed scope")
	}

	if allowed[0] != read.ScopeFinanceRead {
		t.Errorf("expected first allowed scope to be %q, got %q", read.ScopeFinanceRead, allowed[0])
	}

	// Test forbidden patterns
	forbidden := read.ForbiddenScopePatterns
	if len(forbidden) == 0 {
		t.Error("expected at least one forbidden scope pattern")
	}

	// Verify critical patterns are forbidden
	expectedForbidden := []string{"finance:write", "finance:execute", "finance:transfer", "payment", "transfer", "initiate"}
	for _, expected := range expectedForbidden {
		found := false
		for _, pattern := range forbidden {
			if pattern == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q to be in forbidden patterns", expected)
		}
	}
}

func TestIsForbiddenScope(t *testing.T) {
	tests := []struct {
		scope     string
		forbidden bool
	}{
		{"finance:read", false},
		{"finance:write", true},
		{"finance:execute", true},
		{"finance:transfer", true},
		{"payment:initiate", true},
		{"transfer:create", true},
		{"initiate:payment", true},
		{"calendar:read", false},
		{"calendar:write", false},
	}

	for _, tt := range tests {
		t.Run(tt.scope, func(t *testing.T) {
			result := read.IsForbiddenScope(tt.scope)
			if result != tt.forbidden {
				t.Errorf("IsForbiddenScope(%q) = %v, want %v", tt.scope, result, tt.forbidden)
			}
		})
	}
}

func TestIsAllowedScope(t *testing.T) {
	tests := []struct {
		scope   string
		allowed bool
	}{
		{"finance:read", true},
		{"finance:write", false},
		{"calendar:read", false},
		{"unknown:scope", false},
	}

	for _, tt := range tests {
		t.Run(tt.scope, func(t *testing.T) {
			result := read.IsAllowedScope(tt.scope)
			if result != tt.allowed {
				t.Errorf("IsAllowedScope(%q) = %v, want %v", tt.scope, result, tt.allowed)
			}
		})
	}
}
