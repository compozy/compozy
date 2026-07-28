package config

import "time"

// AgentsConfig holds authored agent context settings.
type AgentsConfig struct {
	Soul      SoulConfig      `toml:"soul"`
	Heartbeat HeartbeatConfig `toml:"heartbeat"`
}

// SoulConfig controls optional SOUL.md parsing and projection limits.
type SoulConfig struct {
	Enabled                bool  `toml:"enabled"`
	MaxBodyBytes           int64 `toml:"max_body_bytes"`
	ContextProjectionBytes int64 `toml:"context_projection_bytes"`
}

const minSoulContextProjectionBytes int64 = 256

// HeartbeatConfig controls optional HEARTBEAT.md wake-policy parsing and runtime bounds.
type HeartbeatConfig struct {
	Enabled                      bool          `toml:"enabled"`
	MaxBodyBytes                 int64         `toml:"max_body_bytes"`
	ContextProjectionBytes       int64         `toml:"context_projection_bytes"`
	MinInterval                  time.Duration `toml:"min_interval"`
	DefaultInterval              time.Duration `toml:"default_interval"`
	WakeCooldown                 time.Duration `toml:"wake_cooldown"`
	MaxWakesPerCycle             int           `toml:"max_wakes_per_cycle"`
	ActiveSessionOnly            bool          `toml:"active_session_only"`
	AllowActiveHoursPreferences  bool          `toml:"allow_active_hours_preferences"`
	WakeEventRetention           time.Duration `toml:"wake_event_retention"`
	SessionHealthStaleAfter      time.Duration `toml:"session_health_stale_after"`
	SessionHealthHookMinInterval time.Duration `toml:"session_health_hook_min_interval"`
}

// LimitsConfig defines runtime safety bounds.
type LimitsConfig struct {
	MaxConcurrentAgents int `toml:"max_concurrent_agents"`
}

// SessionConfig defines session-scoped runtime controls.
type SessionConfig struct {
	Limits      SessionLimitsConfig      `toml:"limits"`
	Supervision SessionSupervisionConfig `toml:"supervision"`
	BusyInput   SessionBusyInputConfig   `toml:"busy_input"`
	Compaction  SessionCompactionConfig  `toml:"compaction"`
}

// SessionLimitsConfig defines runtime limits applied to every session.
type SessionLimitsConfig struct {
	Timeout time.Duration `toml:"timeout,omitempty"`
}

// SessionSupervisionConfig defines runtime activity monitoring controls applied to sessions.
type SessionSupervisionConfig struct {
	ActivityHeartbeatInterval time.Duration `toml:"activity_heartbeat_interval,omitempty"`
	ProgressNotifyInterval    time.Duration `toml:"progress_notify_interval,omitempty"`
	PromptDeadline            time.Duration `toml:"prompt_deadline,omitempty"`
	InactivityWarningAfter    time.Duration `toml:"inactivity_warning_after,omitempty"`
	InactivityTimeout         time.Duration `toml:"inactivity_timeout,omitempty"`
	TimeoutCancelGrace        time.Duration `toml:"timeout_cancel_grace,omitempty"`
}

// SessionBusyInputConfig controls operator input submitted while a turn is active.
type SessionBusyInputConfig struct {
	DefaultMode  string `toml:"default_mode,omitempty"`
	QueueCap     int    `toml:"queue_cap,omitempty"`
	MaxTextBytes int    `toml:"max_text_bytes,omitempty"`
}

// SessionCompactionConfig controls pressure-triggered persisted-context compaction.
type SessionCompactionConfig struct {
	Enabled            bool          `toml:"enabled"`
	PressureThreshold  float64       `toml:"pressure_threshold"`
	MaxAttemptsPerTurn int           `toml:"max_attempts_per_turn"`
	FailureCooldown    time.Duration `toml:"failure_cooldown"`
}

const (
	minSessionBusyInputQueueCap  = 1
	maxSessionBusyInputQueueCap  = 1000
	minSessionBusyInputTextBytes = 512
	maxSessionBusyInputTextBytes = 1 << 20
)

// PermissionMode is the static permission policy applied by the daemon.
type PermissionMode string

const (
	// DefaultAgentName is the bootstrap agent name used across the system.
	DefaultAgentName                          = "general"
	PermissionModeDenyAll      PermissionMode = "deny-all"
	PermissionModeApproveReads PermissionMode = "approve-reads"
	PermissionModeApproveAll   PermissionMode = "approve-all"
	// DefaultObservabilityAgentProbeTimeout bounds daemon health probes for configured agents.
	DefaultObservabilityAgentProbeTimeout = 2 * time.Second
)

// PermissionsConfig defines the global default permission policy.
type PermissionsConfig struct {
	Mode PermissionMode `toml:"mode"`
}

// ObservabilityConfig controls global event retention settings.
type ObservabilityConfig struct {
	Enabled           bool                          `toml:"enabled"`
	RetentionDays     int                           `toml:"retention_days"`
	MaxGlobalBytes    int64                         `toml:"max_global_bytes"`
	AgentProbeTimeout time.Duration                 `toml:"agent_probe_timeout"`
	Transcripts       ObservabilityTranscriptConfig `toml:"transcripts"`
}
