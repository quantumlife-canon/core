// Package validate provides output validation for shadow LLM responses.
//
// Phase 19.3: Azure OpenAI Shadow Provider
//
// CRITICAL INVARIANTS:
//   - Output MUST match expected JSON schema
//   - why_generic MUST NOT contain identifiable information
//   - All enum values MUST be from allowed sets
//   - Invalid output results in safe defaults
//   - No goroutines. No time.Now().
//   - Stdlib only.
//
// Reference: docs/ADR/ADR-0044-phase19-3-azure-openai-shadow-provider.md
package validate

import (
	"encoding/json"
	"regexp"
	"strings"

	"quantumlife/internal/shadowllm/prompt"
	"quantumlife/pkg/domain/shadowllm"
)

// Validator validates LLM output for safety and schema compliance.
type Validator struct {
	forbiddenPatterns []*regexp.Regexp
}

// NewValidator creates a new output validator.
func NewValidator() *Validator {
	return &Validator{
		forbiddenPatterns: compileForbiddenPatterns(),
	}
}

// compileForbiddenPatterns compiles regex patterns for forbidden content in output.
func compileForbiddenPatterns() []*regexp.Regexp {
	patterns := []string{
		// Email addresses
		`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`,
		// URLs
		`https?://[^\s"]+`,
		// Currency amounts with symbols
		`[$£€¥]\s*\d+`,
		`\d+\s*[$£€¥]`,
		// Large numbers that could be amounts (4+ digits)
		`\b\d{4,}\b`,
		// Phone numbers
		`\b\d{3}[-.\s]?\d{3}[-.\s]?\d{4}\b`,
		// Dates in common formats (but allow time bucket format YYYY-MM-DD in context)
		`\b\d{1,2}/\d{1,2}/\d{2,4}\b`,
		// Common vendor/company patterns
		`(?i)\b(amazon|google|apple|microsoft|netflix|uber|lyft|venmo|paypal|chase|wells\s*fargo|bank\s*of\s*america|walmart|target|costco|starbucks)\b`,
		// Names with titles
		`(?i)\b(mr\.|mrs\.|ms\.|dr\.)\s+[A-Z][a-z]+`,
		// Specific times
		`\b\d{1,2}:\d{2}\s*(am|pm|AM|PM)?\b`,
	}

	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		if re, err := regexp.Compile(p); err == nil {
			compiled = append(compiled, re)
		}
	}
	return compiled
}

// ValidatedOutput represents a validated and sanitized model output.
type ValidatedOutput struct {
	// Confidence is the validated confidence bucket.
	Confidence shadowllm.ConfidenceBucket

	// Horizon is the validated horizon bucket.
	Horizon shadowllm.Horizon

	// Magnitude is the validated magnitude bucket.
	Magnitude shadowllm.MagnitudeBucket

	// Category is the validated category.
	Category shadowllm.AbstractCategory

	// WhyGeneric is the sanitized generic rationale.
	// Empty if validation failed.
	WhyGeneric string

	// SuggestedActionClass maps to SuggestionType.
	SuggestedActionClass shadowllm.SuggestionType

	// IsValid indicates if the output passed validation.
	IsValid bool

	// ValidationError describes what failed, if any.
	ValidationError string
}

// ParseAndValidate parses JSON output and validates all fields.
//
// CRITICAL: Returns safe defaults if validation fails.
// Never allows invalid data to propagate.
func (v *Validator) ParseAndValidate(jsonOutput string) *ValidatedOutput {
	result := &ValidatedOutput{
		// Safe defaults
		Confidence:           shadowllm.ConfidenceLow,
		Horizon:              shadowllm.HorizonSomeday,
		Magnitude:            shadowllm.MagnitudeNothing,
		Category:             shadowllm.CategoryUnknown,
		WhyGeneric:           "",
		SuggestedActionClass: shadowllm.SuggestHold,
		IsValid:              false,
	}

	// Parse JSON
	var output prompt.ModelOutputSchema
	if err := json.Unmarshal([]byte(jsonOutput), &output); err != nil {
		result.ValidationError = "invalid JSON: " + truncateError(err.Error())
		return result
	}

	// Validate confidence bucket
	conf, ok := validateConfidence(output.ConfidenceBucket)
	if !ok {
		result.ValidationError = "invalid confidence_bucket"
		return result
	}
	result.Confidence = conf

	// Validate horizon bucket
	horizon, ok := validateHorizon(output.HorizonBucket)
	if !ok {
		result.ValidationError = "invalid horizon_bucket"
		return result
	}
	result.Horizon = horizon

	// Validate magnitude bucket
	mag, ok := validateMagnitude(output.MagnitudeBucket)
	if !ok {
		result.ValidationError = "invalid magnitude_bucket"
		return result
	}
	result.Magnitude = mag

	// Validate category
	cat, ok := validateCategory(output.Category)
	if !ok {
		result.ValidationError = "invalid category"
		return result
	}
	result.Category = cat

	// Validate suggested action class
	action, ok := validateActionClass(output.SuggestedActionClass)
	if !ok {
		result.ValidationError = "invalid suggested_action_class"
		return result
	}
	result.SuggestedActionClass = action

	// Validate why_generic for forbidden patterns
	if err := v.validateWhyGeneric(output.WhyGeneric); err != nil {
		result.ValidationError = "why_generic: " + err.Error()
		result.WhyGeneric = "" // Clear it but still return partial result
		return result
	}
	result.WhyGeneric = output.WhyGeneric

	result.IsValid = true
	return result
}

// validateWhyGeneric checks the generic rationale for forbidden patterns.
func (v *Validator) validateWhyGeneric(s string) error {
	// Check length
	if len(s) > shadowllm.MaxWhyGenericLength {
		return &ValidationError{Message: "exceeds max length"}
	}

	// Check for forbidden patterns
	for _, re := range v.forbiddenPatterns {
		if re.MatchString(s) {
			return &ValidationError{Message: "contains forbidden pattern"}
		}
	}

	return nil
}

// validateConfidence validates and maps confidence bucket.
func validateConfidence(s string) (shadowllm.ConfidenceBucket, bool) {
	switch strings.ToLower(s) {
	case "low":
		return shadowllm.ConfidenceLow, true
	case "medium", "med":
		return shadowllm.ConfidenceMed, true
	case "high":
		return shadowllm.ConfidenceHigh, true
	default:
		return shadowllm.ConfidenceLow, false
	}
}

// validateHorizon validates and maps horizon bucket.
func validateHorizon(s string) (shadowllm.Horizon, bool) {
	switch strings.ToLower(s) {
	case "now":
		return shadowllm.HorizonNow, true
	case "soon":
		return shadowllm.HorizonSoon, true
	case "later":
		return shadowllm.HorizonLater, true
	case "someday":
		return shadowllm.HorizonSomeday, true
	default:
		return shadowllm.HorizonSomeday, false
	}
}

// validateMagnitude validates and maps magnitude bucket.
func validateMagnitude(s string) (shadowllm.MagnitudeBucket, bool) {
	switch strings.ToLower(s) {
	case "nothing":
		return shadowllm.MagnitudeNothing, true
	case "a_few":
		return shadowllm.MagnitudeAFew, true
	case "several":
		return shadowllm.MagnitudeSeveral, true
	default:
		return shadowllm.MagnitudeNothing, false
	}
}

// validateCategory validates and maps category.
func validateCategory(s string) (shadowllm.AbstractCategory, bool) {
	switch strings.ToLower(s) {
	case "money":
		return shadowllm.CategoryMoney, true
	case "time":
		return shadowllm.CategoryTime, true
	case "work":
		return shadowllm.CategoryWork, true
	case "home":
		return shadowllm.CategoryHome, true
	case "people":
		return shadowllm.CategoryPeople, true
	case "health":
		return shadowllm.CategoryHealth, true
	case "family":
		return shadowllm.CategoryFamily, true
	case "school":
		return shadowllm.CategorySchool, true
	case "unknown":
		return shadowllm.CategoryUnknown, true
	default:
		return shadowllm.CategoryUnknown, false
	}
}

// validateActionClass validates and maps action class to suggestion type.
func validateActionClass(s string) (shadowllm.SuggestionType, bool) {
	switch strings.ToLower(s) {
	case "none", "hold":
		return shadowllm.SuggestHold, true
	case "surface":
		return shadowllm.SuggestSurfaceCandidate, true
	case "proof":
		// Map "proof" to hold for safety
		return shadowllm.SuggestHold, true
	default:
		return shadowllm.SuggestHold, false
	}
}

// truncateError truncates error messages to avoid leaking details.
func truncateError(s string) string {
	const maxLen = 50
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}

// ValidationError represents an output validation error.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}
