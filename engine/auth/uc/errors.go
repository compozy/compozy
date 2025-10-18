package uc

import "errors"

var (
	// ErrUserNotFound indicates a requested user does not exist.
	ErrUserNotFound = errors.New("auth: user not found")
	// ErrAPIKeyNotFound indicates a requested API key does not exist.
	ErrAPIKeyNotFound = errors.New("auth: api key not found")
	// ErrInvalidCredentials indicates the provided authentication material is invalid.
	ErrInvalidCredentials = errors.New("auth: invalid credentials")
	// ErrTokenExpired indicates the token has expired and must be refreshed.
	ErrTokenExpired = errors.New("auth: token expired")
	// ErrRateLimited indicates the authentication request is rate limited.
	ErrRateLimited = errors.New("auth: rate limited")
)
