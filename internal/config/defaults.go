package config

import (
	"time"

	automationpkg "github.com/compozy/agh/internal/automation/model"
)

// DefaultWithHome returns the built-in default configuration for the supplied AGH home.
func DefaultWithHome(homePaths HomePaths) Config {
	return Config{
		Daemon: defaultDaemonConfig(homePaths),
		HTTP: HTTPConfig{
			Host: "localhost",
			Port: 2123,
		},
		WindowManager: DefaultWindowManagerConfig(),
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
		Session: SessionConfig{
			Limits:      SessionLimitsConfig{},
			Supervision: DefaultSessionSupervisionConfig(),
			BusyInput:   DefaultSessionBusyInputConfig(),
			Compaction:  DefaultSessionCompactionConfig(),
		},
		Permissions: PermissionsConfig{
			Mode: PermissionModeApproveAll,
		},
		Providers:    map[string]ProviderConfig{},
		ModelCatalog: DefaultModelCatalogConfig(),
		Marketplace:  DefaultMarketplaceRuntimeConfig(),
		Sandboxes:    map[string]SandboxProfile{},
		Observability: ObservabilityConfig{
			Enabled:           true,
			RetentionDays:     7,
			MaxGlobalBytes:    1 << 30,
			AgentProbeTimeout: DefaultObservabilityAgentProbeTimeout,
			Transcripts: ObservabilityTranscriptConfig{
				Enabled:            true,
				SegmentBytes:       1 << 20,
				MaxBytesPerSession: 256 << 20,
			},
		},
		Log: LogConfig{
			Level:           configLogLevelInfo,
			MaxSizeMB:       10,
			MaxBackups:      5,
			MaxAgeDays:      30,
			CompressBackups: false,
		},
		Redact:      RedactConfig{Enabled: true},
		Memory:      DefaultMemoryConfig(homePaths),
		Roles:       DefaultRolesConfig(),
		RoleSources: defaultRoleFieldSources(),
		Skills: SkillsConfig{
			Enabled:      true,
			PollInterval: 3 * time.Second,
		},
		Extensions: ExtensionsConfig{},
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
		Autonomy: AutonomyConfig{
			BlockRecurrenceLimit: DefaultBlockRecurrenceLimit,
			Scheduler:            DefaultSchedulerConfig(),
		},
	}
}
