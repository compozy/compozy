package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	automation "github.com/compozy/agh/internal/automation/model"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

// GetTriggerByWebhookID loads a webhook trigger using its stable webhook identifier.
func (g *AutomationRepo) GetTriggerByWebhookID(ctx context.Context, webhookID string) (automation.Trigger, error) {
	if err := g.checkReady(ctx, "get automation trigger by webhook id"); err != nil {
		return automation.Trigger{}, err
	}

	trimmedWebhookID, err := requireAutomationID(webhookID, "automation trigger webhook id")
	if err != nil {
		return automation.Trigger{}, err
	}

	row, err := g.queries.GetAutomationTriggerByWebhookID(ctx, nullableAutomationString(trimmedWebhookID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return automation.Trigger{}, automation.ErrTriggerNotFound
		}
		return automation.Trigger{}, err
	}
	return automationTriggerFromGenerated(row)
}

// CreateRun stores a new automation run history row.
func (g *AutomationRepo) CreateRun(ctx context.Context, run automation.Run) (automation.Run, error) {
	if err := g.checkReady(ctx, "create automation run"); err != nil {
		return automation.Run{}, err
	}

	normalized, err := g.normalizeRunForCreate(run)
	if err != nil {
		return automation.Run{}, err
	}
	metadataJSON, err := encodeAutomationRunMetadata(normalized.Metadata)
	if err != nil {
		return automation.Run{}, err
	}
	networkParticipation, err := encodeOptionalAutomationParticipation(
		normalized.NetworkParticipation,
		normalized.NetworkParticipation == nil,
		"run.network_participation",
	)
	if err != nil {
		return automation.Run{}, err
	}

	if err := g.queries.InsertAutomationRun(
		ctx,
		automationRunParams(normalized, networkParticipation, metadataJSON),
	); err != nil {
		return automation.Run{}, fmt.Errorf(
			"store: create automation run %q: %w",
			normalized.ID,
			mapAutomationRunConstraintError(err),
		)
	}

	return normalized, nil
}

// UpdateRun replaces the mutable fields of a persisted automation run.
func (g *AutomationRepo) UpdateRun(ctx context.Context, run automation.Run) (automation.Run, error) {
	if err := g.checkReady(ctx, "update automation run"); err != nil {
		return automation.Run{}, err
	}

	normalized, err := g.normalizeRunForUpdate(run)
	if err != nil {
		return automation.Run{}, err
	}
	metadataJSON, err := encodeAutomationRunMetadata(normalized.Metadata)
	if err != nil {
		return automation.Run{}, err
	}
	networkParticipation, err := encodeOptionalAutomationParticipation(
		normalized.NetworkParticipation,
		normalized.NetworkParticipation == nil,
		"run.network_participation",
	)
	if err != nil {
		return automation.Run{}, err
	}

	affected, err := g.queries.UpdateAutomationRun(
		ctx,
		automationRunUpdateParams(normalized, networkParticipation, metadataJSON),
	)
	if err != nil {
		return automation.Run{}, fmt.Errorf(
			"store: update automation run %q: %w",
			normalized.ID,
			mapAutomationRunConstraintError(err),
		)
	}

	if err := requireAutomationAffected(
		affected,
		automation.ErrRunNotFound,
		normalized.ID,
		"automation run",
	); err != nil {
		return automation.Run{}, err
	}

	return g.GetRun(ctx, normalized.ID)
}

// DeleteRun removes an automation run history row.
func (g *AutomationRepo) DeleteRun(ctx context.Context, id string) error {
	if err := g.checkReady(ctx, "delete automation run"); err != nil {
		return err
	}

	trimmedID, err := requireAutomationID(id, "automation run id")
	if err != nil {
		return err
	}

	affected, err := g.queries.DeleteAutomationRun(ctx, trimmedID)
	if err != nil {
		return fmt.Errorf("store: delete automation run %q: %w", trimmedID, err)
	}

	return requireAutomationAffected(affected, automation.ErrRunNotFound, trimmedID, "automation run")
}

// GetRun loads one persisted automation run by primary key.
func (g *AutomationRepo) GetRun(ctx context.Context, id string) (automation.Run, error) {
	if err := g.checkReady(ctx, "get automation run"); err != nil {
		return automation.Run{}, err
	}

	trimmedID, err := requireAutomationID(id, "automation run id")
	if err != nil {
		return automation.Run{}, err
	}

	row, err := g.queries.GetAutomationRun(ctx, trimmedID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return automation.Run{}, automation.ErrRunNotFound
		}
		return automation.Run{}, err
	}

	return automationRunFromGetGenerated(row)
}

// ListRuns returns filtered automation run history rows.
func (g *AutomationRepo) ListRuns(
	ctx context.Context,
	query automation.RunQuery,
) (runs []automation.Run, err error) {
	if err := g.checkReady(ctx, "list automation runs"); err != nil {
		return nil, err
	}
	if err := validateAutomationRunQuery(query); err != nil {
		return nil, err
	}

	// dynamic-sql: optional run dimensions, time bounds, and caller limit change the statement shape.
	sqlQuery := `SELECT
		id, job_id, trigger_id, session_id, task_id, task_run_id, fire_id,
		status, attempt, scheduled_at, started_at, ended_at, error,
		delivery_error, delivery_error_at, loop_run_id, network_participation, metadata_json
		FROM automation_runs`
	where, args := buildAutomationRunClauses(query)
	sqlQuery = store.AppendWhere(sqlQuery, where)
	sqlQuery += " ORDER BY started_at DESC, id DESC"
	sqlQuery, args = store.AppendLimit(sqlQuery, args, query.Limit)

	rows, err := g.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query automation runs: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("store: close automation run rows: %w", closeErr))
		}
	}()

	runs = make([]automation.Run, 0)
	for rows.Next() {
		run, scanErr := scanAutomationRun(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate automation runs: %w", err)
	}

	return runs, nil
}

// CountRuns returns the number of automation runs matching the supplied filters.
func (g *AutomationRepo) CountRuns(ctx context.Context, query automation.RunQuery) (int64, error) {
	if err := g.checkReady(ctx, "count automation runs"); err != nil {
		return 0, err
	}
	if err := validateAutomationRunQuery(query); err != nil {
		return 0, err
	}

	// dynamic-sql: optional run dimensions and time bounds change the count predicate set.
	sqlQuery := `SELECT COUNT(*) FROM automation_runs`
	where, args := buildAutomationRunClauses(query)
	sqlQuery = store.AppendWhere(sqlQuery, where)

	var count int64
	if err := g.db.QueryRowContext(ctx, sqlQuery, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("store: count automation runs: %w", err)
	}

	return count, nil
}

// SetJobEnabledOverlay upserts the runtime enabled override for a config-backed job.
func (g *AutomationRepo) SetJobEnabledOverlay(
	ctx context.Context,
	overlay automation.JobEnabledOverlay,
) (automation.JobEnabledOverlay, error) {
	if err := g.checkReady(ctx, "set automation job overlay"); err != nil {
		return automation.JobEnabledOverlay{}, err
	}

	normalized, err := normalizeJobOverlay(overlay, g.now())
	if err != nil {
		return automation.JobEnabledOverlay{}, err
	}
	if err := g.queries.UpsertAutomationJobOverlay(ctx, sqlcgen.UpsertAutomationJobOverlayParams{
		JobID: normalized.JobID, EnabledOverride: normalized.EnabledOverride,
		UpdatedAt: store.FormatTimestamp(normalized.UpdatedAt),
	}); err != nil {
		return automation.JobEnabledOverlay{}, fmt.Errorf(
			"store: set automation job overlay %q: %w",
			normalized.JobID,
			err,
		)
	}

	return normalized, nil
}

// GetJobEnabledOverlay loads one persisted job enabled overlay by job id.
func (g *AutomationRepo) GetJobEnabledOverlay(ctx context.Context, jobID string) (automation.JobEnabledOverlay, error) {
	if err := g.checkReady(ctx, "get automation job overlay"); err != nil {
		return automation.JobEnabledOverlay{}, err
	}

	trimmedID, err := requireAutomationID(jobID, "automation job overlay id")
	if err != nil {
		return automation.JobEnabledOverlay{}, err
	}

	row, err := g.queries.GetAutomationJobOverlay(ctx, trimmedID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return automation.JobEnabledOverlay{}, automation.ErrJobOverlayNotFound
		}
		return automation.JobEnabledOverlay{}, err
	}

	return automationJobOverlayFromGenerated(row)
}

// ListJobEnabledOverlays returns all persisted job enabled overlays.
func (g *AutomationRepo) ListJobEnabledOverlays(
	ctx context.Context,
) (overlays []automation.JobEnabledOverlay, err error) {
	if err := g.checkReady(ctx, "list automation job overlays"); err != nil {
		return nil, err
	}

	rows, err := g.queries.ListAutomationJobOverlays(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: query automation job overlays: %w", err)
	}
	overlays = make([]automation.JobEnabledOverlay, 0, len(rows))
	for _, row := range rows {
		overlay, scanErr := automationJobOverlayFromGenerated(row)
		if scanErr != nil {
			return nil, scanErr
		}
		overlays = append(overlays, overlay)
	}
	return overlays, nil
}

// DeleteJobEnabledOverlay clears a persisted job enabled overlay if it exists.
func (g *AutomationRepo) DeleteJobEnabledOverlay(ctx context.Context, jobID string) error {
	if err := g.checkReady(ctx, "delete automation job overlay"); err != nil {
		return err
	}

	trimmedID, err := requireAutomationID(jobID, "automation job overlay id")
	if err != nil {
		return err
	}

	if err := g.queries.DeleteAutomationJobOverlay(ctx, trimmedID); err != nil {
		return fmt.Errorf("store: delete automation job overlay %q: %w", trimmedID, err)
	}

	return nil
}

// SetTriggerEnabledOverlay upserts the runtime enabled override for a config-backed trigger.
func (g *AutomationRepo) SetTriggerEnabledOverlay(
	ctx context.Context,
	overlay automation.TriggerEnabledOverlay,
) (automation.TriggerEnabledOverlay, error) {
	if err := g.checkReady(ctx, "set automation trigger overlay"); err != nil {
		return automation.TriggerEnabledOverlay{}, err
	}

	normalized, err := normalizeTriggerOverlay(overlay, g.now())
	if err != nil {
		return automation.TriggerEnabledOverlay{}, err
	}
	if err := g.queries.UpsertAutomationTriggerOverlay(ctx, sqlcgen.UpsertAutomationTriggerOverlayParams{
		TriggerID: normalized.TriggerID, EnabledOverride: normalized.EnabledOverride,
		UpdatedAt: store.FormatTimestamp(normalized.UpdatedAt),
	}); err != nil {
		return automation.TriggerEnabledOverlay{}, fmt.Errorf(
			"store: set automation trigger overlay %q: %w",
			normalized.TriggerID,
			err,
		)
	}

	return normalized, nil
}

// GetTriggerEnabledOverlay loads one persisted trigger enabled overlay by trigger id.
func (g *AutomationRepo) GetTriggerEnabledOverlay(
	ctx context.Context,
	triggerID string,
) (automation.TriggerEnabledOverlay, error) {
	if err := g.checkReady(ctx, "get automation trigger overlay"); err != nil {
		return automation.TriggerEnabledOverlay{}, err
	}

	trimmedID, err := requireAutomationID(triggerID, "automation trigger overlay id")
	if err != nil {
		return automation.TriggerEnabledOverlay{}, err
	}

	row, err := g.queries.GetAutomationTriggerOverlay(ctx, trimmedID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return automation.TriggerEnabledOverlay{}, automation.ErrTriggerOverlayNotFound
		}
		return automation.TriggerEnabledOverlay{}, err
	}

	return automationTriggerOverlayFromGenerated(row)
}

// ListTriggerEnabledOverlays returns all persisted trigger enabled overlays.
func (g *AutomationRepo) ListTriggerEnabledOverlays(
	ctx context.Context,
) (overlays []automation.TriggerEnabledOverlay, err error) {
	if err := g.checkReady(ctx, "list automation trigger overlays"); err != nil {
		return nil, err
	}

	rows, err := g.queries.ListAutomationTriggerOverlays(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: query automation trigger overlays: %w", err)
	}
	overlays = make([]automation.TriggerEnabledOverlay, 0, len(rows))
	for _, row := range rows {
		overlay, scanErr := automationTriggerOverlayFromGenerated(row)
		if scanErr != nil {
			return nil, scanErr
		}
		overlays = append(overlays, overlay)
	}
	return overlays, nil
}

// DeleteTriggerEnabledOverlay clears a persisted trigger enabled overlay if it exists.
func (g *AutomationRepo) DeleteTriggerEnabledOverlay(ctx context.Context, triggerID string) error {
	if err := g.checkReady(ctx, "delete automation trigger overlay"); err != nil {
		return err
	}

	trimmedID, err := requireAutomationID(triggerID, "automation trigger overlay id")
	if err != nil {
		return err
	}

	if err := g.queries.DeleteAutomationTriggerOverlay(ctx, trimmedID); err != nil {
		return fmt.Errorf("store: delete automation trigger overlay %q: %w", trimmedID, err)
	}

	return nil
}

func (g *AutomationRepo) insertJob(ctx context.Context, exec globalSQLExecutor, job automation.Job) error {
	params, err := automationJobParams(job)
	if err != nil {
		return err
	}
	if err := sqlcgen.New(exec).InsertAutomationJob(ctx, params); err != nil {
		return mapAutomationJobConstraintError(err)
	}

	return nil
}

func (g *AutomationRepo) insertTrigger(ctx context.Context, exec globalSQLExecutor, trigger automation.Trigger) error {
	params, err := automationTriggerParams(trigger)
	if err != nil {
		return err
	}

	if err := sqlcgen.New(exec).InsertAutomationTrigger(ctx, params); err != nil {
		return mapAutomationTriggerConstraintError(ctx, exec, trigger, err)
	}

	return nil
}
