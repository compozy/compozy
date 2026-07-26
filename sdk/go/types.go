package aghsdk

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

const (
	typesToolIDKey = "tool_id"
)

// SDKName is advertised during extension initialization.
const SDKName = "github.com/compozy/agh/sdk/go"

// SDKVersion is the public SDK protocol implementation version.
const SDKVersion = "0.1.0"

// ProtocolVersion is the AGH extension subprocess protocol version.
const ProtocolVersion = "1"

// CapabilityToolProvider is the provide surface for executable extension-host tools.
const CapabilityToolProvider = "tool.provider"

// CapabilityProvideWatchSource is the provide surface for loop watch-source polling.
const CapabilityProvideWatchSource = "loop.watch_source"

// ExtensionServiceMethodProvideTools is the runtime descriptor service method.
const ExtensionServiceMethodProvideTools = "provide_tools"

// ExtensionServiceMethodToolsCall is the tool invocation service method.
const ExtensionServiceMethodToolsCall = "tools/call"

// ExtensionServiceMethodWatchPoll is the watch-source poll service method.
const ExtensionServiceMethodWatchPoll = "watch/poll"

const (
	initializeMethod  = "initialize"
	healthCheckMethod = "health_check"
	shutdownMethod    = "shutdown"
)

var segmentedIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(?:__[a-z][a-z0-9_]*)*$`)

// ToolID is the canonical public tool identity.
type ToolID string

// Validate ensures the tool id follows the canonical AGH grammar.
func (id ToolID) Validate() error {
	value := string(id)
	switch {
	case value == "":
		return NewInvalidParamsError("tool_id is required", nil)
	case len(value) > 64:
		return NewInvalidParamsError("tool_id exceeds 64 characters", map[string]any{typesToolIDKey: value})
	case strings.Contains(value, "___"):
		return NewInvalidParamsError("tool_id uses ambiguous reserved separator", map[string]any{typesToolIDKey: value})
	case !segmentedIDPattern.MatchString(value):
		return NewInvalidParamsError(
			"tool_id must use lowercase __-separated segments",
			map[string]any{typesToolIDKey: value},
		)
	default:
		for segment := range strings.SplitSeq(value, "__") {
			if segment == "" {
				return NewInvalidParamsError(
					"tool_id contains an empty segment",
					map[string]any{typesToolIDKey: value},
				)
			}
			if strings.HasPrefix(segment, "_") || strings.HasSuffix(segment, "_") {
				return NewInvalidParamsError(
					"tool_id segment uses reserved underscore boundary",
					map[string]any{typesToolIDKey: value},
				)
			}
		}
		return nil
	}
}

// RiskClass classifies tool execution safety.
type RiskClass string

const (
	// RiskRead marks read-only behavior.
	RiskRead RiskClass = "read"
	// RiskMutating marks state-changing behavior.
	RiskMutating RiskClass = "mutating"
	// RiskDestructive marks destructive state-changing behavior.
	RiskDestructive RiskClass = "destructive"
)

// ExtensionDefinition describes one extension process implementation.
type ExtensionDefinition struct {
	Name                string             `json:"name"`
	Version             string             `json:"version"`
	Description         string             `json:"description,omitempty"`
	MinAGHVersion       string             `json:"min_agh_version,omitempty"`
	Capabilities        CapabilitiesConfig `json:"capabilities"`
	Actions             ActionsConfig      `json:"actions"`
	Security            SecurityConfig     `json:"security"`
	SupportedHookEvents []string           `json:"supported_hook_events,omitempty"`
	Metadata            map[string]string  `json:"metadata,omitempty"`
}

// CapabilitiesConfig lists extension-provided capability surfaces.
type CapabilitiesConfig struct {
	Provides []string `json:"provides,omitempty"`
}

// ActionsConfig lists required Host API methods.
type ActionsConfig struct {
	Requires []HostAPIMethod `json:"requires,omitempty"`
}

// SecurityConfig lists required security grants.
type SecurityConfig struct {
	Capabilities []string `json:"capabilities,omitempty"`
}

// InitializeRequest is the AGH -> extension session contract request.
type InitializeRequest struct {
	ProtocolVersion           string                 `json:"protocol_version"`
	SupportedProtocolVersions []string               `json:"supported_protocol_versions"`
	AGHVersion                string                 `json:"agh_version"`
	SessionNonce              string                 `json:"session_nonce"`
	Extension                 InitializeExtension    `json:"extension"`
	Capabilities              InitializeCapabilities `json:"capabilities"`
	Methods                   InitializeMethods      `json:"methods"`
	Runtime                   InitializeRuntime      `json:"runtime"`
}

// InitializeExtension identifies the launched extension.
type InitializeExtension struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	SourceTier string `json:"source_tier"`
}

// InitializeCapabilities carries runtime-granted capabilities.
type InitializeCapabilities struct {
	Provides              []string        `json:"provides"`
	GrantedActions        []HostAPIMethod `json:"granted_actions"`
	GrantedSecurity       []string        `json:"granted_security"`
	GrantedResourceKinds  []string        `json:"granted_resource_kinds"`
	GrantedResourceScopes []string        `json:"granted_resource_scopes"`
}

// InitializeMethods lists callable method families for the session.
type InitializeMethods struct {
	DaemonRequests    []string `json:"daemon_requests"`
	ExtensionServices []string `json:"extension_services"`
}

// InitializeRuntime carries runtime intervals and deadlines.
type InitializeRuntime struct {
	HealthCheckIntervalMS int64           `json:"health_check_interval_ms"`
	HealthCheckTimeoutMS  int64           `json:"health_check_timeout_ms"`
	ShutdownTimeoutMS     int64           `json:"shutdown_timeout_ms"`
	DefaultHookTimeoutMS  int64           `json:"default_hook_timeout_ms"`
	Bridge                json.RawMessage `json:"bridge,omitempty"`
}

// InitializeResponse is the extension -> AGH initialize acknowledgment.
type InitializeResponse struct {
	ProtocolVersion      string                  `json:"protocol_version"`
	ExtensionInfo        InitializeExtensionInfo `json:"extension_info"`
	AcceptedCapabilities AcceptedCapabilities    `json:"accepted_capabilities"`
	ImplementedMethods   []string                `json:"implemented_methods"`
	SupportedHookEvents  []string                `json:"supported_hook_events"`
	WatchSourceKinds     []string                `json:"watch_source_kinds,omitempty"`
	Supports             InitializeSupports      `json:"supports"`
}

// InitializeExtensionInfo identifies the running extension implementation.
type InitializeExtensionInfo struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	SDKName    string `json:"sdk_name,omitempty"`
	SDKVersion string `json:"sdk_version,omitempty"`
}

// AcceptedCapabilities is the subset the extension accepted for this session.
type AcceptedCapabilities struct {
	Provides []string        `json:"provides"`
	Actions  []HostAPIMethod `json:"actions"`
	Security []string        `json:"security"`
}

// InitializeSupports advertises optional protocol features.
type InitializeSupports struct {
	HealthCheck bool `json:"health_check"`
}

// ExtensionSession is the accepted initialization state visible to handlers.
type ExtensionSession struct {
	InitializeRequest    InitializeRequest
	InitializeResponse   InitializeResponse
	Runtime              InitializeRuntime
	AcceptedCapabilities AcceptedCapabilities
}

// HealthCheckResult is returned by health_check handlers.
type HealthCheckResult struct {
	Healthy bool                       `json:"healthy"`
	Message string                     `json:"message,omitempty"`
	Details map[string]json.RawMessage `json:"details,omitempty"`
}

// ShutdownRequest is sent before signal escalation.
type ShutdownRequest struct {
	Reason     string `json:"reason"`
	DeadlineMS int64  `json:"deadline_ms"`
}

// ShutdownResponse acknowledges cooperative shutdown.
type ShutdownResponse struct {
	Acknowledged bool `json:"acknowledged"`
}

// ToolOptions configures one function-backed extension tool.
type ToolOptions struct {
	ID                   ToolID
	Description          string
	InputSchema          any
	OutputSchema         any
	ReadOnly             bool
	Risk                 RiskClass
	Capabilities         []string
	SensitiveInputFields []string
}

// ExtensionToolRuntimeDescriptor is the runtime proof descriptor returned by provide_tools.
type ExtensionToolRuntimeDescriptor struct {
	ID                 ToolID    `json:"id"`
	Handler            string    `json:"handler"`
	InputSchemaDigest  string    `json:"input_schema_digest"`
	OutputSchemaDigest string    `json:"output_schema_digest,omitempty"`
	ReadOnly           bool      `json:"read_only"`
	Risk               RiskClass `json:"risk"`
	Capabilities       []string  `json:"capabilities,omitempty"`
}

// ExtensionProvideToolsResponse is returned by provide_tools.
type ExtensionProvideToolsResponse struct {
	Tools []ExtensionToolRuntimeDescriptor `json:"tools"`
}

// ExtensionToolCallRequest is sent by AGH for tools/call.
type ExtensionToolCallRequest struct {
	ToolID       ToolID          `json:"tool_id"`
	Handler      string          `json:"handler"`
	SessionID    string          `json:"session_id,omitempty"`
	InvocationID string          `json:"invocation_id,omitempty"`
	Input        json.RawMessage `json:"input"`
}

// ClarifyQuestion is one bounded question authored by an extension-host tool.
type ClarifyQuestion struct {
	Question string   `json:"question"`
	Choices  []string `json:"choices,omitempty"`
}

// ClarifyAnswer is the exact operator answer returned to the blocked tool call.
type ClarifyAnswer struct {
	Choice   *int   `json:"choice"`
	Text     string `json:"text"`
	Fallback bool   `json:"fallback"`
}

// ExtensionToolCallResponse wraps a tool result.
type ExtensionToolCallResponse struct {
	Result ToolResult `json:"result"`
}

// WatchSourceOptions reserves future watch-source registration options.
type WatchSourceOptions struct{}

// WatchPollRequest is sent by AGH for watch/poll.
type WatchPollRequest struct {
	Spec                json.RawMessage `json:"spec"`
	ExpectedStateDigest string          `json:"expected_state_digest,omitempty"`
}

// WatchPollResponse is returned by a watch-source handler.
type WatchPollResponse struct {
	Ready       bool            `json:"ready"`
	StateDigest string          `json:"state_digest,omitempty"`
	Payload     json.RawMessage `json:"payload,omitempty"`
	SettledAt   *time.Time      `json:"settled_at,omitempty"`
}

// WatchSourceRequest is passed to a typed watch-source handler.
type WatchSourceRequest[TSpec any] struct {
	Kind                string
	Spec                TSpec
	RawSpec             json.RawMessage
	ExpectedStateDigest string
	Context             ExtensionContext
	Host                *HostAPI
	Session             ExtensionSession
}

// WatchSourceHandlerFunc handles one typed watch-source poll.
type WatchSourceHandlerFunc[TSpec any] func(context.Context, WatchSourceRequest[TSpec]) (WatchPollResponse, error)

// ToolContent is one typed content block returned by a tool.
type ToolContent struct {
	Type     string                     `json:"type"`
	Text     string                     `json:"text,omitempty"`
	Data     json.RawMessage            `json:"data,omitempty"`
	MIMEType string                     `json:"mime_type,omitempty"`
	Metadata map[string]json.RawMessage `json:"metadata,omitempty"`
}

// ArtifactRef points to a durable tool output artifact.
type ArtifactRef struct {
	URI      string `json:"uri"`
	Name     string `json:"name,omitempty"`
	MIMEType string `json:"mime_type,omitempty"`
	Bytes    int64  `json:"bytes,omitempty"`
}

// Redaction records a redaction applied to a result.
type Redaction struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
	Bytes  int64  `json:"bytes,omitempty"`
}

// ToolResult is the canonical result envelope for tool handlers.
type ToolResult struct {
	Content    []ToolContent              `json:"content,omitempty"`
	Structured json.RawMessage            `json:"structured,omitempty"`
	Preview    string                     `json:"preview,omitempty"`
	Artifacts  []ArtifactRef              `json:"artifacts,omitempty"`
	Metadata   map[string]json.RawMessage `json:"metadata,omitempty"`
	Redactions []Redaction                `json:"redactions,omitempty"`
	Truncated  bool                       `json:"truncated"`
	Bytes      int64                      `json:"bytes"`
	DurationMS int64                      `json:"duration_ms"`
}

func (d ExtensionDefinition) validate() error {
	if strings.TrimSpace(d.Name) == "" {
		return NewInvalidParamsError("extension name is required", nil)
	}
	if strings.TrimSpace(d.Version) == "" {
		return NewInvalidParamsError("extension version is required", nil)
	}
	return nil
}

func normalizeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		clean := strings.TrimSpace(value)
		if clean == "" {
			continue
		}
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		normalized = append(normalized, clean)
	}
	slices.Sort(normalized)
	return normalized
}

func ensureSubset(label string, requested []string, granted []string) error {
	grantedSet := make(map[string]struct{}, len(granted))
	for _, value := range granted {
		grantedSet[strings.TrimSpace(value)] = struct{}{}
	}
	var missing []string
	for _, value := range requested {
		if _, ok := grantedSet[value]; !ok {
			missing = append(missing, value)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return NewCapabilityDeniedError(map[string]any{
		"field":    label,
		"required": missing,
		"granted":  granted,
	})
}

func validateProvidedMethodCoverage(provides []string, implemented []string) error {
	implementedSet := make(map[string]struct{}, len(implemented))
	for _, method := range implemented {
		implementedSet[method] = struct{}{}
	}
	requiredByCapability := map[string][]string{
		"bridge.adapter":             {"bridges/deliver"},
		"memory.backend":             {"memory/store", "memory/recall", "memory/forget"},
		CapabilityToolProvider:       {ExtensionServiceMethodProvideTools, ExtensionServiceMethodToolsCall},
		CapabilityProvideWatchSource: {ExtensionServiceMethodWatchPoll},
	}
	for _, capability := range provides {
		for _, method := range requiredByCapability[capability] {
			if _, ok := implementedSet[method]; !ok {
				return NewInternalError(fmt.Sprintf("capability %s requires method %s", capability, method))
			}
		}
	}
	return nil
}
