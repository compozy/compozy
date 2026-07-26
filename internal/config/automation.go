package config

import (
	"errors"
	"fmt"
	"maps"
	"strings"
	"time"

	automationpkg "github.com/compozy/agh/internal/automation/model"
	"github.com/compozy/agh/internal/vault"
)

// AutomationConfig holds TOML-defined automation defaults, jobs, and triggers.
type AutomationConfig struct {
	Enabled           bool                          `toml:"enabled"`
	Timezone          string                        `toml:"timezone,omitempty"`
	MaxConcurrentJobs int                           `toml:"max_concurrent_jobs"`
	DefaultFireLimit  automationpkg.FireLimitConfig `toml:"default_fire_limit"`
	Suggestions       AutomationSuggestionsConfig   `toml:"suggestions"`
	Jobs              []AutomationJob               `toml:"jobs,omitempty"`
	Triggers          []AutomationTrigger           `toml:"triggers,omitempty"`
}

// AutomationJob holds a config-defined scheduled job before workspace resolution.
type AutomationJob struct {
	Scope      automationpkg.Scope           `toml:"scope"`
	Name       string                        `toml:"name"`
	AgentName  string                        `toml:"agent"`
	Workspace  string                        `toml:"workspace,omitempty"`
	Prompt     string                        `toml:"prompt"`
	Schedule   automationpkg.ScheduleSpec    `toml:"schedule"`
	Task       *automationpkg.JobTaskConfig  `toml:"task,omitempty"`
	TargetKind automationpkg.TargetKind      `toml:"target_kind,omitempty"`
	LoopTarget *automationpkg.LoopTarget     `toml:"loop_target,omitempty"`
	Enabled    bool                          `toml:"enabled"`
	Retry      automationpkg.RetryConfig     `toml:"retry,omitempty"`
	FireLimit  automationpkg.FireLimitConfig `toml:"fire_limit,omitempty"`
	Source     automationpkg.JobSource       `toml:"-"`
}

// AutomationTrigger holds a config-defined trigger before workspace resolution.
type AutomationTrigger struct {
	Scope            automationpkg.Scope           `toml:"scope"`
	Name             string                        `toml:"name"`
	AgentName        string                        `toml:"agent"`
	Workspace        string                        `toml:"workspace,omitempty"`
	Prompt           string                        `toml:"prompt"`
	Event            string                        `toml:"event"`
	Filter           map[string]string             `toml:"filter,omitempty"`
	TargetKind       automationpkg.TargetKind      `toml:"target_kind,omitempty"`
	LoopTarget       *automationpkg.LoopTarget     `toml:"loop_target,omitempty"`
	Enabled          bool                          `toml:"enabled"`
	Retry            automationpkg.RetryConfig     `toml:"retry,omitempty"`
	FireLimit        automationpkg.FireLimitConfig `toml:"fire_limit,omitempty"`
	Source           automationpkg.JobSource       `toml:"-"`
	EndpointSlug     string                        `toml:"endpoint_slug,omitempty"`
	WebhookSecretRef string                        `toml:"webhook_secret_ref,omitempty"`
}

type automationOverlay struct {
	Enabled           *bool                          `toml:"enabled"`
	Timezone          *string                        `toml:"timezone"`
	MaxConcurrentJobs *int                           `toml:"max_concurrent_jobs"`
	DefaultFireLimit  *automationpkg.FireLimitConfig `toml:"default_fire_limit"`
	Suggestions       *automationSuggestionsOverlay  `toml:"suggestions"`
	Jobs              []parsedAutomationJob          `toml:"jobs"`
	Triggers          []parsedAutomationTrigger      `toml:"triggers"`
}

type parsedAutomationJob struct {
	Scope      automationpkg.Scope            `toml:"scope"`
	Name       string                         `toml:"name"`
	AgentName  string                         `toml:"agent"`
	Workspace  string                         `toml:"workspace"`
	Prompt     string                         `toml:"prompt"`
	Schedule   *automationpkg.ScheduleSpec    `toml:"schedule"`
	Task       *automationpkg.JobTaskConfig   `toml:"task"`
	TargetKind automationpkg.TargetKind       `toml:"target_kind"`
	LoopTarget *automationpkg.LoopTarget      `toml:"loop_target"`
	Enabled    *bool                          `toml:"enabled"`
	Retry      *automationpkg.RetryConfig     `toml:"retry"`
	FireLimit  *automationpkg.FireLimitConfig `toml:"fire_limit"`
}

type parsedAutomationTrigger struct {
	Scope            automationpkg.Scope            `toml:"scope"`
	Name             string                         `toml:"name"`
	AgentName        string                         `toml:"agent"`
	Workspace        string                         `toml:"workspace"`
	Prompt           string                         `toml:"prompt"`
	Event            string                         `toml:"event"`
	Filter           map[string]string              `toml:"filter"`
	TargetKind       automationpkg.TargetKind       `toml:"target_kind"`
	LoopTarget       *automationpkg.LoopTarget      `toml:"loop_target"`
	Enabled          *bool                          `toml:"enabled"`
	Retry            *automationpkg.RetryConfig     `toml:"retry"`
	FireLimit        *automationpkg.FireLimitConfig `toml:"fire_limit"`
	EndpointSlug     string                         `toml:"endpoint_slug"`
	WebhookSecretRef string                         `toml:"webhook_secret_ref"`
}

// Validate ensures the automation config is internally consistent.
func (c AutomationConfig) Validate() error {
	return c.validateWithEnv(processEnvLookup)
}

func (c AutomationConfig) validateWithEnv(lookup envLookup) error {
	if strings.TrimSpace(c.Timezone) == "" {
		return errors.New("automation.timezone is required")
	}
	if _, err := time.LoadLocation(strings.TrimSpace(c.Timezone)); err != nil {
		return fmt.Errorf("automation.timezone is invalid: %w", err)
	}
	if c.MaxConcurrentJobs <= 0 {
		return fmt.Errorf("automation.max_concurrent_jobs must be positive: %d", c.MaxConcurrentJobs)
	}
	if err := c.DefaultFireLimit.Validate("automation.default_fire_limit"); err != nil {
		return err
	}
	if err := c.Suggestions.validate("automation.suggestions"); err != nil {
		return err
	}

	for i, job := range c.Jobs {
		if err := job.Validate(fmt.Sprintf("automation.jobs[%d]", i)); err != nil {
			return err
		}
	}
	for i, trigger := range c.Triggers {
		if err := trigger.validateWithEnv(fmt.Sprintf("automation.triggers[%d]", i), lookup); err != nil {
			return err
		}
	}

	return nil
}

// Validate ensures the config-defined job is internally consistent before runtime resolution.
func (j AutomationJob) Validate(path string) error {
	if strings.TrimSpace(j.Name) == "" {
		return errors.New(path + ".name is required")
	}
	if _, err := j.validateTarget(path); err != nil {
		return err
	}
	if err := automationpkg.ValidateScopeBinding(j.Scope, j.Workspace, path, "workspace"); err != nil {
		return err
	}
	if err := j.Source.Validate(path + ".source"); err != nil {
		return err
	}
	if err := j.Schedule.Validate(path + ".schedule"); err != nil {
		return err
	}
	if err := j.Retry.Validate(path + ".retry"); err != nil {
		return err
	}
	if err := j.FireLimit.Validate(path + ".fire_limit"); err != nil {
		return err
	}
	if j.Task != nil {
		if err := j.Task.Validate(path + ".task"); err != nil {
			return err
		}
		if err := automationpkg.ValidateDirectTaskParticipationScope(
			j.Scope,
			j.Task.NetworkParticipation,
			path+".task.network_participation",
			path+".scope",
		); err != nil {
			return err
		}
		if j.Retry.Strategy != automationpkg.RetryStrategyNone {
			return fmt.Errorf(
				"%s.strategy must be %q when %s is configured",
				path+".retry",
				automationpkg.RetryStrategyNone,
				path+".task",
			)
		}
	}

	return nil
}

func (j AutomationJob) validateTarget(path string) (automationpkg.TargetKind, error) {
	targetKind := normalizeConfigAutomationTargetKind(j.TargetKind, j.LoopTarget)
	if err := targetKind.Validate(path + ".target_kind"); err != nil {
		return "", err
	}
	if targetKind == automationpkg.TargetKindAgent {
		return targetKind, j.validateAgentTarget(path)
	}
	return targetKind, j.validateLoopTarget(path)
}

func (j AutomationJob) validateAgentTarget(path string) error {
	if j.LoopTarget != nil {
		return fmt.Errorf(
			"%s.loop_target requires %s to be %q",
			path,
			path+".target_kind",
			automationpkg.TargetKindLoop,
		)
	}
	if j.Task == nil && strings.TrimSpace(j.AgentName) == "" {
		return errors.New(path + ".agent is required")
	}
	if j.Task == nil && strings.TrimSpace(j.Prompt) == "" {
		return errors.New(path + ".prompt is required")
	}
	return nil
}

func (j AutomationJob) validateLoopTarget(path string) error {
	if j.LoopTarget == nil {
		return errors.New(path + ".loop_target is required when target_kind is \"loop\"")
	}
	if strings.TrimSpace(j.AgentName) != "" {
		return fmt.Errorf("%s.agent must be empty when target_kind is %q", path, automationpkg.TargetKindLoop)
	}
	if strings.TrimSpace(j.Prompt) != "" {
		return fmt.Errorf("%s.prompt must be empty when target_kind is %q", path, automationpkg.TargetKindLoop)
	}
	if j.Task != nil {
		return fmt.Errorf("%s.task must be empty when target_kind is %q", path, automationpkg.TargetKindLoop)
	}
	return validateConfigLoopTarget(j.LoopTarget, path+".loop_target", true)
}

// Validate ensures the config-defined trigger is internally consistent before runtime resolution.
func (t AutomationTrigger) Validate(path string) error {
	return t.validateWithEnv(path, processEnvLookup)
}

func (t AutomationTrigger) validateWithEnv(path string, _ envLookup) error {
	if strings.TrimSpace(t.Name) == "" {
		return errors.New(path + ".name is required")
	}
	targetKind, err := t.validateTarget(path)
	if err != nil {
		return err
	}
	if strings.TrimSpace(t.Event) == "" {
		return errors.New(path + ".event is required")
	}
	if err := automationpkg.ValidateScopeBinding(t.Scope, t.Workspace, path, "workspace"); err != nil {
		return err
	}
	if err := t.Source.Validate(path + ".source"); err != nil {
		return err
	}
	if err := t.Retry.Validate(path + ".retry"); err != nil {
		return err
	}
	if err := t.FireLimit.Validate(path + ".fire_limit"); err != nil {
		return err
	}
	if err := automationpkg.ValidateTriggerFilter(t.Filter, path+".filter"); err != nil {
		return err
	}
	if targetKind == automationpkg.TargetKindAgent {
		if err := automationpkg.ValidateTriggerPromptTemplate(t.Prompt); err != nil {
			return fmt.Errorf("%s.prompt is invalid: %w", path, err)
		}
	}

	return t.validateWebhookFields(path)
}

func (t AutomationTrigger) validateTarget(path string) (automationpkg.TargetKind, error) {
	targetKind := normalizeConfigAutomationTargetKind(t.TargetKind, t.LoopTarget)
	if err := targetKind.Validate(path + ".target_kind"); err != nil {
		return "", err
	}
	if targetKind == automationpkg.TargetKindAgent {
		return targetKind, t.validateAgentTarget(path)
	}
	return targetKind, t.validateLoopTarget(path)
}

func (t AutomationTrigger) validateAgentTarget(path string) error {
	if t.LoopTarget != nil {
		return fmt.Errorf(
			"%s.loop_target requires %s to be %q",
			path,
			path+".target_kind",
			automationpkg.TargetKindLoop,
		)
	}
	if strings.TrimSpace(t.AgentName) == "" {
		return errors.New(path + ".agent is required")
	}
	if strings.TrimSpace(t.Prompt) == "" {
		return errors.New(path + ".prompt is required")
	}
	return nil
}

func (t AutomationTrigger) validateLoopTarget(path string) error {
	if t.LoopTarget == nil {
		return errors.New(path + ".loop_target is required when target_kind is \"loop\"")
	}
	if strings.TrimSpace(t.AgentName) != "" {
		return fmt.Errorf("%s.agent must be empty when target_kind is %q", path, automationpkg.TargetKindLoop)
	}
	if strings.TrimSpace(t.Prompt) != "" {
		return fmt.Errorf("%s.prompt must be empty when target_kind is %q", path, automationpkg.TargetKindLoop)
	}
	return validateConfigLoopTarget(t.LoopTarget, path+".loop_target", false)
}

func (t AutomationTrigger) validateWebhookFields(path string) error {
	event := strings.TrimSpace(t.Event)
	if event == "webhook" {
		if strings.TrimSpace(t.EndpointSlug) == "" {
			return errors.New(path + ".endpoint_slug is required when event is \"webhook\"")
		}
		secretRef := strings.TrimSpace(t.WebhookSecretRef)
		if secretRef == "" {
			return errors.New(path + ".webhook_secret_ref is required when event is \"webhook\"")
		}
		if err := vault.ValidateRefNamespace(secretRef, "automation"); err != nil {
			return fmt.Errorf("%s.webhook_secret_ref is invalid: %w", path, err)
		}
		return nil
	}
	if strings.TrimSpace(t.EndpointSlug) != "" {
		return fmt.Errorf("%s.endpoint_slug must be empty when event is %q", path, event)
	}
	if strings.TrimSpace(t.WebhookSecretRef) != "" {
		return fmt.Errorf("%s.webhook_secret_ref must be empty when event is %q", path, event)
	}
	return nil
}

func (o automationOverlay) Apply(dst *AutomationConfig) error {
	if dst == nil {
		return errors.New("automation config is required")
	}

	if o.Enabled != nil {
		dst.Enabled = *o.Enabled
	}
	if o.Timezone != nil {
		dst.Timezone = *o.Timezone
	}
	if o.MaxConcurrentJobs != nil {
		dst.MaxConcurrentJobs = *o.MaxConcurrentJobs
	}
	if o.DefaultFireLimit != nil {
		dst.DefaultFireLimit = *o.DefaultFireLimit
	}
	o.Suggestions.apply(&dst.Suggestions)

	for _, raw := range o.Jobs {
		dst.Jobs = append(dst.Jobs, raw.toAutomationJob(dst.DefaultFireLimit))
	}
	for _, raw := range o.Triggers {
		dst.Triggers = append(dst.Triggers, raw.toAutomationTrigger(dst.DefaultFireLimit))
	}

	return nil
}

func (j parsedAutomationJob) toAutomationJob(defaultFireLimit automationpkg.FireLimitConfig) AutomationJob {
	job := AutomationJob{
		Scope:      j.Scope,
		Name:       j.Name,
		AgentName:  j.AgentName,
		Workspace:  j.Workspace,
		Prompt:     j.Prompt,
		TargetKind: j.TargetKind,
		LoopTarget: cloneAutomationLoopTarget(j.LoopTarget),
		Enabled:    true,
		Retry:      automationpkg.DefaultRetryConfig(),
		FireLimit:  defaultFireLimit,
		Source:     automationpkg.JobSourceConfig,
	}
	if j.Schedule != nil {
		job.Schedule = *j.Schedule
	}
	job.Task = cloneParsedJobTaskConfig(j.Task)
	if j.Enabled != nil {
		job.Enabled = *j.Enabled
	}
	if j.Retry != nil {
		job.Retry = *j.Retry
	}
	if j.FireLimit != nil {
		job.FireLimit = *j.FireLimit
	}

	return job
}

func cloneParsedJobTaskConfig(config *automationpkg.JobTaskConfig) *automationpkg.JobTaskConfig {
	if config == nil {
		return nil
	}
	cloned := *config
	if config.Owner != nil {
		owner := *config.Owner
		cloned.Owner = &owner
	}
	return &cloned
}

func cloneAutomationLoopTarget(target *automationpkg.LoopTarget) *automationpkg.LoopTarget {
	if target == nil {
		return nil
	}
	cloned := *target
	if len(target.Inputs) > 0 {
		cloned.Inputs = make(map[string]any, len(target.Inputs))
		maps.Copy(cloned.Inputs, target.Inputs)
	}
	cloned.InputMapping = mergeStringMaps(nil, target.InputMapping)
	return &cloned
}

func normalizeConfigAutomationTargetKind(
	kind automationpkg.TargetKind,
	target *automationpkg.LoopTarget,
) automationpkg.TargetKind {
	normalized := automationpkg.TargetKind(strings.TrimSpace(string(kind)))
	if normalized != "" {
		return normalized
	}
	if target != nil {
		return automationpkg.TargetKindLoop
	}
	return automationpkg.TargetKindAgent
}

func validateConfigLoopTarget(target *automationpkg.LoopTarget, path string, scheduleOnly bool) error {
	if target == nil {
		return errors.New(path + " is required")
	}
	if strings.TrimSpace(target.WorkspaceID) == "" {
		return errors.New(path + ".workspace_id is required")
	}
	if strings.TrimSpace(target.LoopName) == "" {
		return errors.New(path + ".loop_name is required")
	}
	for key := range target.Inputs {
		if strings.TrimSpace(key) == "" {
			return errors.New(path + ".inputs keys must be non-empty")
		}
	}
	if scheduleOnly && len(target.InputMapping) > 0 {
		return errors.New(path + ".input_mapping must be empty for scheduled loop targets")
	}
	for key, value := range target.InputMapping {
		if strings.TrimSpace(key) == "" {
			return errors.New(path + ".input_mapping keys must be non-empty")
		}
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s.input_mapping[%q] is required", path, key)
		}
	}
	return nil
}

func (t parsedAutomationTrigger) toAutomationTrigger(defaultFireLimit automationpkg.FireLimitConfig) AutomationTrigger {
	trigger := AutomationTrigger{
		Scope:            t.Scope,
		Name:             t.Name,
		AgentName:        t.AgentName,
		Workspace:        t.Workspace,
		Prompt:           t.Prompt,
		Event:            t.Event,
		Filter:           mergeStringMaps(nil, t.Filter),
		TargetKind:       t.TargetKind,
		LoopTarget:       cloneAutomationLoopTarget(t.LoopTarget),
		Enabled:          true,
		Retry:            automationpkg.DefaultRetryConfig(),
		FireLimit:        defaultFireLimit,
		Source:           automationpkg.JobSourceConfig,
		EndpointSlug:     t.EndpointSlug,
		WebhookSecretRef: t.WebhookSecretRef,
	}
	if t.Enabled != nil {
		trigger.Enabled = *t.Enabled
	}
	if t.Retry != nil {
		trigger.Retry = *t.Retry
	}
	if t.FireLimit != nil {
		trigger.FireLimit = *t.FireLimit
	}

	return trigger
}
