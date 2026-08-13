package globaldb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/compozy/internal/task"
)

const runLoopAwaitingChildStatus = "awaiting_child"

type runLoopActionResult struct {
	LoopRunID string `json:"loop_run_id"`
	Status    string `json:"status"`
}

func awaitedChildLoopRunID(run taskpkg.Run, result json.RawMessage) (string, bool, error) {
	metadata, ok, err := loopNodeMetadataFromTaskRun(run.Metadata)
	if err != nil {
		return "", false, err
	}
	if !ok || !loopNodeIsRunLoopAction(metadata) {
		return "", false, nil
	}
	var payload runLoopActionResult
	if err := json.Unmarshal(result, &payload); err != nil {
		return "", false, nil
	}
	if strings.TrimSpace(payload.Status) != runLoopAwaitingChildStatus {
		return "", false, nil
	}
	childLoopRunID := strings.TrimSpace(payload.LoopRunID)
	if childLoopRunID == "" {
		return "", false, fmt.Errorf("%w: await-mode run-loop result is missing loop_run_id", looppkg.ErrValidation)
	}
	return childLoopRunID, true, nil
}

func recordAwaitingChildLoopOutputWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run taskpkg.Run,
	childLoopRunID string,
	outputRef string,
	at time.Time,
) error {
	loopRunID := strings.TrimSpace(run.LoopRunID)
	child, err := getLoopRunByIDWithExecutor(ctx, exec, looppkg.RunID(childLoopRunID))
	if err != nil {
		return err
	}
	if child.ParentLoopRunID != looppkg.RunID(loopRunID) {
		return fmt.Errorf(
			"%w: awaited child loop %q does not belong to parent %q",
			looppkg.ErrValidation,
			childLoopRunID,
			loopRunID,
		)
	}
	recorded, err := updateLoopNodeOutputStatusWithExecutor(
		ctx,
		exec,
		run,
		loopRunID,
		runLoopAwaitingChildStatus,
		outputRef,
		childLoopRunID,
	)
	if err != nil {
		return err
	}
	if !recorded {
		return appendLoopNodeLateArrivalWithExecutor(ctx, exec, run, "success", at)
	}
	affected, err := sqlcgen.New(exec).UpdateLoopRunNodeTerminal(ctx, sqlcgen.UpdateLoopRunNodeTerminalParams{
		TerminalAt: store.FormatTimestamp(at), ID: loopRunID,
	})
	if err != nil {
		return fmt.Errorf("store: record loop run %q await progress: %w", loopRunID, err)
	}
	if affected == 0 {
		return fmt.Errorf("store: loop run %q: %w", loopRunID, looppkg.ErrRunNotFound)
	}
	tokensUsed, err := refreshLoopTokensUsedWithExecutor(ctx, exec, loopRunID)
	if err != nil {
		return err
	}
	return appendLoopTokenTickEventWithExecutor(
		ctx,
		exec,
		looppkg.RunID(loopRunID),
		child.WorkspaceID,
		run.ID,
		tokensUsed,
		true,
		at,
	)
}
