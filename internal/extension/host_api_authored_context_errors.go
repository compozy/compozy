package extensionpkg

import (
	"errors"

	"net/http"
	"os"

	apicontract "github.com/compozy/agh/internal/api/contract"

	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/soul"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func mapHostAPISoulRPCError(err error) error {
	if err == nil {
		return nil
	}
	status := hostAPISoulHTTPStatus(err)
	return hostAPIStatusRPCError(status, http.StatusText(status), map[string]string{extensionStateError: err.Error()})
}

func mapHostAPIHeartbeatRPCError(err error) error {
	if err == nil {
		return nil
	}
	status := hostAPIHeartbeatHTTPStatus(err)
	return hostAPIStatusRPCError(status, http.StatusText(status), map[string]string{extensionStateError: err.Error()})
}

func hostAPISoulHTTPStatus(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, workspacepkg.ErrWorkspaceResolverUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, errHostAPIAuthoredValidation):
		return http.StatusBadRequest
	case errors.Is(err, soul.ErrAuthoringConflict),
		errors.Is(err, session.ErrSoulRefreshConflict),
		errors.Is(err, session.ErrSoulRefreshDigestConflict),
		errors.Is(err, session.ErrSessionNotActive):
		return http.StatusConflict
	case errors.Is(err, soul.ErrAuthoringAgentNotFound),
		errors.Is(err, soul.ErrAuthoringMissing),
		errors.Is(err, soul.ErrRevisionNotFound),
		errors.Is(err, soul.ErrSnapshotNotFound),
		errors.Is(err, session.ErrSessionNotFound),
		errors.Is(err, workspacepkg.ErrWorkspaceNotFound),
		errors.Is(err, os.ErrNotExist):
		return http.StatusNotFound
	case errors.Is(err, workspacepkg.ErrWorkspaceRootMissing):
		return http.StatusGone
	case errors.Is(err, soul.ErrInvalid),
		errors.Is(err, soul.ErrInvalidSnapshot),
		errors.Is(err, soul.ErrInvalidRevision),
		errors.Is(err, soul.ErrAuthoringPathRejected),
		errors.Is(err, apicontract.ErrInvalidAuthoredContextEnum):
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}

func hostAPIHeartbeatHTTPStatus(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, workspacepkg.ErrWorkspaceResolverUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, errHostAPIAuthoredValidation):
		return http.StatusBadRequest
	case errors.Is(err, heartbeat.ErrAuthoringConflict):
		return http.StatusConflict
	case errors.Is(err, heartbeat.ErrAuthoringAgentNotFound),
		errors.Is(err, heartbeat.ErrAuthoringNoPolicy),
		errors.Is(err, heartbeat.ErrRevisionNotFound),
		errors.Is(err, heartbeat.ErrSnapshotNotFound),
		errors.Is(err, heartbeat.ErrSessionHealthNotFound),
		errors.Is(err, session.ErrSessionNotFound),
		errors.Is(err, workspacepkg.ErrWorkspaceNotFound),
		errors.Is(err, os.ErrNotExist):
		return http.StatusNotFound
	case errors.Is(err, workspacepkg.ErrWorkspaceRootMissing):
		return http.StatusGone
	case errors.Is(err, heartbeat.ErrInvalid),
		errors.Is(err, heartbeat.ErrInvalidSnapshot),
		errors.Is(err, heartbeat.ErrInvalidRevision),
		errors.Is(err, heartbeat.ErrInvalidSessionHealth),
		errors.Is(err, heartbeat.ErrInvalidWakeEvent),
		errors.Is(err, heartbeat.ErrInvalidWakeState),
		errors.Is(err, heartbeat.ErrAuthoringPathRejected),
		errors.Is(err, apicontract.ErrInvalidAuthoredContextEnum):
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}
