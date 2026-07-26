package globaldb

import (
	"errors"
	"strings"
	"time"

	automation "github.com/compozy/agh/internal/automation/model"
	"github.com/compozy/agh/internal/network/participation"
	taskpkg "github.com/compozy/agh/internal/task"
)

func normalizeAutomationJob(job automation.Job) automation.Job {
	job.ID = strings.TrimSpace(job.ID)
	job.Scope = automation.Scope(strings.TrimSpace(string(job.Scope)))
	job.Name = strings.TrimSpace(job.Name)
	job.AgentName = strings.TrimSpace(job.AgentName)
	job.WorkspaceID = strings.TrimSpace(job.WorkspaceID)
	job.Source = automation.JobSource(strings.TrimSpace(string(job.Source)))
	job.LoopTarget = normalizeAutomationLoopTarget(job.LoopTarget)
	job.TargetKind = normalizeAutomationTargetKind(job.TargetKind, job.LoopTarget)
	job.Retry.BaseDelay = strings.TrimSpace(job.Retry.BaseDelay)
	job.Retry.Strategy = automation.RetryStrategy(strings.TrimSpace(string(job.Retry.Strategy)))
	job.FireLimit.Window = strings.TrimSpace(job.FireLimit.Window)
	if job.Schedule != nil {
		schedule := *job.Schedule
		schedule.Mode = automation.ScheduleMode(strings.TrimSpace(string(schedule.Mode)))
		schedule.Expr = strings.TrimSpace(schedule.Expr)
		schedule.Interval = strings.TrimSpace(schedule.Interval)
		schedule.Time = strings.TrimSpace(schedule.Time)
		job.Schedule = &schedule
	}
	if job.Task != nil {
		taskConfig := *job.Task
		taskConfig.Title = strings.TrimSpace(taskConfig.Title)
		taskConfig.Description = strings.TrimSpace(taskConfig.Description)
		taskConfig.NetworkParticipation = normalizeAutomationParticipationIntent(
			taskConfig.NetworkParticipation,
		)
		if taskConfig.Owner != nil {
			owner := *taskConfig.Owner
			owner.Kind = taskpkg.OwnerKind(strings.TrimSpace(string(owner.Kind)))
			owner.Ref = strings.TrimSpace(owner.Ref)
			taskConfig.Owner = &owner
		}
		job.Task = &taskConfig
	}
	return job
}

func normalizeAutomationTrigger(trigger automation.Trigger) automation.Trigger {
	trigger.ID = strings.TrimSpace(trigger.ID)
	trigger.Scope = automation.Scope(strings.TrimSpace(string(trigger.Scope)))
	trigger.Name = strings.TrimSpace(trigger.Name)
	trigger.AgentName = strings.TrimSpace(trigger.AgentName)
	trigger.WorkspaceID = strings.TrimSpace(trigger.WorkspaceID)
	trigger.Event = strings.TrimSpace(trigger.Event)
	trigger.Source = automation.JobSource(strings.TrimSpace(string(trigger.Source)))
	trigger.WebhookID = strings.TrimSpace(trigger.WebhookID)
	trigger.EndpointSlug = strings.TrimSpace(trigger.EndpointSlug)
	trigger.WebhookSecretRef = strings.TrimSpace(trigger.WebhookSecretRef)
	trigger.LoopTarget = normalizeAutomationLoopTarget(trigger.LoopTarget)
	trigger.TargetKind = normalizeAutomationTargetKind(trigger.TargetKind, trigger.LoopTarget)
	trigger.Retry.BaseDelay = strings.TrimSpace(trigger.Retry.BaseDelay)
	trigger.Retry.Strategy = automation.RetryStrategy(strings.TrimSpace(string(trigger.Retry.Strategy)))
	trigger.FireLimit.Window = strings.TrimSpace(trigger.FireLimit.Window)
	if len(trigger.Filter) > 0 {
		normalized := make(map[string]string, len(trigger.Filter))
		for rawKey, rawValue := range trigger.Filter {
			normalized[strings.TrimSpace(rawKey)] = strings.TrimSpace(rawValue)
		}
		trigger.Filter = normalized
	}
	return trigger
}

func normalizeAutomationRun(run automation.Run) automation.Run {
	run.ID = strings.TrimSpace(run.ID)
	run.JobID = strings.TrimSpace(run.JobID)
	run.TriggerID = strings.TrimSpace(run.TriggerID)
	run.SessionID = strings.TrimSpace(run.SessionID)
	run.TaskID = strings.TrimSpace(run.TaskID)
	run.TaskRunID = strings.TrimSpace(run.TaskRunID)
	run.LoopRunID = strings.TrimSpace(run.LoopRunID)
	run.FireID = strings.TrimSpace(run.FireID)
	run.Status = automation.RunStatus(strings.TrimSpace(string(run.Status)))
	run.Error = strings.TrimSpace(run.Error)
	run.DeliveryError = strings.TrimSpace(run.DeliveryError)
	run.NetworkParticipation = normalizeAutomationParticipationIntent(run.NetworkParticipation)
	return run
}

func normalizeAutomationParticipationIntent(
	request *participation.Request,
) *participation.Request {
	if request == nil {
		return nil
	}
	normalized, err := participation.NormalizeIntent(*request)
	if err != nil {
		return request
	}
	return &normalized
}

func normalizeJobOverlay(overlay automation.JobEnabledOverlay, now time.Time) (automation.JobEnabledOverlay, error) {
	overlay.JobID = strings.TrimSpace(overlay.JobID)
	if overlay.JobID == "" {
		return automation.JobEnabledOverlay{}, errors.New("store: automation job overlay id is required")
	}
	if overlay.UpdatedAt.IsZero() {
		overlay.UpdatedAt = now
	}
	return overlay, nil
}

func normalizeTriggerOverlay(
	overlay automation.TriggerEnabledOverlay,
	now time.Time,
) (automation.TriggerEnabledOverlay, error) {
	overlay.TriggerID = strings.TrimSpace(overlay.TriggerID)
	if overlay.TriggerID == "" {
		return automation.TriggerEnabledOverlay{}, errors.New("store: automation trigger overlay id is required")
	}
	if overlay.UpdatedAt.IsZero() {
		overlay.UpdatedAt = now
	}
	return overlay, nil
}
