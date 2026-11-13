package core

import (
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/windowmanager"
)

// workspaceRemovalHeldError reports a workspace removal that committed its purges but could not
// sweep a stored desktop layout. The removal stays pending and finishes when it is retried.
type workspaceRemovalHeldError struct {
	cause error
}

func (e *workspaceRemovalHeldError) Error() string {
	return "workspace removal is waiting on a desktop layout that could not be read (" + e.cause.Error() +
		"); the removal finishes when it is retried"
}

func (e *workspaceRemovalHeldError) Unwrap() error { return e.cause }

// workspaceRemovalError maps an unregister failure to its transport status. A removal held only
// by unusable layouts answers 503 with the window-manager unavailable code; a failure that also
// carries another cause keeps its workspace status, because a retry would not finish it.
func workspaceRemovalError(err error) (int, error) {
	if onlyWindowManagerUnavailable(err) {
		return http.StatusServiceUnavailable, &workspaceRemovalHeldError{cause: err}
	}
	return StatusForWorkspaceError(err), err
}

// onlyWindowManagerUnavailable reports whether every leaf of err is windowmanager.ErrUnavailable.
// A joined error (one entry per partition) qualifies only when all its branches do, and an error
// that names a second cause beside ErrUnavailable (a quota failure) does not.
func onlyWindowManagerUnavailable(err error) bool {
	switch {
	case err == nil:
		return false
	case err == windowmanager.ErrUnavailable:
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		branches := joined.Unwrap()
		if len(branches) == 0 {
			return false
		}
		for _, branch := range branches {
			if !onlyWindowManagerUnavailable(branch) {
				return false
			}
		}
		return true
	}
	return onlyWindowManagerUnavailable(errors.Unwrap(err))
}

func enrichWorkspaceRemovalError(payload *contract.ErrorPayload, err error) {
	if _, ok := errors.AsType[*workspaceRemovalHeldError](err); ok {
		payload.Code = string(contract.WindowManagerErrorUnavailable)
	}
}
