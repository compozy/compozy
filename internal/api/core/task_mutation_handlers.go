package core

import (
	"context"

	"fmt"

	"net/http"

	"github.com/compozy/agh/internal/api/contract"

	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) bulkForceTaskRuns(c *gin.Context, action string, fail bool) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}
	var req contract.BulkForceTaskRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewTaskValidationError(fmt.Errorf("%s: decode bulk force run request: %w", h.transportName(), err)),
		)
		return
	}
	actor, err := h.taskActorContext(c, action)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	domainReq := taskpkg.BulkForceRunRequest{
		RunIDs:   req.RunIDs,
		Reason:   req.Reason,
		Metadata: req.Metadata,
	}
	var result taskpkg.BulkForceRunResult
	if fail {
		result, err = manager.BulkForceFailRuns(c.Request.Context(), domainReq, actor)
	} else {
		result, err = manager.BulkForceReleaseRuns(c.Request.Context(), domainReq, actor)
	}
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	c.JSON(http.StatusOK, BulkForceTaskRunResponseFromResult(result, h.MaskInternalErrors))
}

// CancelTaskRun cancels one non-terminal run.
func (h *BaseHandlers) CancelTaskRun(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	runID, err := requiredPathID(c.Param("id"), "run id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	var req contract.CancelTaskRunRequest
	if err := decodeOptionalJSON(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewTaskValidationError(fmt.Errorf("%s: decode cancel run request: %w", h.transportName(), err)),
		)
		return
	}

	actor, err := h.taskActorContext(c, taskActionCancelRun)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	cancelReq, err := cancelTaskRunFromRequest(req)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	run, err := manager.CancelRun(c.Request.Context(), runID, cancelReq, actor)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.TaskRunResponse{Run: TaskRunPayloadFromRun(run)})
}

func (h *BaseHandlers) mutateTaskApproval(
	c *gin.Context,
	action string,
	mutate func(context.Context, TaskService, string, taskpkg.ActorContext) (*taskpkg.Task, error),
) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	taskID, err := requiredPathID(c.Param("id"), "task id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	actor, err := h.taskActorContext(c, action)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	record, err := mutate(c.Request.Context(), manager, taskID, actor)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.TaskResponse{Task: TaskPayloadFromTask(record)})
}

func (h *BaseHandlers) mutateTaskTriage(
	c *gin.Context,
	action string,
	mutate func(context.Context, TaskService, string, taskpkg.ActorContext) (taskpkg.TriageState, error),
) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	taskID, err := requiredPathID(c.Param("id"), "task id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	actor, err := h.taskActorContext(c, action)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	state, err := mutate(c.Request.Context(), manager, taskID, actor)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.TaskTriageStateResponse{Triage: TaskTriageStatePayloadFromState(state)})
}
