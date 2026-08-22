package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
)

var _ looppkg.ReconciliationStore = (*LoopRepo)(nil)

type loopReconciliationCandidate struct {
	runID  string
	status sql.NullString
}

// NeutralizeLoopRunOrphans removes claim eligibility before daemon recovery starts.
func (g *LoopRepo) NeutralizeLoopRunOrphans(ctx context.Context) (looppkg.SweepReport, error) {
	return g.reconcileLoopRunOrphans(ctx, "neutralize Loop run orphans")
}

// SweepLoopRunOrphans converges terminal execution records during daemon operation.
func (g *LoopRepo) SweepLoopRunOrphans(ctx context.Context) (looppkg.SweepReport, error) {
	return g.reconcileLoopRunOrphans(ctx, "sweep Loop run orphans")
}

func (g *LoopRepo) reconcileLoopRunOrphans(
	ctx context.Context,
	action string,
) (looppkg.SweepReport, error) {
	if err := g.checkReady(ctx, action); err != nil {
		return looppkg.SweepReport{}, err
	}
	var report looppkg.SweepReport
	err := g.withTaskImmediateTransaction(ctx, action, func(exec taskSQLExecutor) error {
		candidates, err := listLoopReconciliationCandidates(ctx, exec)
		if err != nil {
			return err
		}
		report.RunsExamined = len(candidates)
		for _, candidate := range candidates {
			cause := looppkg.TerminalCauseRunMissing
			reason := runMissingReason
			if candidate.status.Valid {
				cause, err = terminalCauseForLoopStatus(looppkg.Status(candidate.status.String), "")
				if err != nil {
					return err
				}
				reason = reconciledRunTerminalReason
			}
			outcome, err := settleLoopRunTerminalWithReason(ctx, exec, candidate.runID, cause, reason)
			if err != nil {
				return err
			}
			report.RecordsSettled += outcome.recordsSettled
			if outcome.recordsSettled > 0 || outcome.result.RunsCanceled > 0 {
				report.OrphansRepaired++
			}
		}
		return nil
	})
	if err != nil {
		return looppkg.SweepReport{}, err
	}
	return report, nil
}

func listLoopReconciliationCandidates(
	ctx context.Context,
	exec taskSQLExecutor,
) (candidates []loopReconciliationCandidate, err error) {
	rows, err := exec.QueryContext(ctx, `SELECT DISTINCT tr.loop_run_id, lr.status
	FROM task_runs tr LEFT JOIN loop_runs lr ON lr.id = tr.loop_run_id
	WHERE tr.loop_run_id IS NOT NULL AND trim(tr.loop_run_id) <> ''
	AND (lr.id IS NULL OR lr.status IN ('done','no-op','blocked','failed','exhausted','stalled','canceled'))
	AND (EXISTS (SELECT 1 FROM task_runs live WHERE live.loop_run_id = tr.loop_run_id
		AND live.status IN ('queued','claimed','starting','running','needs_attention'))
	 OR EXISTS (SELECT 1 FROM tasks t JOIN task_runs owned ON owned.task_id = t.id
		WHERE owned.loop_run_id = tr.loop_run_id
		AND t.status NOT IN ('completed','failed','canceled')))
	ORDER BY tr.loop_run_id`)
	if err != nil {
		return nil, fmt.Errorf("store: list Loop reconciliation candidates: %w", err)
	}
	defer func() { err = joinRowsCloseError(rows, err, "Loop reconciliation candidates") }()
	for rows.Next() {
		var candidate loopReconciliationCandidate
		if scanErr := rows.Scan(&candidate.runID, &candidate.status); scanErr != nil {
			return nil, fmt.Errorf("store: scan Loop reconciliation candidate: %w", scanErr)
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate Loop reconciliation candidates: %w", err)
	}
	return candidates, nil
}

// BackfillLoopProvenance repairs coordinator metadata from relational ownership only.
func (g *LoopRepo) BackfillLoopProvenance(ctx context.Context) (int, error) {
	if err := g.checkReady(ctx, "backfill Loop provenance"); err != nil {
		return 0, err
	}
	repaired := 0
	err := g.withTaskImmediateTransaction(ctx, "backfill Loop provenance", func(exec taskSQLExecutor) error {
		rows, err := exec.QueryContext(ctx, `SELECT DISTINCT t.id, t.workspace_id, t.metadata_json,
		tr.loop_run_id, lr.loop_name FROM tasks t
		JOIN task_runs tr ON tr.task_id = t.id AND tr.workspace_id = t.workspace_id
			AND tr.run_kind = 'coordinator'
		LEFT JOIN loop_runs lr ON lr.id = tr.loop_run_id AND lr.workspace_id = t.workspace_id
		WHERE tr.loop_run_id IS NOT NULL AND trim(tr.loop_run_id) <> ''`)
		if err != nil {
			return fmt.Errorf("store: list Loop provenance rows: %w", err)
		}
		type row struct {
			taskID, workspaceID, loopRunID string
			metadata                       []byte
			loopName                       sql.NullString
		}
		var records []row
		for rows.Next() {
			var record row
			if err := rows.Scan(
				&record.taskID, &record.workspaceID, &record.metadata, &record.loopRunID, &record.loopName,
			); err != nil {
				closeErr := rows.Close()
				if closeErr != nil {
					return errors.Join(fmt.Errorf("store: scan Loop provenance row: %w", err), closeErr)
				}
				return fmt.Errorf("store: scan Loop provenance row: %w", err)
			}
			records = append(records, record)
		}
		if err := rows.Err(); err != nil {
			if closeErr := rows.Close(); closeErr != nil {
				return errors.Join(fmt.Errorf("store: iterate Loop provenance rows: %w", err), closeErr)
			}
			return fmt.Errorf("store: iterate Loop provenance rows: %w", err)
		}
		if err := rows.Close(); err != nil {
			return fmt.Errorf("store: close Loop provenance rows: %w", err)
		}
		for _, record := range records {
			changed, err := backfillLoopProvenanceRow(ctx, exec, record.taskID, record.workspaceID,
				record.loopRunID, record.loopName, record.metadata)
			if err != nil {
				return err
			}
			if changed {
				repaired++
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return repaired, nil
}

func backfillLoopProvenanceRow(
	ctx context.Context,
	exec taskSQLExecutor,
	taskID string,
	workspaceID string,
	loopRunID string,
	loopName sql.NullString,
	raw []byte,
) (bool, error) {
	metadata := map[string]any{}
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &metadata); err != nil {
			return false, fmt.Errorf("store: decode Loop coordinator %q metadata: %w", taskID, err)
		}
	}
	changed := setMetadataString(metadata, "loop_run_id", loopRunID)
	changed = setMetadataString(metadata, "workspace_id", workspaceID) || changed
	if loopName.Valid {
		changed = setMetadataString(metadata, "loop_name", loopName.String) || changed
	} else if _, exists := metadata["loop_name"]; exists {
		delete(metadata, "loop_name")
		changed = true
	}
	if !changed {
		return false, nil
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return false, fmt.Errorf("store: encode Loop coordinator %q metadata: %w", taskID, err)
	}
	if _, err := exec.ExecContext(ctx, `UPDATE tasks SET metadata_json = ? WHERE id = ?`, encoded, taskID); err != nil {
		return false, fmt.Errorf("store: update Loop coordinator %q metadata: %w", taskID, err)
	}
	return true, nil
}

func setMetadataString(metadata map[string]any, key string, value string) bool {
	trimmed := strings.TrimSpace(value)
	if current, ok := metadata[key].(string); ok && current == trimmed {
		return false
	}
	metadata[key] = trimmed
	return true
}
