package globaldb

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	automation "github.com/compozy/agh/internal/automation/model"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

func requireAutomationAffected(affected int64, notFound error, id string, label string) error {
	if affected == 0 {
		return fmt.Errorf("store: %s %q: %w", label, id, notFound)
	}
	return nil
}

func nullableAutomationString(value string) sql.NullString {
	return store.SQLNullString(value)
}

func nullableAutomationTime(value *time.Time) sql.NullString {
	if value == nil || value.IsZero() {
		return sql.NullString{}
	}
	return sql.NullString{String: store.FormatTimestamp(*value), Valid: true}
}

func nullableAutomationJSON(value any, label string) (sql.NullString, error) {
	if value == nil {
		return sql.NullString{}, nil
	}
	text, ok := value.(string)
	if !ok {
		return sql.NullString{}, fmt.Errorf("store: %s encoded as %T, want string", label, value)
	}
	return nullableAutomationString(text), nil
}

func automationRunParams(
	run automation.Run,
	networkParticipation sql.NullString,
	metadataJSON string,
) sqlcgen.InsertAutomationRunParams {
	return sqlcgen.InsertAutomationRunParams{
		ID:            run.ID,
		JobID:         nullableAutomationString(run.JobID),
		TriggerID:     nullableAutomationString(run.TriggerID),
		SessionID:     nullableAutomationString(run.SessionID),
		TaskID:        nullableAutomationString(run.TaskID),
		TaskRunID:     nullableAutomationString(run.TaskRunID),
		FireID:        nullableAutomationString(run.FireID),
		Status:        string(run.Status),
		Attempt:       int64(run.Attempt),
		ScheduledAt:   nullableAutomationTime(run.ScheduledAt),
		StartedAt:     nullableAutomationTime(run.StartedAt),
		EndedAt:       nullableAutomationTime(run.EndedAt),
		Error:         nullableAutomationString(run.Error),
		DeliveryError: nullableAutomationString(run.DeliveryError),
		DeliveryErrorAt: nullableAutomationTime(
			run.DeliveryErrorAt,
		),
		LoopRunID:            nullableAutomationString(run.LoopRunID),
		NetworkParticipation: networkParticipation,
		MetadataJson:         metadataJSON,
	}
}

func automationRunUpdateParams(
	run automation.Run,
	networkParticipation sql.NullString,
	metadataJSON string,
) sqlcgen.UpdateAutomationRunParams {
	insert := automationRunParams(run, networkParticipation, metadataJSON)
	return sqlcgen.UpdateAutomationRunParams{
		JobID: insert.JobID, TriggerID: insert.TriggerID, SessionID: insert.SessionID,
		TaskID: insert.TaskID, TaskRunID: insert.TaskRunID, FireID: insert.FireID,
		Status: insert.Status, Attempt: insert.Attempt, ScheduledAt: insert.ScheduledAt,
		StartedAt: insert.StartedAt, EndedAt: insert.EndedAt, Error: insert.Error,
		DeliveryError: insert.DeliveryError, DeliveryErrorAt: insert.DeliveryErrorAt,
		LoopRunID: insert.LoopRunID, NetworkParticipation: insert.NetworkParticipation,
		MetadataJson: metadataJSON, ID: run.ID,
	}
}

func automationRunFromGenerated(row sqlcgen.AutomationRun) (automation.Run, error) {
	run := automation.Run{
		ID:        row.ID,
		JobID:     automationNullStringValue(row.JobID),
		TriggerID: automationNullStringValue(row.TriggerID),
		SessionID: automationNullStringValue(row.SessionID),
		TaskID:    automationNullStringValue(row.TaskID),
		TaskRunID: automationNullStringValue(row.TaskRunID),
		FireID:    automationNullStringValue(row.FireID),
		LoopRunID: automationNullStringValue(
			row.LoopRunID,
		),
		Status:  automation.RunStatus(strings.TrimSpace(row.Status)),
		Attempt: int(row.Attempt),
	}
	if err := assignAutomationRunTimestamps(
		&run,
		row.ScheduledAt,
		row.StartedAt,
		row.EndedAt,
		row.DeliveryErrorAt,
	); err != nil {
		return automation.Run{}, err
	}
	assignAutomationRunErrors(&run, row.Error, row.DeliveryError)
	if err := decodeOptionalAutomationJSON(
		row.NetworkParticipation,
		&run.NetworkParticipation,
		"run.network_participation",
	); err != nil {
		return automation.Run{}, err
	}
	if err := decodeAutomationRunMetadata(row.MetadataJson, &run.Metadata); err != nil {
		return automation.Run{}, err
	}
	return run, nil
}

func automationRunFromGetGenerated(row sqlcgen.GetAutomationRunRow) (automation.Run, error) {
	return automationRunFromGenerated(sqlcgen.AutomationRun{
		ID: row.ID, JobID: row.JobID, TriggerID: row.TriggerID, SessionID: row.SessionID,
		TaskID: row.TaskID, TaskRunID: row.TaskRunID, FireID: row.FireID, Status: row.Status,
		Attempt: row.Attempt, ScheduledAt: row.ScheduledAt, StartedAt: row.StartedAt,
		EndedAt: row.EndedAt, Error: row.Error, DeliveryError: row.DeliveryError,
		DeliveryErrorAt: row.DeliveryErrorAt, LoopRunID: row.LoopRunID,
		NetworkParticipation: row.NetworkParticipation, MetadataJson: row.MetadataJson,
	})
}

func automationJobParams(job automation.Job) (sqlcgen.InsertAutomationJobParams, error) {
	scheduleJSON, taskJSON, retryJSON, fireLimitJSON, loopTarget, err := encodeJobRecord(job)
	if err != nil {
		return sqlcgen.InsertAutomationJobParams{}, err
	}
	task := sql.NullString{}
	if taskJSON != nil {
		taskText, ok := taskJSON.(string)
		if !ok {
			return sqlcgen.InsertAutomationJobParams{}, fmt.Errorf(
				"store: job.task encoded as %T, want string",
				taskJSON,
			)
		}
		task = nullableAutomationString(taskText)
	}
	loopInputs, err := nullableAutomationJSON(loopTarget.inputsJSON, "job.loop_inputs")
	if err != nil {
		return sqlcgen.InsertAutomationJobParams{}, err
	}
	loopInputMapping, err := nullableAutomationJSON(loopTarget.inputMappingJSON, "job.loop_input_mapping")
	if err != nil {
		return sqlcgen.InsertAutomationJobParams{}, err
	}
	loopNetworkParticipation, err := nullableAutomationJSON(
		loopTarget.networkParticipationJSON,
		"job.loop_network_participation",
	)
	if err != nil {
		return sqlcgen.InsertAutomationJobParams{}, err
	}
	return sqlcgen.InsertAutomationJobParams{
		ID:          job.ID,
		Scope:       string(job.Scope),
		Name:        job.Name,
		AgentName:   job.AgentName,
		WorkspaceID: nullableAutomationString(job.WorkspaceID),
		Prompt:      job.Prompt,
		Schedule:    nullableAutomationString(scheduleJSON),
		Task:        task,
		Enabled:     job.Enabled,
		Retry:       retryJSON,
		FireLimit:   fireLimitJSON,
		Source:      string(job.Source),
		TargetKind:  string(job.TargetKind),
		LoopWorkspaceID: nullableAutomationString(
			loopTarget.workspaceID,
		),
		LoopName:                 nullableAutomationString(loopTarget.loopName),
		LoopInputs:               loopInputs,
		LoopInputMapping:         loopInputMapping,
		LoopNetworkParticipation: loopNetworkParticipation,
		CreatedAt:                store.FormatTimestamp(job.CreatedAt),
		UpdatedAt:                store.FormatTimestamp(job.UpdatedAt),
	}, nil
}

func automationJobUpdateParams(job automation.Job) (sqlcgen.UpdateAutomationJobParams, error) {
	insert, err := automationJobParams(job)
	if err != nil {
		return sqlcgen.UpdateAutomationJobParams{}, err
	}
	return sqlcgen.UpdateAutomationJobParams{
		Scope: insert.Scope, Name: insert.Name, AgentName: insert.AgentName, WorkspaceID: insert.WorkspaceID,
		Prompt: insert.Prompt, Schedule: insert.Schedule, Task: insert.Task, Enabled: insert.Enabled,
		Retry: insert.Retry, FireLimit: insert.FireLimit, Source: insert.Source, TargetKind: insert.TargetKind,
		LoopWorkspaceID: insert.LoopWorkspaceID, LoopName: insert.LoopName, LoopInputs: insert.LoopInputs,
		LoopInputMapping:         insert.LoopInputMapping,
		LoopNetworkParticipation: insert.LoopNetworkParticipation,
		UpdatedAt:                insert.UpdatedAt, ID: job.ID,
	}, nil
}

func automationJobFromGenerated(row sqlcgen.AutomationJob) (automation.Job, error) {
	job := automation.Job{
		ID: row.ID, Scope: automation.Scope(strings.TrimSpace(row.Scope)), Name: row.Name,
		AgentName: row.AgentName, WorkspaceID: automationNullStringValue(row.WorkspaceID), Prompt: row.Prompt,
		Enabled: row.Enabled, Source: automation.JobSource(strings.TrimSpace(row.Source)),
		TargetKind: automation.TargetKind(strings.TrimSpace(row.TargetKind)),
	}
	if err := decodeAutomationSchedule(row.Schedule, &job.Schedule); err != nil {
		return automation.Job{}, err
	}
	if err := decodeAutomationTaskConfig(row.Task, &job.Task); err != nil {
		return automation.Job{}, err
	}
	if err := decodeAutomationJSON(row.Retry, &job.Retry, "job.retry"); err != nil {
		return automation.Job{}, err
	}
	if err := decodeAutomationJSON(row.FireLimit, &job.FireLimit, "job.fire_limit"); err != nil {
		return automation.Job{}, err
	}
	loopTarget := automationLoopTargetRecord{
		workspaceID: row.LoopWorkspaceID, loopName: row.LoopName,
		inputsRaw: row.LoopInputs, inputMappingRaw: row.LoopInputMapping,
		networkParticipationRaw: row.LoopNetworkParticipation,
	}
	if err := decodeAutomationLoopTarget(loopTarget, job.TargetKind, &job.LoopTarget, "job.loop_target"); err != nil {
		return automation.Job{}, err
	}
	createdAt, err := store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return automation.Job{}, err
	}
	updatedAt, err := store.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return automation.Job{}, err
	}
	job.CreatedAt, job.UpdatedAt = createdAt, updatedAt
	return job, nil
}

func automationTriggerParams(trigger automation.Trigger) (sqlcgen.InsertAutomationTriggerParams, error) {
	filterJSON, retryJSON, fireLimitJSON, loopTarget, err := encodeTriggerRecord(trigger)
	if err != nil {
		return sqlcgen.InsertAutomationTriggerParams{}, err
	}
	filter := sql.NullString{}
	if filterJSON != nil {
		filterText, ok := filterJSON.(string)
		if !ok {
			return sqlcgen.InsertAutomationTriggerParams{}, fmt.Errorf(
				"store: trigger.filter encoded as %T, want string",
				filterJSON,
			)
		}
		filter = nullableAutomationString(filterText)
	}
	loopInputs, err := nullableAutomationJSON(loopTarget.inputsJSON, "trigger.loop_inputs")
	if err != nil {
		return sqlcgen.InsertAutomationTriggerParams{}, err
	}
	loopInputMapping, err := nullableAutomationJSON(loopTarget.inputMappingJSON, "trigger.loop_input_mapping")
	if err != nil {
		return sqlcgen.InsertAutomationTriggerParams{}, err
	}
	loopNetworkParticipation, err := nullableAutomationJSON(
		loopTarget.networkParticipationJSON,
		"trigger.loop_network_participation",
	)
	if err != nil {
		return sqlcgen.InsertAutomationTriggerParams{}, err
	}
	return sqlcgen.InsertAutomationTriggerParams{
		ID:          trigger.ID,
		Scope:       string(trigger.Scope),
		Name:        trigger.Name,
		AgentName:   trigger.AgentName,
		WorkspaceID: nullableAutomationString(trigger.WorkspaceID),
		Prompt:      trigger.Prompt,
		Event:       trigger.Event,
		Filter:      filter,
		Enabled:     trigger.Enabled,
		Retry:       retryJSON,
		FireLimit:   fireLimitJSON,
		Source:      string(trigger.Source),
		WebhookID:   nullableAutomationString(trigger.WebhookID),
		EndpointSlug: nullableAutomationString(
			trigger.EndpointSlug,
		),
		WebhookSecretRef:         nullableAutomationString(trigger.WebhookSecretRef),
		TargetKind:               string(trigger.TargetKind),
		LoopWorkspaceID:          nullableAutomationString(loopTarget.workspaceID),
		LoopName:                 nullableAutomationString(loopTarget.loopName),
		LoopInputs:               loopInputs,
		LoopInputMapping:         loopInputMapping,
		LoopNetworkParticipation: loopNetworkParticipation,
		CreatedAt:                store.FormatTimestamp(trigger.CreatedAt),
		UpdatedAt:                store.FormatTimestamp(trigger.UpdatedAt),
	}, nil
}

func automationTriggerUpdateParams(trigger automation.Trigger) (sqlcgen.UpdateAutomationTriggerParams, error) {
	insert, err := automationTriggerParams(trigger)
	if err != nil {
		return sqlcgen.UpdateAutomationTriggerParams{}, err
	}
	return sqlcgen.UpdateAutomationTriggerParams{
		Scope: insert.Scope, Name: insert.Name, AgentName: insert.AgentName, WorkspaceID: insert.WorkspaceID,
		Prompt: insert.Prompt, Event: insert.Event, Filter: insert.Filter, Enabled: insert.Enabled,
		Retry: insert.Retry, FireLimit: insert.FireLimit, Source: insert.Source, WebhookID: insert.WebhookID,
		EndpointSlug: insert.EndpointSlug, WebhookSecretRef: insert.WebhookSecretRef, TargetKind: insert.TargetKind,
		LoopWorkspaceID: insert.LoopWorkspaceID, LoopName: insert.LoopName, LoopInputs: insert.LoopInputs,
		LoopInputMapping:         insert.LoopInputMapping,
		LoopNetworkParticipation: insert.LoopNetworkParticipation,
		UpdatedAt:                insert.UpdatedAt, ID: trigger.ID,
	}, nil
}

func automationTriggerFromGenerated(row sqlcgen.AutomationTrigger) (automation.Trigger, error) {
	trigger := automation.Trigger{
		ID: row.ID, Scope: automation.Scope(strings.TrimSpace(row.Scope)), Name: row.Name,
		AgentName: row.AgentName, WorkspaceID: automationNullStringValue(row.WorkspaceID), Prompt: row.Prompt,
		Event: row.Event, Enabled: row.Enabled, Source: automation.JobSource(strings.TrimSpace(row.Source)),
		WebhookID: automationNullStringValue(row.WebhookID), EndpointSlug: automationNullStringValue(row.EndpointSlug),
		WebhookSecretRef: automationNullStringValue(row.WebhookSecretRef),
		TargetKind:       automation.TargetKind(strings.TrimSpace(row.TargetKind)),
	}
	if err := decodeAutomationFilter(row.Filter, &trigger.Filter); err != nil {
		return automation.Trigger{}, err
	}
	if err := decodeAutomationJSON(row.Retry, &trigger.Retry, "trigger.retry"); err != nil {
		return automation.Trigger{}, err
	}
	if err := decodeAutomationJSON(row.FireLimit, &trigger.FireLimit, "trigger.fire_limit"); err != nil {
		return automation.Trigger{}, err
	}
	loopTarget := automationLoopTargetRecord{
		workspaceID: row.LoopWorkspaceID, loopName: row.LoopName,
		inputsRaw: row.LoopInputs, inputMappingRaw: row.LoopInputMapping,
		networkParticipationRaw: row.LoopNetworkParticipation,
	}
	if err := decodeAutomationLoopTarget(
		loopTarget,
		trigger.TargetKind,
		&trigger.LoopTarget,
		"trigger.loop_target",
	); err != nil {
		return automation.Trigger{}, err
	}
	if err := assignAutomationTriggerTimestamps(&trigger, row.CreatedAt, row.UpdatedAt); err != nil {
		return automation.Trigger{}, err
	}
	return trigger, nil
}

func automationJobOverlayFromGenerated(row sqlcgen.AutomationJobOverlay) (automation.JobEnabledOverlay, error) {
	updatedAt, err := store.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return automation.JobEnabledOverlay{}, err
	}
	return automation.JobEnabledOverlay{
		JobID:           row.JobID,
		EnabledOverride: row.EnabledOverride,
		UpdatedAt:       updatedAt,
	}, nil
}

func automationTriggerOverlayFromGenerated(
	row sqlcgen.AutomationTriggerOverlay,
) (automation.TriggerEnabledOverlay, error) {
	updatedAt, err := store.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return automation.TriggerEnabledOverlay{}, err
	}
	return automation.TriggerEnabledOverlay{
		TriggerID:       row.TriggerID,
		EnabledOverride: row.EnabledOverride,
		UpdatedAt:       updatedAt,
	}, nil
}

func automationSchedulerParams(state automation.SchedulerState) sqlcgen.UpsertAutomationSchedulerStateParams {
	return sqlcgen.UpsertAutomationSchedulerStateParams{
		JobID:     state.JobID,
		NextRunAt: nullableAutomationTime(state.NextRunAt),
		LastRunAt: nullableAutomationTime(
			state.LastRunAt,
		),
		LastScheduledAt:           nullableAutomationTime(state.LastScheduledAt),
		LastFireID:                state.LastFireID,
		ScheduleHash:              state.ScheduleHash,
		CatchUpPolicy:             string(state.CatchUpPolicy),
		MisfireGraceSeconds:       int64(state.MisfireGraceSeconds),
		ConsecutiveResumeFailures: int64(state.ConsecutiveResumeFailures),
		LastMisfireAt:             nullableAutomationTime(state.LastMisfireAt),
		MisfireCount:              int64(state.MisfireCount),
		UpdatedAt:                 store.FormatTimestamp(state.UpdatedAt),
	}
}

func automationSchedulerFromGenerated(row sqlcgen.AutomationSchedulerState) (automation.SchedulerState, error) {
	state := automation.SchedulerState{
		JobID:                     row.JobID,
		LastFireID:                strings.TrimSpace(row.LastFireID),
		ScheduleHash:              strings.TrimSpace(row.ScheduleHash),
		CatchUpPolicy:             automation.SchedulerCatchUpPolicy(strings.TrimSpace(row.CatchUpPolicy)),
		MisfireGraceSeconds:       int(row.MisfireGraceSeconds),
		ConsecutiveResumeFailures: int(row.ConsecutiveResumeFailures),
		MisfireCount:              int(row.MisfireCount),
	}
	var err error
	if state.NextRunAt, err = parseNullableAutomationTime(row.NextRunAt); err != nil {
		return automation.SchedulerState{}, err
	}
	if state.LastRunAt, err = parseNullableAutomationTime(row.LastRunAt); err != nil {
		return automation.SchedulerState{}, err
	}
	if state.LastScheduledAt, err = parseNullableAutomationTime(row.LastScheduledAt); err != nil {
		return automation.SchedulerState{}, err
	}
	if state.LastMisfireAt, err = parseNullableAutomationTime(row.LastMisfireAt); err != nil {
		return automation.SchedulerState{}, err
	}
	state.UpdatedAt, err = store.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return automation.SchedulerState{}, err
	}
	if err := state.Validate("scheduler_state"); err != nil {
		return automation.SchedulerState{}, err
	}
	return state, nil
}

func automationSQLiteBool(value any) (bool, error) {
	switch typed := value.(type) {
	case bool:
		return typed, nil
	case int64:
		return typed != 0, nil
	case nil:
		return false, nil
	default:
		return false, fmt.Errorf("store: automation enabled value has SQLite type %T", value)
	}
}

func automationJobFromHydrated(row sqlcgen.HydrateAutomationJobCatalogRow) (automation.Job, error) {
	enabled, err := automationSQLiteBool(row.Enabled)
	if err != nil {
		return automation.Job{}, err
	}
	return automationJobFromGenerated(sqlcgen.AutomationJob{
		ID: row.ID, Scope: row.Scope, Name: row.Name, AgentName: row.AgentName,
		WorkspaceID: row.WorkspaceID, Prompt: row.Prompt, Schedule: row.Schedule, Task: row.Task,
		Enabled: enabled, Retry: row.Retry, FireLimit: row.FireLimit, Source: row.Source,
		TargetKind: row.TargetKind, LoopWorkspaceID: row.LoopWorkspaceID, LoopName: row.LoopName,
		LoopInputs: row.LoopInputs, LoopInputMapping: row.LoopInputMapping,
		LoopNetworkParticipation: row.LoopNetworkParticipation,
		CreatedAt:                row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
}

func automationTriggerFromHydrated(row sqlcgen.HydrateAutomationTriggerCatalogRow) (automation.Trigger, error) {
	enabled, err := automationSQLiteBool(row.Enabled)
	if err != nil {
		return automation.Trigger{}, err
	}
	return automationTriggerFromGenerated(sqlcgen.AutomationTrigger{
		ID: row.ID, Scope: row.Scope, Name: row.Name, AgentName: row.AgentName,
		WorkspaceID: row.WorkspaceID, Prompt: row.Prompt, Event: row.Event, Filter: row.Filter,
		Enabled: enabled, Retry: row.Retry, FireLimit: row.FireLimit, Source: row.Source,
		WebhookID: row.WebhookID, EndpointSlug: row.EndpointSlug, WebhookSecretRef: row.WebhookSecretRef,
		TargetKind: row.TargetKind, LoopWorkspaceID: row.LoopWorkspaceID, LoopName: row.LoopName,
		LoopInputs: row.LoopInputs, LoopInputMapping: row.LoopInputMapping,
		LoopNetworkParticipation: row.LoopNetworkParticipation,
		CreatedAt:                row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
}
