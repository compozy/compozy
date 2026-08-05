package workspaceaccess

import "errors"

// ErrInvalidRequest identifies a structurally invalid workspace-access request.
var ErrInvalidRequest = errors.New("workspace access: invalid request")

type requestValidationError struct {
	message string
}

func (e *requestValidationError) Error() string {
	return e.message
}

func (e *requestValidationError) Unwrap() error {
	return ErrInvalidRequest
}

func invalidRequest(message string) error {
	return &requestValidationError{message: message}
}
