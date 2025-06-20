package apikey

import "errors"

// Domain errors
var (
	ErrAPIKeyNotFound    = errors.New("API key not found")
	ErrAPIKeyRevoked     = errors.New("API key has been revoked")
	ErrAPIKeyExpired     = errors.New("API key has expired")
	ErrInvalidAPIKey     = errors.New("invalid API key")
	ErrInvalidKeyFormat  = errors.New("invalid API key format")
	ErrDuplicateKeyName  = errors.New("API key name already exists")
	ErrTooManyKeys       = errors.New("too many API keys for user")
	ErrInvalidTransition = errors.New("invalid status transition")
)
