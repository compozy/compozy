package org

import "errors"

// Domain errors
var (
	ErrInvalidEmail            = errors.New("invalid email format")
	ErrInvalidOrgName          = errors.New("invalid organization name")
	ErrInvalidRole             = errors.New("invalid role")
	ErrInvalidAPIKey           = errors.New("invalid API key")
	ErrExpiredAPIKey           = errors.New("API key has expired")
	ErrInvalidTransition       = errors.New("invalid status transition")
	ErrDuplicateOrgName        = errors.New("organization name already exists")
	ErrInvalidNamespace        = errors.New("invalid namespace format")
	ErrRateLimitExceeded       = errors.New("rate limit exceeded")
	ErrUnauthorized            = errors.New("unauthorized access")
	ErrInsufficientPermissions = errors.New("insufficient permissions")
	ErrOrganizationNotFound    = errors.New("organization not found")
)
