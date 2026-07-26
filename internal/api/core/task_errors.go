package core

import (
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/compozy/agh/internal/admission"
	"github.com/compozy/agh/internal/agentidentity"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

// NewTaskValidationError wraps a task validation failure with the shared sentinel.
func NewTaskValidationError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", taskpkg.ErrValidation, err)
}

// StatusForTaskError maps task-domain, workspace, and session failures to transport statuses.
func StatusForTaskError(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, admission.ErrDraining):
		return http.StatusServiceUnavailable
	case errors.Is(err, taskpkg.ErrValidation),
		errors.Is(err, taskpkg.ErrInvalidScopeBinding),
		errors.Is(err, taskpkg.ErrImmutableField):
		return http.StatusBadRequest
	case errors.Is(err, taskpkg.ErrPayloadTooLarge):
		return http.StatusRequestEntityTooLarge
	case errors.Is(err, taskpkg.ErrPermissionDenied),
		errors.Is(err, taskpkg.ErrForbiddenOperatorAction):
		return http.StatusForbidden
	case errors.Is(err, taskpkg.ErrForceOpRateLimited):
		return http.StatusTooManyRequests
	case errors.Is(err, taskpkg.ErrForceOpRequiresReason),
		errors.Is(err, taskpkg.ErrRetryChainTooDeep),
		errors.Is(err, taskpkg.ErrBulkTooLarge):
		return http.StatusUnprocessableEntity
	case errors.Is(err, errAgentIdentityUnavailable),
		errors.Is(err, agentidentity.ErrIdentityLookupUnavailable),
		errors.Is(err, workspacepkg.ErrWorkspaceResolverUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, agentidentity.ErrIdentityUnauthorized):
		return http.StatusForbidden
	case errors.Is(err, agentidentity.ErrIdentityRequired),
		errors.Is(err, agentidentity.ErrIdentityMismatch),
		errors.Is(err, agentidentity.ErrIdentityStale):
		return http.StatusUnauthorized
	case errors.Is(err, taskpkg.ErrTaskNotFound),
		errors.Is(err, taskpkg.ErrTaskRunNotFound),
		errors.Is(err, taskpkg.ErrTaskDependencyNotFound),
		errors.Is(err, taskpkg.ErrTaskBlockNotFound),
		errors.Is(err, taskpkg.ErrTaskEventNotFound),
		errors.Is(err, taskpkg.ErrTaskRunIdempotencyNotFound),
		errors.Is(err, taskpkg.ErrExecutionProfileNotFound),
		errors.Is(err, taskpkg.ErrRunReviewNotFound),
		errors.Is(err, workspacepkg.ErrWorkspaceNotFound),
		errors.Is(err, session.ErrSessionNotFound),
		errors.Is(err, store.ErrSessionNotFound),
		errors.Is(err, os.ErrNotExist):
		return http.StatusNotFound
	case errors.Is(err, workspacepkg.ErrWorkspaceRootMissing):
		return http.StatusGone
	case errors.Is(err, taskpkg.ErrInvalidStatusTransition),
		errors.Is(err, taskpkg.ErrConflict),
		errors.Is(err, taskpkg.ErrHallucinatedTaskRefs),
		errors.Is(err, taskpkg.ErrGraphLimitExceeded),
		errors.Is(err, taskpkg.ErrCycleDetected),
		errors.Is(err, taskpkg.ErrSessionAlreadyBound),
		errors.Is(err, taskpkg.ErrSessionAttachNotAllowed),
		errors.Is(err, store.ErrSessionAttachLocked),
		errors.Is(err, store.ErrSessionNotAttachable),
		errors.Is(err, taskpkg.ErrNoClaimableRun),
		errors.Is(err, taskpkg.ErrInvalidClaimToken),
		errors.Is(err, taskpkg.ErrLeaseExpired),
		errors.Is(err, taskpkg.ErrActiveRunLease),
		errors.Is(err, taskpkg.ErrWorkspaceActiveRunCapReached):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
