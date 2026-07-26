// Package tools defines canonical Tool Registry contracts.
package tools

import (
	"context"
	"encoding/json"
)

// BackendKind identifies the executable backend class.
type BackendKind string

const (
	// BackendNativeGo executes a daemon-compiled Go handler.
	BackendNativeGo BackendKind = "native_go"
	// BackendExtensionHost executes through the extension host subprocess runtime.
	BackendExtensionHost BackendKind = "extension_host"
	// BackendMCP executes through daemon-owned MCP client adapters.
	BackendMCP BackendKind = "mcp"
	// BackendBridge is reserved for a later bridge adapter TechSpec.
	BackendBridge BackendKind = "bridge"
)

// Validate ensures the backend kind is documented.
func (k BackendKind) Validate(field string) error {
	switch k {
	case BackendNativeGo, BackendExtensionHost, BackendMCP, BackendBridge:
		return nil
	default:
		return NewValidationError(field, ReasonBackendNotExecutable, "unsupported backend kind")
	}
}

// BackendRef binds a descriptor to its executable backend.
type BackendRef struct {
	Kind                 BackendKind `json:"kind"`
	ExtensionID          string      `json:"extension_id,omitempty"`
	Handler              string      `json:"handler,omitempty"`
	MCPServer            string      `json:"mcp_server,omitempty"`
	MCPTool              string      `json:"mcp_tool,omitempty"`
	NativeName           string      `json:"native_name,omitempty"`
	RequiresCapabilities []string    `json:"requires_capabilities,omitempty"`
}

// Validate ensures the backend reference has the fields required for its kind.
func (b BackendRef) Validate(field string) error {
	if err := b.Kind.Validate(field + ".kind"); err != nil {
		return err
	}
	switch b.Kind {
	case BackendNativeGo:
		if b.NativeName == "" {
			return NewValidationError(
				field+".native_name",
				ReasonDependencyMissing,
				"native backend requires native_name",
			)
		}
	case BackendExtensionHost:
		if b.ExtensionID == "" {
			return NewValidationError(
				field+".extension_id",
				ReasonExtensionInactive,
				"extension_host backend requires extension_id",
			)
		}
		if b.Handler == "" {
			return NewValidationError(field+".handler", ReasonHandlerMissing, "extension_host backend requires handler")
		}
	case BackendMCP:
		if b.MCPServer == "" {
			return NewValidationError(field+".mcp_server", ReasonMCPUnreachable, "mcp backend requires mcp_server")
		}
		if b.MCPTool == "" {
			return NewValidationError(field+".mcp_tool", ReasonDependencyMissing, "mcp backend requires mcp_tool")
		}
	case BackendBridge:
		return NewValidationError(field+".kind", ReasonBackendNotExecutable, "bridge backend is reserved post-MVP")
	}
	return nil
}

// SourceKind identifies the provenance class for a descriptor.
type SourceKind string

const (
	// SourceBuiltin marks daemon-defined tools.
	SourceBuiltin SourceKind = "builtin"
	// SourceMCP marks tools discovered from MCP servers.
	SourceMCP SourceKind = "mcp"
	// SourceExtension marks tools provided by extensions.
	SourceExtension SourceKind = "extension"
	// SourceDynamic marks future runtime-assembled tools.
	SourceDynamic SourceKind = "dynamic"
)

// ToolSource preserves the public source-name type used by existing resource contracts.
type ToolSource = SourceKind

const (
	// ToolSourceBuiltin marks daemon-defined tools.
	ToolSourceBuiltin = SourceBuiltin
	// ToolSourceMCP marks tools discovered from MCP servers.
	ToolSourceMCP = SourceMCP
	// ToolSourceExtension marks tools provided by extensions.
	ToolSourceExtension = SourceExtension
	// ToolSourceDynamic marks future runtime-assembled tools.
	ToolSourceDynamic = SourceDynamic
)

// String returns the stable source kind text.
func (k SourceKind) String() string {
	return string(k)
}

// Validate ensures the source kind is documented.
func (k SourceKind) Validate(field string) error {
	switch k {
	case SourceBuiltin, SourceMCP, SourceExtension, SourceDynamic:
		return nil
	default:
		return NewValidationError(field, ReasonSourceDisabled, "unsupported source kind")
	}
}

// SourceRef preserves provenance without creating alternate tool identities.
type SourceRef struct {
	Kind            SourceKind `json:"kind"`
	Owner           string     `json:"owner"`
	RawServerName   string     `json:"raw_server_name,omitempty"`
	RawToolName     string     `json:"raw_tool_name,omitempty"`
	ResourceID      string     `json:"resource_id,omitempty"`
	ResourceVersion string     `json:"resource_version,omitempty"`
	WorkspaceID     string     `json:"workspace_id,omitempty"`
	Scope           string     `json:"scope,omitempty"`
}

// Validate ensures source provenance can support deterministic diagnostics.
func (s SourceRef) Validate(field string) error {
	if err := s.Kind.Validate(field + ".kind"); err != nil {
		return err
	}
	if s.Owner == "" {
		return NewValidationError(field+".owner", ReasonSourceDisabled, "source owner is required")
	}
	if s.Kind == SourceMCP && (s.RawServerName == "" || s.RawToolName == "") {
		return NewValidationError(field, ReasonMCPUnreachable, "mcp sources require raw server and tool names")
	}
	return nil
}

// Visibility identifies which surfaces may display a descriptor.
type Visibility string

const (
	// VisibilityInternal limits a tool to daemon-internal use.
	VisibilityInternal Visibility = "internal"
	// VisibilityOperator exposes a tool to operator diagnostics.
	VisibilityOperator Visibility = "operator"
	// VisibilitySession exposes a tool to session-scoped views.
	VisibilitySession Visibility = "session"
	// VisibilityModel exposes a tool to model-visible projections.
	VisibilityModel Visibility = "model"
)

// Validate ensures the visibility is documented.
func (v Visibility) Validate(field string) error {
	switch v {
	case VisibilityInternal, VisibilityOperator, VisibilitySession, VisibilityModel:
		return nil
	default:
		return NewValidationError(field, ReasonPolicyDenied, "unsupported visibility")
	}
}

// RiskClass classifies the safety posture of a tool.
type RiskClass string

const (
	// RiskRead marks read-only local inspection.
	RiskRead RiskClass = "read"
	// RiskMutating marks state-changing behavior.
	RiskMutating RiskClass = "mutating"
	// RiskOpenWorld marks access to arbitrary external state.
	RiskOpenWorld RiskClass = "open_world"
	// RiskDestructive marks destructive or irreversible behavior.
	RiskDestructive RiskClass = "destructive"
)

// Validate ensures the risk class is documented.
func (r RiskClass) Validate(field string) error {
	switch r {
	case RiskRead, RiskMutating, RiskOpenWorld, RiskDestructive:
		return nil
	default:
		return NewValidationError(field, ReasonPolicyDenied, "unsupported risk class")
	}
}

// Registry owns tool discovery and dispatch for all surfaces.
type Registry interface {
	List(ctx context.Context, scope Scope) ([]ToolView, error)
	Search(ctx context.Context, scope Scope, q SearchQuery) ([]ToolView, error)
	Get(ctx context.Context, scope Scope, id ToolID) (ToolView, error)
	Call(ctx context.Context, scope Scope, req CallRequest) (ToolResult, error)
}

// Handle is the executable runtime contract for one tool.
type Handle interface {
	Descriptor() Descriptor
	Availability(ctx context.Context, scope Scope) Availability
	Call(ctx context.Context, req CallRequest) (ToolResult, error)
}

// Provider contributes descriptors and executable handles from one source.
type Provider interface {
	ID() SourceRef
	List(ctx context.Context, scope Scope) ([]Descriptor, error)
	Resolve(ctx context.Context, scope Scope, id ToolID) (Handle, bool, error)
}

// NativeToolFunc is the daemon-compiled function signature for native tools.
type NativeToolFunc func(ctx context.Context, scope Scope, req CallRequest) (ToolResult, error)

// ExtensionToolInvoker invokes out-of-process extension tool providers.
type ExtensionToolInvoker interface {
	ProvideTools(ctx context.Context, extensionID string) ([]ExtensionToolRuntimeDescriptor, error)
	CallTool(ctx context.Context, extensionID string, req ExtensionToolCallRequest) (ToolResult, error)
}

// MCPCallExecutor lists and calls MCP tools without exposing credential material.
type MCPCallExecutor interface {
	ListTools(ctx context.Context, source SourceRef) ([]MCPToolDescriptor, error)
	CallTool(ctx context.Context, source SourceRef, req MCPToolCallRequest) (ToolResult, error)
}

// MCPAuthStatusProvider returns redacted MCP auth status for diagnostics.
type MCPAuthStatusProvider interface {
	Status(ctx context.Context, source SourceRef) (MCPAuthStatus, error)
}

// PolicyEvaluator computes the effective policy decision for a descriptor.
type PolicyEvaluator interface {
	Evaluate(ctx context.Context, scope Scope, d Descriptor) (EffectiveToolDecision, error)
}

// ResultProcessor sanitizes provider output before hooks and finalizes the public result once afterward.
type ResultProcessor interface {
	Sanitize(ctx context.Context, d Descriptor, result ToolResult) (ToolResult, error)
	Finalize(ctx context.Context, scope Scope, d Descriptor, result ToolResult) (ToolResult, error)
}

// HookRunner runs typed registry hooks around dispatch.
type HookRunner interface {
	PreCall(ctx context.Context, call CallRequest) (CallRequest, EffectiveToolDecision, error)
	PostCall(ctx context.Context, call CallRequest, result ToolResult) (ToolResult, error)
	PostError(ctx context.Context, call CallRequest, err error) error
}

// ApprovalBridge mediates approval-required calls before provider execution.
type ApprovalBridge interface {
	RequestToolApproval(ctx context.Context, scope Scope, call CallRequest, view *ToolView) error
}

// Scope identifies the caller context used for projections and dispatch.
type Scope struct {
	WorkspaceID string `json:"workspace_id,omitempty"`
	SessionID   string `json:"session_id,omitempty"`
	AgentName   string `json:"agent_name,omitempty"`
	ActorKind   string `json:"actor_kind,omitempty"`
	Operator    bool   `json:"operator,omitempty"`
}

// SearchQuery describes a registry search request.
type SearchQuery struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

// ToolView is a descriptor plus effective diagnostics for a caller.
type ToolView struct {
	Descriptor   Descriptor            `json:"descriptor"`
	Availability Availability          `json:"availability"`
	Decision     EffectiveToolDecision `json:"decision"`
}

// CallRequest is the canonical dispatch request.
type CallRequest struct {
	ToolID               ToolID          `json:"tool_id"`
	ToolCallID           string          `json:"tool_call_id,omitempty"`
	TurnID               string          `json:"turn_id,omitempty"`
	SessionID            string          `json:"session_id,omitempty"`
	WorkspaceID          string          `json:"workspace_id,omitempty"`
	AgentName            string          `json:"agent_name,omitempty"`
	ActorKind            string          `json:"actor_kind,omitempty"`
	CorrelationID        string          `json:"correlation_id,omitempty"`
	Input                json.RawMessage `json:"input"`
	SensitiveInputFields []string        `json:"sensitive_input_fields,omitempty"`
	ApprovalToken        string          `json:"approval_token,omitempty"`
}

// EffectiveToolDecision records the combined policy and availability decision.
type EffectiveToolDecision struct {
	VisibleToOperator    bool         `json:"visible_to_operator"`
	VisibleToSession     bool         `json:"visible_to_session"`
	Callable             bool         `json:"callable"`
	ApprovalRequired     bool         `json:"approval_required"`
	SystemPermissionMode string       `json:"system_permission_mode,omitempty"`
	SessionPolicyResult  string       `json:"session_policy_result,omitempty"`
	AgentPolicyResult    string       `json:"agent_policy_result,omitempty"`
	RegistryPolicyResult string       `json:"registry_policy_result,omitempty"`
	SourcePolicyResult   string       `json:"source_policy_result,omitempty"`
	AvailabilityResult   string       `json:"availability_result,omitempty"`
	HookResult           string       `json:"hook_result,omitempty"`
	ReasonCodes          []ReasonCode `json:"reason_codes,omitempty"`
}

// ToolCallEventKind identifies one structured dispatch observability event.
type ToolCallEventKind string

const (
	// ToolCallStarted reports that dispatch passed identity resolution and began.
	ToolCallStarted ToolCallEventKind = "tool.call_started"
	// ToolCallCompleted reports successful provider execution and result limiting.
	ToolCallCompleted ToolCallEventKind = "tool.call_completed"
	// ToolCallFailed reports schema, hook, backend, cancellation, or timeout failures.
	ToolCallFailed ToolCallEventKind = "tool.call_failed"
	// ToolCallDenied reports policy, availability, approval, conflict, or hook denial.
	ToolCallDenied ToolCallEventKind = "tool.call_denied"
	// ToolResultTruncated reports deterministic result truncation.
	ToolResultTruncated ToolCallEventKind = "tool.result_truncated"
)

// ToolCallEvent is the redacted dispatch event envelope emitted by Registry.Call.
type ToolCallEvent struct {
	Kind                 ToolCallEventKind `json:"kind"`
	ToolID               ToolID            `json:"tool_id"`
	DisplayTitle         string            `json:"display_title,omitempty"`
	SourceKind           SourceKind        `json:"source_kind,omitempty"`
	SourceOwner          string            `json:"source_owner,omitempty"`
	WorkspaceID          string            `json:"workspace_id,omitempty"`
	SessionID            string            `json:"session_id,omitempty"`
	AgentName            string            `json:"agent_name,omitempty"`
	Risk                 RiskClass         `json:"risk,omitempty"`
	ReadOnly             bool              `json:"read_only"`
	Destructive          bool              `json:"destructive"`
	OpenWorld            bool              `json:"open_world"`
	ApprovalMode         string            `json:"approval_mode,omitempty"`
	Decision             string            `json:"decision,omitempty"`
	ReasonCodes          []ReasonCode      `json:"reason_codes,omitempty"`
	DurationMS           int64             `json:"duration_ms,omitempty"`
	ResultBytes          int64             `json:"result_bytes,omitempty"`
	Truncated            bool              `json:"truncated"`
	CorrelationID        string            `json:"correlation_id,omitempty"`
	ErrorCode            ErrorCode         `json:"error_code,omitempty"`
	InputDigest          string            `json:"input_digest,omitempty"`
	RedactedInputFields  []string          `json:"redacted_input_fields,omitempty"`
	ResultDigest         string            `json:"result_digest,omitempty"`
	ResultRedactionPaths []string          `json:"result_redaction_paths,omitempty"`
}

// ToolEventSink receives redacted dispatch events from the registry.
type ToolEventSink interface {
	EmitToolEvent(ctx context.Context, event ToolCallEvent) error
}

// ValidateProvider rejects nil or malformed providers before registry use.
func ValidateProvider(provider Provider) error {
	if isNilInterface(provider) {
		return NewValidationError("provider", ReasonDependencyMissing, "provider is required")
	}
	source := provider.ID()
	if err := source.Validate("provider.id"); err != nil {
		return err
	}
	return nil
}

// ValidateHandle rejects nil or malformed handles before dispatch.
func ValidateHandle(handle Handle) error {
	if isNilInterface(handle) {
		return NewValidationError("handle", ReasonBackendNotExecutable, "handle is required")
	}
	if err := handle.Descriptor().Validate(); err != nil {
		return err
	}
	return nil
}

func cloneRawMessage(src json.RawMessage) json.RawMessage {
	if len(src) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), src...)
}

func cloneStrings(src []string) []string {
	if len(src) == 0 {
		return nil
	}
	return append([]string(nil), src...)
}

func cloneToolsets(src []ToolsetID) []ToolsetID {
	if len(src) == 0 {
		return nil
	}
	return append([]ToolsetID(nil), src...)
}
