// Package azureopenai provides Azure OpenAI providers for shadow LLM.
//
// Phase 19.3c: Shared types for Azure OpenAI providers.
//
// CRITICAL INVARIANTS:
//   - Stdlib only.
//   - No goroutines. No time.Now().
//
// Reference: docs/ADR/ADR-0050-phase19-3c-real-azure-chat-shadow.md
package azureopenai

// ChatRequest is the Azure OpenAI chat request structure.
// Used by both Provider and ChatProvider.
type ChatRequest struct {
	Messages       []ChatMessage        `json:"messages"`
	MaxTokens      int                  `json:"max_tokens,omitempty"`
	Temperature    float64              `json:"temperature,omitempty"`
	ResponseFormat *ChatResponseFormat  `json:"response_format,omitempty"`
}

// ChatMessage is a single message in the chat conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatResponseFormat specifies the response format.
type ChatResponseFormat struct {
	Type string `json:"type"`
}

// ChatResponse is the Azure OpenAI chat response structure.
type ChatResponse struct {
	Choices []ChatChoice `json:"choices"`
}

// ChatChoice is a single choice in the response.
type ChatChoice struct {
	Message ChatMessage `json:"message"`
}
