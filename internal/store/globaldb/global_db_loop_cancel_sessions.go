package globaldb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
)

type cancellationSessionSource struct {
	sessionID   string
	sourceKind  looppkg.SessionCleanupSourceKind
	sourceID    string
	sourceEpoch int64
	createdAt   time.Time
}

func listRunCancellationSessions(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.CancellationMutation,
) ([]string, error) {
	return listAndEnqueueCancellationSessions(
		ctx,
		exec,
		mutation,
		looppkg.SessionCleanupCauseOperatorCancel,
	)
}

func listNodeCancellationSessions(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.CancellationMutation,
) ([]string, error) {
	return listAndEnqueueCancellationSessions(ctx, exec, mutation, looppkg.SessionCleanupCauseTerminal)
}

func listAndEnqueueCancellationSessions(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.CancellationMutation,
	cause looppkg.SessionCleanupCause,
) ([]string, error) {
	sources, err := queryCancellationSessionSources(ctx, exec, mutation)
	if err != nil {
		return nil, err
	}
	for _, source := range sources {
		createdAt := mutation.RequestedAt.UTC()
		if source.createdAt.After(createdAt) {
			createdAt = source.createdAt
		}
		var enqueueErr error
		switch source.sourceKind {
		case looppkg.SessionCleanupSourceGoalBinding:
			enqueueErr = enqueueLoopSessionCleanupWithExecutor(ctx, exec, looppkg.SessionCleanupObligation{
				CleanupID: loopSessionCleanupID(
					mutation.RunID,
					source.sourceKind,
					source.sourceID,
					source.sourceEpoch,
				),
				WorkspaceID: mutation.WorkspaceID,
				LoopRunID:   mutation.RunID,
				SourceKind:  source.sourceKind,
				SourceID:    source.sourceID,
				SourceEpoch: source.sourceEpoch,
				SessionID:   source.sessionID,
				Cause:       cause,
				CreatedAt:   createdAt,
			})
		case looppkg.SessionCleanupSourceTaskRun:
			enqueueErr = enqueueTaskRunSessionCleanupWithExecutor(
				ctx,
				exec,
				mutation.WorkspaceID,
				mutation.RunID,
				source.sourceID,
				source.sessionID,
				cause,
				createdAt,
			)
		default:
			enqueueErr = fmt.Errorf(
				"%w: unsupported cancellation session source %q",
				looppkg.ErrValidation,
				source.sourceKind,
			)
		}
		if enqueueErr != nil {
			return nil, enqueueErr
		}
	}
	sessionIDs := make([]string, 0, len(sources))
	for _, source := range sources {
		sessionIDs = append(sessionIDs, source.sessionID)
	}
	return sessionIDs, nil
}

func queryCancellationSessionSources(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.CancellationMutation,
) ([]cancellationSessionSource, error) {
	bindingQuery := `SELECT 0 AS priority, binding.session_id, 'goal-binding' AS source_kind,
		binding.handle AS source_id, binding.binding_epoch AS source_epoch,
		binding.created_at AS source_created_at
		FROM loop_session_bindings AS binding
		WHERE binding.loop_run_id = ? AND binding.workspace_id = ?
		  AND binding.ownership = 'run-owned' AND binding.state IN ('creating','active')`
	bindingArgs := []any{mutation.RunID, mutation.WorkspaceID}
	if mutation.NodeID != "" {
		bindingQuery += ` AND EXISTS (
			SELECT 1 FROM task_runs AS task_run
			JOIN loop_generation_outputs AS output
			  ON output.loop_run_id = task_run.loop_run_id AND output.task_run_id = task_run.id
			WHERE task_run.loop_run_id = binding.loop_run_id
			  AND json_extract(task_run.metadata_json, '$.session_handle') = binding.handle
			  AND output.node_id = ?`
		bindingArgs = append(bindingArgs, mutation.NodeID)
		if mutation.ItemIndex != nil {
			bindingQuery += ` AND output.item_index = ?`
			bindingArgs = append(bindingArgs, *mutation.ItemIndex)
		}
		bindingQuery += `)`
	}

	taskQuery := `SELECT 1 AS priority, task_run.session_id, 'task-run' AS source_kind,
		task_run.id AS source_id, 0 AS source_epoch, task_run.queued_at AS source_created_at
		FROM task_runs AS task_run
		JOIN loop_runs AS run ON run.id = task_run.loop_run_id
		WHERE task_run.loop_run_id = ? AND task_run.workspace_id = ? AND task_run.run_kind = 'worker'
		  AND length(trim(COALESCE(task_run.session_id, ''))) > 0
		  AND task_run.session_id != COALESCE(run.origin_session_id, '')`
	taskArgs := []any{mutation.RunID, mutation.WorkspaceID}
	if mutation.NodeID != "" {
		taskQuery += ` AND json_extract(task_run.metadata_json, '$.node_id') = ?`
		taskArgs = append(taskArgs, mutation.NodeID)
		if mutation.ItemIndex != nil {
			taskQuery += ` AND CAST(json_extract(task_run.metadata_json, '$.item_index') AS INTEGER) = ?`
			taskArgs = append(taskArgs, *mutation.ItemIndex)
		}
	}

	query := `SELECT priority, session_id, source_kind, source_id, source_epoch, source_created_at FROM (` +
		bindingQuery + ` UNION ALL ` + taskQuery + `) ORDER BY session_id, priority, source_id`
	args := make([]any, 0, len(bindingArgs)+len(taskArgs))
	args = append(args, bindingArgs...)
	args = append(args, taskArgs...)
	rows, err := exec.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list Loop cancellation session sources: %w", err)
	}
	return scanCancellationSessionSources(rows)
}

type cancellationSessionRows interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close() error
}

func scanCancellationSessionSources(rows cancellationSessionRows) ([]cancellationSessionSource, error) {
	sources := make([]cancellationSessionSource, 0)
	seenSessionIDs := make(map[string]struct{})
	for rows.Next() {
		var priority int
		var source cancellationSessionSource
		var createdAtRaw string
		if err := rows.Scan(
			&priority,
			&source.sessionID,
			&source.sourceKind,
			&source.sourceID,
			&source.sourceEpoch,
			&createdAtRaw,
		); err != nil {
			return nil, errors.Join(
				fmt.Errorf("store: scan Loop cancellation session: %w", err),
				rows.Close(),
			)
		}
		createdAt, err := parseLoopRunSummaryTimestamp(createdAtRaw)
		if err != nil {
			return nil, errors.Join(err, rows.Close())
		}
		source.sessionID = strings.TrimSpace(source.sessionID)
		source.createdAt = createdAt
		if _, found := seenSessionIDs[source.sessionID]; found {
			continue
		}
		seenSessionIDs[source.sessionID] = struct{}{}
		sources = append(sources, source)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Join(
			fmt.Errorf("store: iterate Loop cancellation sessions: %w", err),
			rows.Close(),
		)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("store: close Loop cancellation sessions: %w", err)
	}
	return sources, nil
}
