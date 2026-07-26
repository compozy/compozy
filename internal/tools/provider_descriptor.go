package tools

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/compozy/agh/internal/toolmeta"
)

// ExtensionToolRuntimeDescriptor is the runtime reconciliation proof for an extension tool.
type ExtensionToolRuntimeDescriptor struct {
	ID                 ToolID    `json:"id"`
	Handler            string    `json:"handler"`
	FriendlyVerb       string    `json:"friendly_verb,omitempty"`
	Preview            string    `json:"preview,omitempty"`
	InputSchemaDigest  string    `json:"input_schema_digest"`
	OutputSchemaDigest string    `json:"output_schema_digest,omitempty"`
	ReadOnly           bool      `json:"read_only"`
	Risk               RiskClass `json:"risk"`
	Capabilities       []string  `json:"capabilities,omitempty"`
}

// Validate checks runtime reconciliation metadata for an extension tool.
func (d ExtensionToolRuntimeDescriptor) Validate() error {
	if err := d.ID.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(d.Handler) == "" {
		return NewValidationError("handler", ReasonHandlerMissing, "handler is required")
	}
	if err := toolmeta.ValidateDescriptorMetadata(d.FriendlyVerb, d.Preview); err != nil {
		return NewValidationError("presentation", ReasonRuntimeDescriptorMismatch, err.Error())
	}
	if strings.TrimSpace(d.InputSchemaDigest) == "" {
		return NewValidationError(
			"input_schema_digest",
			ReasonRuntimeDescriptorMismatch,
			"input schema digest is required",
		)
	}
	if err := d.Risk.Validate("risk"); err != nil {
		return err
	}
	return nil
}

// ExtensionToolCallRequest is the extension host call request.
type ExtensionToolCallRequest struct {
	ToolID       ToolID          `json:"tool_id"`
	Handler      string          `json:"handler"`
	SessionID    string          `json:"session_id,omitempty"`
	InvocationID string          `json:"invocation_id,omitempty"`
	Input        json.RawMessage `json:"input"`
}

// ExtensionProvideToolsResponse is the extension host runtime descriptor response.
type ExtensionProvideToolsResponse struct {
	Tools []ExtensionToolRuntimeDescriptor `json:"tools"`
}

// ExtensionToolCallResponse is the extension host call response.
type ExtensionToolCallResponse struct {
	Result ToolResult `json:"result"`
}

// MCPToolDescriptor describes one externally discovered MCP tool.
type MCPToolDescriptor struct {
	ID           ToolID          `json:"id"`
	RawName      string          `json:"raw_name"`
	Title        string          `json:"title,omitempty"`
	FriendlyVerb string          `json:"friendly_verb,omitempty"`
	Preview      string          `json:"preview,omitempty"`
	Description  string          `json:"description,omitempty"`
	InputSchema  json.RawMessage `json:"input_schema"`
	OutputSchema json.RawMessage `json:"output_schema,omitempty"`
	Source       SourceRef       `json:"source"`
	ReadOnly     bool            `json:"read_only"`
}

// MCPToolCallRequest is the daemon-owned MCP adapter call request.
type MCPToolCallRequest struct {
	ToolID      ToolID          `json:"tool_id"`
	RawToolName string          `json:"raw_tool_name"`
	Input       json.RawMessage `json:"input"`
}

// MCPToolCallResponse is the daemon-owned MCP adapter call response.
type MCPToolCallResponse struct {
	Result ToolResult `json:"result"`
}

// MCPAuthStatus is a redacted auth diagnostic for external MCP sources.
type MCPAuthStatus struct {
	ServerName   string     `json:"server_name"`
	Status       string     `json:"status"`
	AuthType     string     `json:"auth_type,omitempty"`
	ClientID     string     `json:"client_id,omitempty"`
	Scopes       []string   `json:"scopes,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	Refreshable  bool       `json:"refreshable"`
	TokenPresent bool       `json:"token_present"`
	Diagnostic   string     `json:"diagnostic,omitempty"`
}
