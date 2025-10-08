package uc

import "errors"

var (
	ErrInvalidInput   = errors.New("invalid input")
	ErrProjectMissing = errors.New("project missing")
	ErrIDMissing      = errors.New("id missing")
	ErrIDMismatch     = errors.New("id mismatch")
	ErrNotFound       = errors.New("knowledge base not found")
	ErrAlreadyExists  = errors.New("knowledge base already exists")
	ErrETagMismatch   = errors.New("etag mismatch")
	ErrStaleIfMatch   = errors.New("stale if-match")
	ErrValidationFail = errors.New("knowledge validation failed")
	ErrReferenced     = errors.New("knowledge base referenced")
)
