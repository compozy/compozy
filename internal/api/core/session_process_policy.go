package core

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/sessionprocess"
	"github.com/gin-gonic/gin"
)

// Capture ownership before stop/archive/delete can remove the live lineage or session row.
func (h *BaseHandlers) prepareProcessPolicy(c *gin.Context, policy contract.SessionProcessPolicy) bool {
	if policy != "" && policy != contract.SessionProcessesKeep && policy != contract.SessionProcessesTerminate {
		h.respondError(c, http.StatusBadRequest, errors.New("processes must be keep or terminate"))
		return false
	}
	if h.SessionProcesses == nil {
		return policy != contract.SessionProcessesTerminate || h.requireSessionProcesses(c)
	}
	if _, err := h.SessionProcesses.Refresh(c.Request.Context()); err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return false
	}
	return true
}

func (h *BaseHandlers) finishProcessPolicy(
	c *gin.Context,
	info *session.Info,
	policy contract.SessionProcessPolicy,
) bool {
	if policy != contract.SessionProcessesTerminate {
		return true
	}
	results, err := h.SessionProcesses.Terminate(
		c.Request.Context(),
		info.ID,
		nil,
		true,
		func(o sessionprocess.Owner) bool {
			return o.ProfileID == info.ProfileID && o.WorkspaceID == info.WorkspaceID
		},
	)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return false
	}
	for _, r := range results {
		if r.Outcome != "terminated" && r.Outcome != "already_exited" {
			h.respondError(c, http.StatusConflict, fmt.Errorf("process %d termination is %s", r.PID, r.Outcome))
			return false
		}
	}
	return true
}
