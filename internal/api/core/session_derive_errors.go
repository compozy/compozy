package core

import (
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/admission"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/gin-gonic/gin"
)

// Public continue/fork/preview error codes (session-continue-fork _dx.md error matrix).
const (
	deriveCodeInvalidRequest       = "invalid_request"
	deriveCodeSessionNotFound      = "session_not_found"
	deriveCodeAdmissionUnavailable = "new_work_admission_unavailable"
	deriveCodeSessionNotDerivable  = "session_not_derivable"
	deriveCodeSessionArchived      = "session_archived"
	deriveCodeAgentNotFound        = "agent_not_found"
	deriveCodeRouteNotFound        = "route_not_found"
	deriveCodeFenceConflict        = "session_fence_conflict"
	deriveCodeIdempotencyConflict  = "idempotency_conflict"
	deriveCodeTurnInProgress       = "session_turn_in_progress"
	deriveCodeMessageNotFound      = "message_not_found"
)

// deriveBoundaryError is a continue/fork/preview failure as the derive routes report it:
// the boundary-only code (validation, missing source, closed admission map to codes
// only here, never on unrelated endpoints) and, when the child was already committed,
// its id so the caller can still open it.
type deriveBoundaryError struct {
	code           string
	childSessionID string
	err            error
}

func (e *deriveBoundaryError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *deriveBoundaryError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

// invalidDeriveRequest marks a request-shape failure (strict decode, missing path id).
func invalidDeriveRequest(err error) error {
	return &deriveBoundaryError{code: deriveCodeInvalidRequest, err: err}
}

// deriveFailure attaches the derive boundary code and the committed child id, if any.
func deriveFailure(err error, childSessionID string) error {
	if err == nil {
		return nil
	}
	if existing, ok := errors.AsType[*deriveBoundaryError](err); ok && existing != nil {
		if existing.childSessionID == "" {
			existing.childSessionID = strings.TrimSpace(childSessionID)
		}
		return err
	}
	return &deriveBoundaryError{
		code:           deriveBoundaryCode(err),
		childSessionID: strings.TrimSpace(childSessionID),
		err:            err,
	}
}

// DeriveChildSessionID returns the committed child id a derive failure carries.
func DeriveChildSessionID(err error) string {
	boundary, ok := errors.AsType[*deriveBoundaryError](err)
	if !ok || boundary == nil {
		return ""
	}
	return boundary.childSessionID
}

// DeriveFailure wraps a continue/fork failure for non-HTTP transports (native tools):
// the same code and committed child id the HTTP/UDS routes report.
func DeriveFailure(err error, result session.DeriveResult) error {
	return deriveFailure(err, committedDeriveChildID(result))
}

// committedDeriveChildID is the child a failed derive already committed; empty
// when the failure happened before the commit.
func committedDeriveChildID(result session.DeriveResult) string {
	if id := strings.TrimSpace(result.ChildSessionID); id != "" {
		return id
	}
	if result.Child != nil {
		return strings.TrimSpace(result.Child.ID)
	}
	return ""
}

// respondDeriveError writes a continue/fork/preview failure with its derive code and,
// after the child was committed, its id.
func (h *BaseHandlers) respondDeriveError(c *gin.Context, status int, err error) {
	err = deriveFailure(err, "")
	normalized := normalizeErrorStatus(status, err, h.MaskInternalErrors)
	c.JSON(normalized.status, deriveErrorPayload(
		errorPayloadForNormalizedStatus(normalized.status, normalized.err, normalized.maskInternalErrors),
		err,
	))
}

// DeriveErrorPayload is the continue/fork error payload for non-HTTP transports.
func DeriveErrorPayload(err error) contract.SessionDeriveErrorPayload {
	return deriveErrorPayload(ErrorPayloadForError(err), err)
}

// deriveErrorPayload completes a continue/fork error payload: the committed child id and,
// when no derive code applies, the structured diagnostic's code as the top-level code, so a
// provider/model refusal after the commit (for example model_unavailable) reads the same
// on HTTP, UDS, native tools, and the CLI.
func deriveErrorPayload(payload contract.ErrorPayload, err error) contract.SessionDeriveErrorPayload {
	if payload.Code == "" && payload.Diagnostic != nil {
		payload.Code = strings.TrimSpace(payload.Diagnostic.Code)
	}
	return contract.SessionDeriveErrorPayload{ErrorPayload: payload, ChildSessionID: DeriveChildSessionID(err)}
}

// deriveBoundaryCode maps a derive failure to its public code, including the
// generic validation, missing-source, and admission failures the derive routes promise.
func deriveBoundaryCode(err error) string {
	if code := DeriveErrorCode(err); code != "" {
		return code
	}
	switch {
	case errors.Is(err, admission.ErrDraining):
		return deriveCodeAdmissionUnavailable
	case errors.Is(err, session.ErrSessionNotFound), errors.Is(err, store.ErrSessionNotFound),
		errors.Is(err, errWorkspaceScopedResourceNotFound):
		return deriveCodeSessionNotFound
	case errors.Is(err, session.ErrValidation):
		return deriveCodeInvalidRequest
	default:
		return ""
	}
}

// DeriveErrorCode returns the stable public code of a continue/fork failure.
func DeriveErrorCode(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, session.ErrSessionNotDerivable):
		return deriveCodeSessionNotDerivable
	case errors.Is(err, session.ErrDeriveSourceArchived):
		return deriveCodeSessionArchived
	case errors.Is(err, session.ErrDeriveAgentNotFound):
		return deriveCodeAgentNotFound
	case errors.Is(err, session.ErrDeriveRouteNotFound):
		return deriveCodeRouteNotFound
	case errors.Is(err, session.ErrDeriveFenceConflict):
		return deriveCodeFenceConflict
	case errors.Is(err, session.ErrDeriveIdempotencyConflict):
		return deriveCodeIdempotencyConflict
	case errors.Is(err, session.ErrDeriveTurnInProgress):
		return deriveCodeTurnInProgress
	case errors.Is(err, session.ErrDeriveMessageNotFound):
		return deriveCodeMessageNotFound
	case errors.Is(err, errDeriveRuntimeRouteExclusive), errors.Is(err, errDerivePartialFences):
		return deriveCodeInvalidRequest
	}
	if boundary, ok := errors.AsType[*deriveBoundaryError](err); ok && boundary != nil {
		return boundary.code
	}
	return ""
}

func statusForDeriveError(err error) (int, bool) {
	switch DeriveErrorCode(err) {
	case deriveCodeSessionNotDerivable, deriveCodeInvalidRequest:
		return http.StatusBadRequest, true
	case deriveCodeAgentNotFound, deriveCodeMessageNotFound, deriveCodeSessionNotFound:
		return http.StatusNotFound, true
	case deriveCodeSessionArchived, deriveCodeRouteNotFound, deriveCodeFenceConflict,
		deriveCodeIdempotencyConflict, deriveCodeTurnInProgress:
		return http.StatusConflict, true
	case deriveCodeAdmissionUnavailable:
		return http.StatusServiceUnavailable, true
	default:
		return 0, false
	}
}
