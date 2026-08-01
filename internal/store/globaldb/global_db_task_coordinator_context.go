package globaldb

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/loop"
	taskpkg "github.com/compozy/compozy/internal/task"
)

type coordinatorResultContextPayload struct {
	WorkspaceID string `json:"workspace_id,omitempty"`
	Name        string `json:"loop_name,omitempty"`
	ParentRunID string `json:"parent_loop_run_id,omitempty"`
	Generation  int    `json:"generation,omitempty"`
	Status      string `json:"status,omitempty"`
}

func coordinatorResultContext(
	run loop.Run,
	generation int,
	status loop.Status,
) (json.RawMessage, error) {
	payload := coordinatorResultContextPayload{
		WorkspaceID: string(run.WorkspaceID),
		Name:        strings.TrimSpace(run.LoopName),
		ParentRunID: string(run.ParentLoopRunID),
		Generation:  generation,
		Status:      string(status),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("store: marshal coordinator context: %w", err)
	}
	return data, nil
}

func updateCoordinatorResultContext(
	result *taskpkg.CoordinatorCompletionResult,
	generation int,
	status loop.Status,
) error {
	var payload coordinatorResultContextPayload
	if len(result.Context) > 0 {
		if err := json.Unmarshal(result.Context, &payload); err != nil {
			return fmt.Errorf("store: decode coordinator context: %w", err)
		}
	}
	if generation > 0 {
		payload.Generation = generation
	}
	if strings.TrimSpace(string(status)) != "" {
		payload.Status = string(status)
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("store: marshal coordinator context: %w", err)
	}
	result.Context = data
	return nil
}
