package core

import (
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/cmdpalette"
)

func cmdPaletteViewStatus(err error) int {
	var notFound *cmdpalette.ViewNotFoundError
	var validation *cmdpalette.ViewValidationError
	var unknownKind *cmdpalette.UnknownViewKindError
	var mismatch *cmdpalette.ViewRevisionMismatchError
	switch {
	case errors.As(err, &notFound):
		return http.StatusNotFound
	case errors.As(err, &validation), errors.As(err, &unknownKind):
		return http.StatusUnprocessableEntity
	case errors.As(err, &mismatch):
		return http.StatusConflict
	case cmdPaletteViewBadRequest(err):
		return http.StatusBadRequest
	default:
		return http.StatusServiceUnavailable
	}
}

func cmdPaletteViewSessionStatus(err error) (int, string) {
	var notFound *cmdpalette.ViewNotFoundError
	var validation *cmdpalette.ViewValidationError
	var unknownKind *cmdpalette.UnknownViewKindError
	var mismatch *cmdpalette.ViewRevisionMismatchError
	switch {
	case errors.Is(err, cmdpalette.ErrClientUnauthorized):
		return http.StatusUnauthorized, "client_unauthorized"
	case errors.Is(err, cmdpalette.ErrViewSessionForbidden):
		return http.StatusForbidden, "session_forbidden"
	case errors.Is(err, cmdpalette.ErrViewSessionGone):
		return http.StatusGone, "session_gone"
	case errors.Is(err, cmdpalette.ErrViewBusy):
		return http.StatusConflict, "view_busy"
	case errors.As(err, &notFound):
		return http.StatusNotFound, "view_not_found"
	case errors.As(err, &validation), errors.As(err, &unknownKind):
		return http.StatusUnprocessableEntity, "invalid_view"
	case errors.As(err, &mismatch):
		return http.StatusConflict, "revision_mismatch"
	case cmdPaletteViewBadRequest(err):
		return http.StatusBadRequest, cmdPaletteInvalidRequestError
	default:
		return http.StatusServiceUnavailable, runtimeUnavailableErrorCode
	}
}

func cmdPaletteViewBadRequest(err error) bool {
	return errors.Is(err, cmdpalette.ErrViewInvalidSequence) ||
		errors.Is(err, cmdpalette.ErrViewStreamEpochRequired) ||
		errors.Is(err, cmdpalette.ErrViewFrameStale) ||
		errors.Is(err, cmdpalette.ErrUnsafeURL) ||
		errors.Is(err, cmdpalette.ErrViewEventInvalid) ||
		errors.Is(err, cmdpalette.ErrViewEventSeqNotIncreasing) ||
		errors.Is(err, cmdpalette.ErrViewEventRevisionStale)
}
