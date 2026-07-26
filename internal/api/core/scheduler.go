package core

import (
	"fmt"
	"net/http"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/gin-gonic/gin"
)

// PauseTask routes task pauses through the service so claim eligibility remains centralized.
func (h *BaseHandlers) PauseTask(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}
	taskID, err := requiredPathID(c.Param("id"), "task id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	var req contract.PauseTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewTaskValidationError(fmt.Errorf("%s: decode pause task request: %w", h.transportName(), err)),
		)
		return
	}
	actor, err := h.taskActorContext(c, taskActionPauseTask)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	taskRecord, err := manager.PauseTask(
		c.Request.Context(),
		taskID,
		taskpkg.PauseTaskRequest{Reason: req.Reason, Metadata: req.Metadata},
		actor,
	)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.TaskResponse{Task: TaskPayloadFromTask(taskRecord)})
}

// ResumeTask keeps pause recovery on the same service path that owns claim eligibility.
func (h *BaseHandlers) ResumeTask(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}
	taskID, err := requiredPathID(c.Param("id"), "task id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	var req contract.ResumeTaskRequest
	if err := decodeOptionalJSON(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewTaskValidationError(fmt.Errorf("%s: decode resume task request: %w", h.transportName(), err)),
		)
		return
	}
	actor, err := h.taskActorContext(c, taskActionResumeTask)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	taskRecord, err := manager.ResumeTask(
		c.Request.Context(),
		taskID,
		taskpkg.ResumeTaskRequest{Metadata: req.Metadata},
		actor,
	)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.TaskResponse{Task: TaskPayloadFromTask(taskRecord)})
}

// GetScheduler exposes service-owned scheduler state without deriving queue pressure in the API.
func (h *BaseHandlers) GetScheduler(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}
	actor, err := h.taskActorContext(c, taskActionSchedulerStatus)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	status, err := manager.SchedulerStatus(c.Request.Context(), actor)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.SchedulerStatusResponse{Scheduler: SchedulerStatusPayloadFromDomain(status)})
}

// PauseScheduler delegates global pause state to the task service's transactional controls.
func (h *BaseHandlers) PauseScheduler(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}
	var req contract.SchedulerPauseRequest
	if err := decodeOptionalJSON(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewTaskValidationError(fmt.Errorf("%s: decode scheduler pause request: %w", h.transportName(), err)),
		)
		return
	}
	actor, err := h.taskActorContext(c, taskActionSchedulerPause)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	status, err := manager.PauseScheduler(
		c.Request.Context(),
		taskpkg.SchedulerPauseRequest{Reason: req.Reason},
		actor,
	)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.SchedulerStatusResponse{Scheduler: SchedulerStatusPayloadFromDomain(status)})
}

// ResumeScheduler clears global pause state through the same control store that claims read.
func (h *BaseHandlers) ResumeScheduler(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}
	var req contract.SchedulerResumeRequest
	if err := decodeOptionalJSON(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewTaskValidationError(fmt.Errorf("%s: decode scheduler resume request: %w", h.transportName(), err)),
		)
		return
	}
	actor, err := h.taskActorContext(c, taskActionSchedulerResume)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	status, err := manager.ResumeScheduler(
		c.Request.Context(),
		taskpkg.SchedulerResumeRequest{Reason: req.Reason},
		actor,
	)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.SchedulerStatusResponse{Scheduler: SchedulerStatusPayloadFromDomain(status)})
}

// DrainScheduler lets the task service own drain policy while the API only maps request overrides.
func (h *BaseHandlers) DrainScheduler(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}
	var req contract.SchedulerDrainRequest
	if err := decodeOptionalJSON(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewTaskValidationError(fmt.Errorf("%s: decode scheduler drain request: %w", h.transportName(), err)),
		)
		return
	}
	timeout := taskpkg.DefaultSchedulerDrainTimeout()
	if req.TimeoutSeconds != nil {
		if *req.TimeoutSeconds < 0 {
			h.respondError(
				c,
				http.StatusBadRequest,
				NewTaskValidationError(fmt.Errorf("scheduler timeout_seconds must be non-negative")),
			)
			return
		}
		timeout = time.Duration(*req.TimeoutSeconds) * time.Second
	}
	actor, err := h.taskActorContext(c, taskActionSchedulerDrain)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	result, err := manager.DrainScheduler(
		c.Request.Context(),
		taskpkg.SchedulerDrainRequest{
			Reason:  req.Reason,
			Timeout: timeout,
		},
		actor,
	)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	c.JSON(http.StatusOK, SchedulerDrainResponseFromDomain(result))
}

// GetSchedulerBacklog keeps pause-aware backlog projection behind the task service boundary.
func (h *BaseHandlers) GetSchedulerBacklog(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}
	var req contract.SchedulerBacklogQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewTaskValidationError(fmt.Errorf("%s: decode scheduler backlog query: %w", h.transportName(), err)),
		)
		return
	}
	actor, err := h.taskActorContext(c, taskActionSchedulerBacklog)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	backlog, err := manager.SchedulerBacklog(c.Request.Context(), contract.SchedulerBacklogDomainQuery(req), actor)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.SchedulerBacklogResponse{Backlog: SchedulerBacklogPayloadFromDomain(backlog)})
}

// SchedulerStatusPayloadFromDomain keeps transports on one scheduler DTO shape.
func SchedulerStatusPayloadFromDomain(status taskpkg.SchedulerStatus) contract.SchedulerStatusPayload {
	return contract.SchedulerStatusPayload{
		Paused:                 status.Paused,
		PausedBy:               status.PausedBy,
		PausedAt:               optionalTime(status.PausedAt),
		PausedReason:           status.PausedReason,
		ActiveClaimCount:       status.ActiveClaimCount,
		QueuedRunCount:         status.QueuedRunCount,
		PausedTaskCount:        status.PausedTaskCount,
		StarvedRunCount:        status.StarvedRunCount,
		NeedsAttentionRunCount: status.NeedsAttentionRunCount,
		AsOf:                   status.AsOf,
	}
}

// SchedulerDrainResponseFromDomain preserves drain result semantics across HTTP and UDS.
func SchedulerDrainResponseFromDomain(result taskpkg.SchedulerDrainResult) contract.SchedulerDrainResponse {
	return contract.SchedulerDrainResponse{
		Scheduler:       SchedulerStatusPayloadFromDomain(result.Status),
		Completed:       result.Completed,
		TimedOut:        result.TimedOut,
		RemainingClaims: result.RemainingClaims,
		StartedAt:       result.StartedAt,
		CompletedAt:     result.CompletedAt,
	}
}

// SchedulerBacklogPayloadFromDomain exposes effective pause state without leaking store rows.
func SchedulerBacklogPayloadFromDomain(backlog taskpkg.SchedulerBacklog) contract.SchedulerBacklogPayload {
	runs := make([]contract.SchedulerBacklogRunPayload, 0, len(backlog.Runs))
	for idx := range backlog.Runs {
		item := &backlog.Runs[idx]
		taskPayload := TaskSummaryPayloadFromTask(&item.Task)
		taskPayload.EffectivePaused = item.EffectivePaused
		taskPayload.PausedByTaskID = item.PausedByTaskID
		runs = append(runs, contract.SchedulerBacklogRunPayload{
			Task: taskPayload,
			Run:  TaskRunPayloadFromRun(&item.Run),
		})
	}
	return contract.SchedulerBacklogPayload{Runs: runs, Total: backlog.Total}
}

// TaskSummaryPayloadFromTask provides the compact task shape used by list and backlog surfaces.
func TaskSummaryPayloadFromTask(record *taskpkg.Task) contract.TaskSummaryPayload {
	if record == nil {
		return contract.TaskSummaryPayload{}
	}
	return contract.TaskSummaryPayload{
		ID:                 record.ID,
		Identifier:         record.Identifier,
		Scope:              record.Scope,
		WorkspaceID:        record.WorkspaceID,
		ParentTaskID:       record.ParentTaskID,
		Title:              record.Title,
		Priority:           record.Priority,
		MaxAttempts:        record.MaxAttempts,
		AutoEnqueueOnReady: record.AutoEnqueueOnReady,
		Status:             record.Status,
		ApprovalPolicy:     record.ApprovalPolicy,
		ApprovalState:      record.ApprovalState,
		Draft:              record.Status.Normalize() == taskpkg.TaskStatusDraft,
		Owner:              cloneOwnership(record.Owner),
		CurrentRunID:       record.CurrentRunID,
		LatestEventSeq:     record.LatestEventSeq,
		Paused:             record.Paused,
		PausedBy:           record.PausedBy,
		PausedAt:           optionalTime(record.PausedAt),
		PausedReason:       record.PausedReason,
		EffectivePaused:    record.Paused,
		PausedByTaskID: func() string {
			if record.Paused {
				return record.ID
			}
			return ""
		}(),
		CreatedBy:      record.CreatedBy,
		Origin:         record.Origin,
		CreatedAt:      record.CreatedAt,
		UpdatedAt:      record.UpdatedAt,
		ClosedAt:       optionalTime(record.ClosedAt),
		LastActivityAt: optionalTime(record.UpdatedAt),
	}
}
