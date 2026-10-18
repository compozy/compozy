package globaldb

import (
	"encoding/json"
	"fmt"
	"strings"

	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func executionProfileFromGenerated(row sqlcgen.GetTaskExecutionProfileRow) (taskpkg.ExecutionProfile, error) {
	createdAt, err := store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return taskpkg.ExecutionProfile{}, fmt.Errorf("store: parse task execution profile created_at: %w", err)
	}
	updatedAt, err := store.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return taskpkg.ExecutionProfile{}, fmt.Errorf("store: parse task execution profile updated_at: %w", err)
	}
	workerACPOptions, err := decodeTaskProfileACPOptions(row.WorkerAcpOptionsJson)
	if err != nil {
		return taskpkg.ExecutionProfile{}, fmt.Errorf("store: decode task worker ACP options: %w", err)
	}
	reviewACPOptions, err := decodeTaskProfileACPOptions(row.ReviewAcpOptionsJson)
	if err != nil {
		return taskpkg.ExecutionProfile{}, fmt.Errorf("store: decode task review ACP options: %w", err)
	}
	return taskpkg.ExecutionProfile{
		TaskID: row.TaskID,
		Coordinator: taskpkg.CoordinatorProfile{
			Mode: taskpkg.CoordinatorMode(row.CoordinatorMode), AgentName: row.CoordinatorAgentName,
			Provider: row.CoordinatorProvider, Model: row.CoordinatorModel, Guidance: row.CoordinatorGuidance,
		},
		Worker: taskpkg.WorkerProfile{
			Mode: taskpkg.WorkerMode(row.WorkerMode), AgentName: row.WorkerAgentName,
			Provider: row.WorkerProvider, Model: row.WorkerModel,
			ReasoningEffort: row.WorkerReasoningEffort,
			Speed:           speedpkg.Speed(row.WorkerSpeed),
			ACPOptions:      workerACPOptions,
		},
		Review: taskpkg.ReviewProfile{
			AgentName: row.ReviewAgentName, Provider: row.ReviewProvider, Model: row.ReviewModel,
			ReasoningEffort: row.ReviewReasoningEffort,
			Speed:           speedpkg.Speed(row.ReviewSpeed),
			ACPOptions:      reviewACPOptions,
		},

		Worktree: taskpkg.WorktreePolicy{
			Mode: taskpkg.WorktreeMode(row.WorktreeMode), WorktreeRef: row.WorktreeRef,
		},
		Runtime: taskpkg.RuntimePolicy{Mode: taskpkg.RuntimeMode(row.RuntimeMode)},

		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}

func encodeTaskProfileACPOptions(selections []taskpkg.ACPOptionSelection) (string, error) {
	if len(selections) == 0 {
		return "[]", nil
	}
	encoded, err := json.Marshal(selections)
	if err != nil {
		return "", fmt.Errorf("marshal ACP option selections: %w", err)
	}
	return string(encoded), nil
}

func decodeTaskProfileACPOptions(encoded string) ([]taskpkg.ACPOptionSelection, error) {
	if strings.TrimSpace(encoded) == "" {
		return nil, nil
	}
	var selections []taskpkg.ACPOptionSelection
	if err := json.Unmarshal([]byte(encoded), &selections); err != nil {
		return nil, fmt.Errorf("unmarshal ACP option selections: %w", err)
	}
	return selections, nil
}
