package core

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	looppkg "github.com/compozy/compozy/internal/loop"
	taskpkg "github.com/compozy/compozy/internal/task"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

// ErrLoopVersionConflict reports a failed expected_version compare-and-swap.
var ErrLoopVersionConflict = errors.New("loop version conflict")

// LoopVersionConflictError carries the current published version for CAS responses.
type LoopVersionConflictError struct {
	CurrentVersion int
}

func (e *LoopVersionConflictError) Error() string {
	if e == nil {
		return ErrLoopVersionConflict.Error()
	}
	return fmt.Sprintf("%s: current_version=%d", ErrLoopVersionConflict, e.CurrentVersion)
}

func (e *LoopVersionConflictError) Unwrap() error {
	return ErrLoopVersionConflict
}

// LoopLintFailedError carries authoring diagnostics for 422 responses.
type LoopLintFailedError struct {
	Errors []contract.LoopLintErrorPayload
}

func (e *LoopLintFailedError) Error() string {
	if e == nil {
		return "loop lint failed"
	}
	return fmt.Sprintf("loop lint failed: %d error(s)", len(e.Errors))
}

// StatusForLoopError maps loop-domain failures to transport statuses.
func StatusForLoopError(err error) int {
	var lintErr *LoopLintFailedError
	var runtimeErr *looppkg.RuntimeValidationError
	var inputValidationErr *looppkg.InputValidationError
	if status := StatusForTaskError(err); status != http.StatusInternalServerError {
		return status
	}
	switch {
	case errors.As(err, &lintErr):
		return http.StatusUnprocessableEntity
	case errors.As(err, &runtimeErr):
		return http.StatusUnprocessableEntity
	case errors.As(err, &inputValidationErr):
		return http.StatusUnprocessableEntity
	case errors.Is(err, ErrLoopVersionConflict),
		errors.Is(err, looppkg.ErrConcurrencyConflict),
		errors.Is(err, looppkg.ErrTransitionConflict),
		errors.Is(err, looppkg.ErrDefinitionExists),
		errors.Is(err, looppkg.ErrRequestAlreadyAnswered),
		errors.Is(err, looppkg.ErrRerunBusy),
		errors.Is(err, looppkg.ErrTimeTravelKeyReuse):
		return http.StatusConflict
	case errors.Is(err, looppkg.ErrDefinitionNotFound),
		errors.Is(err, looppkg.ErrRunNotFound),
		errors.Is(err, looppkg.ErrConfigNotFound),
		errors.Is(err, looppkg.ErrRequestNotFound),
		errors.Is(err, looppkg.ErrForkGenerationUnknown):
		return http.StatusNotFound
	case errors.Is(err, looppkg.ErrDefinitionReadOnly),
		errors.Is(err, taskpkg.ErrPermissionDenied),
		errors.Is(err, looppkg.ErrRespondNotPermitted),
		errors.Is(err, looppkg.ErrRespondSelfDenied),
		errors.Is(err, looppkg.ErrTimeTravelSelfDenied):
		return http.StatusForbidden
	case errors.Is(err, looppkg.ErrRequestExpired),
		errors.Is(err, looppkg.ErrRequestCanceled):
		return http.StatusGone
	case errors.Is(err, looppkg.ErrRequestValidationFailed):
		return http.StatusUnprocessableEntity
	case errors.Is(err, looppkg.ErrAmendNotParked),
		errors.Is(err, looppkg.ErrAmendNoOutput),
		errors.Is(err, looppkg.ErrAmendSchemaMissing),
		errors.Is(err, looppkg.ErrRerunNodeUnsettled),
		errors.Is(err, looppkg.ErrDiffCrossLoop):
		return http.StatusUnprocessableEntity
	case errors.Is(err, looppkg.ErrValidation),
		errors.Is(err, looppkg.ErrCatalogQueryInvalid),
		errors.Is(err, looppkg.ErrCatalogCursorInvalid):
		return http.StatusBadRequest
	case errors.Is(err, workspacepkg.ErrWorkspaceNotFound),
		errors.Is(err, workspacepkg.ErrWorkspaceRootMissing),
		errors.Is(err, workspacepkg.ErrWorkspaceResolverUnavailable):
		return StatusForWorkspaceError(err)
	case errors.Is(err, looppkg.ErrCatalogUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, looppkg.ErrInvalidTransition),
		errors.Is(err, looppkg.ErrAncestryRejected):
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}
