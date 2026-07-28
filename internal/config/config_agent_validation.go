package config

import (
	"errors"
	"fmt"

	"math"

	"strings"
	"time"
)

// DefaultSoulConfig returns the built-in Agent Soul resolver limits.
func DefaultSoulConfig() SoulConfig {
	return SoulConfig{
		Enabled:                true,
		MaxBodyBytes:           32768,
		ContextProjectionBytes: 2048,
	}
}

// DefaultHeartbeatConfig returns built-in Agent Heartbeat wake-policy limits.
func DefaultHeartbeatConfig() HeartbeatConfig {
	return HeartbeatConfig{
		Enabled:                      true,
		MaxBodyBytes:                 32768,
		ContextProjectionBytes:       4096,
		MinInterval:                  5 * time.Minute,
		DefaultInterval:              30 * time.Minute,
		WakeCooldown:                 time.Minute,
		MaxWakesPerCycle:             25,
		ActiveSessionOnly:            true,
		AllowActiveHoursPreferences:  true,
		WakeEventRetention:           168 * time.Hour,
		SessionHealthStaleAfter:      2 * time.Minute,
		SessionHealthHookMinInterval: time.Minute,
	}
}

// Validate ensures authored agent context settings are internally consistent.
func (c AgentsConfig) Validate() error {
	if err := c.Soul.Validate(); err != nil {
		return err
	}
	return c.Heartbeat.Validate()
}

// Validate ensures SOUL.md limits are internally consistent.
func (c SoulConfig) Validate() error {
	switch {
	case c.MaxBodyBytes <= 0:
		return fmt.Errorf("agents.soul.max_body_bytes must be positive: %d", c.MaxBodyBytes)
	case c.ContextProjectionBytes <= 0:
		return fmt.Errorf(
			"agents.soul.context_projection_bytes must be positive: %d",
			c.ContextProjectionBytes,
		)
	case c.ContextProjectionBytes > c.MaxBodyBytes:
		return fmt.Errorf(
			"agents.soul.context_projection_bytes must be <= agents.soul.max_body_bytes: %d > %d",
			c.ContextProjectionBytes,
			c.MaxBodyBytes,
		)
	case c.ContextProjectionBytes < minSoulContextProjectionBytes:
		if !c.Enabled {
			return nil
		}
		return fmt.Errorf(
			"agents.soul.context_projection_bytes must be >= %d bytes: %d",
			minSoulContextProjectionBytes,
			c.ContextProjectionBytes,
		)
	default:
		return nil
	}
}

// Validate ensures HEARTBEAT.md limits and timing bounds are internally consistent.
func (c HeartbeatConfig) Validate() error {
	const (
		maxHeartbeatBodyBytes = int64(1 << 20)
		minWakeEventRetention = time.Hour
	)
	switch {
	case c.MaxBodyBytes <= 0:
		return fmt.Errorf("agents.heartbeat.max_body_bytes must be positive: %d", c.MaxBodyBytes)
	case c.MaxBodyBytes > maxHeartbeatBodyBytes:
		return fmt.Errorf(
			"agents.heartbeat.max_body_bytes must be <= %d: %d",
			maxHeartbeatBodyBytes,
			c.MaxBodyBytes,
		)
	case c.ContextProjectionBytes <= 0:
		return fmt.Errorf(
			"agents.heartbeat.context_projection_bytes must be positive: %d",
			c.ContextProjectionBytes,
		)
	case c.ContextProjectionBytes > c.MaxBodyBytes:
		return fmt.Errorf(
			"agents.heartbeat.context_projection_bytes must be <= agents.heartbeat.max_body_bytes: %d > %d",
			c.ContextProjectionBytes,
			c.MaxBodyBytes,
		)
	case c.MinInterval <= 0:
		return fmt.Errorf("agents.heartbeat.min_interval must be positive: %s", c.MinInterval)
	case c.DefaultInterval <= 0:
		return fmt.Errorf("agents.heartbeat.default_interval must be positive: %s", c.DefaultInterval)
	case c.MinInterval > c.DefaultInterval:
		return fmt.Errorf(
			"agents.heartbeat.min_interval must be <= agents.heartbeat.default_interval: %s > %s",
			c.MinInterval,
			c.DefaultInterval,
		)
	case c.WakeCooldown <= 0:
		return fmt.Errorf("agents.heartbeat.wake_cooldown must be positive: %s", c.WakeCooldown)
	case c.MaxWakesPerCycle <= 0:
		return fmt.Errorf("agents.heartbeat.max_wakes_per_cycle must be positive: %d", c.MaxWakesPerCycle)
	case c.WakeEventRetention < minWakeEventRetention:
		return fmt.Errorf(
			"agents.heartbeat.wake_event_retention must be >= %s: %s",
			minWakeEventRetention,
			c.WakeEventRetention,
		)
	case c.SessionHealthStaleAfter <= 0:
		return fmt.Errorf(
			"agents.heartbeat.session_health_stale_after must be positive: %s",
			c.SessionHealthStaleAfter,
		)
	case c.SessionHealthHookMinInterval <= 0:
		return fmt.Errorf(
			"agents.heartbeat.session_health_hook_min_interval must be positive: %s",
			c.SessionHealthHookMinInterval,
		)
	default:
		return nil
	}
}

// Validate ensures the configured limits are positive.
func (c LimitsConfig) Validate() error {
	switch {
	case c.MaxConcurrentAgents <= 0:
		return fmt.Errorf("limits.max_concurrent_agents must be positive: %d", c.MaxConcurrentAgents)
	default:
		return nil
	}
}

// Validate ensures session-scoped controls are internally consistent.
func (c SessionConfig) Validate() error {
	if err := c.Limits.Validate(); err != nil {
		return err
	}
	if err := c.Supervision.Validate(); err != nil {
		return err
	}
	if err := c.BusyInput.Validate(); err != nil {
		return err
	}
	return c.Compaction.Validate()
}

// Validate ensures session timeout settings are internally consistent.
func (c SessionLimitsConfig) Validate() error {
	if c.Timeout < 0 {
		return fmt.Errorf("session.limits.timeout must be zero or positive: %s", c.Timeout)
	}
	return nil
}

// DefaultSessionSupervisionConfig returns the default runtime activity supervision settings.
func DefaultSessionSupervisionConfig() SessionSupervisionConfig {
	return SessionSupervisionConfig{
		ActivityHeartbeatInterval: 30 * time.Second,
		ProgressNotifyInterval:    10 * time.Minute,
		PromptDeadline:            0,
		InactivityWarningAfter:    15 * time.Minute,
		InactivityTimeout:         30 * time.Minute,
		TimeoutCancelGrace:        30 * time.Second,
	}
}

// DefaultSessionBusyInputConfig returns the default busy-input behavior.
func DefaultSessionBusyInputConfig() SessionBusyInputConfig {
	return SessionBusyInputConfig{
		DefaultMode:  "queue",
		QueueCap:     10,
		MaxTextBytes: 64 << 10,
	}
}

// Validate ensures compaction pressure and retry guards are internally consistent.
func (c SessionCompactionConfig) Validate() error {
	switch {
	case math.IsNaN(c.PressureThreshold) || math.IsInf(c.PressureThreshold, 0) ||
		c.PressureThreshold < 0 || c.PressureThreshold > 1:
		return fmt.Errorf(
			"session.compaction.pressure_threshold must be between 0 and 1: %v",
			c.PressureThreshold,
		)
	case c.MaxAttemptsPerTurn <= 0:
		return fmt.Errorf(
			"session.compaction.max_attempts_per_turn must be positive: %d",
			c.MaxAttemptsPerTurn,
		)
	case c.FailureCooldown < 0:
		return fmt.Errorf(
			"session.compaction.failure_cooldown must be zero or positive: %s",
			c.FailureCooldown,
		)
	default:
		return nil
	}
}

// Normalize returns a config copy with implicit defaults applied.
func (c SessionBusyInputConfig) Normalize() SessionBusyInputConfig {
	normalized := c
	if strings.TrimSpace(normalized.DefaultMode) == "" {
		normalized.DefaultMode = DefaultSessionBusyInputConfig().DefaultMode
	} else {
		normalized.DefaultMode = strings.TrimSpace(normalized.DefaultMode)
	}
	if normalized.QueueCap == 0 {
		normalized.QueueCap = DefaultSessionBusyInputConfig().QueueCap
	}
	if normalized.MaxTextBytes == 0 {
		normalized.MaxTextBytes = DefaultSessionBusyInputConfig().MaxTextBytes
	}
	return normalized
}

// Validate ensures busy-input controls are internally consistent.
func (c SessionBusyInputConfig) Validate() error {
	normalized := c.Normalize()
	switch strings.TrimSpace(normalized.DefaultMode) {
	case "queue", "interrupt", "steer":
	case "":
		return errors.New("session.busy_input.default_mode is required")
	default:
		return fmt.Errorf(
			"session.busy_input.default_mode must be queue, interrupt, or steer: %q",
			normalized.DefaultMode,
		)
	}
	if normalized.QueueCap < minSessionBusyInputQueueCap || normalized.QueueCap > maxSessionBusyInputQueueCap {
		return fmt.Errorf(
			"session.busy_input.queue_cap must be between %d and %d: %d",
			minSessionBusyInputQueueCap,
			maxSessionBusyInputQueueCap,
			normalized.QueueCap,
		)
	}
	if normalized.MaxTextBytes < minSessionBusyInputTextBytes ||
		normalized.MaxTextBytes > maxSessionBusyInputTextBytes {
		return fmt.Errorf(
			"session.busy_input.max_text_bytes must be between %d and %d: %d",
			minSessionBusyInputTextBytes,
			maxSessionBusyInputTextBytes,
			normalized.MaxTextBytes,
		)
	}
	return nil
}

// Validate ensures session supervision settings are internally consistent.
func (c SessionSupervisionConfig) Validate() error {
	switch {
	case c.ActivityHeartbeatInterval <= 0:
		return fmt.Errorf(
			"session.supervision.activity_heartbeat_interval must be positive: %s",
			c.ActivityHeartbeatInterval,
		)
	case c.ProgressNotifyInterval < 0:
		return fmt.Errorf(
			"session.supervision.progress_notify_interval "+
				"must be zero or positive: %s",
			c.ProgressNotifyInterval,
		)
	case c.PromptDeadline < 0:
		return fmt.Errorf(
			"session.supervision.prompt_deadline must be zero or positive: %s",
			c.PromptDeadline,
		)
	case c.InactivityWarningAfter < 0:
		return fmt.Errorf(
			"session.supervision.inactivity_warning_after "+
				"must be zero or positive: %s",
			c.InactivityWarningAfter,
		)
	case c.InactivityTimeout < 0:
		return fmt.Errorf("session.supervision.inactivity_timeout must be zero or positive: %s", c.InactivityTimeout)
	case c.InactivityWarningAfter > 0 &&
		c.InactivityTimeout > 0 &&
		c.InactivityWarningAfter > c.InactivityTimeout:
		return fmt.Errorf(
			"session.supervision.inactivity_warning_after must be <= session.supervision.inactivity_timeout: %s > %s",
			c.InactivityWarningAfter,
			c.InactivityTimeout,
		)
	case c.TimeoutCancelGrace <= 0:
		return fmt.Errorf("session.supervision.timeout_cancel_grace must be positive: %s", c.TimeoutCancelGrace)
	default:
		return nil
	}
}

// Validate ensures the permission mode is supported.
func (c PermissionsConfig) Validate() error {
	return c.Mode.Validate("permissions.mode")
}

// Validate ensures the permission mode is supported.
func (m PermissionMode) Validate(path string) error {
	switch m {
	case PermissionModeDenyAll, PermissionModeApproveReads, PermissionModeApproveAll:
		return nil
	default:
		return fmt.Errorf(
			"%s must be one of %q, %q, %q: %q",
			path,
			PermissionModeDenyAll,
			PermissionModeApproveReads,
			PermissionModeApproveAll,
			m,
		)
	}
}

// Validate ensures observability settings are sensible.
func (c ObservabilityConfig) Validate() error {
	if c.RetentionDays < 0 {
		return fmt.Errorf("observability.retention_days must be zero or positive: %d", c.RetentionDays)
	}
	if c.MaxGlobalBytes <= 0 {
		return fmt.Errorf("observability.max_global_bytes must be positive: %d", c.MaxGlobalBytes)
	}
	if c.AgentProbeTimeout < 0 {
		return fmt.Errorf("observability.agent_probe_timeout must be zero or positive: %s", c.AgentProbeTimeout)
	}

	return c.Transcripts.Validate()
}

// AgentProbeTimeoutOrDefault returns the configured agent probe timeout or the default.
func (c ObservabilityConfig) AgentProbeTimeoutOrDefault() time.Duration {
	if c.AgentProbeTimeout <= 0 {
		return DefaultObservabilityAgentProbeTimeout
	}
	return c.AgentProbeTimeout
}

// Validate ensures transcript retention settings are sensible.
func (c ObservabilityTranscriptConfig) Validate() error {
	if c.SegmentBytes <= 0 {
		return fmt.Errorf("observability.transcripts.segment_bytes must be positive: %d", c.SegmentBytes)
	}
	if c.MaxBytesPerSession <= 0 {
		return fmt.Errorf("observability.transcripts.max_bytes_per_session must be positive: %d", c.MaxBytesPerSession)
	}

	return nil
}
