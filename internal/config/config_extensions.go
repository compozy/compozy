package config

import (
	"time"

	"github.com/compozy/compozy/internal/resources"
)

// SkillsConfig controls skill loading and discovery.
type SkillsConfig struct {
	Enabled                 bool          `toml:"enabled"`
	Sources                 []string      `toml:"sources,omitempty"`
	CustomSources           []string      `toml:"custom_sources,omitempty"`
	DisabledSkills          []string      `toml:"disabled_skills,omitempty"`
	PollInterval            time.Duration `toml:"poll_interval"`
	AllowedMarketplaceHooks []string      `toml:"allowed_marketplace_hooks,omitempty"`
}

// ExtensionsConfig controls extension trust, sources, development, and resource policy.
type ExtensionsConfig struct {
	Trust     ExtensionsTrustConfig     `toml:"trust"`
	Sources   ExtensionsSourcesConfig   `toml:"sources"`
	Dev       ExtensionsDevConfig       `toml:"dev"`
	Resources ExtensionsResourcesConfig `toml:"resources,omitempty"`
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

// Config is the fully merged Compozy configuration.
type Config struct {
	Daemon        DaemonConfig              `toml:"daemon"`
	HTTP          HTTPConfig                `toml:"http"`
	App           AppConfig                 `toml:"app"`
	Shell         ShellConfig               `toml:"shell"`
	Attention     AttentionConfig           `toml:"attention"`
	WindowManager WindowManagerConfig       `toml:"window_manager"`
	Terminal      TerminalConfig            `toml:"terminal"`
	CmdPalette    CmdPaletteConfig          `toml:"cmd_palette"`
	Defaults      DefaultsConfig            `toml:"defaults"`
	Agents        AgentsConfig              `toml:"agents"`
	Limits        LimitsConfig              `toml:"limits"`
	Session       SessionConfig             `toml:"session"`
	Permissions   PermissionsConfig         `toml:"permissions"`
	MCP           MCPConfig                 `toml:"mcp"`
	MCPServers    []MCPServer               `toml:"mcp_servers,omitempty"`
	Providers     map[string]ProviderConfig `toml:"providers"`
	ModelCatalog  ModelCatalogConfig        `toml:"model_catalog"`
	Marketplace   MarketplaceRuntimeConfig  `toml:"marketplace"`
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
	Gateway       GatewayConfig             `toml:"gateway"`
	Autonomy      AutonomyConfig            `toml:"autonomy"`
	Worktrees     WorktreesConfig           `toml:"worktrees"`
}
