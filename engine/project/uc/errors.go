package uc

import "errors"

var (
	ErrInvalidInput   = errors.New("invalid input")
	ErrProjectMissing = errors.New("project missing")
	ErrNameMismatch   = errors.New("name mismatch")
	ErrNotFound       = errors.New("project not found")
	ErrETagMismatch   = errors.New("etag mismatch")
	ErrStaleIfMatch   = errors.New("stale if-match")
)
