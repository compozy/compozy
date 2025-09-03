package llmadapter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/engine/core"
)

// LLMRequest represents a request to the LLM, independent of provider
// Role constants for message roles
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

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
	// ToolCalls carries function/tool calls emitted by the assistant.
	// Constraint: only messages with Role == "assistant" may contain ToolCalls.
	ToolCalls []ToolCall
	// ToolResults carries tool responses provided by the runtime.
	// Constraint: only messages with Role == "tool" may contain ToolResults.
	ToolResults []ToolResult
}

// ToolDefinition represents a tool available to the LLM
type ToolDefinition struct {
	Name        string
	Description string
	Parameters  map[string]any // JSON Schema
}

// ToolResult represents a tool's response payload for the LLM
type ToolResult struct {
	ID   string
	Name string
	// Content is the textual tool output (for text-first tools)
	Content string
	// JSONContent carries raw JSON payloads to avoid double-encoding.
	// Adapters that support structured tool output can prefer this field
	// when present; otherwise, fall back to Content.
	JSONContent json.RawMessage
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
	Arguments json.RawMessage // JSON bytes
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
	// Close cleans up any resources held by the client
	Close() error
}

// Factory creates LLMClient instances based on provider configuration
type Factory interface {
	// CreateClient creates a new LLMClient for the given provider
	CreateClient(config *core.ProviderConfig) (LLMClient, error)
}

// ValidateConversation asserts role-specific constraints for messages:
// - Only assistant messages may contain ToolCalls
// - Only tool messages may contain ToolResults
// This helps catch wiring mistakes early.
func ValidateConversation(messages []Message) error {
	for i, m := range messages {
		if len(m.ToolCalls) > 0 && m.Role != RoleAssistant {
			return fmt.Errorf("message[%d] role %q cannot contain ToolCalls", i, m.Role)
		}
		if len(m.ToolResults) > 0 && m.Role != RoleTool {
			return fmt.Errorf("message[%d] role %q cannot contain ToolResults", i, m.Role)
		}
	}
	return nil
}
