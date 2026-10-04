package core

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/contract"
	taskpkg "github.com/compozy/compozy/internal/task"
)

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

	h.respondTaskInspect(c, view)
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

	h.respondTaskInspect(c, view)
}

func (h *BaseHandlers) respondTaskInspect(c *gin.Context, view *taskpkg.InspectView) {
	payload := TaskInspectPayloadFromView(view)
	owners, err := h.profileOwnerIdentities(c.Request.Context())
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	if err := setTaskSummaryProfileOwner(owners, &payload.Task, h == nil || h.Profiles == nil); err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, contract.TaskInspectResponse{Inspect: payload})
}
