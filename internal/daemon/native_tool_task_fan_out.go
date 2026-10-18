package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"

	compozyconfig "github.com/compozy/compozy/internal/config"

	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) taskFanOutRuns(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input taskFanOutRunsInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	if n.deps.TaskDesignations == nil {
		return toolspkg.ToolResult{}, nativeUnavailableError(req.ToolID, "task designation store is unavailable")
	}
	taskID, err := requiredNativeString(req.ToolID, "task_id", input.TaskID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	maxDesignations := n.deps.Config.Task.Orchestration.DesignatedRunMax
	if maxDesignations <= 0 {
		maxDesignations = compozyconfig.DefaultTaskDesignatedRunMax
	}
	if len(input.Designations) == 0 {
		return toolspkg.ToolResult{}, nativeRequiredInputError(req.ToolID, "designations")
	}
	prepared, err := prepareNativeFanOutDesignations(input, maxDesignations)
	if err != nil {
		return toolspkg.ToolResult{}, nativeInputError(req.ToolID, err)
	}
	actor, err := actorContextFromScope(scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	groupID, err := store.NewID("tdg")
	if err != nil {
		return toolspkg.ToolResult{}, fmt.Errorf("daemon: generate task designation group id: %w", err)
	}
	runs := make([]taskpkg.Run, 0, len(input.Designations))
	for index := range input.Designations {
		run, enqueueErr := n.deps.Tasks.EnqueueRun(ctx, taskpkg.EnqueueRun{
			TaskID:             taskID,
			IdempotencyKey:     prepared[index].idempotencyKey,
			DesignationGroupID: groupID,
			WorktreePerRun:     input.WorktreePerRun,
			Metadata:           prepared[index].metadata,
		}, actor)
		if enqueueErr != nil {
			return toolspkg.ToolResult{}, enqueueErr
		}
		runs = append(runs, *run)
	}
	if err := n.deps.TaskDesignations.PutTaskDesignationRollup(ctx, store.TaskDesignationRollup{
		DesignationGroupID: groupID,
		TaskID:             taskID,
		SummaryJSON:        nativeFanOutDesignationRollupJSON(runs),
		CreatedAt:          time.Now().UTC(),
	}); err != nil {
		return toolspkg.ToolResult{}, nativeInputError(req.ToolID, err)
	}
	return structuredResult(
		map[string]any{"designation_group_id": groupID, nativeToolsRunsKey: runs},
		fmt.Sprintf("%d runs", len(runs)),
	)
}

type nativePreparedFanOutDesignation struct {
	idempotencyKey string
	metadata       json.RawMessage
}

func prepareNativeFanOutDesignations(
	input taskFanOutRunsInput,
	maxDesignations int,
) ([]nativePreparedFanOutDesignation, error) {
	if len(input.Designations) == 0 {
		return nil, errors.New("designations are required")
	}
	if len(input.Designations) > maxDesignations {
		return nil, fmt.Errorf("designations cannot exceed %d", maxDesignations)
	}
	prepared := make([]nativePreparedFanOutDesignation, 0, len(input.Designations))
	for index, designation := range input.Designations {
		metadata, err := nativeFanOutDesignationMetadata(index, designation)
		if err != nil {
			return nil, err
		}
		idempotencyKey := nativeFanOutDesignationIdempotencyKey(input.IdempotencyKey, designation, index)
		if idempotencyKey == "" {
			return nil, fmt.Errorf(
				"designations[%d].idempotency_key is required when task fan-out idempotency_key is empty",
				index,
			)
		}
		prepared = append(prepared, nativePreparedFanOutDesignation{
			idempotencyKey: idempotencyKey,
			metadata:       metadata,
		})
	}
	return prepared, nil
}

type nativeFanOutDesignationMetadataPayload struct {
	Designation nativeFanOutDesignationMetadataDetail `json:"designation"`
	Metadata    json.RawMessage                       `json:"metadata,omitempty"`
}

type nativeFanOutDesignationMetadataDetail struct {
	Index int    `json:"index"`
	Brief string `json:"brief"`
}

func nativeFanOutDesignationMetadata(
	index int,
	designation contract.TaskFanOutRunDesignationRequest,
) (json.RawMessage, error) {
	brief := strings.TrimSpace(designation.Brief)
	if brief == "" {
		return nil, fmt.Errorf("designations[%d].brief is required", index)
	}
	metadata := cloneJSON(designation.Metadata)
	if len(metadata) > 0 && !json.Valid(metadata) {
		return nil, fmt.Errorf("designations[%d].metadata must be valid JSON", index)
	}
	encoded, err := json.Marshal(nativeFanOutDesignationMetadataPayload{
		Designation: nativeFanOutDesignationMetadataDetail{Index: index, Brief: brief},
		Metadata:    metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("task fan-out metadata: %w", err)
	}
	return encoded, nil
}

func nativeFanOutDesignationIdempotencyKey(
	requestKey string,
	designation contract.TaskFanOutRunDesignationRequest,
	index int,
) string {
	if key := strings.TrimSpace(designation.IdempotencyKey); key != "" {
		return key
	}
	if key := strings.TrimSpace(requestKey); key != "" {
		return fmt.Sprintf("%s:%d", key, index)
	}
	return ""
}

func nativeFanOutDesignationRollupJSON(runs []taskpkg.Run) json.RawMessage {
	runIDs := make([]string, 0, len(runs))
	for _, run := range runs {
		if id := strings.TrimSpace(run.ID); id != "" {
			runIDs = append(runIDs, id)
		}
	}
	encoded, err := json.Marshal(map[string]any{
		"run_ids": runIDs,
		"count":   len(runIDs),
	})
	if err != nil {
		return json.RawMessage(`{"run_ids":[],"count":0}`)
	}
	return encoded
}
