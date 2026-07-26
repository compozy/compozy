package core

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/gin-gonic/gin"
)

// EnqueueTaskRun creates one new queue-first run for the supplied task.
func (h *BaseHandlers) EnqueueTaskRun(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	taskID, err := requiredPathID(c.Param("id"), "task id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	var req contract.EnqueueTaskRunRequest
	if err := decodeOptionalJSON(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewTaskValidationError(fmt.Errorf("%s: decode enqueue run request: %w", h.transportName(), err)),
		)
		return
	}

	actor, err := h.taskActorContext(c, taskActionEnqueueRun)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	spec, err := enqueueTaskRunFromRequest(taskID, req)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	run, err := manager.EnqueueRun(c.Request.Context(), spec, actor)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	c.JSON(http.StatusCreated, contract.TaskRunResponse{Run: TaskRunPayloadFromRun(run)})
}

// FanOutTaskRuns creates designated sibling runs for one task.
func (h *BaseHandlers) FanOutTaskRuns(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}
	networkStore, err := h.networkStoreRequired()
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	taskID, err := requiredPathID(c.Param("id"), "task id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	var req contract.FanOutTaskRunsRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewTaskValidationError(fmt.Errorf("%s: decode fan-out task runs request: %w", h.transportName(), err)),
		)
		return
	}
	maxDesignations := h.Config.Task.Orchestration.DesignatedRunMax
	if maxDesignations <= 0 {
		maxDesignations = aghconfig.DefaultTaskDesignatedRunMax
	}
	prepared, err := prepareFanOutTaskRunsRequest(req, maxDesignations)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	actor, err := h.taskActorContext(c, taskActionFanOutRuns)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	groupID := store.NewID("tdg")
	runs, err := enqueueFanOutTaskRuns(c.Request.Context(), manager, actor, taskID, groupID, req, prepared)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	now := h.nowUTC()
	if err := networkStore.PutTaskDesignationRollup(
		c.Request.Context(),
		store.TaskDesignationRollup{
			DesignationGroupID: groupID,
			TaskID:             taskID,
			SummaryJSON:        fanOutDesignationRollupJSON(runs, now),
			CreatedAt:          now,
		},
	); err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	c.JSON(http.StatusCreated, contract.FanOutTaskRunsResponse{
		DesignationGroupID: groupID,
		Runs:               TaskRunPayloadsFromRuns(runs),
	})
}

// StartTaskRun starts one claimed run.
func (h *BaseHandlers) StartTaskRun(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	runID, err := requiredPathID(c.Param("id"), "run id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	var req contract.StartTaskRunRequest
	if err := decodeOptionalJSON(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewTaskValidationError(fmt.Errorf("%s: decode start run request: %w", h.transportName(), err)),
		)
		return
	}

	actor, err := h.taskActorContext(c, taskActionStartRun)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	startReq, err := startTaskRunFromRequest(req)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	run, err := manager.StartRun(c.Request.Context(), runID, startReq, actor)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.TaskRunResponse{Run: TaskRunPayloadFromRun(run)})
}

// AttachTaskRunSession binds one existing session to a run.
func (h *BaseHandlers) AttachTaskRunSession(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	runID, err := requiredPathID(c.Param("id"), "run id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	var req contract.AttachTaskRunSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewTaskValidationError(fmt.Errorf("%s: decode attach run session request: %w", h.transportName(), err)),
		)
		return
	}

	actor, err := h.taskActorContext(c, taskActionAttachRun)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	sessionID, err := attachTaskRunSessionIDFromRequest(req)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	run, err := manager.AttachRunSession(c.Request.Context(), runID, sessionID, actor)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.TaskRunResponse{Run: TaskRunPayloadFromRun(run)})
}

// CompleteTaskRun marks one running run as completed.
func (h *BaseHandlers) CompleteTaskRun(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	runID, err := requiredPathID(c.Param("id"), "run id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	var req contract.CompleteTaskRunRequest
	if err := decodeOptionalJSON(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewTaskValidationError(fmt.Errorf("%s: decode complete run request: %w", h.transportName(), err)),
		)
		return
	}

	actor, err := h.taskActorContext(c, taskActionCompleteRun)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	result, err := completeTaskRunFromRequest(req)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	run, err := manager.CompleteRun(c.Request.Context(), runID, result, actor)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.TaskRunResponse{Run: TaskRunPayloadFromRun(run)})
}

// FailTaskRun marks one run as failed.
func (h *BaseHandlers) FailTaskRun(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	runID, err := requiredPathID(c.Param("id"), "run id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	var req contract.FailTaskRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewTaskValidationError(fmt.Errorf("%s: decode fail run request: %w", h.transportName(), err)),
		)
		return
	}

	actor, err := h.taskActorContext(c, taskActionFailRun)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	failure, err := failTaskRunFromRequest(req)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	run, err := manager.FailRun(c.Request.Context(), runID, failure, actor)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.TaskRunResponse{Run: TaskRunPayloadFromRun(run)})
}

// ForceReleaseTaskRun force releases one claimed run without requiring the raw claim token.
func (h *BaseHandlers) ForceReleaseTaskRun(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}
	runID, err := requiredPathID(c.Param("id"), "run id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	var req contract.ForceReleaseTaskRunRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewTaskValidationError(fmt.Errorf("%s: decode force release run request: %w", h.transportName(), err)),
		)
		return
	}
	actor, err := h.taskActorContext(c, taskActionForceReleaseRun)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	run, err := manager.ForceReleaseRun(
		c.Request.Context(),
		runID,
		taskpkg.ForceReleaseRun{Reason: req.Reason, Metadata: req.Metadata},
		actor,
	)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.TaskRunResponse{Run: TaskRunPayloadFromRun(run)})
}

// ForceFailTaskRun force fails one queued or claimed run without requiring the raw claim token.
func (h *BaseHandlers) ForceFailTaskRun(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}
	runID, err := requiredPathID(c.Param("id"), "run id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	var req contract.ForceFailTaskRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewTaskValidationError(fmt.Errorf("%s: decode force fail run request: %w", h.transportName(), err)),
		)
		return
	}
	actor, err := h.taskActorContext(c, taskActionForceFailRun)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	run, err := manager.ForceFailRun(
		c.Request.Context(),
		runID,
		taskpkg.ForceFailRun{Reason: req.Reason, Metadata: req.Metadata},
		actor,
	)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.TaskRunResponse{Run: TaskRunPayloadFromRun(run)})
}

// RetryTaskRun enqueues a new run linked to one failed source run.
func (h *BaseHandlers) RetryTaskRun(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}
	runID, err := requiredPathID(c.Param("id"), "run id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	var req contract.RetryTaskRunRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewTaskValidationError(fmt.Errorf("%s: decode retry run request: %w", h.transportName(), err)),
		)
		return
	}
	actor, err := h.taskActorContext(c, taskActionRetryRun)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	result, err := manager.RetryRun(
		c.Request.Context(),
		runID,
		taskpkg.RetryRunRequest{Metadata: req.Metadata},
		actor,
	)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	c.JSON(http.StatusCreated, RetryTaskRunResponseFromResult(result))
}

// RecoverTaskRun terminalizes one needs_attention run and queues a fresh child to resume work.
func (h *BaseHandlers) RecoverTaskRun(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}
	runID, err := requiredPathID(c.Param("id"), "run id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	var req contract.RecoverTaskRunRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewTaskValidationError(fmt.Errorf("%s: decode recover run request: %w", h.transportName(), err)),
		)
		return
	}
	actor, err := h.taskActorContext(c, taskActionRecoverRun)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	result, err := manager.RecoverRun(
		c.Request.Context(),
		runID,
		taskpkg.RecoverRunRequest{Reason: req.Reason, Metadata: req.Metadata},
		actor,
	)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	c.JSON(http.StatusCreated, RetryTaskRunResponseFromResult(result))
}

// BulkForceReleaseTaskRuns force releases a bounded set of runs.
func (h *BaseHandlers) BulkForceReleaseTaskRuns(c *gin.Context) {
	h.bulkForceTaskRuns(c, taskActionBulkReleaseRuns, false)
}

// BulkForceFailTaskRuns force fails a bounded set of runs.
func (h *BaseHandlers) BulkForceFailTaskRuns(c *gin.Context) {
	h.bulkForceTaskRuns(c, taskActionBulkFailRuns, true)
}
