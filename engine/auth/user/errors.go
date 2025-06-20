package user

import "errors"

// Domain errors
var (
	ErrUserNotFound      = errors.New("user not found")
	ErrDuplicateEmail    = errors.New("email already exists")
	ErrInvalidEmail      = errors.New("invalid email format")
	ErrInvalidRole       = errors.New("invalid role")
	ErrInvalidTransition = errors.New("invalid status transition")
)
