package globaldb

import (
	"database/sql"
	"fmt"
	"strings"

	automation "github.com/compozy/agh/internal/automation/model"
	"github.com/compozy/agh/internal/store"
)

func scanAutomationJob(scanner rowScanner) (automation.Job, error) {
	var (
		job         automation.Job
		scope       string
		workspaceID sql.NullString
		scheduleRaw sql.NullString
		taskRaw     sql.NullString
		retryRaw    string
		fireLimit   string
		source      string
		targetKind  string
		loopTarget  automationLoopTargetRecord
		createdAt   string
		updatedAt   string
	)
	if err := scanner.Scan(
		&job.ID,
		&scope,
		&job.Name,
		&job.AgentName,
		&workspaceID,
		&job.Prompt,
		&scheduleRaw,
		&taskRaw,
		&job.Enabled,
		&retryRaw,
		&fireLimit,
		&source,
		&targetKind,
		&loopTarget.workspaceID,
		&loopTarget.loopName,
		&loopTarget.inputsRaw,
		&loopTarget.inputMappingRaw,
		&loopTarget.networkParticipationRaw,
		&createdAt,
		&updatedAt,
	); err != nil {
		return automation.Job{}, fmt.Errorf("store: scan automation job: %w", err)
	}

	job.Scope = automation.Scope(strings.TrimSpace(scope))
	job.WorkspaceID = automationNullStringValue(workspaceID)
	job.Source = automation.JobSource(strings.TrimSpace(source))
	job.TargetKind = automation.TargetKind(strings.TrimSpace(targetKind))

	if err := decodeAutomationSchedule(scheduleRaw, &job.Schedule); err != nil {
		return automation.Job{}, err
	}
	if err := decodeAutomationTaskConfig(taskRaw, &job.Task); err != nil {
		return automation.Job{}, err
	}
	if err := decodeAutomationJSON(retryRaw, &job.Retry, "job.retry"); err != nil {
		return automation.Job{}, err
	}
	if err := decodeAutomationJSON(fireLimit, &job.FireLimit, "job.fire_limit"); err != nil {
		return automation.Job{}, err
	}
	if err := decodeAutomationLoopTarget(loopTarget, job.TargetKind, &job.LoopTarget, "job.loop_target"); err != nil {
		return automation.Job{}, err
	}

	parsedCreatedAt, err := store.ParseTimestamp(createdAt)
	if err != nil {
		return automation.Job{}, err
	}
	parsedUpdatedAt, err := store.ParseTimestamp(updatedAt)
	if err != nil {
		return automation.Job{}, err
	}
	job.CreatedAt = parsedCreatedAt
	job.UpdatedAt = parsedUpdatedAt

	return job, nil
}

func scanAutomationTrigger(scanner rowScanner) (automation.Trigger, error) {
	var (
		trigger          automation.Trigger
		scope            string
		workspaceID      sql.NullString
		filterRaw        sql.NullString
		retryRaw         string
		fireLimitRaw     string
		source           string
		webhookID        sql.NullString
		endpointSlug     sql.NullString
		webhookSecretRef sql.NullString
		targetKind       string
		loopTarget       automationLoopTargetRecord
		createdAt        string
		updatedAt        string
	)
	if err := scanner.Scan(
		&trigger.ID,
		&scope,
		&trigger.Name,
		&trigger.AgentName,
		&workspaceID,
		&trigger.Prompt,
		&trigger.Event,
		&filterRaw,
		&trigger.Enabled,
		&retryRaw,
		&fireLimitRaw,
		&source,
		&webhookID,
		&endpointSlug,
		&webhookSecretRef,
		&targetKind,
		&loopTarget.workspaceID,
		&loopTarget.loopName,
		&loopTarget.inputsRaw,
		&loopTarget.inputMappingRaw,
		&loopTarget.networkParticipationRaw,
		&createdAt,
		&updatedAt,
	); err != nil {
		return automation.Trigger{}, fmt.Errorf("store: scan automation trigger: %w", err)
	}

	trigger.Scope = automation.Scope(strings.TrimSpace(scope))
	trigger.WorkspaceID = automationNullStringValue(workspaceID)
	trigger.Source = automation.JobSource(strings.TrimSpace(source))
	trigger.WebhookID = automationNullStringValue(webhookID)
	trigger.EndpointSlug = automationNullStringValue(endpointSlug)
	trigger.WebhookSecretRef = automationNullStringValue(webhookSecretRef)
	trigger.TargetKind = automation.TargetKind(strings.TrimSpace(targetKind))

	if err := decodeAutomationFilter(filterRaw, &trigger.Filter); err != nil {
		return automation.Trigger{}, err
	}
	if err := decodeAutomationJSON(retryRaw, &trigger.Retry, "trigger.retry"); err != nil {
		return automation.Trigger{}, err
	}
	if err := decodeAutomationJSON(fireLimitRaw, &trigger.FireLimit, "trigger.fire_limit"); err != nil {
		return automation.Trigger{}, err
	}
	if err := decodeAutomationLoopTarget(
		loopTarget,
		trigger.TargetKind,
		&trigger.LoopTarget,
		"trigger.loop_target",
	); err != nil {
		return automation.Trigger{}, err
	}

	if err := assignAutomationTriggerTimestamps(&trigger, createdAt, updatedAt); err != nil {
		return automation.Trigger{}, err
	}

	return trigger, nil
}

func assignAutomationTriggerTimestamps(trigger *automation.Trigger, createdAt string, updatedAt string) error {
	parsedCreatedAt, err := store.ParseTimestamp(createdAt)
	if err != nil {
		return err
	}
	parsedUpdatedAt, err := store.ParseTimestamp(updatedAt)
	if err != nil {
		return err
	}
	trigger.CreatedAt = parsedCreatedAt
	trigger.UpdatedAt = parsedUpdatedAt
	return nil
}

func scanAutomationRun(scanner rowScanner) (automation.Run, error) {
	var (
		run                     automation.Run
		jobID                   sql.NullString
		triggerID               sql.NullString
		sessionID               sql.NullString
		taskID                  sql.NullString
		taskRunID               sql.NullString
		fireID                  sql.NullString
		status                  string
		scheduledAt             sql.NullString
		startedAt               sql.NullString
		endedAt                 sql.NullString
		runErr                  sql.NullString
		deliveryErr             sql.NullString
		deliveryErrAt           sql.NullString
		loopRunID               sql.NullString
		networkParticipationRaw sql.NullString
		metadataRaw             string
	)
	if err := scanner.Scan(
		&run.ID,
		&jobID,
		&triggerID,
		&sessionID,
		&taskID,
		&taskRunID,
		&fireID,
		&status,
		&run.Attempt,
		&scheduledAt,
		&startedAt,
		&endedAt,
		&runErr,
		&deliveryErr,
		&deliveryErrAt,
		&loopRunID,
		&networkParticipationRaw,
		&metadataRaw,
	); err != nil {
		return automation.Run{}, fmt.Errorf("store: scan automation run: %w", err)
	}

	run.JobID = automationNullStringValue(jobID)
	run.TriggerID = automationNullStringValue(triggerID)
	run.SessionID = automationNullStringValue(sessionID)
	run.TaskID = automationNullStringValue(taskID)
	run.TaskRunID = automationNullStringValue(taskRunID)
	run.FireID = automationNullStringValue(fireID)
	run.LoopRunID = automationNullStringValue(loopRunID)
	run.Status = automation.RunStatus(strings.TrimSpace(status))
	if err := assignAutomationRunTimestamps(&run, scheduledAt, startedAt, endedAt, deliveryErrAt); err != nil {
		return automation.Run{}, err
	}
	assignAutomationRunErrors(&run, runErr, deliveryErr)
	if err := decodeOptionalAutomationJSON(
		networkParticipationRaw,
		&run.NetworkParticipation,
		"run.network_participation",
	); err != nil {
		return automation.Run{}, err
	}
	if err := decodeAutomationRunMetadata(metadataRaw, &run.Metadata); err != nil {
		return automation.Run{}, err
	}

	return run, nil
}

func assignAutomationRunTimestamps(
	run *automation.Run,
	scheduledAt sql.NullString,
	startedAt sql.NullString,
	endedAt sql.NullString,
	deliveryErrAt sql.NullString,
) error {
	if scheduledAt.Valid {
		value, err := store.ParseTimestamp(scheduledAt.String)
		if err != nil {
			return err
		}
		run.ScheduledAt = &value
	}
	if startedAt.Valid {
		value, err := store.ParseTimestamp(startedAt.String)
		if err != nil {
			return err
		}
		run.StartedAt = &value
	}
	if endedAt.Valid {
		value, err := store.ParseTimestamp(endedAt.String)
		if err != nil {
			return err
		}
		run.EndedAt = &value
	}
	if deliveryErrAt.Valid {
		value, err := store.ParseTimestamp(deliveryErrAt.String)
		if err != nil {
			return err
		}
		run.DeliveryErrorAt = &value
	}
	return nil
}

func assignAutomationRunErrors(run *automation.Run, runErr sql.NullString, deliveryErr sql.NullString) {
	if runErr.Valid {
		run.Error = runErr.String
	}
	if deliveryErr.Valid {
		run.DeliveryError = deliveryErr.String
	}
}
