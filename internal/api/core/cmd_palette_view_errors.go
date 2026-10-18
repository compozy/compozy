package core

import (
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/cmdpalette"
)

func cmdPaletteViewStatus(err error) int {
	if _, ok := errors.AsType[*cmdpalette.ViewNotFoundError](err); ok {
		return http.StatusNotFound
	}
	if _, ok := errors.AsType[*cmdpalette.ViewValidationError](err); ok {
		return http.StatusUnprocessableEntity
	}
	if _, ok := errors.AsType[*cmdpalette.UnknownViewKindError](err); ok {
		return http.StatusUnprocessableEntity
	}
	if _, ok := errors.AsType[*cmdpalette.ViewRevisionMismatchError](err); ok {
		return http.StatusConflict
	}
	if cmdPaletteViewBadRequest(err) {
		return http.StatusBadRequest
	}
	return http.StatusServiceUnavailable
}

func cmdPaletteViewSessionStatus(err error) (int, string) {
	if errors.Is(err, cmdpalette.ErrClientUnauthorized) {
		return http.StatusUnauthorized, "client_unauthorized"
	}
	if errors.Is(err, cmdpalette.ErrViewSessionForbidden) {
		return http.StatusForbidden, "session_forbidden"
	}
	if errors.Is(err, cmdpalette.ErrViewSessionGone) {
		return http.StatusGone, "session_gone"
	}
	if errors.Is(err, cmdpalette.ErrViewBusy) {
		return http.StatusConflict, "view_busy"
	}
	if _, ok := errors.AsType[*cmdpalette.ViewNotFoundError](err); ok {
		return http.StatusNotFound, "view_not_found"
	}
	if _, ok := errors.AsType[*cmdpalette.ViewValidationError](err); ok {
		return http.StatusUnprocessableEntity, "invalid_view"
	}
	if _, ok := errors.AsType[*cmdpalette.UnknownViewKindError](err); ok {
		return http.StatusUnprocessableEntity, "invalid_view"
	}
	if _, ok := errors.AsType[*cmdpalette.ViewRevisionMismatchError](err); ok {
		return http.StatusConflict, "revision_mismatch"
	}
	if cmdPaletteViewBadRequest(err) {
		return http.StatusBadRequest, cmdPaletteInvalidRequestError
	}
	return http.StatusServiceUnavailable, runtimeUnavailableErrorCode
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
