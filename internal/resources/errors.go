package resources

import "errors"

var (
	// ErrNotFound reports that no persisted resource matched the lookup.
	ErrNotFound = errors.New("resources: record not found")
	// ErrValidation reports that a resource payload or actor failed validation.
	ErrValidation = errors.New("resources: validation failed")
	// ErrInvalidScopeBinding reports that a scope and scope identifier are inconsistent.
	ErrInvalidScopeBinding = errors.New("resources: invalid scope binding")
	// ErrPermissionDenied reports that the resolved actor lacks authority for the request.
	ErrPermissionDenied = errors.New("resources: permission denied")
	// ErrDirectMutationNotAllowed reports that the actor cannot use direct CRUD paths.
	ErrDirectMutationNotAllowed = errors.New("resources: direct mutation not allowed")
	// ErrConflict reports optimistic concurrency or ownership conflicts.
	ErrConflict = errors.New("resources: conflict")
	// ErrPayloadTooLarge reports that a record or snapshot exceeded configured limits.
	ErrPayloadTooLarge = errors.New("resources: payload too large")
	// ErrRateLimited reports that the caller exceeded a configured resource rate limit.
	ErrRateLimited = errors.New("resources: rate limited")
	// ErrSessionNotActive reports that the provided session nonce is not the active nonce for the source.
	ErrSessionNotActive = errors.New("resources: session nonce not active")
	// ErrStaleSourceVersion reports that the snapshot source version is stale or out of sequence.
	ErrStaleSourceVersion = errors.New("resources: stale source version")
	// ErrCodecNotFound reports that no typed codec is registered for a resource kind.
	ErrCodecNotFound = errors.New("resources: codec not found")
	// ErrCodecTypeMismatch reports that a registered codec kind was resolved with the wrong spec type.
	ErrCodecTypeMismatch = errors.New("resources: codec type mismatch")
)
