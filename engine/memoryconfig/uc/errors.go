package uc

import "errors"

var (
	ErrInvalidInput   = errors.New("invalid input")
	ErrProjectMissing = errors.New("project missing")
	ErrIDMissing      = errors.New("id missing")
	ErrNotFound       = errors.New("memory not found")
	ErrETagMismatch   = errors.New("etag mismatch")
	ErrStaleIfMatch   = errors.New("stale if-match")
	ErrReferenced     = errors.New("memory referenced")
)
