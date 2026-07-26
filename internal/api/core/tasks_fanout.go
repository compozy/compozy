package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/network/participation"
	taskpkg "github.com/compozy/agh/internal/task"
)

type preparedFanOutDesignation struct {
	idempotencyKey string
	metadata       json.RawMessage
}

func prepareFanOutTaskRunsRequest(
	req contract.FanOutTaskRunsRequest,
	maxDesignations int,
) ([]preparedFanOutDesignation, error) {
	if len(req.Designations) == 0 {
		return nil, NewTaskValidationError(errors.New("designations are required"))
	}
	if len(req.Designations) > maxDesignations {
		return nil, NewTaskValidationError(fmt.Errorf("designations cannot exceed %d", maxDesignations))
	}
	prepared := make([]preparedFanOutDesignation, 0, len(req.Designations))
	for index, designation := range req.Designations {
		metadata, err := fanOutDesignationMetadata(index, designation)
		if err != nil {
			return nil, err
		}
		idempotencyKey := fanOutDesignationIdempotencyKey(req.IdempotencyKey, designation, index)
		if idempotencyKey == "" {
			return nil, NewTaskValidationError(fmt.Errorf(
				"designations[%d].idempotency_key is required when fan_out_runs.idempotency_key is empty",
				index,
			))
		}
		prepared = append(prepared, preparedFanOutDesignation{
			idempotencyKey: idempotencyKey,
			metadata:       metadata,
		})
	}
	return prepared, nil
}

func enqueueFanOutTaskRuns(
	ctx context.Context,
	manager TaskService,
	actor taskpkg.ActorContext,
	taskID string,
	groupID string,
	req contract.FanOutTaskRunsRequest,
	prepared []preparedFanOutDesignation,
) ([]taskpkg.Run, error) {
	runs := make([]taskpkg.Run, 0, len(req.Designations))
	for index := range req.Designations {
		run, err := manager.EnqueueRun(ctx, taskpkg.EnqueueRun{
			TaskID:               taskID,
			IdempotencyKey:       prepared[index].idempotencyKey,
			DesignationGroupID:   groupID,
			Metadata:             prepared[index].metadata,
			NetworkParticipation: participation.CloneRequest(req.NetworkParticipation),
		}, actor)
		if err != nil {
			return nil, err
		}
		runs = append(runs, *run)
	}
	return runs, nil
}
