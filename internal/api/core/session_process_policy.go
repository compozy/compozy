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
	ctx := c.Request.Context()
	if policy != contract.SessionProcessesTerminate {
		// Keep remains available if capture times out; retained discovery may use an incomplete pre-stop snapshot.
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, sessionprocess.CaptureTimeout)
		defer cancel()
	}
	snapshot, err := h.SessionProcesses.Refresh(ctx)
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
	return h.finishTreeProcessPolicy(c, []*session.Info{info}, policy)
}

// finishTreeProcessPolicy attempts every member, whose session may be gone and
// beyond any retry, then reports the first failure.
func (h *BaseHandlers) finishTreeProcessPolicy(
	c *gin.Context,
	members []*session.Info,
	policy contract.SessionProcessPolicy,
) bool {
	var failure *processPolicyFailure
	for _, member := range members {
		if current := h.settleProcessPolicy(c, member, policy); failure == nil {
			failure = current
		}
	}
	if failure != nil {
		h.respondError(c, failure.status, failure.err)
		return false
	}
	return true
}

type processPolicyFailure struct {
	status int
	err    error
}

func (h *BaseHandlers) settleProcessPolicy(
	c *gin.Context,
	info *session.Info,
	policy contract.SessionProcessPolicy,
) *processPolicyFailure {
	if policy != contract.SessionProcessesTerminate {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 2*time.Minute)
	defer cancel()
	results, err := h.SessionProcesses.Terminate(
		ctx,
		info.ID,
		nil,
		sessionprocess.AllIncludingTools,
		func(o sessionprocess.Owner) bool {
			return o.ProfileID == info.ProfileID && o.WorkspaceID == info.WorkspaceID
		},
	)
	if err != nil {
		return &processPolicyFailure{status: http.StatusInternalServerError, err: err}
	}
	for _, r := range results {
		if r.Outcome != "terminated" && r.Outcome != "already_exited" {
			return &processPolicyFailure{
				status: http.StatusConflict,
				err:    fmt.Errorf("process %d termination is %s", r.PID, r.Outcome),
			}
		}
	}
	return nil
}

// A caller deadline does not cancel the accepted shared stop operation.
func (h *BaseHandlers) continueProcessCleanup(id string, policy contract.SessionProcessPolicy, err error) {
	if policy != contract.SessionProcessesTerminate ||
		(!errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled)) {
		return
	}
	if manager, ok := h.Sessions.(SessionStopManager); ok {
		h.SessionProcesses.AfterStop(id, func(ctx context.Context) error {
			_, err := manager.AwaitStopped(ctx, id)
			return err
		})
	}
}
