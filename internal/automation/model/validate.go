package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/vault"
)

const (
	validateKindKey = "kind"
)

const (
	defaultRetryMaxRetries = 3
	defaultRetryBaseDelay  = "2s"
	defaultFireLimitMax    = 12
	defaultFireLimitWindow = "1h"
	webhookIDPrefix        = "wbh_"
)

// DefaultRetryConfig returns the default retry policy for automation definitions.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{Strategy: RetryStrategyNone}
}

// DefaultBackoffRetryConfig returns the default exponential backoff retry policy.
func DefaultBackoffRetryConfig() RetryConfig {
	return RetryConfig{
		Strategy:   RetryStrategyBackoff,
		MaxRetries: defaultRetryMaxRetries,
		BaseDelay:  defaultRetryBaseDelay,
	}
}

// DefaultFireLimitConfig returns the default rolling fire-limit policy.
func DefaultFireLimitConfig() FireLimitConfig {
	return FireLimitConfig{
		Max:    defaultFireLimitMax,
		Window: defaultFireLimitWindow,
	}
}

// Validate ensures the scope is one of the supported automation scope values.
func (s Scope) Validate(path string) error {
	switch s {
	case AutomationScopeGlobal, AutomationScopeWorkspace:
		return nil
	default:
		return fmt.Errorf("%s must be one of %q or %q: %q", path, AutomationScopeGlobal, AutomationScopeWorkspace, s)
	}
}

// Validate ensures the source is one of the supported automation source values.
func (s JobSource) Validate(path string) error {
	switch s {
	case JobSourceConfig, JobSourcePackage, JobSourceDynamic:
		return nil
	default:
		return fmt.Errorf(
			"%s must be one of %q, %q, or %q: %q",
			path,
			JobSourceConfig,
			JobSourcePackage,
			JobSourceDynamic,
			s,
		)
	}
}

// Validate ensures the run status is one of the supported lifecycle states.
func (s RunStatus) Validate(path string) error {
	switch s {
	case RunScheduled, RunRunning, RunDelegated, RunCompleted, RunFailed, RunCancelled:
		return nil
	default:
		return fmt.Errorf(
			"%s must be one of %q, %q, %q, %q, %q, or %q: %q",
			path,
			RunScheduled,
			RunRunning,
			RunDelegated,
			RunCompleted,
			RunFailed,
			RunCancelled,
			s,
		)
	}
}

// Validate ensures the scheduler catch-up policy is supported.
func (p SchedulerCatchUpPolicy) Validate(path string) error {
	switch p {
	case SchedulerCatchUpPolicySkipMissed,
		SchedulerCatchUpPolicyCoalesce,
		SchedulerCatchUpPolicyReplay,
		SchedulerCatchUpPolicyRunOnce:
		return nil
	default:
		return fmt.Errorf(
			"%s must be one of %q, %q, %q, or %q: %q",
			path,
			SchedulerCatchUpPolicySkipMissed,
			SchedulerCatchUpPolicyCoalesce,
			SchedulerCatchUpPolicyReplay,
			SchedulerCatchUpPolicyRunOnce,
			p,
		)
	}
}

// Validate ensures the scheduler skip reason is supported.
func (r SchedulerSkipReason) Validate(path string) error {
	switch r {
	case SchedulerSkipReasonGraceExceeded, SchedulerSkipReasonSelfOverlap:
		return nil
	default:
		return fmt.Errorf(
			"%s must be one of %q or %q: %q",
			path,
			SchedulerSkipReasonGraceExceeded,
			SchedulerSkipReasonSelfOverlap,
			r,
		)
	}
}

// Validate ensures the activation source is one of the supported ingress values.
func (s ActivationSource) Validate(path string) error {
	switch s {
	case ActivationSourceObserver, ActivationSourceHook, ActivationSourceWebhook, ActivationSourceExtension:
		return nil
	default:
		return fmt.Errorf(
			"%s must be one of %q, %q, %q, or %q: %q",
			path,
			ActivationSourceObserver,
			ActivationSourceHook,
			ActivationSourceWebhook,
			ActivationSourceExtension,
			s,
		)
	}
}

// Validate ensures the schedule mode is one of the supported scheduling modes.
func (m ScheduleMode) Validate(path string) error {
	switch m {
	case ScheduleModeCron, ScheduleModeEvery, ScheduleModeAt:
		return nil
	default:
		return fmt.Errorf(
			"%s must be one of %q, %q, or %q: %q",
			path,
			ScheduleModeCron,
			ScheduleModeEvery,
			ScheduleModeAt,
			m,
		)
	}
}

// Validate ensures the retry strategy is supported.
func (s RetryStrategy) Validate(path string) error {
	switch s {
	case RetryStrategyNone, RetryStrategyBackoff:
		return nil
	default:
		return fmt.Errorf("%s must be one of %q or %q: %q", path, RetryStrategyNone, RetryStrategyBackoff, s)
	}
}

// ValidateScopeBinding enforces the global/workspace binding invariants shared by jobs, triggers, and envelopes.
func ValidateScopeBinding(scope Scope, workspaceBinding string, path string, workspaceField string) error {
	scopePath := nestedPath(path, "scope")
	if err := scope.Validate(scopePath); err != nil {
		return err
	}

	workspacePath := nestedPath(path, workspaceField)
	switch scope {
	case AutomationScopeGlobal:
		if strings.TrimSpace(workspaceBinding) != "" {
			return fmt.Errorf("%s must be empty when %s is %q", workspacePath, scopePath, AutomationScopeGlobal)
		}
	case AutomationScopeWorkspace:
		if strings.TrimSpace(workspaceBinding) == "" {
			return fmt.Errorf("%s is required when %s is %q", workspacePath, scopePath, AutomationScopeWorkspace)
		}
	}

	return nil
}

// Validate ensures the retry configuration is internally consistent.
func (c RetryConfig) Validate(path string) error {
	if err := c.Strategy.Validate(nestedPath(path, "strategy")); err != nil {
		return err
	}

	switch c.Strategy {
	case RetryStrategyNone:
		if c.MaxRetries != 0 {
			return fmt.Errorf(
				"%s must be zero when retry.strategy is %q: %d",
				nestedPath(path, "max_retries"),
				RetryStrategyNone,
				c.MaxRetries,
			)
		}
		if strings.TrimSpace(c.BaseDelay) != "" {
			return fmt.Errorf(
				"%s must be empty when retry.strategy is %q: %q",
				nestedPath(path, "base_delay"),
				RetryStrategyNone,
				c.BaseDelay,
			)
		}
	case RetryStrategyBackoff:
		if c.MaxRetries <= 0 {
			return fmt.Errorf(
				"%s must be positive when retry.strategy is %q: %d",
				nestedPath(path, "max_retries"),
				RetryStrategyBackoff,
				c.MaxRetries,
			)
		}
		if strings.TrimSpace(c.BaseDelay) == "" {
			return errors.New(nestedPath(path, "base_delay") + " is required when retry.strategy is \"backoff\"")
		}
		delay, err := time.ParseDuration(strings.TrimSpace(c.BaseDelay))
		if err != nil {
			return fmt.Errorf("%s is invalid: %w", nestedPath(path, "base_delay"), err)
		}
		if delay <= 0 {
			return fmt.Errorf("%s must be positive: %s", nestedPath(path, "base_delay"), delay)
		}
	}

	return nil
}

// Validate ensures the rolling fire-limit configuration is internally consistent.
func (c FireLimitConfig) Validate(path string) error {
	if c.Max <= 0 {
		return fmt.Errorf("%s must be positive: %d", nestedPath(path, "max"), c.Max)
	}
	if strings.TrimSpace(c.Window) == "" {
		return errors.New(nestedPath(path, "window") + " is required")
	}

	window, err := time.ParseDuration(strings.TrimSpace(c.Window))
	if err != nil {
		return fmt.Errorf("%s is invalid: %w", nestedPath(path, "window"), err)
	}
	if window <= 0 {
		return fmt.Errorf("%s must be positive: %s", nestedPath(path, "window"), window)
	}

	return nil
}

func validateWebhookTriggerFields(t Trigger, path string) error {
	if strings.TrimSpace(t.EndpointSlug) == "" && strings.TrimSpace(t.WebhookID) == "" {
		return errors.New(
			nestedPath(path, "endpoint_slug") + " or " +
				nestedPath(path, "webhook_id") +
				" is required when event is \"webhook\"",
		)
	}
	webhookID := strings.TrimSpace(t.WebhookID)
	if webhookID != "" && !strings.HasPrefix(webhookID, webhookIDPrefix) {
		return fmt.Errorf("%s must start with %q: %q", nestedPath(path, "webhook_id"), webhookIDPrefix, webhookID)
	}
	if strings.TrimSpace(t.WebhookSecretRef) == "" {
		return errors.New(nestedPath(path, "webhook_secret_ref") + " is required when event is \"webhook\"")
	}
	if err := vault.ValidateRefNamespace(t.WebhookSecretRef, "automation"); err != nil {
		return fmt.Errorf("%s is invalid: %w", nestedPath(path, "webhook_secret_ref"), err)
	}
	return nil
}

func validateNonWebhookTriggerFields(t Trigger, path string) error {
	event := strings.TrimSpace(t.Event)
	if strings.TrimSpace(t.EndpointSlug) != "" {
		return fmt.Errorf("%s must be empty when event is %q", nestedPath(path, "endpoint_slug"), event)
	}
	if strings.TrimSpace(t.WebhookID) != "" {
		return fmt.Errorf("%s must be empty when event is %q", nestedPath(path, "webhook_id"), event)
	}
	if strings.TrimSpace(t.WebhookSecretRef) != "" {
		return fmt.Errorf("%s must be empty when event is %q", nestedPath(path, "webhook_secret_ref"), event)
	}
	return nil
}

// Validate ensures the durable scheduler cursor is internally consistent.
func (s SchedulerState) Validate(path string) error {
	if strings.TrimSpace(s.JobID) == "" {
		return errors.New(nestedPath(path, "job_id") + " is required")
	}
	if err := s.CatchUpPolicy.Validate(nestedPath(path, "catch_up_policy")); err != nil {
		return err
	}
	if s.MisfireGraceSeconds < 0 {
		return fmt.Errorf(
			"%s must be zero or positive: %d",
			nestedPath(path, "misfire_grace_seconds"),
			s.MisfireGraceSeconds,
		)
	}
	if s.ConsecutiveResumeFailures < 0 {
		return fmt.Errorf(
			"%s must be zero or positive: %d",
			nestedPath(path, "consecutive_resume_failures"),
			s.ConsecutiveResumeFailures,
		)
	}
	if s.MisfireCount < 0 {
		return fmt.Errorf("%s must be zero or positive: %d", nestedPath(path, "misfire_count"), s.MisfireCount)
	}
	if s.UpdatedAt.IsZero() {
		return errors.New(nestedPath(path, "updated_at") + " is required")
	}
	return nil
}

// Validate ensures a scheduled fire claim can be persisted atomically.
func (c SchedulerClaim) Validate(path string) error {
	if strings.TrimSpace(c.JobID) == "" {
		return errors.New(nestedPath(path, "job_id") + " is required")
	}
	if strings.TrimSpace(c.RunID) == "" {
		return errors.New(nestedPath(path, "run_id") + " is required")
	}
	if strings.TrimSpace(c.FireID) == "" {
		return errors.New(nestedPath(path, "fire_id") + " is required")
	}
	if c.ScheduledAt.IsZero() {
		return errors.New(nestedPath(path, "scheduled_at") + " is required")
	}
	if c.ClaimedAt.IsZero() {
		return errors.New(nestedPath(path, "claimed_at") + " is required")
	}
	if c.NetworkParticipation != nil {
		if _, err := participation.NormalizeIntent(*c.NetworkParticipation); err != nil {
			return fmt.Errorf("%s is invalid: %w", nestedPath(path, "network_participation"), err)
		}
	}
	if c.CatchUpPolicy != "" {
		if err := c.CatchUpPolicy.Validate(nestedPath(path, "catch_up_policy")); err != nil {
			return err
		}
	}
	if c.MisfireGraceSeconds < 0 {
		return fmt.Errorf(
			"%s must be zero or positive: %d",
			nestedPath(path, "misfire_grace_seconds"),
			c.MisfireGraceSeconds,
		)
	}
	if c.SkipReason != "" {
		if err := c.SkipReason.Validate(nestedPath(path, "skip_reason")); err != nil {
			return err
		}
	}
	return nil
}

// Validate ensures the direct task materialization configuration is internally consistent.
func (c JobTaskConfig) Validate(path string) error {
	if _, err := NormalizeDirectTaskParticipation(c.NetworkParticipation); err != nil {
		return fmt.Errorf("%s is invalid: %w", nestedPath(path, "network_participation"), err)
	}
	if c.Owner != nil {
		if err := c.Owner.Validate(nestedPath(path, "owner")); err != nil {
			return err
		}
	}
	return nil
}

// Validate ensures the normalized activation envelope satisfies the shared scope and source invariants.
func (e ActivationEnvelope) Validate(path string) error {
	if strings.TrimSpace(e.Kind) == "" {
		return errors.New(nestedPath(path, validateKindKey) + " is required")
	}
	if err := ValidateScopeBinding(e.Scope, e.WorkspaceID, path, "workspace_id"); err != nil {
		return err
	}
	if err := e.Source.Validate(nestedPath(path, "source")); err != nil {
		return err
	}
	return nil
}

// ValidateTriggerFilter ensures trigger filters only reference supported activation-envelope field paths.
func ValidateTriggerFilter(filter map[string]string, path string) error {
	for rawKey, rawValue := range filter {
		key := strings.TrimSpace(rawKey)
		value := strings.TrimSpace(rawValue)
		if key == "" {
			return errors.New(path + " contains an empty field path")
		}
		if value == "" {
			return fmt.Errorf("%s[%q] must not be empty", path, rawKey)
		}
		if err := validateTriggerFilterPath(key); err != nil {
			return fmt.Errorf("%s[%q]: %w", path, rawKey, err)
		}
	}
	return nil
}

func validateTriggerFilterPath(path string) error {
	switch path {
	case validateKindKey, "scope", "workspace_id", "source":
		return nil
	}
	if dataPath, ok := strings.CutPrefix(path, "data."); ok && validTriggerFilterDataPath(dataPath) {
		return nil
	}
	return fmt.Errorf("unsupported filter path %q", path)
}

func validTriggerFilterDataPath(path string) bool {
	remaining := path
	for {
		segment, rest, found := strings.Cut(remaining, ".")
		if strings.TrimSpace(segment) == "" {
			return false
		}
		if !found {
			return true
		}
		remaining = rest
	}
}

func nestedPath(path string, field string) string {
	trimmedPath := strings.TrimSpace(path)
	trimmedField := strings.TrimSpace(field)
	switch {
	case trimmedPath == "":
		return trimmedField
	case trimmedField == "":
		return trimmedPath
	default:
		return trimmedPath + "." + trimmedField
	}
}
