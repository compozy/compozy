package config

import (
	"time"

	"github.com/compozy/agh/internal/resources"
)

// SkillsConfig controls skill loading and discovery.
type SkillsConfig struct {
	Enabled                 bool              `toml:"enabled"`
	DisabledSkills          []string          `toml:"disabled_skills,omitempty"`
	PollInterval            time.Duration     `toml:"poll_interval"`
	AllowedMarketplaceMCP   []string          `toml:"allowed_marketplace_mcp,omitempty"`
	AllowedMarketplaceHooks []string          `toml:"allowed_marketplace_hooks,omitempty"`
	Marketplace             MarketplaceConfig `toml:"marketplace,omitempty"`
}

// ExtensionsConfig controls extension marketplace discovery and install behavior.
type ExtensionsConfig struct {
	Marketplace ExtensionsMarketplaceConfig `toml:"marketplace,omitempty"`
	Resources   ExtensionsResourcesConfig   `toml:"resources,omitempty"`
}

// ExtensionsResourcesConfig controls resource publication policy for extensions.
type ExtensionsResourcesConfig struct {
	AllowedKinds           []resources.ResourceKind          `toml:"allowed_kinds,omitempty"`
	MaxScope               resources.ResourceScopeKind       `toml:"max_scope,omitempty"`
	SnapshotRateLimit      ExtensionsResourceRateLimitConfig `toml:"snapshot_rate_limit,omitempty"`
	OperatorWriteRateLimit ExtensionsResourceRateLimitConfig `toml:"operator_write_rate_limit,omitempty"`
}

// ExtensionsResourceRateLimitConfig controls one resource publication rate-limit bucket.
type ExtensionsResourceRateLimitConfig struct {
	Requests int           `toml:"requests"`
	Window   time.Duration `toml:"window"`
	Queue    int           `toml:"queue"`
}

// SandboxProfile defines one reusable execution sandbox profile.
type SandboxProfile struct {
	Backend     string            `toml:"backend"`
	SyncMode    string            `toml:"sync_mode,omitempty"`
	Persistence string            `toml:"persistence,omitempty"`
	RuntimeRoot string            `toml:"runtime_root,omitempty"`
	Env         map[string]string `toml:"env,omitempty"`
	SecretEnv   map[string]string `toml:"secret_env,omitempty"`
	Network     NetworkProfile    `toml:"network,omitempty"`
	Daytona     DaytonaProfile    `toml:"daytona,omitempty"`
}

// NetworkProfile defines provider-neutral network policy intent.
type NetworkProfile struct {
	AllowPublicIngress bool     `toml:"allow_public_ingress,omitempty"`
	AllowOutbound      bool     `toml:"allow_outbound,omitempty"`
	AllowList          []string `toml:"allow_list,omitempty"`
	DenyList           []string `toml:"deny_list,omitempty"`
	Required           bool     `toml:"required,omitempty"`
}

// DaytonaProfile defines Daytona-specific execution sandbox settings.
type DaytonaProfile struct {
	APIURL      string `toml:"api_url,omitempty"`
	Target      string `toml:"target,omitempty"`
	Image       string `toml:"image,omitempty"`
	Snapshot    string `toml:"snapshot,omitempty"`
	Class       string `toml:"class,omitempty"`
	AutoStop    string `toml:"auto_stop,omitempty"`
	AutoArchive string `toml:"auto_archive,omitempty"`
}

// Config is the fully merged AGH configuration.
type Config struct {
	Daemon        DaemonConfig              `toml:"daemon"`
	HTTP          HTTPConfig                `toml:"http"`
	WindowManager WindowManagerConfig       `toml:"window_manager"`
	Defaults      DefaultsConfig            `toml:"defaults"`
	Agents        AgentsConfig              `toml:"agents"`
	Limits        LimitsConfig              `toml:"limits"`
	Session       SessionConfig             `toml:"session"`
	Permissions   PermissionsConfig         `toml:"permissions"`
	MCPServers    []MCPServer               `toml:"mcp_servers,omitempty"`
	Providers     map[string]ProviderConfig `toml:"providers"`
	ModelCatalog  ModelCatalogConfig        `toml:"model_catalog"`
	Marketplace   MarketplaceRuntimeConfig  `toml:"marketplace"`
	Sandboxes     map[string]SandboxProfile `toml:"sandboxes"`
	Observability ObservabilityConfig       `toml:"observability"`
	Log           LogConfig                 `toml:"log"`
	Redact        RedactConfig              `toml:"redact"`
	Memory        MemoryConfig              `toml:"memory"`
	Roles         RolesConfig               `toml:"roles"`
	RoleSources   RoleFieldSources          `toml:"-"                     json:"-"`
	Skills        SkillsConfig              `toml:"skills"`
	Extensions    ExtensionsConfig          `toml:"extensions"`
	Tools         ToolsConfig               `toml:"tools"`
	Automation    AutomationConfig          `toml:"automation"`
	Loops         LoopsConfig               `toml:"loops"`
	Goals         GoalsConfig               `toml:"goals"`
	Task          TaskConfig                `toml:"task"`
	Hooks         HooksConfig               `toml:"hooks"`
	Network       NetworkConfig             `toml:"network"`
	Autonomy      AutonomyConfig            `toml:"autonomy"`
}
