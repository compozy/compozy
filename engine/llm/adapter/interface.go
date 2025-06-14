package llmadapter

import (
	"context"

	"github.com/compozy/compozy/engine/core"
)

// LLMRequest represents a request to the LLM, independent of provider
type LLMRequest struct {
	SystemPrompt string
	Messages     []Message
	Tools        []ToolDefinition
	Options      CallOptions
}

// Message represents a conversation message
type Message struct {
	Role    string // "system", "user", "assistant", "tool"
	Content string
}

// ToolDefinition represents a tool available to the LLM
type ToolDefinition struct {
	Name        string
	Description string
	Parameters  map[string]any // JSON Schema
}

// CallOptions represents options for the LLM call
type CallOptions struct {
	Temperature      float64
	MaxTokens        int32
	StopWords        []string
	UseJSONMode      bool
	ToolChoice       string // "auto", "none", or specific tool name
	StructuredOutput bool
}

// LLMResponse represents the response from the LLM
type LLMResponse struct {
	Content   string
	ToolCalls []ToolCall
	Usage     *Usage
}

// ToolCall represents a tool invocation request from the LLM
type ToolCall struct {
	ID        string
	Name      string
	Arguments string // JSON string
}

// Usage represents token usage information
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// LLMClient is the main interface for LLM interactions
type LLMClient interface {
	// GenerateContent sends a request to the LLM and returns a response
	GenerateContent(ctx context.Context, req *LLMRequest) (*LLMResponse, error)
}

// Factory creates LLMClient instances based on provider configuration
type Factory interface {
	// CreateClient creates a new LLMClient for the given provider
	CreateClient(config *core.ProviderConfig) (LLMClient, error)
}
