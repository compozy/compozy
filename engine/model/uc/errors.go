package uc

import "errors"

var (
	ErrInvalidInput   = errors.New("invalid input")
	ErrProjectMissing = errors.New("project missing")
	ErrIDMissing      = errors.New("id missing")
	ErrIDMismatch     = errors.New("id mismatch")
	ErrNotFound       = errors.New("model not found")
	ErrETagMismatch   = errors.New("etag mismatch")
	ErrStaleIfMatch   = errors.New("stale if-match")
	ErrReferenced     = errors.New("model referenced")
)
