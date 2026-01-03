// Package config provides multi-circle configuration loading.
//
// This loader parses .qlconf files using stdlib only (no YAML/JSON libs).
// The format is line-based and deterministic.
//
// Note: The config types are defined in pkg/domain/config to allow
// cross-package usage. This package provides the loading functionality.
package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	pkgconfig "quantumlife/pkg/domain/config"
	"quantumlife/pkg/domain/identity"
)

// Type aliases for convenience (re-export from pkg/domain/config)
type (
	MultiCircleConfig   = pkgconfig.MultiCircleConfig
	CircleConfig        = pkgconfig.CircleConfig
	EmailIntegration    = pkgconfig.EmailIntegration
	CalendarIntegration = pkgconfig.CalendarIntegration
	FinanceIntegration  = pkgconfig.FinanceIntegration
	RoutingConfig       = pkgconfig.RoutingConfig
	ConfigError         = pkgconfig.ConfigError
)

// LoadFromFile loads a MultiCircleConfig from a .qlconf file.
func LoadFromFile(path string, loadedAt time.Time) (*MultiCircleConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	config := &MultiCircleConfig{
		Circles:    make(map[identity.EntityID]*CircleConfig),
		Shadow:     pkgconfig.DefaultShadowConfig(), // CRITICAL: OFF by default
		SourcePath: path,
		LoadedAt:   loadedAt,
	}

	scanner := bufio.NewScanner(file)
	lineNum := 0

	var currentSection string
	var currentCircleID identity.EntityID

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Section header: [circle:id] or [routing]
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			header := line[1 : len(line)-1]

			if strings.HasPrefix(header, "circle:") {
				currentSection = "circle"
				currentCircleID = identity.EntityID(strings.TrimPrefix(header, "circle:"))
				if _, exists := config.Circles[currentCircleID]; !exists {
					config.Circles[currentCircleID] = &CircleConfig{
						ID: currentCircleID,
					}
				}
			} else if header == "routing" {
				currentSection = "routing"
				currentCircleID = ""
			} else if header == "shadow" {
				currentSection = "shadow"
				currentCircleID = ""
			} else {
				return nil, &ParseError{Line: lineNum, Message: "unknown section: " + header}
			}
			continue
		}

		// Key = value pair
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, &ParseError{Line: lineNum, Message: "invalid line format, expected 'key = value'"}
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch currentSection {
		case "circle":
			if currentCircleID == "" {
				return nil, &ParseError{Line: lineNum, Message: "circle key outside of circle section"}
			}
			circle := config.Circles[currentCircleID]

			switch key {
			case "name":
				circle.Name = value

			case "email":
				integration, err := parseEmailIntegration(value)
				if err != nil {
					return nil, &ParseError{Line: lineNum, Message: err.Error()}
				}
				circle.EmailIntegrations = append(circle.EmailIntegrations, integration)

			case "calendar":
				integration, err := parseCalendarIntegration(value)
				if err != nil {
					return nil, &ParseError{Line: lineNum, Message: err.Error()}
				}
				circle.CalendarIntegrations = append(circle.CalendarIntegrations, integration)

			case "finance":
				integration, err := parseFinanceIntegration(value)
				if err != nil {
					return nil, &ParseError{Line: lineNum, Message: err.Error()}
				}
				circle.FinanceIntegrations = append(circle.FinanceIntegrations, integration)

			default:
				return nil, &ParseError{Line: lineNum, Message: "unknown circle key: " + key}
			}

		case "routing":
			switch key {
			case "work_domains":
				config.Routing.WorkDomains = parseCSV(value)
			case "personal_domains":
				config.Routing.PersonalDomains = parseCSV(value)
			case "vip_senders":
				config.Routing.VIPSenders = parseCSV(value)
			case "family_members":
				config.Routing.FamilyMembers = parseCSV(value)
			default:
				return nil, &ParseError{Line: lineNum, Message: "unknown routing key: " + key}
			}

		case "shadow":
			switch key {
			case "mode":
				// Validate mode value
				if value != "off" && value != "observe" {
					return nil, &ParseError{Line: lineNum, Message: "invalid shadow mode: " + value + " (must be 'off' or 'observe')"}
				}
				config.Shadow.Mode = value
			case "model":
				// Validate model value (Phase 19.3: expanded to include azure_openai)
				if value != "stub" && value != "azure_openai" && value != "local_slm" {
					return nil, &ParseError{Line: lineNum, Message: "invalid shadow model: " + value + " (must be 'stub', 'azure_openai', or 'local_slm')"}
				}
				config.Shadow.ModelName = value
			case "provider_kind":
				// Phase 19.3: provider kind
				if value != "none" && value != "stub" && value != "azure_openai" && value != "local_slm" {
					return nil, &ParseError{Line: lineNum, Message: "invalid shadow provider_kind: " + value}
				}
				config.Shadow.ProviderKind = value
			case "real_allowed":
				// Phase 19.3: explicit opt-in for real providers
				config.Shadow.RealAllowed = value == "true"
			case "max_suggestions":
				// Phase 19.3c: max suggestions per run
				n := parsePositiveInt(value)
				if n > 0 {
					config.Shadow.MaxSuggestions = n
				}
			case "azure_endpoint":
				// Phase 19.3: Azure OpenAI endpoint (optional - env var preferred)
				config.Shadow.AzureOpenAI.Endpoint = value
			case "azure_deployment":
				// Phase 19.3: Azure OpenAI deployment name (optional - env var preferred)
				config.Shadow.AzureOpenAI.Deployment = value
			case "azure_chat_deployment":
				// Phase 19.3b: Explicit chat deployment name
				config.Shadow.AzureOpenAI.ChatDeployment = value
			case "azure_embed_deployment":
				// Phase 19.3b: Embeddings deployment name (optional)
				config.Shadow.AzureOpenAI.EmbedDeployment = value
			case "azure_api_version":
				// Phase 19.3: Azure OpenAI API version
				config.Shadow.AzureOpenAI.APIVersion = value
			case "azure_key_env_name":
				// Phase 19.3b: Environment variable name for API key
				config.Shadow.AzureOpenAI.APIKeyEnvName = value
			default:
				return nil, &ParseError{Line: lineNum, Message: "unknown shadow key: " + key}
			}

		default:
			return nil, &ParseError{Line: lineNum, Message: "key outside of section"}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	// Validate the config
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return config, nil
}

// parseEmailIntegration parses "provider:identifier:scope1,scope2"
func parseEmailIntegration(value string) (EmailIntegration, error) {
	parts := strings.SplitN(value, ":", 3)
	if len(parts) < 2 {
		return EmailIntegration{}, fmt.Errorf("email integration requires at least 'provider:identifier'")
	}

	integration := EmailIntegration{
		Provider:   strings.TrimSpace(parts[0]),
		Identifier: strings.TrimSpace(parts[1]),
	}

	if len(parts) == 3 && parts[2] != "" {
		integration.Scopes = parseCSV(parts[2])
	} else {
		// Default scope for email
		integration.Scopes = []string{"email:read"}
	}

	return integration, nil
}

// parseCalendarIntegration parses "provider:calendar_id:scope1,scope2"
func parseCalendarIntegration(value string) (CalendarIntegration, error) {
	parts := strings.SplitN(value, ":", 3)
	if len(parts) < 2 {
		return CalendarIntegration{}, fmt.Errorf("calendar integration requires at least 'provider:calendar_id'")
	}

	integration := CalendarIntegration{
		Provider:   strings.TrimSpace(parts[0]),
		CalendarID: strings.TrimSpace(parts[1]),
	}

	if len(parts) == 3 && parts[2] != "" {
		integration.Scopes = parseCSV(parts[2])
	} else {
		// Default scope for calendar
		integration.Scopes = []string{"calendar:read"}
	}

	return integration, nil
}

// parseFinanceIntegration parses "provider:identifier:scope1,scope2"
func parseFinanceIntegration(value string) (FinanceIntegration, error) {
	parts := strings.SplitN(value, ":", 3)
	if len(parts) < 2 {
		return FinanceIntegration{}, fmt.Errorf("finance integration requires at least 'provider:identifier'")
	}

	integration := FinanceIntegration{
		Provider:   strings.TrimSpace(parts[0]),
		Identifier: strings.TrimSpace(parts[1]),
	}

	if len(parts) == 3 && parts[2] != "" {
		integration.Scopes = parseCSV(parts[2])
	} else {
		// Default scope for finance (read-only)
		integration.Scopes = []string{"finance:read"}
	}

	return integration, nil
}

// parseCSV parses a comma-separated list of values.
func parseCSV(value string) []string {
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// parsePositiveInt parses a positive integer from string.
// Returns 0 if parsing fails or value is <= 0.
func parsePositiveInt(s string) int {
	if s == "" {
		return 0
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// ParseError represents a parsing error with line information.
type ParseError struct {
	Line    int
	Message string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("parse error at line %d: %s", e.Line, e.Message)
}

// LoadFromString loads config from a string (useful for testing).
func LoadFromString(content string, loadedAt time.Time) (*MultiCircleConfig, error) {
	// Create a temporary file to reuse the file loader
	// This ensures consistent behavior between file and string loading.
	tmpFile, err := os.CreateTemp("", "qlconf-*.qlconf")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()

	return LoadFromFile(tmpFile.Name(), loadedAt)
}

// DefaultConfig returns a minimal default configuration.
func DefaultConfig(loadedAt time.Time) *MultiCircleConfig {
	return &MultiCircleConfig{
		Circles: map[identity.EntityID]*CircleConfig{
			"personal": {
				ID:   "personal",
				Name: "Personal",
			},
		},
		Shadow:     pkgconfig.DefaultShadowConfig(), // CRITICAL: OFF by default
		LoadedAt:   loadedAt,
		SourcePath: "(default)",
	}
}
