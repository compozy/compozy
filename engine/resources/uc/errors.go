package uc

import "errors"

var (
	ErrNotFound              = errors.New("resource not found")
	ErrInvalidPayload        = errors.New("invalid resource payload")
	ErrMissingID             = errors.New("missing id in body")
	ErrIDMismatch            = errors.New("id mismatch between path and body")
	ErrTypeMismatch          = errors.New("type mismatch between path and body")
	ErrProjectInBody         = errors.New("project field is not allowed in body")
	ErrInvalidID             = errors.New("invalid id")
	ErrETagMismatch          = errors.New("etag mismatch")
	ErrIfMatchStaleOrMissing = errors.New("stale or missing resource for If-Match")
)
