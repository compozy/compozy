package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

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
	snapshot, err := h.SessionProcesses.Refresh(c.Request.Context())
	if policy != contract.SessionProcessesTerminate {
		if err != nil && h.Logger != nil {
			h.Logger.Warn("Capture processes before keeping session descendants", "error", err)
		}
		return true
	}
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return false
	}
	if !snapshot.Supported {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("process termination unsupported on this host"))
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
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 2*time.Minute)
	defer cancel()
	results, err := h.SessionProcesses.Terminate(
		ctx,
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
