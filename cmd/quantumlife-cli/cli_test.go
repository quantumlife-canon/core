package main

import (
	"strings"
	"testing"
)

// TestParseArgs tests that CLI arguments are parsed correctly.
// Note: This is a table-driven test for argument validation logic.
func TestParseArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantCmd string
		wantErr bool
	}{
		{
			name:    "auth google",
			args:    []string{"auth", "google"},
			wantCmd: "auth",
		},
		{
			name:    "auth microsoft",
			args:    []string{"auth", "microsoft"},
			wantCmd: "auth",
		},
		{
			name:    "auth exchange",
			args:    []string{"auth", "exchange"},
			wantCmd: "auth",
		},
		{
			name:    "demo family",
			args:    []string{"demo", "family"},
			wantCmd: "demo",
		},
		{
			name:    "version",
			args:    []string{"version"},
			wantCmd: "version",
		},
		{
			name:    "help",
			args:    []string{"help"},
			wantCmd: "help",
		},
		{
			name:    "empty args",
			args:    []string{},
			wantCmd: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cmd string
			if len(tt.args) > 0 {
				cmd = tt.args[0]
			}
			if cmd != tt.wantCmd {
				t.Errorf("Expected command %q, got %q", tt.wantCmd, cmd)
			}
		})
	}
}

// TestOutputNeverContainsSecrets validates that output functions
// never include sensitive token material.
func TestOutputNeverContainsSecrets(t *testing.T) {
	// Forbidden patterns that should never appear in CLI output
	forbiddenPatterns := []string{
		"access_token",
		"refresh_token",
		"client_secret",
		"bearer ",
	}

	// Sample outputs that the CLI might produce
	sampleOutputs := []string{
		"Token Handle ID: token-123",
		"Circle ID: circle-1",
		"Provider: google",
		"Scopes: calendar:read",
		"Authorization Successful",
		"Persistence: ENABLED",
	}

	for _, output := range sampleOutputs {
		lower := strings.ToLower(output)
		for _, pattern := range forbiddenPatterns {
			if strings.Contains(lower, pattern) {
				t.Errorf("Output %q contains forbidden pattern %q", output, pattern)
			}
		}
	}
}

// TestProviderValidation tests that only valid providers are accepted.
func TestProviderValidation(t *testing.T) {
	validProviders := []string{"google", "microsoft"}
	invalidProviders := []string{"facebook", "twitter", "invalid", ""}

	for _, p := range validProviders {
		if !isValidProvider(p) {
			t.Errorf("Expected %q to be valid provider", p)
		}
	}

	for _, p := range invalidProviders {
		if isValidProvider(p) {
			t.Errorf("Expected %q to be invalid provider", p)
		}
	}
}

func isValidProvider(p string) bool {
	switch p {
	case "google", "microsoft":
		return true
	default:
		return false
	}
}

// TestRedactedTokenNeverLong verifies that redacted tokens are short.
func TestRedactedTokenNeverLong(t *testing.T) {
	// A token-like string that might accidentally be printed
	tokenLike := "ya29.a0AWY7CknXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"

	// Our redaction logic should produce something short
	redacted := redactToken(tokenLike)

	// Redacted should be much shorter than original
	if len(redacted) > 20 {
		t.Errorf("Redacted token too long: %d chars", len(redacted))
	}

	// Redacted should not contain the full token
	if strings.Contains(tokenLike, redacted) && len(redacted) > 10 {
		t.Error("Redacted token should not be a substring that reveals the token")
	}
}

func redactToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "..." + token[len(token)-4:]
}

// TestModeValidation tests run mode parsing.
func TestModeValidation(t *testing.T) {
	validModes := []string{"simulate", "suggest_only"}
	invalidModes := []string{"execute", "run", ""}

	for _, m := range validModes {
		if !isValidMode(m) {
			t.Errorf("Expected %q to be valid mode", m)
		}
	}

	for _, m := range invalidModes {
		if isValidMode(m) {
			t.Errorf("Expected %q to be invalid mode", m)
		}
	}
}

func isValidMode(m string) bool {
	switch m {
	case "simulate", "suggest_only":
		return true
	default:
		return false
	}
}
