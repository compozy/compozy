package llmadapter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
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
	// Parts carries multi-modal content parts (text, images, binaries).
	// When non-empty, adapters should include these parts in addition to
	// the textual Content (if provided). This enables vision/multimodal
	// prompts while preserving backward compatibility with text-only flows.
	Parts []ContentPart `json:"-"` // adapter-specific translation only
	// ToolCalls carries function/tool calls emitted by the assistant.
	// Constraint: only messages with Role == "assistant" may contain ToolCalls.
	ToolCalls []ToolCall
	// ToolResults carries tool responses provided by the runtime.
	// Constraint: only messages with Role == "tool" may contain ToolResults.
	ToolResults []ToolResult
}

// ContentPart is an interface for multi-modal message parts.
// Implementations: TextPart, ImageURLPart, BinaryPart.
type ContentPart interface{ isPart() }

// TextPart represents a plain-text content part.
type TextPart struct{ Text string }

func (TextPart) isPart() {}

// ImageURLPart represents an image referenced by a URL.
// The optional Detail can hint the provider about quality (e.g. "low", "high").
type ImageURLPart struct {
	URL    string
	Detail string // optional
}

func (ImageURLPart) isPart() {}

// BinaryPart represents a binary payload with MIME type (e.g. image/png).
// BinaryPart.Data may be large; document expectations (small thumbnails, chunks, or streaming) to avoid copies.
type BinaryPart struct {
	MIMEType string
	Data     []byte
}

func (BinaryPart) isPart() {}

// ToolDefinition represents a tool available to the LLM
type ToolDefinition struct {
	Name        string
	Description string
	Parameters  map[string]any // JSON Schema
}

// ToolResult represents a tool's response payload for the LLM
type ToolResult struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	// Content is the textual tool output (for text-first tools)
	Content string `json:"content,omitempty"`
	// JSONContent carries raw JSON payloads to avoid double-encoding.
	// Adapters that support structured tool output can prefer this field
	// when present; otherwise, fall back to Content.
	JSONContent json.RawMessage `json:"json,omitempty"`
}

type OutputFormatKind int

const (
	OutputFormatKindDefault OutputFormatKind = iota
	OutputFormatKindJSONSchema
)

type OutputFormat struct {
	Kind   OutputFormatKind
	Name   string
	Schema *schema.Schema
	Strict bool
}

func (f OutputFormat) IsJSONSchema() bool {
	return f.Kind == OutputFormatKindJSONSchema && f.Schema != nil
}

func DefaultOutputFormat() OutputFormat {
	return OutputFormat{Kind: OutputFormatKindDefault}
}

func NewJSONSchemaOutputFormat(name string, schema *schema.Schema, strict bool) OutputFormat {
	return OutputFormat{Kind: OutputFormatKindJSONSchema, Name: name, Schema: schema, Strict: strict}
}

// CallOptions represents options for the LLM call
type CallOptions struct {
	Temperature       float64
	TemperatureSet    bool
	MaxTokens         int32
	StopWords         []string
	ToolChoice        string // "auto", "none", or specific tool name
	OutputFormat      OutputFormat
	ForceJSON         bool
	ResponseMIME      string
	Provider          core.ProviderName
	Model             string
	TopP              float64
	TopK              int
	FrequencyPenalty  float64
	PresencePenalty   float64
	Seed              int
	N                 int
	CandidateCount    int
	RepetitionPenalty float64
	MaxLength         int
	MinLength         int
	Metadata          map[string]any
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

// Usage represents token usage information returned by the provider.
// Optional token categories remain nil when the provider omits the data.
type Usage struct {
	PromptTokens       int
	CompletionTokens   int
	TotalTokens        int
	ReasoningTokens    *int
	CachedPromptTokens *int
	InputAudioTokens   *int
	OutputAudioTokens  *int
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
	// CreateClient creates a new LLMClient for the given provider.
	// The provided context carries logger/trace/config values and must be
	// threaded into any SDK initializations that require it (e.g., GoogleAI).
	CreateClient(ctx context.Context, config *core.ProviderConfig) (LLMClient, error)
	// BuildRoute constructs a provider route for the specified config and optional fallbacks.
	BuildRoute(config *core.ProviderConfig, fallbacks ...*core.ProviderConfig) (*ProviderRoute, error)
	// Capabilities returns capability metadata for a provider.
	Capabilities(provider core.ProviderName) (ProviderCapabilities, error)
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
