package config

import (
	"time"

	automationpkg "github.com/compozy/compozy/internal/automation/model"
)

// DefaultMCPClientMetadataURL is the public OAuth client identity used by MCP authorization.
const DefaultMCPClientMetadataURL = "https://compozy.com/.well-known/mcp-client.json"

// DefaultWithHome returns the built-in default configuration for the supplied Compozy home.
func defaultSessionConfig() SessionConfig {
	return SessionConfig{
		Limits:      SessionLimitsConfig{},
		Supervision: DefaultSessionSupervisionConfig(),
		BusyInput:   DefaultSessionBusyInputConfig(),
		Compaction:  DefaultSessionCompactionConfig(),
		Attachments: DefaultSessionAttachmentsConfig(),
	}
}

func DefaultWithHome(homePaths HomePaths) Config {
	return Config{
		Daemon:        defaultDaemonConfig(homePaths),
		HTTP:          HTTPConfig{Host: "localhost", Port: 2123},
		App:           defaultAppConfig(),
		WindowManager: DefaultWindowManagerConfig(),
		Terminal:      DefaultTerminalConfig(),
		CmdPalette:    DefaultCmdPaletteConfig(),
		Defaults: DefaultsConfig{
			Agent: DefaultAgentName,
		},
		Agents: AgentsConfig{
			Soul:      DefaultSoulConfig(),
			Heartbeat: DefaultHeartbeatConfig(),
		},
		Limits: LimitsConfig{
			MaxConcurrentAgents: 20,
		},
		Session: defaultSessionConfig(),
		Permissions: PermissionsConfig{
			Mode: PermissionModeApproveAll,
		},
		MCP: MCPConfig{OAuth: MCPOAuthConfig{
			ClientMetadataURL: DefaultMCPClientMetadataURL,
			RedirectURI:       "http://127.0.0.1:2123/api/mcp/oauth/callback",
		}},
		Providers:     map[string]ProviderConfig{},
		ModelCatalog:  DefaultModelCatalogConfig(),
		Marketplace:   DefaultMarketplaceRuntimeConfig(),
		Sandboxes:     map[string]SandboxProfile{},
		Observability: defaultObservabilityConfig(),
		Log:           defaultLogConfig(),
		Redact:        RedactConfig{Enabled: true},
		Memory:        DefaultMemoryConfig(homePaths),
		Shell:         DefaultShellConfig(),
		Attention:     DefaultAttentionConfig(),
		Roles:         DefaultRolesConfig(),
		RoleSources:   defaultRoleFieldSources(),
		Skills: SkillsConfig{
			Enabled:       true,
			Sources:       []string{SkillSourceAgents},
			CustomSources: []string{},
			PollInterval:  3 * time.Second,
		},
		Extensions: defaultExtensionsConfig(),
		Tools:      DefaultToolsConfig(),
		Automation: AutomationConfig{
			Enabled:           true,
			Timezone:          automationpkg.DefaultTimezone,
			MaxConcurrentJobs: automationpkg.DefaultMaxConcurrentJobs,
			DefaultFireLimit:  automationpkg.DefaultFireLimitConfig(),
			Suggestions: AutomationSuggestionsConfig{
				PendingCap: automationpkg.DefaultSuggestionPendingCap,
			},
		},
		Loops:   DefaultLoopsConfig(),
		Goals:   DefaultGoalsConfig(),
		Task:    DefaultTaskConfig(),
		Network: DefaultNetworkConfig(),
		Gateway: defaultGatewayConfig(homePaths),
		Autonomy: AutonomyConfig{
			BlockRecurrenceLimit: DefaultBlockRecurrenceLimit,
			Scheduler:            DefaultSchedulerConfig(),
		},
		Worktrees: DefaultWorktreesConfig(),
	}
}

func defaultObservabilityConfig() ObservabilityConfig {
	return ObservabilityConfig{
		Enabled:           true,
		RetentionDays:     7,
		MaxGlobalBytes:    1 << 30,
		AgentProbeTimeout: DefaultObservabilityAgentProbeTimeout,
		Transcripts: ObservabilityTranscriptConfig{
			Enabled: true, SegmentBytes: 1 << 20, MaxBytesPerSession: 256 << 20,
		},
	}
}

func defaultLogConfig() LogConfig {
	return LogConfig{
		Level: configLogLevelInfo, MaxSizeMB: 10, MaxBackups: 5, MaxAgeDays: 30, CompressBackups: false,
	}
}
