package core

import (
	"maps"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	automationpkg "github.com/compozy/agh/internal/automation"
	"github.com/compozy/agh/internal/network/participation"
)

// AutomationSchedulerStatePayloadFromState converts durable scheduler metadata
// into the shared response payload.
func AutomationSchedulerStatePayloadFromState(
	state automationpkg.ScheduledJobState,
) contract.AutomationSchedulerStatePayload {
	payload := contract.AutomationSchedulerStatePayload{
		JobID:               state.JobID,
		Registered:          state.Registered,
		NextRunAt:           state.NextRun,
		LastRunAt:           state.LastRun,
		LastScheduledAt:     state.LastScheduledAt,
		LastFireID:          state.LastFireID,
		CatchUpPolicy:       state.CatchUpPolicy,
		MisfireGraceSeconds: state.MisfireGraceSeconds,
		LastMisfireAt:       state.LastMisfireAt,
		MisfireCount:        state.MisfireCount,
	}
	if state.Durable != nil {
		payload.ConsecutiveResumeFailures = state.Durable.ConsecutiveResumeFailures
		updatedAt := state.Durable.UpdatedAt
		if !updatedAt.IsZero() {
			payload.UpdatedAt = &updatedAt
		}
	}
	return payload
}

// AutomationSchedulerStatePayloadsFromStates converts scheduler states into response payloads.
func AutomationSchedulerStatePayloadsFromStates(
	states []automationpkg.ScheduledJobState,
) []contract.AutomationSchedulerStatePayload {
	payloads := make([]contract.AutomationSchedulerStatePayload, 0, len(states))
	for _, state := range states {
		payloads = append(payloads, AutomationSchedulerStatePayloadFromState(state))
	}
	return payloads
}

// JobPayloadFromJob converts an automation job into the shared response
// payload, optionally enriching it with scheduler next-run metadata.
func JobPayloadFromJob(
	job automationpkg.Job,
	nextRun *time.Time,
	schedulerState *contract.AutomationSchedulerStatePayload,
) contract.JobPayload {
	payload := contract.JobPayload{
		ID:          job.ID,
		Scope:       job.Scope,
		Name:        job.Name,
		TargetKind:  job.TargetKind,
		AgentName:   job.AgentName,
		WorkspaceID: job.WorkspaceID,
		Prompt:      job.Prompt,
		LoopTarget:  cloneAutomationLoopTarget(job.LoopTarget),
		Enabled:     job.Enabled,
		Retry:       job.Retry,
		FireLimit:   job.FireLimit,
		Source:      job.Source,
		CreatedAt:   job.CreatedAt,
		UpdatedAt:   job.UpdatedAt,
		NextRun:     nextRun,
		Scheduler:   schedulerState,
	}
	if job.Schedule != nil {
		schedule := *job.Schedule
		payload.Schedule = &schedule
	}
	if job.Task != nil {
		taskConfig := *job.Task
		if job.Task.Owner != nil {
			owner := *job.Task.Owner
			taskConfig.Owner = &owner
		}
		payload.Task = &taskConfig
	}
	return payload
}

// JobPayloadsFromJobs converts a slice of jobs into response payloads using
// the supplied next-run map.
func JobPayloadsFromJobs(
	jobs []automationpkg.Job,
	schedulerStateByID map[string]contract.AutomationSchedulerStatePayload,
) []contract.JobPayload {
	payloads := make([]contract.JobPayload, 0, len(jobs))
	for _, job := range jobs {
		var schedulerState *contract.AutomationSchedulerStatePayload
		if state, ok := schedulerStateByID[job.ID]; ok {
			stateCopy := state
			schedulerState = &stateCopy
		}
		payloads = append(payloads, JobPayloadFromJob(job, schedulerNextRun(schedulerState), schedulerState))
	}
	return payloads
}

func schedulerNextRun(state *contract.AutomationSchedulerStatePayload) *time.Time {
	if state == nil || state.NextRunAt == nil {
		return nil
	}
	nextRun := state.NextRunAt.UTC()
	return &nextRun
}

// TriggerPayloadFromTrigger converts an automation trigger into the shared response payload.
func TriggerPayloadFromTrigger(trigger automationpkg.Trigger) contract.TriggerPayload {
	return contract.TriggerPayloadFromTrigger(trigger)
}

// TriggerPayloadsFromTriggers converts a slice of triggers into response payloads.
func TriggerPayloadsFromTriggers(triggers []automationpkg.Trigger) []contract.TriggerPayload {
	return contract.TriggerPayloadsFromTriggers(triggers)
}

// RunPayloadFromRun converts an automation run into the shared response payload.
func RunPayloadFromRun(run automationpkg.Run) contract.RunPayload {
	return contract.RunPayload{
		ID:                   run.ID,
		JobID:                run.JobID,
		TriggerID:            run.TriggerID,
		SessionID:            run.SessionID,
		TaskID:               run.TaskID,
		TaskRunID:            run.TaskRunID,
		LoopRunID:            run.LoopRunID,
		FireID:               run.FireID,
		Status:               run.Status,
		Attempt:              run.Attempt,
		NetworkParticipation: participation.CloneRequest(run.NetworkParticipation),
		ScheduledAt:          run.ScheduledAt,
		StartedAt:            run.StartedAt,
		EndedAt:              run.EndedAt,
		Error:                run.Error,
		DeliveryError:        run.DeliveryError,
		DeliveryErrorAt:      run.DeliveryErrorAt,
		Metadata:             maps.Clone(run.Metadata),
	}
}

// RunPayloadsFromRuns converts a slice of runs into response payloads.
func RunPayloadsFromRuns(runs []automationpkg.Run) []contract.RunPayload {
	payloads := make([]contract.RunPayload, 0, len(runs))
	for _, run := range runs {
		payloads = append(payloads, RunPayloadFromRun(run))
	}
	return payloads
}

func cloneAutomationLoopTarget(source *automationpkg.LoopTarget) *automationpkg.LoopTarget {
	if source == nil {
		return nil
	}
	cloned := *source
	if len(source.Inputs) > 0 {
		cloned.Inputs = make(map[string]any, len(source.Inputs))
		maps.Copy(cloned.Inputs, source.Inputs)
	}
	if len(source.InputMapping) > 0 {
		cloned.InputMapping = make(map[string]string, len(source.InputMapping))
		maps.Copy(cloned.InputMapping, source.InputMapping)
	}
	return &cloned
}
