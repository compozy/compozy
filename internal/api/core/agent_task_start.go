package core

import (
	"fmt"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/gin-gonic/gin"
)

const agentTaskActionStart = "agent.task.start"

// AgentTaskStart starts one claimed task run under the calling session's lease.
func (h *BaseHandlers) AgentTaskStart(c *gin.Context) {
	manager, caller, runID, ok := h.agentTaskLeaseMutationSetup(c, agentTaskActionStart)
	if !ok {
		return
	}

	var req contract.StartTaskRunRequest
	if err := decodeOptionalJSON(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewTaskValidationError(fmt.Errorf("%s: decode agent task start request: %w", h.transportName(), err)),
		)
		return
	}
	startReq, err := startTaskRunFromRequest(req)
	if err != nil {
		h.respondError(c, statusForAgentTaskError(err), err)
		return
	}
	handle, err := h.lookupAgentTaskLease(c.Request.Context(), manager, caller, runID)
	if err != nil {
		h.respondError(c, statusForAgentTaskError(err), err)
		return
	}
	startReq.ClaimToken = handle.ClaimToken
	run, err := manager.StartRun(c.Request.Context(), runID, startReq, caller.Actor)
	if err != nil {
		h.respondError(c, statusForAgentTaskError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.TaskRunResponse{Run: TaskRunPayloadFromRun(run)})
}
