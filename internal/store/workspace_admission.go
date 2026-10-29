package store

import (
	"context"
	"database/sql"
	"errors"
)

var ErrWorkspaceArchived = errors.New("workspace is archived; unarchive it before starting work")

type workspaceAdmissionReader interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// RequireWorkspaceActive runs inside the same write transaction as admission.
// The archive writer holds the same SQLite write lock while checking blockers.
func RequireWorkspaceActive(ctx context.Context, tx workspaceAdmissionReader, ids ...string) error {
	for _, id := range ids {
		if id == "" {
			continue
		}
		var archived bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (
   SELECT 1 FROM workspaces WHERE id = ? AND archived_at IS NOT NULL
  )`, id).Scan(&archived); err != nil {
			return err
		}
		if archived {
			return ErrWorkspaceArchived
		}
	}
	return nil
}

// RequireAutomationWorkspaceActive accounts for runtime overrides on managed definitions.
func RequireAutomationWorkspaceActive(ctx context.Context, tx workspaceAdmissionReader,
	kind, id, workspaceID, loopWorkspaceID string, enabled, managed bool,
) error {
	if managed {
		var override bool
		var err error
		switch kind {
		case "automation.job":
			err = tx.QueryRowContext(ctx, `SELECT enabled_override FROM automation_job_overlays WHERE job_id = ?`, id).
				Scan(&override)
		case "automation.trigger":
			err = tx.QueryRowContext(ctx, `SELECT enabled_override FROM automation_trigger_overlays WHERE trigger_id = ?`, id).
				Scan(&override)
		}
		if err == nil {
			enabled = override
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
	}
	if !enabled {
		return nil
	}
	return RequireWorkspaceActive(ctx, tx, workspaceID, loopWorkspaceID)
}

// RequireAutomationOverlayActive validates the resulting effective definition before an override write.
// Resources are authoritative when present; legacy definitions remain supported during boot imports.
func RequireAutomationOverlayActive(
	ctx context.Context,
	tx workspaceAdmissionReader,
	kind, id string,
	override *bool,
) error {
	var workspaceID, loopWorkspaceID string
	var enabled bool
	err := tx.QueryRowContext(ctx, `SELECT workspace_id, loop_workspace_id, enabled FROM (
  SELECT COALESCE(json_extract(spec_json, '$.workspace_id'), '') AS workspace_id,
   COALESCE(json_extract(spec_json, '$.loop_target.workspace_id'), '') AS loop_workspace_id,
   COALESCE(json_extract(spec_json, '$.enabled'), 0) AS enabled, 0 AS precedence
  FROM resource_records WHERE kind = ? AND id = ?
  UNION ALL SELECT COALESCE(workspace_id, ''), COALESCE(loop_workspace_id, ''), enabled, 1
  FROM automation_jobs WHERE ? = 'automation.job' AND id = ?
  UNION ALL SELECT COALESCE(workspace_id, ''), COALESCE(loop_workspace_id, ''), enabled, 1
  FROM automation_triggers WHERE ? = 'automation.trigger' AND id = ?
 ) ORDER BY precedence LIMIT 1`, kind, id, kind, id, kind, id).Scan(&workspaceID, &loopWorkspaceID, &enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if override != nil {
		enabled = *override
	}
	if !enabled {
		return nil
	}
	return RequireWorkspaceActive(ctx, tx, workspaceID, loopWorkspaceID)
}
