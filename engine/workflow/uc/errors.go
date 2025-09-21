package uc

import "errors"

var (
	ErrInvalidInput   = errors.New("invalid input")
	ErrNotFound       = errors.New("workflow not found")
	ErrProjectMissing = errors.New("project is required")
	ErrIDMismatch     = errors.New("id mismatch")
	ErrWeakETag       = errors.New("weak etag not allowed")
	ErrETagMismatch   = errors.New("etag mismatch")
	ErrStaleIfMatch   = errors.New("if-match stale or missing")
)
