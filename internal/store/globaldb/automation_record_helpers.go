package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	automation "github.com/compozy/agh/internal/automation/model"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	aghworkspace "github.com/compozy/agh/internal/workspace"
)

func encodeJobRecord(job automation.Job) (string, any, string, string, automationLoopTargetEncoded, error) {
	scheduleJSON, err := encodeAutomationJSON(job.Schedule, "job.schedule")
	if err != nil {
		return "", nil, "", "", automationLoopTargetEncoded{}, err
	}
	taskJSON, err := encodeOptionalAutomationJSON(job.Task, job.Task == nil, "job.task")
	if err != nil {
		return "", nil, "", "", automationLoopTargetEncoded{}, err
	}
	retryJSON, err := encodeAutomationJSON(job.Retry, "job.retry")
	if err != nil {
		return "", nil, "", "", automationLoopTargetEncoded{}, err
	}
	fireLimitJSON, err := encodeAutomationJSON(job.FireLimit, "job.fire_limit")
	if err != nil {
		return "", nil, "", "", automationLoopTargetEncoded{}, err
	}
	loopTarget, err := encodeAutomationLoopTarget(job.LoopTarget, "job.loop_target")
	if err != nil {
		return "", nil, "", "", automationLoopTargetEncoded{}, err
	}

	return scheduleJSON, taskJSON, retryJSON, fireLimitJSON, loopTarget, nil
}

func encodeTriggerRecord(trigger automation.Trigger) (any, string, string, automationLoopTargetEncoded, error) {
	filterJSON, err := encodeOptionalAutomationJSON(trigger.Filter, len(trigger.Filter) == 0, "trigger.filter")
	if err != nil {
		return nil, "", "", automationLoopTargetEncoded{}, err
	}
	retryJSON, err := encodeAutomationJSON(trigger.Retry, "trigger.retry")
	if err != nil {
		return nil, "", "", automationLoopTargetEncoded{}, err
	}
	fireLimitJSON, err := encodeAutomationJSON(trigger.FireLimit, "trigger.fire_limit")
	if err != nil {
		return nil, "", "", automationLoopTargetEncoded{}, err
	}
	loopTarget, err := encodeAutomationLoopTarget(trigger.LoopTarget, "trigger.loop_target")
	if err != nil {
		return nil, "", "", automationLoopTargetEncoded{}, err
	}

	return filterJSON, retryJSON, fireLimitJSON, loopTarget, nil
}

func validateAutomationRunQuery(query automation.RunQuery) error {
	if query.Limit < 0 {
		return fmt.Errorf("store: invalid automation run limit %d", query.Limit)
	}
	if query.Status != "" {
		if err := query.Status.Validate("run_query.status"); err != nil {
			return err
		}
	}
	if !query.Until.IsZero() && !query.Since.IsZero() && query.Until.Before(query.Since) {
		return errors.New("store: automation run query until must not be before since")
	}
	return nil
}

func validateAutomationRunRecord(run automation.Run) error {
	if err := run.Validate("run"); err != nil {
		return err
	}
	jobID := strings.TrimSpace(run.JobID)
	triggerID := strings.TrimSpace(run.TriggerID)
	taskID := strings.TrimSpace(run.TaskID)
	taskRunID := strings.TrimSpace(run.TaskRunID)
	loopRunID := strings.TrimSpace(run.LoopRunID)
	taskDelegated := taskID != "" && taskRunID != ""
	loopDelegated := loopRunID != ""
	switch {
	case jobID == "" && triggerID == "":
		return errors.New("store: automation run job_id or trigger_id is required")
	case jobID != "" && triggerID != "":
		return errors.New("store: automation run must reference either a job or a trigger, not both")
	case taskRunID != "" && taskID == "":
		return errors.New("store: automation run task_id is required when task_run_id is set")
	case taskID != "" && taskRunID == "":
		return errors.New("store: automation run task_run_id is required when task_id is set")
	case loopRunID != "" && run.Status != automation.RunDelegated:
		return errors.New("store: automation run loop_run_id requires delegated status")
	case run.Status == automation.RunDelegated && taskDelegated == loopDelegated:
		return errors.New("store: delegated automation run requires exactly one task run or loop run target")
	default:
		return nil
	}
}

func buildAutomationRunClauses(query automation.RunQuery) ([]string, []any) {
	where, args := store.BuildClauses(
		store.StringClause("job_id", query.JobID),
		store.StringClause("trigger_id", query.TriggerID),
		store.StringClause("status", string(query.Status)),
		store.NotStringClause("id", query.ExcludeID),
		store.TimeClause("started_at", ">=", query.Since),
		store.TimeClause("started_at", "<=", query.Until),
	)
	return where, args
}

func requireAutomationID(value string, label string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("store: %s is required", label)
	}
	return trimmed, nil
}

func mapAutomationJobConstraintError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case isSQLiteUniqueConstraint(err):
		return automation.ErrJobNameTaken
	case isSQLiteForeignKeyConstraint(err):
		return aghworkspace.ErrWorkspaceNotFound
	default:
		return err
	}
}

func mapAutomationTriggerConstraintError(
	ctx context.Context,
	exec globalSQLExecutor,
	trigger automation.Trigger,
	err error,
) error {
	if err == nil {
		return nil
	}
	if isSQLiteForeignKeyConstraint(err) {
		return aghworkspace.ErrWorkspaceNotFound
	}
	if !isSQLiteUniqueConstraint(err) {
		return err
	}
	webhookID := strings.TrimSpace(trigger.WebhookID)
	if webhookID == "" {
		return automation.ErrTriggerNameTaken
	}
	existing, queryErr := sqlcgen.New(exec).GetAutomationTriggerByWebhookID(
		ctx,
		nullableAutomationString(webhookID),
	)
	switch {
	case queryErr == nil && existing.ID != trigger.ID:
		return automation.ErrTriggerWebhookIDTaken
	case queryErr == nil, errors.Is(queryErr, sql.ErrNoRows):
		return automation.ErrTriggerNameTaken
	default:
		return errors.Join(
			err,
			fmt.Errorf("store: classify automation trigger uniqueness conflict: %w", queryErr),
		)
	}
}

func mapAutomationRunConstraintError(err error) error {
	if err == nil {
		return nil
	}

	if isSQLiteUniqueConstraint(err) || isSQLitePrimaryKeyConstraint(err) {
		return automation.ErrRunAlreadyExists
	}
	return err
}

func encodeAutomationJSON(value any, label string) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("store: encode %s: %w", label, err)
	}
	return string(data), nil
}

func encodeAutomationRunMetadata(metadata map[string]any) (string, error) {
	if metadata == nil {
		metadata = map[string]any{}
	}
	return encodeAutomationJSON(metadata, "run.metadata")
}

func encodeOptionalAutomationParticipation(value any, empty bool, label string) (sql.NullString, error) {
	encoded, err := encodeOptionalAutomationJSON(value, empty, label)
	if err != nil {
		return sql.NullString{}, err
	}
	return nullableAutomationJSON(encoded, label)
}

func decodeAutomationRunMetadata(raw string, target *map[string]any) error {
	if strings.TrimSpace(raw) == "" {
		*target = map[string]any{}
		return nil
	}
	if err := json.Unmarshal([]byte(raw), target); err != nil {
		return fmt.Errorf("store: decode run.metadata: %w", err)
	}
	if *target == nil {
		*target = map[string]any{}
	}
	return nil
}

func encodeOptionalAutomationJSON(value any, empty bool, label string) (any, error) {
	if empty {
		return nil, nil
	}
	encoded, err := encodeAutomationJSON(value, label)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func decodeAutomationJSON[T any](raw string, target *T, label string) error {
	if strings.TrimSpace(raw) == "" {
		return fmt.Errorf("store: %s is required", label)
	}
	if err := json.Unmarshal([]byte(raw), target); err != nil {
		return fmt.Errorf("store: decode %s: %w", label, err)
	}
	return nil
}

func decodeAutomationSchedule(raw sql.NullString, target **automation.ScheduleSpec) error {
	if !raw.Valid || strings.TrimSpace(raw.String) == "" {
		*target = nil
		return nil
	}

	var schedule automation.ScheduleSpec
	if err := json.Unmarshal([]byte(raw.String), &schedule); err != nil {
		return fmt.Errorf("store: decode job.schedule: %w", err)
	}
	*target = &schedule
	return nil
}

func decodeAutomationTaskConfig(raw sql.NullString, target **automation.JobTaskConfig) error {
	if !raw.Valid || strings.TrimSpace(raw.String) == "" {
		*target = nil
		return nil
	}

	var taskConfig automation.JobTaskConfig
	if err := json.Unmarshal([]byte(raw.String), &taskConfig); err != nil {
		return fmt.Errorf("store: decode job.task: %w", err)
	}
	*target = &taskConfig
	return nil
}

func decodeAutomationFilter(raw sql.NullString, target *map[string]string) error {
	if !raw.Valid || strings.TrimSpace(raw.String) == "" {
		*target = nil
		return nil
	}
	var filter map[string]string
	if err := json.Unmarshal([]byte(raw.String), &filter); err != nil {
		return fmt.Errorf("store: decode trigger.filter: %w", err)
	}
	*target = filter
	return nil
}

func nullableAutomationTimestamp(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return store.FormatTimestamp(*value)
}

func automationNullStringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return strings.TrimSpace(value.String)
}
