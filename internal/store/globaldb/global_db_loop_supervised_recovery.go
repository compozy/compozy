package globaldb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/goal"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/compozy/internal/task"
)

// loadSupervisedLoopRecovery admits ordinary task work or an unchanged, recoverable Loop cell and binding.
func loadSupervisedLoopRecovery(
	ctx context.Context, exec taskSQLExecutor, source taskpkg.Run,
) (loopNodeRunMetadata, looppkg.Run, bool, error) {
	if !source.IsLoopWorker() {
		return loopNodeRunMetadata{}, looppkg.Run{}, true, nil
	}
	metadata, run, err := loadBoundLoopTaskRunCell(ctx, exec, source)
	if errors.Is(err, looppkg.ErrTransitionConflict) {
		return metadata, run, false, nil
	}
	if err != nil || run.Status != looppkg.StatusRunning {
		return metadata, run, false, err
	}
	allowed, err := supervisedLoopRecoveryAllowed(ctx, exec, source, metadata)
	return metadata, run, allowed, err
}

// supervisedLoopRecoveryAllowed honors durable controls and requires the current run-owned session-binding epoch.
func supervisedLoopRecoveryAllowed(
	ctx context.Context, exec taskSQLExecutor, source taskpkg.Run, metadata loopNodeRunMetadata,
) (bool, error) {
	controls, err := sqlcgen.New(exec).ListLoopNodeControls(ctx, sqlcgen.ListLoopNodeControlsParams{
		WorkspaceID: source.WorkspaceID, LoopRunID: source.LoopRunID,
	})
	if err != nil {
		return false, err
	}
	for _, control := range controls {
		if control.NodeID == metadata.NodeID && (control.Paused != 0 || control.Quarantined != 0 ||
			control.CancelState != "" ||
			(control.AttentionFlag != "" && control.AttentionFlag != looppkg.AttentionSilence)) {
			return false, nil
		}
	}
	var bindingMetadata runAgentBindingMetadata
	if err := json.Unmarshal(source.Metadata, &bindingMetadata); err != nil {
		return false, err
	}
	if bindingMetadata.NodeKind != "run-agent" {
		return false, nil
	}
	binding, found, err := findSessionBindingAttemptWithExecutor(ctx, exec, goal.BindingKey{
		WorkspaceID: looppkg.WorkspaceID(source.WorkspaceID), LoopRunID: looppkg.RunID(source.LoopRunID),
		Handle: bindingMetadata.SessionHandle,
	}, metadata.Epoch+1)
	if err != nil {
		return false, err
	}
	if !found || binding.SessionID != source.SessionID {
		return false, fmt.Errorf("%w: supervised Loop session binding changed", looppkg.ErrTransitionConflict)
	}
	return binding.Ownership == goal.BindingOwnershipRunOwned && binding.State == goal.BindingStateActive, nil
}
