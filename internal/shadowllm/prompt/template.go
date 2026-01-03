// Package prompt provides the prompt template for shadow LLM providers.
//
// Phase 19.3c: Real Azure Chat Shadow Run
//
// CRITICAL INVARIANTS:
//   - Prompt MUST NOT request any identifiable information
//   - Prompt MUST instruct model to output ONLY abstract buckets
//   - Prompt MUST enforce JSON schema for output parsing
//   - Output MUST be array of suggestions (v1.1.0+)
//   - No goroutines. No time.Now().
//   - Stdlib only.
//
// Reference: docs/ADR/ADR-0050-phase19-3c-real-azure-chat-shadow.md
package prompt

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"quantumlife/internal/shadowllm/privacy"
	"quantumlife/pkg/domain/shadowllm"
)

// TemplateVersion identifies the prompt template version.
// Incremented when the prompt structure changes.
// v1.0.0: Single suggestion output
// v1.1.0: Array of suggestions output (Phase 19.3c)
const TemplateVersion = "v1.1.0"

// TemplateHash returns a hash of the current template version.
func TemplateHash() string {
	h := sha256.Sum256([]byte("PROMPT_TEMPLATE|" + TemplateVersion + "|" + systemPrompt))
	return hex.EncodeToString(h[:16])
}

// systemPrompt is the system instruction for the shadow model.
// CRITICAL: Does NOT request any identifiable information.
// v1.1.0: Array of suggestions output.
const systemPrompt = `You are an assistant analyzing abstract activity patterns. You will receive ONLY abstract statistics about categories and magnitudes. You MUST output a JSON object with an array of suggestions (1-5 suggestions).

CRITICAL RULES:
1. Output ONLY valid JSON matching the schema
2. Output 1-5 suggestions covering the most relevant categories
3. Each "why_generic" MUST be a short generic sentence (max 140 chars)
4. "why_generic" MUST NOT contain: names, emails, companies, amounts, dates, or any identifiable information
5. Use ONLY the allowed enum values for each field

OUTPUT SCHEMA:
{
  "suggestions": [
    {
      "category": "money" | "time" | "work" | "home" | "people" | "health" | "family" | "school" | "unknown",
      "horizon": "now" | "soon" | "later" | "someday",
      "magnitude": "nothing" | "a_few" | "several",
      "confidence": "low" | "medium" | "high",
      "why_generic": "short generic sentence without any identifiers"
    }
  ]
}

EXAMPLES OF GOOD why_generic:
- "There are a few items in this category that might need attention."
- "Activity levels suggest reviewing this area soon."
- "Current patterns indicate low urgency."

EXAMPLES OF BAD why_generic (NEVER DO THIS):
- "John's email about the Amazon order needs attention."
- "The $500 payment to Netflix is due."
- "Meeting with Sarah at 3pm tomorrow."

You will now receive the abstract input. Analyze it and output ONLY the JSON object with your suggestions array.`

// userPromptTemplate is the template for the user message.
// Uses placeholders for abstract data.
const userPromptTemplate = `Analyze the following abstract activity summary:

Circle: {{CIRCLE_ID}}
Time Window: {{TIME_BUCKET}}

Category Magnitudes (obligations):
{{OBLIGATION_MAGNITUDES}}

Category Magnitudes (held items):
{{HELD_MAGNITUDES}}

Summary:
- Surface candidates: {{SURFACE_MAGNITUDE}}
- Draft candidates: {{DRAFT_MAGNITUDE}}
- Triggers seen: {{TRIGGERS_SEEN}}
- Mirror magnitude: {{MIRROR_MAGNITUDE}}

Categories with activity: {{ACTIVE_CATEGORIES}}

Based on this abstract data, provide your analysis as a single JSON object.`

// RenderPrompt renders the complete prompt from a privacy-safe input.
func RenderPrompt(input *privacy.ShadowInput) (system, user string) {
	system = systemPrompt
	user = renderUserPrompt(input)
	return system, user
}

// renderUserPrompt fills in the user prompt template.
func renderUserPrompt(input *privacy.ShadowInput) string {
	prompt := userPromptTemplate

	// Replace placeholders
	prompt = strings.ReplaceAll(prompt, "{{CIRCLE_ID}}", string(input.CircleID))
	prompt = strings.ReplaceAll(prompt, "{{TIME_BUCKET}}", input.TimeBucket)

	// Obligation magnitudes
	var oblMags strings.Builder
	for _, cat := range allCategories() {
		mag := input.ObligationMagnitudes[cat]
		if mag == "" {
			mag = "nothing"
		}
		oblMags.WriteString("  ")
		oblMags.WriteString(string(cat))
		oblMags.WriteString(": ")
		oblMags.WriteString(string(mag))
		oblMags.WriteString("\n")
	}
	prompt = strings.ReplaceAll(prompt, "{{OBLIGATION_MAGNITUDES}}", oblMags.String())

	// Held magnitudes
	var heldMags strings.Builder
	for _, cat := range allCategories() {
		mag := input.HeldMagnitudes[cat]
		if mag == "" {
			mag = "nothing"
		}
		heldMags.WriteString("  ")
		heldMags.WriteString(string(cat))
		heldMags.WriteString(": ")
		heldMags.WriteString(string(mag))
		heldMags.WriteString("\n")
	}
	prompt = strings.ReplaceAll(prompt, "{{HELD_MAGNITUDES}}", heldMags.String())

	// Summary values
	prompt = strings.ReplaceAll(prompt, "{{SURFACE_MAGNITUDE}}", string(input.SurfaceCandidateMagnitude))
	prompt = strings.ReplaceAll(prompt, "{{DRAFT_MAGNITUDE}}", string(input.DraftCandidateMagnitude))
	if input.TriggersSeen {
		prompt = strings.ReplaceAll(prompt, "{{TRIGGERS_SEEN}}", "yes")
	} else {
		prompt = strings.ReplaceAll(prompt, "{{TRIGGERS_SEEN}}", "no")
	}
	prompt = strings.ReplaceAll(prompt, "{{MIRROR_MAGNITUDE}}", string(input.MirrorMagnitude))

	// Active categories
	var active []string
	for cat, present := range input.CategoryPresence {
		if present {
			active = append(active, string(cat))
		}
	}
	if len(active) == 0 {
		prompt = strings.ReplaceAll(prompt, "{{ACTIVE_CATEGORIES}}", "none")
	} else {
		prompt = strings.ReplaceAll(prompt, "{{ACTIVE_CATEGORIES}}", strings.Join(active, ", "))
	}

	return prompt
}

// allCategories returns categories in sorted order for determinism.
func allCategories() []shadowllm.AbstractCategory {
	return shadowllm.AllCategories()
}

// ModelOutputSchema describes the expected JSON output format (v1.0.0 legacy).
// Used for backward compatibility with single-suggestion responses.
type ModelOutputSchema struct {
	ConfidenceBucket     string `json:"confidence_bucket"`
	HorizonBucket        string `json:"horizon_bucket"`
	MagnitudeBucket      string `json:"magnitude_bucket"`
	Category             string `json:"category"`
	WhyGeneric           string `json:"why_generic"`
	SuggestedActionClass string `json:"suggested_action_class"`
}

// ModelOutputArraySchema describes the v1.1.0 array output format.
// Contains 1-5 suggestions with category-specific analysis.
type ModelOutputArraySchema struct {
	Suggestions []SuggestionSchema `json:"suggestions"`
}

// SuggestionSchema describes a single suggestion in the array.
type SuggestionSchema struct {
	Category   string `json:"category"`
	Horizon    string `json:"horizon"`
	Magnitude  string `json:"magnitude"`
	Confidence string `json:"confidence"`
	WhyGeneric string `json:"why_generic"`
}
