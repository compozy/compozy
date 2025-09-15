package cache

import "errors"

// Canonical, backend-neutral errors adapters must return.
var (
	ErrNotFound     = errors.New("cache: not found")
	ErrDuplicate    = errors.New("cache: duplicate")
	ErrNotSupported = errors.New("cache: not supported")
)
