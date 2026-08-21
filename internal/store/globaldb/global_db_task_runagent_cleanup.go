package globaldb

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/goal"
	taskpkg "github.com/compozy/compozy/internal/task"
)

type completedRunAgentMetadata struct {
	NodeKind      string `json:"node_kind"`
	SessionHandle string `json:"session_handle"`
	Epoch         int64  `json:"epoch"`
}

func closeCompletedRunAgentBinding(
	ctx context.Context,
	exec taskSQLExecutor,
	run taskpkg.Run,
	completedAt time.Time,
) error {
	if !run.IsLoopWorker() {
		return nil
	}
	var metadata completedRunAgentMetadata
	if err := json.Unmarshal(run.Metadata, &metadata); err != nil {
		return fmt.Errorf("store: decode completed run-agent metadata: %w", err)
	}
	if dsl.ActionKind(strings.TrimSpace(metadata.NodeKind)) != dsl.ActionRunAgent {
		return nil
	}
	metadata.SessionHandle = strings.TrimSpace(metadata.SessionHandle)
	if metadata.SessionHandle == "" || metadata.Epoch < 0 || metadata.Epoch == math.MaxInt64 {
		return fmt.Errorf("%w: completed run-agent binding identity is invalid", looppkg.ErrValidation)
	}
	key := goal.BindingKey{
		WorkspaceID: looppkg.WorkspaceID(strings.TrimSpace(run.WorkspaceID)),
		LoopRunID:   looppkg.RunID(strings.TrimSpace(run.LoopRunID)),
		Handle:      metadata.SessionHandle,
	}
	if err := key.Validate(); err != nil {
		return err
	}
	binding, found, err := findSessionBindingAttemptWithExecutor(ctx, exec, key, metadata.Epoch+1)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if binding.Ownership != goal.BindingOwnershipRunOwned {
		return fmt.Errorf("%w: run-agent binding is not run-owned", looppkg.ErrTransitionConflict)
	}
	return closeGoalBindingWithCleanup(
		ctx,
		exec,
		key,
		binding.BindingEpoch,
		binding.SessionID,
		goal.SessionCleanupCauseTerminal,
		completedAt,
	)
}
