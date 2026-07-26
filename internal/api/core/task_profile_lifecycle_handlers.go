package core

import (
	"fmt"

	"net/http"

	"github.com/compozy/agh/internal/api/contract"

	"github.com/gin-gonic/gin"
)

// GetTaskExecutionProfile returns one task-owned execution profile.
func (h *BaseHandlers) GetTaskExecutionProfile(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	taskID, err := requiredPathID(c.Param("id"), "task id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	actor, err := h.taskActorContext(c, taskActionGetProfile)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	profile, err := manager.GetExecutionProfile(c.Request.Context(), taskID, actor)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.TaskExecutionProfileResponse{Profile: profile})
}

// SetTaskExecutionProfile replaces one task-owned execution profile.
func (h *BaseHandlers) SetTaskExecutionProfile(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	taskID, err := requiredPathID(c.Param("id"), "task id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	var req contract.SetTaskExecutionProfileRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewTaskValidationError(fmt.Errorf("%s: decode task execution profile request: %w", h.transportName(), err)),
		)
		return
	}

	profile, err := taskExecutionProfileFromRequest(taskID, &req)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	actor, err := h.taskActorContext(c, taskActionSetProfile)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	stored, err := manager.SetExecutionProfile(c.Request.Context(), taskID, profile, actor)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.TaskExecutionProfileResponse{Profile: stored})
}

// DeleteTaskExecutionProfile removes one persisted task-owned execution profile.
func (h *BaseHandlers) DeleteTaskExecutionProfile(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	taskID, err := requiredPathID(c.Param("id"), "task id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	actor, err := h.taskActorContext(c, taskActionDeleteProfile)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	if err := manager.DeleteExecutionProfile(c.Request.Context(), taskID, actor); err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	c.Status(http.StatusNoContent)
}

// PublishTask publishes one draft task into the canonical runnable lifecycle.
func (h *BaseHandlers) PublishTask(c *gin.Context) {
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
			NewTaskValidationError(fmt.Errorf("%s: decode publish task request: %w", h.transportName(), err)),
		)
		return
	}

	actor, err := h.taskActorContext(c, taskActionPublish)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	executionReq, err := taskExecutionRequestFromRequest(req)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	execution, err := manager.PublishTask(c.Request.Context(), taskID, executionReq, actor)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	c.JSON(http.StatusOK, TaskExecutionResponseFromExecution(execution))
}

// StartTask explicitly enqueues one executable run for an existing task.
func (h *BaseHandlers) StartTask(c *gin.Context) {
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
			NewTaskValidationError(fmt.Errorf("%s: decode start task request: %w", h.transportName(), err)),
		)
		return
	}

	actor, err := h.taskActorContext(c, taskActionStart)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	executionReq, err := taskExecutionRequestFromRequest(req)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	execution, err := manager.StartTask(c.Request.Context(), taskID, executionReq, actor)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	c.JSON(http.StatusCreated, TaskExecutionResponseFromExecution(execution))
}

// CancelTask requests cancellation for one task tree.
func (h *BaseHandlers) CancelTask(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	taskID, err := requiredPathID(c.Param("id"), "task id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	var req contract.CancelTaskRequest
	if err := decodeOptionalJSON(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewTaskValidationError(fmt.Errorf("%s: decode cancel task request: %w", h.transportName(), err)),
		)
		return
	}

	actor, err := h.taskActorContext(c, taskActionCancel)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	cancelReq, err := cancelTaskFromRequest(req)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	record, err := manager.CancelTask(c.Request.Context(), taskID, cancelReq, actor)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.TaskResponse{Task: TaskPayloadFromTask(record)})
}

// CreateChildTask creates one child task beneath the supplied parent.
func (h *BaseHandlers) CreateChildTask(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	parentTaskID, err := requiredPathID(c.Param("id"), "task id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	var req contract.CreateTaskChildRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewTaskValidationError(fmt.Errorf("%s: decode create child task request: %w", h.transportName(), err)),
		)
		return
	}

	actor, err := h.taskActorContext(c, taskActionCreateChild)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	spec, err := h.createChildTaskSpecFromRequest(c.Request.Context(), req)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	record, err := manager.CreateChildTask(c.Request.Context(), parentTaskID, spec, actor)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	c.JSON(http.StatusCreated, contract.TaskResponse{Task: TaskPayloadFromTask(record)})
}

// AddTaskDependency adds one blocking dependency edge.
func (h *BaseHandlers) AddTaskDependency(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	taskID, err := requiredPathID(c.Param("id"), "task id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	var req contract.AddTaskDependencyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewTaskValidationError(fmt.Errorf("%s: decode add dependency request: %w", h.transportName(), err)),
		)
		return
	}

	actor, err := h.taskActorContext(c, taskActionAddDependency)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	spec, err := addTaskDependencyFromRequest(taskID, req)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	if err := manager.AddDependency(c.Request.Context(), spec, actor); err != nil {
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

// RemoveTaskDependency removes one blocking dependency edge.
func (h *BaseHandlers) RemoveTaskDependency(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	taskID, err := requiredPathID(c.Param("id"), "task id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	dependsOnID, err := requiredPathID(c.Param("depends_on_id"), "depends_on_id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	actor, err := h.taskActorContext(c, taskActionRemoveDependency)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	if err := manager.RemoveDependency(c.Request.Context(), taskID, dependsOnID, actor); err != nil {
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
