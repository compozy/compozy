package auth

import "errors"

var (
	// ErrPermissionDenied is returned when a user lacks required permissions
	ErrPermissionDenied = errors.New("permission denied")
	// ErrUnauthorized is returned when authentication fails
	ErrUnauthorized = errors.New("unauthorized")
	// ErrInvalidAPIKey is returned when an API key is invalid
	ErrInvalidAPIKey = errors.New("invalid API key")
	// ErrAPIKeyRevoked is returned when an API key has been revoked
	ErrAPIKeyRevoked = errors.New("API key revoked")
	// ErrOrganizationSuspended is returned when an organization is suspended
	ErrOrganizationSuspended = errors.New("organization suspended")
)
