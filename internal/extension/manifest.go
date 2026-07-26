package extensionpkg

import (
	"encoding/json"
	"errors"

	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges"

	"github.com/compozy/agh/internal/resources"
)

const (
	manifestMustBeASemanticVersionValue = "must be a semantic version"
	manifestNameKey                     = "name"
	manifestNullKey                     = "null"
	manifestResourcesPublishPath        = "resources.publish"
)

const (
	manifestTOMLFileName = "extension.toml"
	manifestJSONFileName = "extension.json"
)

var (
	// ErrManifestNotFound reports that an extension directory does not contain
	// either supported manifest file.
	ErrManifestNotFound = errors.New("extension: manifest not found")
	// ErrManifestInvalid reports that the manifest schema or content is invalid.
	ErrManifestInvalid = errors.New("extension: invalid manifest")
	// ErrManifestIncompatible reports that the manifest requires a newer daemon
	// version than the current build provides.
	ErrManifestIncompatible = errors.New("extension: incompatible manifest")
)

// Manifest describes one extension without executing any extension code.
type Manifest struct {
	Name                 string                      `toml:"name"                            json:"name"`
	Version              string                      `toml:"version"                         json:"version"`
	Description          string                      `toml:"description,omitempty"           json:"description,omitempty"`
	MinAGHVersion        string                      `toml:"min_agh_version"                 json:"min_agh_version"`
	RequiresEnv          []string                    `toml:"requires_env,omitempty"          json:"requires_env,omitempty"`
	NetworkParticipation *manifestNetworkRequirement `toml:"network_participation,omitempty" json:"network_participation,omitempty"`
	Resources            ResourcesConfig             `toml:"resources"                       json:"resources"`
	Capabilities         CapabilitiesConfig          `toml:"capabilities"                    json:"capabilities"`
	Actions              ActionsConfig               `toml:"actions"                         json:"actions"`
	Subprocess           SubprocessConfig            `toml:"subprocess"                      json:"subprocess"`
	Security             SecurityConfig              `toml:"security"                        json:"security"`
	Bridge               BridgeConfig                `toml:"bridge"                          json:"bridge"`
}

// ResourcesConfig declares static assets bundled with an extension.
type ResourcesConfig struct {
	Skills     []string                   `toml:"skills,omitempty"      json:"skills,omitempty"`
	Loops      []string                   `toml:"loops,omitempty"       json:"loops,omitempty"`
	Agents     []string                   `toml:"agents,omitempty"      json:"agents,omitempty"`
	Bundles    []string                   `toml:"bundles,omitempty"     json:"bundles,omitempty"`
	Hooks      []HookConfig               `toml:"hooks,omitempty"       json:"hooks,omitempty"`
	Tools      map[string]ToolConfig      `toml:"tools,omitempty"       json:"tools,omitempty"`
	MCPServers map[string]MCPServerConfig `toml:"mcp_servers,omitempty" json:"mcp_servers,omitempty"`
	Publish    ResourceGrantRequest       `toml:"publish,omitempty"     json:"publish"`
}

// ResourceGrantRequest declares the resource families and scope ceiling an extension requests.
type ResourceGrantRequest struct {
	Families []string                    `toml:"families,omitempty"  json:"families,omitempty"`
	MaxScope resources.ResourceScopeKind `toml:"max_scope,omitempty" json:"max_scope,omitempty"`
}

// CapabilitiesConfig declares the runtime interfaces the extension provides.
type CapabilitiesConfig struct {
	Provides []string `toml:"provides,omitempty" json:"provides,omitempty"`
}

// ActionsConfig declares Host API methods the extension wants to call.
type ActionsConfig struct {
	Requires []string `toml:"requires,omitempty" json:"requires,omitempty"`
}

// SubprocessConfig describes how to launch and monitor the extension process.
type SubprocessConfig struct {
	Command             string            `toml:"command,omitempty"               json:"command,omitempty"`
	Args                []string          `toml:"args,omitempty"                  json:"args,omitempty"`
	Env                 map[string]string `toml:"env,omitempty"                   json:"env,omitempty"`
	SecretEnv           map[string]string `toml:"secret_env,omitempty"            json:"secret_env,omitempty"`
	HealthCheckInterval Duration          `toml:"health_check_interval,omitempty" json:"health_check_interval,omitempty"`
	ShutdownTimeout     Duration          `toml:"shutdown_timeout,omitempty"      json:"shutdown_timeout,omitempty"`
}

// SecurityConfig declares the security grants the extension requests.
type SecurityConfig struct {
	Capabilities []string `toml:"capabilities,omitempty" json:"capabilities,omitempty"`
}

// BridgeConfig declares provider metadata for bridge-capable extensions.
type BridgeConfig struct {
	Platform     string                                `toml:"platform,omitempty"      json:"platform,omitempty"`
	DisplayName  string                                `toml:"display_name,omitempty"  json:"display_name,omitempty"`
	SecretSlots  []bridgepkg.BridgeSecretSlot          `toml:"secret_slots,omitempty"  json:"secret_slots,omitempty"`
	ConfigSchema *bridgepkg.BridgeProviderConfigSchema `toml:"config_schema,omitempty" json:"config_schema,omitempty"`
}

// HookConfig mirrors the hook declaration shape accepted from extension manifests.
type HookConfig struct {
	Name      string             `toml:"name"                 json:"name"`
	Event     string             `toml:"event"                json:"event"`
	Mode      string             `toml:"mode,omitempty"       json:"mode,omitempty"`
	Required  bool               `toml:"required,omitempty"   json:"required,omitempty"`
	Priority  *int               `toml:"priority,omitempty"   json:"priority,omitempty"`
	Timeout   Duration           `toml:"timeout,omitempty"    json:"timeout,omitempty"`
	Matcher   HookMatcherConfig  `toml:"matcher,omitempty"    json:"matcher"`
	Command   string             `toml:"command,omitempty"    json:"command,omitempty"`
	Args      []string           `toml:"args,omitempty"       json:"args,omitempty"`
	Env       map[string]string  `toml:"env,omitempty"        json:"env,omitempty"`
	SecretEnv map[string]string  `toml:"secret_env,omitempty" json:"secret_env,omitempty"`
	Executor  HookExecutorConfig `toml:"executor,omitempty"   json:"executor"`
}

// HookExecutorConfig selects the hook execution boundary and command.
type HookExecutorConfig struct {
	Kind      string            `toml:"kind,omitempty"       json:"kind,omitempty"`
	Command   string            `toml:"command,omitempty"    json:"command,omitempty"`
	Args      []string          `toml:"args,omitempty"       json:"args,omitempty"`
	Env       map[string]string `toml:"env,omitempty"        json:"env,omitempty"`
	SecretEnv map[string]string `toml:"secret_env,omitempty" json:"secret_env,omitempty"`
}

// HookMatcherConfig narrows when a hook is eligible to run.
type HookMatcherConfig struct {
	AgentName          string `toml:"agent_name,omitempty"          json:"agent_name,omitempty"`
	AgentType          string `toml:"agent_type,omitempty"          json:"agent_type,omitempty"`
	WorkspaceID        string `toml:"workspace_id,omitempty"        json:"workspace_id,omitempty"`
	WorkspaceRoot      string `toml:"workspace_root,omitempty"      json:"workspace_root,omitempty"`
	SessionType        string `toml:"session_type,omitempty"        json:"session_type,omitempty"`
	InputClass         string `toml:"input_class,omitempty"         json:"input_class,omitempty"`
	ACPEventType       string `toml:"acp_event_type,omitempty"      json:"acp_event_type,omitempty"`
	TurnID             string `toml:"turn_id,omitempty"             json:"turn_id,omitempty"`
	ToolID             string `toml:"tool_id,omitempty"             json:"tool_id,omitempty"`
	ToolName           string `toml:"tool_name,omitempty"           json:"tool_name,omitempty"`
	ToolReadOnly       *bool  `toml:"tool_read_only,omitempty"      json:"tool_read_only,omitempty"`
	DecisionClass      string `toml:"decision_class,omitempty"      json:"decision_class,omitempty"`
	MessageRole        string `toml:"message_role,omitempty"        json:"message_role,omitempty"`
	MessageDeltaType   string `toml:"message_delta_type,omitempty"  json:"message_delta_type,omitempty"`
	Channel            string `toml:"channel,omitempty"             json:"channel,omitempty"`
	Surface            string `toml:"surface,omitempty"             json:"surface,omitempty"`
	Kind               string `toml:"kind,omitempty"                json:"kind,omitempty"`
	Direction          string `toml:"direction,omitempty"           json:"direction,omitempty"`
	WorkState          string `toml:"work_state,omitempty"          json:"work_state,omitempty"`
	CompactionReason   string `toml:"compaction_reason,omitempty"   json:"compaction_reason,omitempty"`
	CompactionStrategy string `toml:"compaction_strategy,omitempty" json:"compaction_strategy,omitempty"`
}

// MCPServerConfig declares one MCP server bundled by the extension.
type MCPServerConfig struct {
	Command   string            `toml:"command"              json:"command"`
	Args      []string          `toml:"args,omitempty"       json:"args,omitempty"`
	Env       map[string]string `toml:"env,omitempty"        json:"env,omitempty"`
	SecretEnv map[string]string `toml:"secret_env,omitempty" json:"secret_env,omitempty"`
}

// ToolConfig declares one static tool bundled by the extension.
type ToolConfig struct {
	ID                   string            `toml:"id,omitempty"                    json:"id,omitempty"`
	DisplayTitle         string            `toml:"display_title,omitempty"         json:"display_title,omitempty"`
	FriendlyVerb         string            `toml:"friendly_verb,omitempty"         json:"friendly_verb,omitempty"`
	Preview              string            `toml:"preview,omitempty"               json:"preview,omitempty"`
	Description          string            `toml:"description,omitempty"           json:"description,omitempty"`
	Handler              string            `toml:"handler,omitempty"               json:"handler,omitempty"`
	Backend              ToolBackendConfig `toml:"backend,omitempty"               json:"backend"`
	InputSchema          json.RawMessage   `toml:"input_schema,omitempty"          json:"input_schema,omitempty"`
	OutputSchema         json.RawMessage   `toml:"output_schema,omitempty"         json:"output_schema,omitempty"`
	Risk                 string            `toml:"risk,omitempty"                  json:"risk,omitempty"`
	ReadOnly             bool              `toml:"read_only,omitempty"             json:"read_only,omitempty"`
	Destructive          bool              `toml:"destructive,omitempty"           json:"destructive,omitempty"`
	OpenWorld            bool              `toml:"open_world,omitempty"            json:"open_world,omitempty"`
	RequiresInteraction  bool              `toml:"requires_interaction,omitempty"  json:"requires_interaction,omitempty"`
	ConcurrencySafe      bool              `toml:"concurrency_safe,omitempty"      json:"concurrency_safe,omitempty"`
	MaxResultBytes       int64             `toml:"max_result_bytes,omitempty"      json:"max_result_bytes,omitempty"`
	Toolsets             []string          `toml:"toolsets,omitempty"              json:"toolsets,omitempty"`
	Tags                 []string          `toml:"tags,omitempty"                  json:"tags,omitempty"`
	SearchHints          []string          `toml:"search_hints,omitempty"          json:"search_hints,omitempty"`
	RequiresEnv          []string          `toml:"requires_env,omitempty"          json:"requires_env,omitempty"`
	RequiredCapabilities []string          `toml:"required_capabilities,omitempty" json:"required_capabilities,omitempty"`
	Visibility           string            `toml:"visibility,omitempty"            json:"visibility,omitempty"`
}

// ToolBackendConfig binds a manifest tool to its backend metadata.
type ToolBackendConfig struct {
	Kind    string `toml:"kind,omitempty"    json:"kind,omitempty"`
	Handler string `toml:"handler,omitempty" json:"handler,omitempty"`
	Server  string `toml:"server,omitempty"  json:"server,omitempty"`
	Tool    string `toml:"tool,omitempty"    json:"tool,omitempty"`
}

// Duration stores time.Duration values while decoding TOML strings and JSON
// strings consistently.
type Duration time.Duration

type manifestDocument struct {
	Extension            manifestCore                `toml:"extension"                       json:"extension"`
	Name                 string                      `toml:"name"                            json:"name"`
	Version              string                      `toml:"version"                         json:"version"`
	Description          string                      `toml:"description,omitempty"           json:"description,omitempty"`
	MinAGHVersion        string                      `toml:"min_agh_version"                 json:"min_agh_version"`
	RequiresEnv          []string                    `toml:"requires_env,omitempty"          json:"requires_env,omitempty"`
	NetworkParticipation *manifestNetworkRequirement `toml:"network_participation,omitempty" json:"network_participation,omitempty"`
	Resources            ResourcesConfig             `toml:"resources"                       json:"resources"`
	Capabilities         CapabilitiesConfig          `toml:"capabilities"                    json:"capabilities"`
	Actions              ActionsConfig               `toml:"actions"                         json:"actions"`
	Subprocess           SubprocessConfig            `toml:"subprocess"                      json:"subprocess"`
	Security             SecurityConfig              `toml:"security"                        json:"security"`
	Bridge               BridgeConfig                `toml:"bridge"                          json:"bridge"`
}

type manifestCore struct {
	Name          string   `toml:"name"                   json:"name"`
	Version       string   `toml:"version"                json:"version"`
	Description   string   `toml:"description,omitempty"  json:"description,omitempty"`
	MinAGHVersion string   `toml:"min_agh_version"        json:"min_agh_version"`
	RequiresEnv   []string `toml:"requires_env,omitempty" json:"requires_env,omitempty"`
}
