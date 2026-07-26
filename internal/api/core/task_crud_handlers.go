package core

import (
	"context"

	"errors"
	"fmt"

	"net/http"
	"strings"

	"github.com/compozy/agh/internal/api/contract"

	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/gin-gonic/gin"
)

// ListTasks returns the filtered task list.
func (h *BaseHandlers) ListTasks(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	actor, err := h.taskActorContext(c, taskActionList)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	transportQuery, err := ParseTaskListQuery(c)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	query, err := h.taskListDomainQuery(c.Request.Context(), transportQuery)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	page, err := manager.ListTaskCatalog(c.Request.Context(), query, actor)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	c.JSON(http.StatusOK, TaskCatalogResponseFromPage(page))
}

// CreateTask creates one new task.
func (h *BaseHandlers) CreateTask(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	var req contract.CreateTaskRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewTaskValidationError(fmt.Errorf("%s: decode create task request: %w", h.transportName(), err)),
		)
		return
	}

	spec, err := h.createTaskSpecFromRequest(c.Request.Context(), req)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	actor, err := h.taskActorContextForWorkspace(c, taskActionCreate, spec.WorkspaceID)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	record, err := manager.CreateTask(c.Request.Context(), spec, actor)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	c.JSON(http.StatusCreated, contract.TaskResponse{Task: TaskPayloadFromTask(record)})
}

// GetTask returns one expanded task view.
func (h *BaseHandlers) GetTask(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	taskID, err := requiredPathID(c.Param("id"), "task id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	actor, err := h.taskActorContext(c, taskActionGet)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	view, err := manager.GetTask(c.Request.Context(), taskID, actor)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	payload, err := h.taskDetailPayload(c.Request.Context(), view)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.TaskDetailResponse{Task: payload})
}

// BlockTask creates one runtime-declared block for a task.
func (h *BaseHandlers) BlockTask(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	taskID, err := requiredPathID(c.Param("id"), "task id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	var req contract.CreateTaskBlockRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewTaskValidationError(fmt.Errorf("%s: decode block task request: %w", h.transportName(), err)),
		)
		return
	}

	actor, err := h.taskActorContext(c, taskActionBlock)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	claimToken, err := h.taskBlockClaimToken(c, manager, req)
	if err != nil {
		h.respondError(c, statusForTaskBlockError(err), err)
		return
	}
	blockReq, err := createTaskBlockFromRequest(taskID, req, claimToken)
	if err != nil {
		h.respondError(c, statusForTaskBlockError(err), err)
		return
	}
	block, err := manager.BlockTask(c.Request.Context(), blockReq, actor)
	if err != nil {
		h.respondError(c, statusForTaskBlockError(err), err)
		return
	}

	c.JSON(http.StatusCreated, contract.TaskBlockResponse{Block: TaskBlockPayloadFromBlock(block)})
}

func (h *BaseHandlers) taskBlockClaimToken(
	c *gin.Context,
	manager TaskService,
	req contract.CreateTaskBlockRequest,
) (string, error) {
	claimToken := strings.TrimSpace(c.GetHeader(taskClaimTokenHeader))
	if claimToken != "" || strings.TrimSpace(req.RunID) == "" {
		return claimToken, nil
	}
	credentials := agentCallerCredentialsFromRequest(c)
	if !hasAgentCallerIdentityCredentials(credentials) {
		return "", nil
	}
	caller, err := h.resolveAgentCallerForWorkspace(
		c.Request.Context(),
		credentials,
		"tasks."+taskActionBlock,
		"",
	)
	if err != nil {
		return "", err
	}
	handle, err := h.lookupAgentTaskLease(c.Request.Context(), manager, caller, req.RunID)
	if err != nil {
		return "", err
	}
	return handle.ClaimToken, nil
}

// ListTaskBlocks returns task block rows for one task.
func (h *BaseHandlers) ListTaskBlocks(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	taskID, err := requiredPathID(c.Param("id"), "task id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	includeCleared, err := parseBoolQuery(c, "include_cleared")
	if err != nil {
		h.respondError(c, http.StatusBadRequest, NewTaskValidationError(err))
		return
	}

	actor, err := h.taskActorContext(c, taskActionListBlocks)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	blocks, err := manager.ListTaskBlocks(c.Request.Context(), taskID, includeCleared, actor)
	if err != nil {
		h.respondError(c, statusForTaskBlockError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.TaskBlocksResponse{Blocks: TaskBlockPayloadsFromBlocks(blocks)})
}

// ClearTaskBlock clears one open task block.
func (h *BaseHandlers) ClearTaskBlock(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	taskID, err := requiredPathID(c.Param("id"), "task id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	blockID, err := requiredPathID(c.Param("block_id"), "block id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	var req contract.ClearTaskBlockRequest
	if err := decodeOptionalJSON(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewTaskValidationError(fmt.Errorf("%s: decode clear task block request: %w", h.transportName(), err)),
		)
		return
	}

	actor, err := h.taskActorContext(c, taskActionClearBlock)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	block, err := manager.ClearTaskBlock(c.Request.Context(), taskID, blockID, strings.TrimSpace(req.Note), actor)
	if err != nil {
		h.respondError(c, statusForTaskBlockError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.TaskBlockResponse{Block: TaskBlockPayloadFromBlock(block)})
}

// RecoverTask clears task-level needs_attention state.
func (h *BaseHandlers) RecoverTask(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	taskID, err := requiredPathID(c.Param("id"), "task id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	var req contract.RecoverTaskRequest
	if err := decodeOptionalJSON(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewTaskValidationError(fmt.Errorf("%s: decode recover task request: %w", h.transportName(), err)),
		)
		return
	}

	actor, err := h.taskActorContext(c, taskActionRecover)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	record, err := manager.RecoverTask(c.Request.Context(), taskID, strings.TrimSpace(req.Note), actor)
	if err != nil {
		h.respondError(c, statusForTaskBlockError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.TaskResponse{Task: TaskPayloadFromTask(record)})
}

func (h *BaseHandlers) taskDetailPayload(ctx context.Context, view *taskpkg.View) (contract.TaskDetailPayload, error) {
	payload := TaskDetailPayloadFromView(view)
	if h == nil || h.NetworkStore == nil || view == nil || strings.TrimSpace(view.Task.ID) == "" {
		return payload, nil
	}
	rollups, err := h.NetworkStore.ListTaskDesignationRollups(ctx, store.TaskDesignationRollupQuery{
		TaskID: strings.TrimSpace(view.Task.ID),
		Limit:  taskDesignationRollupDetailLimit,
	})
	if err != nil {
		return contract.TaskDetailPayload{}, fmt.Errorf("api: list task designation rollups: %w", err)
	}
	payload.DesignationRollups = TaskDesignationRollupPayloadsFromStore(rollups)
	return payload, nil
}

// InspectTask returns a diagnostic snapshot for one task.
func (h *BaseHandlers) InspectTask(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	taskID, err := requiredPathID(c.Param("id"), "task id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	actor, err := h.taskActorContext(c, taskActionInspect)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	view, err := manager.InspectTask(c.Request.Context(), taskID, actor)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.TaskInspectResponse{Inspect: TaskInspectPayloadFromView(view)})
}

// InspectRun returns a diagnostic snapshot rooted at one run.
func (h *BaseHandlers) InspectRun(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	runID, err := requiredPathID(c.Param("id"), "run id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	actor, err := h.taskActorContext(c, taskActionInspect)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	view, err := manager.InspectRun(c.Request.Context(), runID, actor)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.TaskInspectResponse{Inspect: TaskInspectPayloadFromView(view)})
}

// DeleteTask removes one task record and any cascade-owned child rows.
func (h *BaseHandlers) DeleteTask(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	taskID, err := requiredPathID(c.Param("id"), "task id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	actor, err := h.taskActorContext(c, taskActionDelete)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	if err := manager.DeleteTask(c.Request.Context(), taskID, actor); err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	c.Status(http.StatusNoContent)
}

// UpdateTask patches one mutable task surface.
func (h *BaseHandlers) UpdateTask(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	taskID, err := requiredPathID(c.Param("id"), "task id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	var req contract.UpdateTaskRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewTaskValidationError(fmt.Errorf("%s: decode update task request: %w", h.transportName(), err)),
		)
		return
	}
	if !req.HasChanges() {
		err := NewTaskValidationError(errors.New("task update must include at least one mutable field"))
		h.respondError(c, http.StatusBadRequest, err)
		return
	}

	actor, err := h.taskActorContext(c, taskActionUpdate)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	patch, err := taskPatchFromRequest(req)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	record, err := manager.UpdateTask(c.Request.Context(), taskID, patch, actor)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.TaskResponse{Task: TaskPayloadFromTask(record)})
}
