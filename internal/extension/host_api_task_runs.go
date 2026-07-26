package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"

	"strings"
)

func (h *HostAPIHandler) handleTasksRuns(ctx context.Context, raw json.RawMessage) (any, error) {
	var params hostAPITaskRunsParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	taskID := strings.TrimSpace(params.ID)
	if taskID == "" {
		return nil, invalidParamsRPCError(errors.New("id is required"))
	}

	manager, actor, err := h.taskManagerAndActor(ctx)
	if err != nil {
		return nil, err
	}
	query, err := taskRunQueryFromParams(params.TaskRunListQuery)
	if err != nil {
		return nil, err
	}

	runs, err := manager.ListTaskRuns(ctx, taskID, query, actor)
	if err != nil {
		return nil, mapTaskRPCError(taskID, err)
	}
	return taskRunPayloadsFromRuns(runs), nil
}

func (h *HostAPIHandler) handleTasksRunsGet(ctx context.Context, raw json.RawMessage) (any, error) {
	var params hostAPITaskRunGetParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	runID := strings.TrimSpace(params.ID)
	if runID == "" {
		return nil, invalidParamsRPCError(errors.New("id is required"))
	}

	manager, actor, err := h.taskManagerAndActor(ctx)
	if err != nil {
		return nil, err
	}

	view, err := manager.RunDetail(ctx, runID, actor)
	if err != nil {
		return nil, mapTaskRPCError(runID, err)
	}
	payload, err := h.taskRunDetailPayloadFromView(ctx, view)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func (h *HostAPIHandler) handleTasksRunsEnqueue(ctx context.Context, raw json.RawMessage) (any, error) {
	var params hostAPITaskRunEnqueueParams
	if err := decodeHostAPIParamsStrict(raw, &params); err != nil {
		return nil, err
	}
	taskID := strings.TrimSpace(params.TaskID)
	if taskID == "" {
		return nil, invalidParamsRPCError(errors.New("task_id is required"))
	}

	manager, actor, err := h.taskManagerAndActor(ctx)
	if err != nil {
		return nil, err
	}
	spec, err := enqueueTaskRunFromRequest(taskID, params.EnqueueTaskRunRequest)
	if err != nil {
		return nil, err
	}

	run, err := manager.EnqueueRun(ctx, spec, actor)
	if err != nil {
		return nil, mapTaskRPCError(taskID, err)
	}
	return taskRunPayloadFromRun(run), nil
}

func (h *HostAPIHandler) handleTasksRunsStart(ctx context.Context, raw json.RawMessage) (any, error) {
	var params hostAPITaskRunStartParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	runID := strings.TrimSpace(params.ID)
	if runID == "" {
		return nil, invalidParamsRPCError(errors.New("id is required"))
	}

	manager, actor, err := h.taskManagerAndActor(ctx)
	if err != nil {
		return nil, err
	}
	startReq, err := startTaskRunFromRequest(params.StartTaskRunRequest)
	if err != nil {
		return nil, err
	}

	run, err := manager.StartRun(ctx, runID, startReq, actor)
	if err != nil {
		return nil, mapTaskRPCError(runID, err)
	}
	return taskRunPayloadFromRun(run), nil
}

func (h *HostAPIHandler) handleTasksRunsAttachSession(ctx context.Context, raw json.RawMessage) (any, error) {
	var params hostAPITaskRunAttachSessionParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	runID := strings.TrimSpace(params.ID)
	if runID == "" {
		return nil, invalidParamsRPCError(errors.New("id is required"))
	}

	manager, actor, err := h.taskManagerAndActor(ctx)
	if err != nil {
		return nil, err
	}
	sessionID, err := attachTaskRunSessionIDFromRequest(params.AttachTaskRunSessionRequest)
	if err != nil {
		return nil, err
	}

	run, err := manager.AttachRunSession(ctx, runID, sessionID, actor)
	if err != nil {
		return nil, mapTaskRPCError(runID, err)
	}
	return taskRunPayloadFromRun(run), nil
}

func (h *HostAPIHandler) handleTasksRunsComplete(ctx context.Context, raw json.RawMessage) (any, error) {
	var params hostAPITaskRunCompleteParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	runID := strings.TrimSpace(params.ID)
	if runID == "" {
		return nil, invalidParamsRPCError(errors.New("id is required"))
	}

	manager, actor, err := h.taskManagerAndActor(ctx)
	if err != nil {
		return nil, err
	}
	result, err := completeTaskRunFromRequest(params.CompleteTaskRunRequest)
	if err != nil {
		return nil, err
	}

	run, err := manager.CompleteRun(ctx, runID, result, actor)
	if err != nil {
		return nil, mapTaskRPCError(runID, err)
	}
	return taskRunPayloadFromRun(run), nil
}

func (h *HostAPIHandler) handleTasksRunsFail(ctx context.Context, raw json.RawMessage) (any, error) {
	var params hostAPITaskRunFailParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	runID := strings.TrimSpace(params.ID)
	if runID == "" {
		return nil, invalidParamsRPCError(errors.New("id is required"))
	}

	manager, actor, err := h.taskManagerAndActor(ctx)
	if err != nil {
		return nil, err
	}
	failure, err := failTaskRunFromRequest(params.FailTaskRunRequest)
	if err != nil {
		return nil, err
	}

	run, err := manager.FailRun(ctx, runID, failure, actor)
	if err != nil {
		return nil, mapTaskRPCError(runID, err)
	}
	return taskRunPayloadFromRun(run), nil
}

func (h *HostAPIHandler) handleTasksRunsCancel(ctx context.Context, raw json.RawMessage) (any, error) {
	var params hostAPITaskRunCancelParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	runID := strings.TrimSpace(params.ID)
	if runID == "" {
		return nil, invalidParamsRPCError(errors.New("id is required"))
	}

	manager, actor, err := h.taskManagerAndActor(ctx)
	if err != nil {
		return nil, err
	}
	cancelReq, err := cancelTaskRunFromRequest(params.CancelTaskRunRequest)
	if err != nil {
		return nil, err
	}

	run, err := manager.CancelRun(ctx, runID, cancelReq, actor)
	if err != nil {
		return nil, mapTaskRPCError(runID, err)
	}
	return taskRunPayloadFromRun(run), nil
}
