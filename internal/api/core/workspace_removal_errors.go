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

// workspaceRemovalError maps an unregister failure to its transport status. An unusable layout
// holding the removal answers 503 with the window-manager unavailable code.
func workspaceRemovalError(err error) (int, error) {
	if errors.Is(err, windowmanager.ErrUnavailable) {
		return http.StatusServiceUnavailable, &workspaceRemovalHeldError{cause: err}
	}
	return StatusForWorkspaceError(err), err
}

func enrichWorkspaceRemovalError(payload *contract.ErrorPayload, err error) {
	if _, ok := errors.AsType[*workspaceRemovalHeldError](err); ok {
		payload.Code = string(contract.WindowManagerErrorUnavailable)
	}
}
