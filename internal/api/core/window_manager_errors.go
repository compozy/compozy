package core

import (
	"errors"
	"net/http"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/windowmanager"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) respondWindowManagerError(c *gin.Context, workspaceID windowmanager.WorkspaceID, err error) {
	status, payload := windowManagerErrorPayload(workspaceID, err)
	if status >= http.StatusInternalServerError && h.Logger != nil {
		h.Logger.Error("window-manager request failed", "workspace_id", workspaceID, "error", err)
	}
	c.JSON(status, payload)
}

func windowManagerErrorPayload(
	workspaceID windowmanager.WorkspaceID,
	err error,
) (int, contract.WindowManagerErrorPayload) {
	status, code := windowManagerErrorStatus(err)
	payload := contract.WindowManagerErrorPayload{
		Error: string(code), Code: code, WorkspaceID: workspaceID,
	}
	var conflict *windowmanager.RevisionConflictError
	if errors.As(err, &conflict) {
		if conflict.Current <= windowmanager.Revision(contract.WindowManagerMaxSafeRevision) {
			current := contract.WindowManagerRevision(conflict.Current)
			payload.CurrentRevision = &current
		}
		payload.Conflicts = append([]windowmanager.Conflict(nil), conflict.Details...)
	}
	var topology *windowmanager.TopologyError
	if errors.As(err, &topology) {
		payload.Diagnostics = append([]windowmanager.Diagnostic(nil), topology.Diagnostics...)
	}
	return status, payload
}

func windowManagerErrorStatus(err error) (int, contract.WindowManagerErrorCode) {
	switch {
	case errors.Is(err, windowmanager.ErrWorkspaceNotFound), errors.Is(err, windowmanager.ErrSnapshotNotFound):
		return http.StatusNotFound, contract.WindowManagerErrorWorkspaceNotFound
	case errors.Is(err, windowmanager.ErrDesktopNotFound):
		return http.StatusNotFound, contract.WindowManagerErrorDesktopNotFound
	case errors.Is(err, windowmanager.ErrWindowNotFound):
		return http.StatusNotFound, contract.WindowManagerErrorWindowNotFound
	case errors.Is(err, windowmanager.ErrClientNotFound):
		return http.StatusNotFound, contract.WindowManagerErrorClientNotFound
	case errors.Is(err, windowmanager.ErrLayoutResourceNotFound):
		return http.StatusNotFound, contract.WindowManagerErrorLayoutNotFound
	case errors.Is(err, windowmanager.ErrRevisionConflict):
		return http.StatusConflict, contract.WindowManagerErrorRevisionConflict
	case errors.Is(err, windowmanager.ErrFinalDesktop):
		return http.StatusConflict, contract.WindowManagerErrorFinalDesktop
	case errors.Is(err, windowmanager.ErrHistoryBoundary):
		return http.StatusConflict, contract.WindowManagerErrorHistoryBoundary
	case errors.Is(err, windowmanager.ErrSlowConsumer):
		return http.StatusConflict, contract.WindowManagerErrorSlowConsumer
	case errors.Is(err, windowmanager.ErrDestinationRequired):
		return http.StatusUnprocessableEntity, contract.WindowManagerErrorDestination
	case errors.Is(err, windowmanager.ErrInvalidTopology):
		return http.StatusUnprocessableEntity, contract.WindowManagerErrorInvalidTopology
	case errors.Is(err, windowmanager.ErrInvalidCommand), errors.Is(err, ErrUnknownJSONField):
		return http.StatusUnprocessableEntity, contract.WindowManagerErrorInvalidCommand
	default:
		return http.StatusServiceUnavailable, contract.WindowManagerErrorUnavailable
	}
}
