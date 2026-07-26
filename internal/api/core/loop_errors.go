package core

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/compozy/agh/internal/api/contract"
	looppkg "github.com/compozy/agh/internal/loop"
	taskpkg "github.com/compozy/agh/internal/task"
	workspacepkg "github.com/compozy/agh/internal/workspace"
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
	if status := StatusForTaskError(err); status != http.StatusInternalServerError {
		return status
	}
	switch {
	case errors.As(err, &lintErr):
		return http.StatusUnprocessableEntity
	case errors.Is(err, ErrLoopVersionConflict),
		errors.Is(err, looppkg.ErrConcurrencyConflict),
		errors.Is(err, looppkg.ErrTransitionConflict),
		errors.Is(err, looppkg.ErrDefinitionExists):
		return http.StatusConflict
	case errors.Is(err, looppkg.ErrDefinitionNotFound),
		errors.Is(err, looppkg.ErrRunNotFound),
		errors.Is(err, looppkg.ErrConfigNotFound):
		return http.StatusNotFound
	case errors.Is(err, taskpkg.ErrPermissionDenied):
		return http.StatusForbidden
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
