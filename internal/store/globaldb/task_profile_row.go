package globaldb

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func upsertExecutionProfileRow(
	ctx context.Context,
	exec taskSQLExecutor,
	profile *taskpkg.ExecutionProfile,
) error {
	workerACPOptionsJSON, err := encodeTaskProfileACPOptions(profile.Worker.ACPOptions)
	if err != nil {
		return fmt.Errorf("store: encode task worker ACP options: %w", err)
	}
	reviewACPOptionsJSON, err := encodeTaskProfileACPOptions(profile.Review.ACPOptions)
	if err != nil {
		return fmt.Errorf("store: encode task review ACP options: %w", err)
	}
	if err := sqlcgen.New(exec).UpsertTaskExecutionProfile(ctx, sqlcgen.UpsertTaskExecutionProfileParams{
		TaskID:                profile.TaskID,
		CoordinatorMode:       string(profile.Coordinator.Mode),
		CoordinatorAgentName:  profile.Coordinator.AgentName,
		CoordinatorProvider:   profile.Coordinator.Provider,
		CoordinatorModel:      profile.Coordinator.Model,
		CoordinatorGuidance:   profile.Coordinator.Guidance,
		WorkerMode:            string(profile.Worker.Mode),
		WorkerAgentName:       profile.Worker.AgentName,
		WorkerProvider:        profile.Worker.Provider,
		WorkerModel:           profile.Worker.Model,
		WorkerReasoningEffort: profile.Worker.ReasoningEffort,
		WorkerSpeed:           string(profile.Worker.Speed),
		WorkerAcpOptionsJson:  workerACPOptionsJSON,
		ReviewAgentName:       profile.Review.AgentName,
		ReviewProvider:        profile.Review.Provider,
		ReviewModel:           profile.Review.Model,
		ReviewReasoningEffort: profile.Review.ReasoningEffort,
		ReviewSpeed:           string(profile.Review.Speed),
		ReviewAcpOptionsJson:  reviewACPOptionsJSON,

		WorktreeMode: string(profile.Worktree.Mode),
		WorktreeRef:  profile.Worktree.WorktreeRef,
		RuntimeMode:  string(profile.Runtime.Mode),
		CreatedAt:    store.FormatTimestamp(profile.CreatedAt),
		UpdatedAt:    store.FormatTimestamp(profile.UpdatedAt),
	}); err != nil {
		return fmt.Errorf("store: upsert task execution profile %q: %w", profile.TaskID, err)
	}
	return nil
}
