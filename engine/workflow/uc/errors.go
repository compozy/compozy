package uc

import "errors"

// Note: When returning these sentinel errors from use-case methods, wrap them
// with additional context using fmt.Errorf("context: %w", ErrInvalidInput)
// to preserve the original cause while improving diagnostics.
var (
	ErrInvalidInput   = errors.New("invalid input")
	ErrNotFound       = errors.New("workflow not found")
	ErrProjectMissing = errors.New("project missing")
	ErrIDMissing      = errors.New("id missing")
	ErrIDMismatch     = errors.New("id mismatch")
	ErrWeakETag       = errors.New("weak etag not allowed")
	ErrETagMismatch   = errors.New("etag mismatch")
	ErrStaleIfMatch   = errors.New("stale if-match")
)
