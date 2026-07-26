package core

import (
	"context"

	"errors"
	"fmt"

	"net/http"

	"github.com/compozy/agh/internal/api/contract"

	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/gin-gonic/gin"
)

// ListTaskRuns returns the filtered run list for one task.
func (h *BaseHandlers) ListTaskRuns(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	taskID, err := requiredPathID(c.Param("id"), "task id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	actor, err := h.taskActorContext(c, taskActionListRuns)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	transportQuery, err := ParseTaskRunListQuery(c)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	query, err := taskRunListDomainQuery(transportQuery)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	runs, err := manager.ListTaskRuns(c.Request.Context(), taskID, query, actor)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.TaskRunsResponse{Runs: TaskRunPayloadsFromRuns(runs)})
}

// GetTaskRun returns one run-detail view.
func (h *BaseHandlers) GetTaskRun(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	runID, err := requiredPathID(c.Param("id"), "run id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	actor, err := h.taskActorContext(c, taskActionGetRun)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	view, err := manager.RunDetail(c.Request.Context(), runID, actor)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	if view == nil {
		h.respondError(c, http.StatusInternalServerError, errors.New("api: task run detail is required"))
		return
	}

	payload := TaskRunDetailPayloadFromView(view)
	payload.Network, err = h.taskRunNetworkPayload(c.Request.Context(), view.Run)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, contract.TaskRunDetailResponse{Run: payload})
}

// TaskTimeline returns the task-native live timeline for one task.
func (h *BaseHandlers) TaskTimeline(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	taskID, err := requiredPathID(c.Param("id"), "task id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	actor, err := h.taskActorContext(c, taskActionTimeline)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	transportQuery, err := ParseTaskTimelineQuery(c)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	query, err := taskTimelineDomainQuery(transportQuery)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	items, err := manager.Timeline(c.Request.Context(), taskID, query, actor)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.TaskTimelineResponse{Timeline: TaskTimelineItemPayloadsFromItems(items)})
}

// StreamTask streams task-native live events over SSE.
func (h *BaseHandlers) StreamTask(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	taskID, err := requiredPathID(c.Param("id"), "task id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	actor, err := h.taskActorContext(c, taskActionStream)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	transportQuery, err := ParseTaskStreamQuery(c)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	query, err := h.taskStreamDomainQuery(c, transportQuery)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	stream, err := manager.Stream(c.Request.Context(), taskID, query, actor)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	writer, err := PrepareSSE(c)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-h.StreamDoneChannel():
			return
		case event, ok := <-stream:
			if !ok {
				return
			}
			if err := WriteTaskStreamEvent(writer, event); err != nil {
				h.logSSEWriteFailure(event.Type, err)
				return
			}
		}
	}
}

// TaskTree returns one task-tree live view.
func (h *BaseHandlers) TaskTree(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	taskID, err := requiredPathID(c.Param("id"), "task id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	actor, err := h.taskActorContext(c, taskActionTree)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	view, err := manager.Tree(c.Request.Context(), taskID, actor)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.TaskTreeResponse{Tree: TaskTreePayloadFromView(view)})
}

// TaskDashboard returns the observer-backed task dashboard view.
func (h *BaseHandlers) TaskDashboard(c *gin.Context) {
	observer, ok := h.requireTaskObserver(c)
	if !ok {
		return
	}

	transportQuery, err := ParseTaskDashboardQuery(c)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	query, err := h.taskDashboardDomainQuery(c.Request.Context(), transportQuery)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	view, err := observer.QueryTaskDashboard(c.Request.Context(), query)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.TaskDashboardResponse{Dashboard: TaskDashboardPayloadFromView(&view)})
}

// TaskInbox returns the observer-backed task inbox view.
func (h *BaseHandlers) TaskInbox(c *gin.Context) {
	observer, ok := h.requireTaskObserver(c)
	if !ok {
		return
	}

	actor, err := h.taskActorContext(c, taskActionInbox)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	transportQuery, err := ParseTaskInboxQuery(c)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	query, err := h.taskInboxDomainQuery(c.Request.Context(), transportQuery)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	view, err := observer.QueryTaskInbox(c.Request.Context(), query, actor.Actor)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.TaskInboxResponse{Inbox: TaskInboxPayloadFromView(view)})
}

// ApproveTask records one approval decision for an approval-gated task.
func (h *BaseHandlers) ApproveTask(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	taskID, err := requiredPathID(c.Param("id"), "task id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	var req contract.TaskExecutionRequest
	if err := decodeOptionalJSON(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewTaskValidationError(fmt.Errorf("%s: decode approve task request: %w", h.transportName(), err)),
		)
		return
	}

	actor, err := h.taskActorContext(c, taskActionApprove)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	executionReq, err := taskExecutionRequestFromRequest(req)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	execution, err := manager.ApproveTask(c.Request.Context(), taskID, executionReq, actor)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	c.JSON(http.StatusCreated, TaskExecutionResponseFromExecution(execution))
}

// RejectTask records one rejection decision for an approval-gated task.
func (h *BaseHandlers) RejectTask(c *gin.Context) {
	h.mutateTaskApproval(c, taskActionReject, func(
		ctx context.Context,
		manager TaskService,
		taskID string,
		actor taskpkg.ActorContext,
	) (*taskpkg.Task, error) {
		return manager.RejectTask(ctx, taskID, actor)
	})
}

// MarkTaskRead marks one task triage record as read for the current actor.
func (h *BaseHandlers) MarkTaskRead(c *gin.Context) {
	h.mutateTaskTriage(c, taskActionTriageRead, func(
		ctx context.Context,
		manager TaskService,
		taskID string,
		actor taskpkg.ActorContext,
	) (taskpkg.TriageState, error) {
		return manager.MarkTaskRead(ctx, taskID, actor)
	})
}

// ArchiveTask archives one task triage record for the current actor.
func (h *BaseHandlers) ArchiveTask(c *gin.Context) {
	h.mutateTaskTriage(c, taskActionTriageArchive, func(
		ctx context.Context,
		manager TaskService,
		taskID string,
		actor taskpkg.ActorContext,
	) (taskpkg.TriageState, error) {
		return manager.ArchiveTask(ctx, taskID, actor)
	})
}

// DismissTask dismisses one task triage record for the current actor.
func (h *BaseHandlers) DismissTask(c *gin.Context) {
	h.mutateTaskTriage(c, taskActionTriageDismiss, func(
		ctx context.Context,
		manager TaskService,
		taskID string,
		actor taskpkg.ActorContext,
	) (taskpkg.TriageState, error) {
		return manager.DismissTask(ctx, taskID, actor)
	})
}
