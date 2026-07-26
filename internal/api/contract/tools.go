package contract

import (
	"encoding/json"
	"time"

	"github.com/compozy/agh/internal/tools"
)

// ToolDescriptorPayload is the public descriptor shape exposed by daemon APIs.
type ToolDescriptorPayload struct {
	ToolID              tools.ToolID          `json:"tool_id"`
	Backend             ToolBackendRefPayload `json:"backend"`
	DisplayTitle        string                `json:"display_title,omitempty"`
	FriendlyVerb        string                `json:"friendly_verb,omitempty"`
	Preview             string                `json:"preview,omitempty"`
	Description         string                `json:"description"`
	InputSchema         json.RawMessage       `json:"input_schema"`
	OutputSchema        json.RawMessage       `json:"output_schema,omitempty"`
	InputSchemaDigest   string                `json:"input_schema_digest"`
	OutputSchemaDigest  string                `json:"output_schema_digest,omitempty"`
	Source              ToolSourceRefPayload  `json:"source"`
	Visibility          tools.Visibility      `json:"visibility"`
	Risk                tools.RiskClass       `json:"risk"`
	ReadOnly            bool                  `json:"read_only"`
	Destructive         bool                  `json:"destructive"`
	OpenWorld           bool                  `json:"open_world"`
	RequiresInteraction bool                  `json:"requires_interaction"`
	ConcurrencySafe     bool                  `json:"concurrency_safe"`
	MaxResultBytes      int64                 `json:"max_result_bytes,omitempty"`
	Toolsets            []tools.ToolsetID     `json:"toolsets,omitempty"`
	Tags                []string              `json:"tags,omitempty"`
	SearchHints         []string              `json:"search_hints,omitempty"`
}

// ToolBackendRefPayload identifies the executable backend without leaking backend secrets.
type ToolBackendRefPayload struct {
	Kind                 tools.BackendKind `json:"kind"`
	ExtensionID          string            `json:"extension_id,omitempty"`
	Handler              string            `json:"handler,omitempty"`
	MCPServer            string            `json:"mcp_server,omitempty"`
	MCPTool              string            `json:"mcp_tool,omitempty"`
	NativeName           string            `json:"native_name,omitempty"`
	RequiresCapabilities []string          `json:"requires_capabilities,omitempty"`
}

// ToolSourceRefPayload preserves descriptor provenance without alternate identities.
type ToolSourceRefPayload struct {
	Kind            tools.SourceKind `json:"kind"`
	Owner           string           `json:"owner"`
	RawServerName   string           `json:"raw_server_name,omitempty"`
	RawToolName     string           `json:"raw_tool_name,omitempty"`
	ResourceID      string           `json:"resource_id,omitempty"`
	ResourceVersion string           `json:"resource_version,omitempty"`
	WorkspaceID     string           `json:"workspace_id,omitempty"`
	Scope           string           `json:"scope,omitempty"`
}

// ToolAvailabilityPayload records composable runtime availability diagnostics.
type ToolAvailabilityPayload struct {
	Registered  bool               `json:"registered"`
	Enabled     bool               `json:"enabled"`
	Available   bool               `json:"available"`
	Authorized  bool               `json:"authorized"`
	Executable  bool               `json:"executable"`
	Conflicted  bool               `json:"conflicted"`
	ReasonCodes []tools.ReasonCode `json:"reason_codes,omitempty"`
}

// ToolPolicyDecisionPayload records effective policy and availability decisions for a caller.
type ToolPolicyDecisionPayload struct {
	VisibleToOperator    bool               `json:"visible_to_operator"`
	VisibleToSession     bool               `json:"visible_to_session"`
	Callable             bool               `json:"callable"`
	ApprovalRequired     bool               `json:"approval_required"`
	SystemPermissionMode string             `json:"system_permission_mode,omitempty"`
	SessionPolicyResult  string             `json:"session_policy_result,omitempty"`
	AgentPolicyResult    string             `json:"agent_policy_result,omitempty"`
	RegistryPolicyResult string             `json:"registry_policy_result,omitempty"`
	SourcePolicyResult   string             `json:"source_policy_result,omitempty"`
	AvailabilityResult   string             `json:"availability_result,omitempty"`
	HookResult           string             `json:"hook_result,omitempty"`
	ReasonCodes          []tools.ReasonCode `json:"reason_codes,omitempty"`
}

// ToolPayload is one public registry projection row.
type ToolPayload struct {
	Descriptor   ToolDescriptorPayload     `json:"descriptor"`
	Availability ToolAvailabilityPayload   `json:"availability"`
	Decision     ToolPolicyDecisionPayload `json:"decision"`
}

// ToolsResponse returns a registry projection.
type ToolsResponse struct {
	Tools []ToolPayload `json:"tools"`
}

// ToolResponse returns one registry tool projection.
type ToolResponse struct {
	Tool ToolPayload `json:"tool"`
}

// ToolSearchRequest filters the scoped registry projection.
type ToolSearchRequest struct {
	Query       string `json:"query"`
	Limit       int    `json:"limit,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	SessionID   string `json:"session_id,omitempty"`
	AgentName   string `json:"agent_name,omitempty"`
}

// ToolInvokeRequest executes a concrete tool call through registry dispatch.
type ToolInvokeRequest struct {
	SessionID            string          `json:"session_id,omitempty"`
	WorkspaceID          string          `json:"workspace_id,omitempty"`
	AgentName            string          `json:"agent_name,omitempty"`
	ToolCallID           string          `json:"tool_call_id,omitempty"`
	TurnID               string          `json:"turn_id,omitempty"`
	CorrelationID        string          `json:"correlation_id,omitempty"`
	Input                json.RawMessage `json:"input"`
	SensitiveInputFields []string        `json:"sensitive_input_fields,omitempty"`
	ApprovalToken        string          `json:"approval_token,omitempty"`
}

// ToolInvokeResponse is the stable result envelope returned by invoke endpoints.
type ToolInvokeResponse struct {
	ToolID     tools.ToolID           `json:"tool_id"`
	Status     string                 `json:"status"`
	Result     tools.ToolResult       `json:"result"`
	Truncated  bool                   `json:"truncated"`
	DurationMS int64                  `json:"duration_ms"`
	Events     []ToolCallEventPayload `json:"events"`
}

// ToolArtifactPageResponse is one exact byte page from a retained oversized result.
type ToolArtifactPageResponse struct {
	Artifact   tools.ArtifactRef `json:"artifact"`
	Offset     int64             `json:"offset"`
	Bytes      int64             `json:"bytes"`
	TotalBytes int64             `json:"total_bytes"`
	DataBase64 string            `json:"data_base64"`
	NextOffset int64             `json:"next_offset"`
	EOF        bool              `json:"eof"`
}

// ToolCallEventPayload is a redacted dispatch event surfaced when a handler can collect events.
type ToolCallEventPayload struct {
	Kind                 tools.ToolCallEventKind `json:"kind"`
	ToolID               tools.ToolID            `json:"tool_id"`
	DisplayTitle         string                  `json:"display_title,omitempty"`
	SourceKind           tools.SourceKind        `json:"source_kind,omitempty"`
	SourceOwner          string                  `json:"source_owner,omitempty"`
	WorkspaceID          string                  `json:"workspace_id,omitempty"`
	SessionID            string                  `json:"session_id,omitempty"`
	AgentName            string                  `json:"agent_name,omitempty"`
	Risk                 tools.RiskClass         `json:"risk,omitempty"`
	ReadOnly             bool                    `json:"read_only"`
	Destructive          bool                    `json:"destructive"`
	OpenWorld            bool                    `json:"open_world"`
	ApprovalMode         string                  `json:"approval_mode,omitempty"`
	Decision             string                  `json:"decision,omitempty"`
	ReasonCodes          []tools.ReasonCode      `json:"reason_codes,omitempty"`
	DurationMS           int64                   `json:"duration_ms,omitempty"`
	ResultBytes          int64                   `json:"result_bytes,omitempty"`
	Truncated            bool                    `json:"truncated"`
	CorrelationID        string                  `json:"correlation_id,omitempty"`
	ErrorCode            tools.ErrorCode         `json:"error_code,omitempty"`
	InputDigest          string                  `json:"input_digest,omitempty"`
	RedactedInputFields  []string                `json:"redacted_input_fields,omitempty"`
	ResultDigest         string                  `json:"result_digest,omitempty"`
	ResultRedactionPaths []string                `json:"result_redaction_paths,omitempty"`
}

// ToolApprovalRequest requests one local approval reference for a concrete invocation.
type ToolApprovalRequest struct {
	SessionID   string          `json:"session_id"`
	WorkspaceID string          `json:"workspace_id,omitempty"`
	AgentName   string          `json:"agent_name,omitempty"`
	Input       json.RawMessage `json:"input,omitempty"`
	InputDigest string          `json:"input_digest,omitempty"`
}

// ToolApprovalPayload is the only response shape that can expose a raw approval token.
type ToolApprovalPayload struct {
	ApprovalToken string       `json:"approval_token"`
	ExpiresAt     time.Time    `json:"expires_at"`
	ToolID        tools.ToolID `json:"tool_id"`
	InputDigest   string       `json:"input_digest"`
}

// ToolApprovalResponse returns one approval reference.
type ToolApprovalResponse struct {
	Approval ToolApprovalPayload `json:"approval"`
}

// ToolsetPayload describes one named toolset and its expansion state.
type ToolsetPayload struct {
	ID            tools.ToolsetID    `json:"id"`
	Tools         []string           `json:"tools,omitempty"`
	Toolsets      []tools.ToolsetID  `json:"toolsets,omitempty"`
	ExpandedTools []tools.ToolID     `json:"expanded_tools,omitempty"`
	Status        string             `json:"status"`
	ReasonCodes   []tools.ReasonCode `json:"reason_codes,omitempty"`
}

// ToolsetsResponse returns the known toolset catalog projection.
type ToolsetsResponse struct {
	Toolsets []ToolsetPayload `json:"toolsets"`
}

// ToolsetResponse returns one toolset projection.
type ToolsetResponse struct {
	Toolset ToolsetPayload `json:"toolset"`
}

// ToolErrorPayload is the structured error envelope for tool registry routes.
type ToolErrorPayload struct {
	Code          tools.ErrorCode            `json:"code"`
	Message       string                     `json:"message"`
	ToolID        tools.ToolID               `json:"tool_id,omitempty"`
	ReasonCodes   []tools.ReasonCode         `json:"reason_codes,omitempty"`
	Layer         string                     `json:"layer,omitempty"`
	Details       map[string]json.RawMessage `json:"details,omitempty"`
	PartialResult *tools.ToolResult          `json:"partial_result,omitempty"`
}

// ToolErrorResponse returns one structured tool error.
type ToolErrorResponse struct {
	Error ToolErrorPayload `json:"error"`
}
