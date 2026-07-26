package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/network/participation"
)

// Validate ensures the target kind is supported.
func (k TargetKind) Validate(path string) error {
	switch k.Normalize() {
	case TargetKindAgent, TargetKindLoop:
		return nil
	default:
		return fmt.Errorf("%s must be one of %q or %q: %q", path, TargetKindAgent, TargetKindLoop, k)
	}
}

// Normalize trims the target kind.
func (k TargetKind) Normalize() TargetKind {
	return TargetKind(strings.TrimSpace(string(k)))
}

// IsLoopTarget reports whether the job delegates to a Loop.
func (j Job) IsLoopTarget() bool {
	return normalizedTargetKind(j.TargetKind, j.LoopTarget) == TargetKindLoop
}

// IsLoopTarget reports whether the trigger delegates to a Loop.
func (t Trigger) IsLoopTarget() bool {
	return normalizedTargetKind(t.TargetKind, t.LoopTarget) == TargetKindLoop
}

// Validate ensures the scheduled job definition is internally consistent.
func (j Job) Validate(path string) error {
	if strings.TrimSpace(j.Name) == "" {
		return errors.New(nestedPath(path, "name") + " is required")
	}
	targetKind := normalizedTargetKind(j.TargetKind, j.LoopTarget)
	if err := targetKind.Validate(nestedPath(path, "target_kind")); err != nil {
		return err
	}
	if err := validateJobTarget(j, targetKind, path); err != nil {
		return err
	}
	if err := ValidateScopeBinding(j.Scope, j.WorkspaceID, path, "workspace_id"); err != nil {
		return err
	}
	if err := j.Source.Validate(nestedPath(path, "source")); err != nil {
		return err
	}
	if j.Schedule == nil {
		return errors.New(nestedPath(path, "schedule") + " is required")
	}
	if err := j.Schedule.Validate(nestedPath(path, "schedule")); err != nil {
		return err
	}
	if err := j.Retry.Validate(nestedPath(path, "retry")); err != nil {
		return err
	}
	if err := j.FireLimit.Validate(nestedPath(path, "fire_limit")); err != nil {
		return err
	}
	if j.Task != nil {
		if err := j.Task.Validate(nestedPath(path, "task")); err != nil {
			return err
		}
		if err := ValidateDirectTaskParticipationScope(
			j.Scope,
			j.Task.NetworkParticipation,
			nestedPath(path, "task.network_participation"),
			nestedPath(path, "scope"),
		); err != nil {
			return err
		}
		if j.Retry.Strategy != RetryStrategyNone {
			return fmt.Errorf(
				"%s.strategy must be %q when %s is configured",
				nestedPath(path, "retry"),
				RetryStrategyNone,
				nestedPath(path, "task"),
			)
		}
	}

	return nil
}

// Validate ensures the trigger definition is internally consistent.
func (t Trigger) Validate(path string) error {
	if strings.TrimSpace(t.Name) == "" {
		return errors.New(nestedPath(path, "name") + " is required")
	}
	targetKind := normalizedTargetKind(t.TargetKind, t.LoopTarget)
	if err := targetKind.Validate(nestedPath(path, "target_kind")); err != nil {
		return err
	}
	if err := validateTriggerTarget(t, targetKind, path); err != nil {
		return err
	}
	if strings.TrimSpace(t.Event) == "" {
		return errors.New(nestedPath(path, "event") + " is required")
	}
	if err := ValidateScopeBinding(t.Scope, t.WorkspaceID, path, "workspace_id"); err != nil {
		return err
	}
	if err := t.Source.Validate(nestedPath(path, "source")); err != nil {
		return err
	}
	if err := t.Retry.Validate(nestedPath(path, "retry")); err != nil {
		return err
	}
	if err := t.FireLimit.Validate(nestedPath(path, "fire_limit")); err != nil {
		return err
	}
	if err := ValidateTriggerFilter(t.Filter, nestedPath(path, "filter")); err != nil {
		return err
	}
	if targetKind == TargetKindAgent {
		if err := ValidateTriggerPromptTemplate(t.Prompt); err != nil {
			return fmt.Errorf("%s is invalid: %w", nestedPath(path, "prompt"), err)
		}
	}
	if strings.TrimSpace(t.Event) == "webhook" {
		return validateWebhookTriggerFields(t, path)
	}
	return validateNonWebhookTriggerFields(t, path)
}

// Validate ensures the run record is internally consistent.
func (r Run) Validate(path string) error {
	if err := r.Status.Validate(nestedPath(path, "status")); err != nil {
		return err
	}
	if r.Attempt < 0 {
		return fmt.Errorf("%s must be zero or positive: %d", nestedPath(path, "attempt"), r.Attempt)
	}
	if r.ScheduledAt != nil && r.ScheduledAt.IsZero() {
		return errors.New(nestedPath(path, "scheduled_at") + " must not be zero")
	}
	if r.StartedAt != nil && r.EndedAt != nil && r.EndedAt.Before(*r.StartedAt) {
		return fmt.Errorf("%s must not be before %s", nestedPath(path, "ended_at"), nestedPath(path, "started_at"))
	}
	if r.DeliveryErrorAt != nil && strings.TrimSpace(r.DeliveryError) == "" {
		return fmt.Errorf(
			"%s is required when %s is set",
			nestedPath(path, "delivery_error"),
			nestedPath(path, "delivery_error_at"),
		)
	}
	if r.NetworkParticipation != nil {
		if _, err := participation.NormalizeIntent(*r.NetworkParticipation); err != nil {
			return fmt.Errorf("%s is invalid: %w", nestedPath(path, "network_participation"), err)
		}
	}
	return validateDelegatedRunRefs(r, path)
}

func validateJobTarget(job Job, targetKind TargetKind, path string) error {
	switch targetKind {
	case TargetKindAgent:
		if hasLoopTarget(job.LoopTarget) {
			return errors.New(nestedPath(path, "loop_target") + " must be empty when target_kind is \"agent\"")
		}
		if job.Task == nil && strings.TrimSpace(job.AgentName) == "" {
			return errors.New(nestedPath(path, "agent_name") + " is required")
		}
		if job.Task == nil && strings.TrimSpace(job.Prompt) == "" {
			return errors.New(nestedPath(path, "prompt") + " is required")
		}
	case TargetKindLoop:
		if !hasLoopTarget(job.LoopTarget) {
			return errors.New(nestedPath(path, "loop_target") + " is required when target_kind is \"loop\"")
		}
		if strings.TrimSpace(job.AgentName) != "" {
			return errors.New(nestedPath(path, "agent_name") + " must be empty when target_kind is \"loop\"")
		}
		if strings.TrimSpace(job.Prompt) != "" {
			return errors.New(nestedPath(path, "prompt") + " must be empty when target_kind is \"loop\"")
		}
		if job.Task != nil {
			return errors.New(nestedPath(path, "task") + " must be empty when target_kind is \"loop\"")
		}
		if err := job.LoopTarget.Validate(nestedPath(path, "loop_target"), job.Scope, job.WorkspaceID); err != nil {
			return err
		}
		if len(job.LoopTarget.InputMapping) > 0 {
			return errors.New(nestedPath(path, "loop_target.input_mapping") + " must be empty for scheduled jobs")
		}
	}
	return nil
}

func validateTriggerTarget(trigger Trigger, targetKind TargetKind, path string) error {
	switch targetKind {
	case TargetKindAgent:
		if hasLoopTarget(trigger.LoopTarget) {
			return errors.New(nestedPath(path, "loop_target") + " must be empty when target_kind is \"agent\"")
		}
		if strings.TrimSpace(trigger.AgentName) == "" {
			return errors.New(nestedPath(path, "agent_name") + " is required")
		}
		if strings.TrimSpace(trigger.Prompt) == "" {
			return errors.New(nestedPath(path, "prompt") + " is required")
		}
	case TargetKindLoop:
		if !hasLoopTarget(trigger.LoopTarget) {
			return errors.New(nestedPath(path, "loop_target") + " is required when target_kind is \"loop\"")
		}
		if strings.TrimSpace(trigger.AgentName) != "" {
			return errors.New(nestedPath(path, "agent_name") + " must be empty when target_kind is \"loop\"")
		}
		if strings.TrimSpace(trigger.Prompt) != "" {
			return errors.New(nestedPath(path, "prompt") + " must be empty when target_kind is \"loop\"")
		}
		return trigger.LoopTarget.Validate(nestedPath(path, "loop_target"), trigger.Scope, trigger.WorkspaceID)
	}
	return nil
}

// Validate ensures the loop target satisfies automation scope and basic payload shape.
func (t *LoopTarget) Validate(path string, scope Scope, automationWorkspaceID string) error {
	if t == nil {
		return errors.New(path + " is required")
	}
	if strings.TrimSpace(t.LoopName) == "" {
		return errors.New(nestedPath(path, "loop_name") + " is required")
	}
	targetWorkspaceID := strings.TrimSpace(t.WorkspaceID)
	switch scope {
	case AutomationScopeWorkspace:
		if targetWorkspaceID == "" {
			return errors.New(nestedPath(path, "workspace_id") + " is required")
		}
		if targetWorkspaceID != strings.TrimSpace(automationWorkspaceID) {
			return fmt.Errorf(
				"%s must equal automation workspace_id %q when scope is %q",
				nestedPath(path, "workspace_id"),
				strings.TrimSpace(automationWorkspaceID),
				AutomationScopeWorkspace,
			)
		}
	case AutomationScopeGlobal:
		if targetWorkspaceID == "" {
			return errors.New(nestedPath(path, "workspace_id") + " is required when automation scope is \"global\"")
		}
	default:
		if err := scope.Validate(nestedPath(path, "scope")); err != nil {
			return err
		}
	}
	if err := validateLoopTargetInputs(t.Inputs, nestedPath(path, "inputs")); err != nil {
		return err
	}
	if t.NetworkParticipation != nil {
		if _, err := participation.NormalizeIntent(*t.NetworkParticipation); err != nil {
			return fmt.Errorf("%s is invalid: %w", nestedPath(path, "network_participation"), err)
		}
	}
	return validateLoopTargetInputMapping(t.InputMapping, nestedPath(path, "input_mapping"))
}

func validateLoopTargetInputs(inputs map[string]any, path string) error {
	for rawKey := range inputs {
		if strings.TrimSpace(rawKey) == "" {
			return errors.New(path + " contains an empty input key")
		}
	}
	return nil
}

func validateLoopTargetInputMapping(mapping map[string]string, path string) error {
	// The loop start-binding layer validates the ADR-020 template grammar; the model layer only checks shape.
	for rawKey, rawValue := range mapping {
		key := strings.TrimSpace(rawKey)
		if key == "" {
			return errors.New(path + " contains an empty input key")
		}
		if strings.TrimSpace(rawValue) == "" {
			return fmt.Errorf("%s[%q] is required", path, rawKey)
		}
	}
	return nil
}

func validateDelegatedRunRefs(run Run, path string) error {
	taskID := strings.TrimSpace(run.TaskID)
	taskRunID := strings.TrimSpace(run.TaskRunID)
	loopRunID := strings.TrimSpace(run.LoopRunID)
	if taskRunID != "" && taskID == "" {
		return fmt.Errorf("%s is required when %s is set", nestedPath(path, "task_id"), nestedPath(path, "task_run_id"))
	}
	if loopRunID != "" && run.Status != RunDelegated {
		return fmt.Errorf(
			"%s requires %s to be %q",
			nestedPath(path, "loop_run_id"),
			nestedPath(path, "status"),
			RunDelegated,
		)
	}
	if run.Status != RunDelegated {
		return nil
	}
	taskDelegated := taskID != "" && taskRunID != ""
	loopDelegated := loopRunID != ""
	switch {
	case taskDelegated == loopDelegated:
		return fmt.Errorf(
			"%s requires exactly one of (%s and %s) or %s",
			nestedPath(path, "status"),
			nestedPath(path, "task_id"),
			nestedPath(path, "task_run_id"),
			nestedPath(path, "loop_run_id"),
		)
	case taskID != "" && taskRunID == "":
		return fmt.Errorf("%s is required when %s is set", nestedPath(path, "task_run_id"), nestedPath(path, "task_id"))
	default:
		return nil
	}
}

func normalizedTargetKind(kind TargetKind, loopTarget *LoopTarget) TargetKind {
	normalized := kind.Normalize()
	if normalized == "" {
		if hasLoopTarget(loopTarget) {
			return TargetKindLoop
		}
		return TargetKindAgent
	}
	return normalized
}

func hasLoopTarget(target *LoopTarget) bool {
	if target == nil {
		return false
	}
	return strings.TrimSpace(target.WorkspaceID) != "" ||
		strings.TrimSpace(target.LoopName) != "" ||
		len(target.Inputs) > 0 ||
		len(target.InputMapping) > 0 ||
		target.NetworkParticipation != nil
}
