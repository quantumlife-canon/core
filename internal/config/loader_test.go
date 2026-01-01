package config

import (
	"os"
	"testing"
	"time"

	"quantumlife/pkg/domain/identity"
)

func TestLoadFromString_BasicConfig(t *testing.T) {
	content := `
# Test configuration
[circle:work]
name = Work
email = google:work@company.com:email:read
calendar = google:primary:calendar:read

[circle:personal]
name = Personal
email = google:me@gmail.com:email:read
calendar = google:personal:calendar:read

[routing]
work_domains = company.com, corp.company.com
personal_domains = gmail.com, yahoo.com
`
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	config, err := LoadFromString(content, now)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify circles
	if len(config.Circles) != 2 {
		t.Errorf("expected 2 circles, got %d", len(config.Circles))
	}

	work := config.GetCircle("work")
	if work == nil {
		t.Fatal("work circle not found")
	}
	if work.Name != "Work" {
		t.Errorf("expected work name 'Work', got %q", work.Name)
	}
	if len(work.EmailIntegrations) != 1 {
		t.Errorf("expected 1 email integration, got %d", len(work.EmailIntegrations))
	}
	if work.EmailIntegrations[0].Provider != "google" {
		t.Errorf("expected email provider 'google', got %q", work.EmailIntegrations[0].Provider)
	}
	if work.EmailIntegrations[0].Identifier != "work@company.com" {
		t.Errorf("expected email identifier 'work@company.com', got %q", work.EmailIntegrations[0].Identifier)
	}

	personal := config.GetCircle("personal")
	if personal == nil {
		t.Fatal("personal circle not found")
	}
	if personal.Name != "Personal" {
		t.Errorf("expected personal name 'Personal', got %q", personal.Name)
	}

	// Verify routing
	if len(config.Routing.WorkDomains) != 2 {
		t.Errorf("expected 2 work domains, got %d", len(config.Routing.WorkDomains))
	}
	if config.Routing.WorkDomains[0] != "company.com" {
		t.Errorf("expected first work domain 'company.com', got %q", config.Routing.WorkDomains[0])
	}
}

func TestLoadFromString_Determinism(t *testing.T) {
	content := `
[circle:alpha]
name = Alpha
email = google:alpha@test.com

[circle:beta]
name = Beta
email = microsoft:beta@test.com

[routing]
work_domains = test.com
`
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	config1, err := LoadFromString(content, now)
	if err != nil {
		t.Fatalf("failed to load config 1: %v", err)
	}

	config2, err := LoadFromString(content, now)
	if err != nil {
		t.Fatalf("failed to load config 2: %v", err)
	}

	hash1 := config1.Hash()
	hash2 := config2.Hash()

	if hash1 != hash2 {
		t.Errorf("config hashes not deterministic: %s != %s", hash1, hash2)
	}

	// Verify canonical string is deterministic
	canonical1 := config1.CanonicalString()
	canonical2 := config2.CanonicalString()

	if canonical1 != canonical2 {
		t.Errorf("canonical strings not deterministic:\n%s\nvs\n%s", canonical1, canonical2)
	}
}

func TestCircleIDs_SortedOrder(t *testing.T) {
	content := `
[circle:zebra]
name = Zebra

[circle:apple]
name = Apple

[circle:mango]
name = Mango
`
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	config, err := LoadFromString(content, now)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	ids := config.CircleIDs()
	if len(ids) != 3 {
		t.Fatalf("expected 3 circle IDs, got %d", len(ids))
	}

	// Should be sorted alphabetically
	expected := []identity.EntityID{"apple", "mango", "zebra"}
	for i, id := range ids {
		if id != expected[i] {
			t.Errorf("circle ID at index %d: expected %q, got %q", i, expected[i], id)
		}
	}
}

func TestLoadFromString_MultipleIntegrations(t *testing.T) {
	content := `
[circle:work]
name = Work
email = google:work1@company.com:email:read
email = google:work2@company.com:email:read
calendar = google:primary:calendar:read
calendar = microsoft:secondary:calendar:read
finance = plaid:checking:finance:read
`
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	config, err := LoadFromString(content, now)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	work := config.GetCircle("work")
	if work == nil {
		t.Fatal("work circle not found")
	}

	if len(work.EmailIntegrations) != 2 {
		t.Errorf("expected 2 email integrations, got %d", len(work.EmailIntegrations))
	}
	if len(work.CalendarIntegrations) != 2 {
		t.Errorf("expected 2 calendar integrations, got %d", len(work.CalendarIntegrations))
	}
	if len(work.FinanceIntegrations) != 1 {
		t.Errorf("expected 1 finance integration, got %d", len(work.FinanceIntegrations))
	}
}

func TestLoadFromString_DefaultScopes(t *testing.T) {
	content := `
[circle:work]
name = Work
email = google:work@company.com
calendar = google:primary
finance = plaid:checking
`
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	config, err := LoadFromString(content, now)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	work := config.GetCircle("work")

	// Check default scopes
	if len(work.EmailIntegrations[0].Scopes) != 1 || work.EmailIntegrations[0].Scopes[0] != "email:read" {
		t.Errorf("expected default email scope 'email:read', got %v", work.EmailIntegrations[0].Scopes)
	}
	if len(work.CalendarIntegrations[0].Scopes) != 1 || work.CalendarIntegrations[0].Scopes[0] != "calendar:read" {
		t.Errorf("expected default calendar scope 'calendar:read', got %v", work.CalendarIntegrations[0].Scopes)
	}
	if len(work.FinanceIntegrations[0].Scopes) != 1 || work.FinanceIntegrations[0].Scopes[0] != "finance:read" {
		t.Errorf("expected default finance scope 'finance:read', got %v", work.FinanceIntegrations[0].Scopes)
	}
}

func TestLoadFromString_ParseError(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name:    "unknown section",
			content: "[unknown]\nkey = value",
			wantErr: "unknown section",
		},
		{
			name:    "invalid line format",
			content: "[circle:work]\nname Work",
			wantErr: "invalid line format",
		},
		{
			name:    "key outside section",
			content: "name = Work",
			wantErr: "key outside of section",
		},
		{
			name:    "unknown circle key",
			content: "[circle:work]\nname = Work\nunknown = value",
			wantErr: "unknown circle key",
		},
		{
			name:    "unknown routing key",
			content: "[circle:work]\nname = Work\n[routing]\nunknown = value",
			wantErr: "unknown routing key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
			_, err := LoadFromString(tt.content, now)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !containsString(err.Error(), tt.wantErr) {
				t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestValidate_EmptyCircles(t *testing.T) {
	config := &MultiCircleConfig{
		Circles: map[identity.EntityID]*CircleConfig{},
	}

	err := config.Validate()
	if err == nil {
		t.Fatal("expected validation error for empty circles")
	}

	configErr, ok := err.(*ConfigError)
	if !ok {
		t.Fatalf("expected ConfigError, got %T", err)
	}
	if configErr.Field != "circles" {
		t.Errorf("expected field 'circles', got %q", configErr.Field)
	}
}

func TestValidate_EmptyCircleName(t *testing.T) {
	config := &MultiCircleConfig{
		Circles: map[identity.EntityID]*CircleConfig{
			"work": {
				ID:   "work",
				Name: "", // Empty name
			},
		},
	}

	err := config.Validate()
	if err == nil {
		t.Fatal("expected validation error for empty circle name")
	}

	configErr, ok := err.(*ConfigError)
	if !ok {
		t.Fatalf("expected ConfigError, got %T", err)
	}
	if configErr.Field != "circle.name" {
		t.Errorf("expected field 'circle.name', got %q", configErr.Field)
	}
}

func TestLoadFromFile(t *testing.T) {
	content := `
[circle:work]
name = Work
email = google:work@company.com

[routing]
work_domains = company.com
`
	// Create temp file
	tmpFile, err := os.CreateTemp("", "test-*.qlconf")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	tmpFile.Close()

	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	config, err := LoadFromFile(tmpFile.Name(), now)
	if err != nil {
		t.Fatalf("failed to load config from file: %v", err)
	}

	if config.SourcePath != tmpFile.Name() {
		t.Errorf("expected source path %q, got %q", tmpFile.Name(), config.SourcePath)
	}
	if !config.LoadedAt.Equal(now) {
		t.Errorf("expected loaded at %v, got %v", now, config.LoadedAt)
	}
}

func TestDefaultConfig(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	config := DefaultConfig(now)

	if len(config.Circles) != 1 {
		t.Errorf("expected 1 default circle, got %d", len(config.Circles))
	}

	personal := config.GetCircle("personal")
	if personal == nil {
		t.Fatal("expected default personal circle")
	}
	if personal.Name != "Personal" {
		t.Errorf("expected default personal circle name 'Personal', got %q", personal.Name)
	}

	if err := config.Validate(); err != nil {
		t.Errorf("default config should be valid: %v", err)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
