package core

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/gin-gonic/gin"
)

func respondProfileBindingError(c *gin.Context, err error) {
	respondProfileError(c, fmt.Errorf("%w: %v", profilepkg.ErrInvalidInput, err))
}

func respondProfileError(c *gin.Context, err error) {
	status, payload := profileErrorResponse(err)
	c.AbortWithStatusJSON(status, payload)
}

func profileErrorResponse(err error) (int, contract.ProfileErrorPayload) {
	var typed *profilepkg.Error
	if errors.As(err, &typed) {
		return profileErrorStatus(typed), contract.ProfileErrorPayload{Error: contract.ProfileError{
			Code: typed.Code, Message: typed.Message, Action: typed.Action,
		}}
	}
	code, message, action, status := "profile_internal_error", "profile operation failed", "retry the operation", http.StatusInternalServerError
	switch {
	case errors.Is(err, profilepkg.ErrNotFound):
		code, message, action, status = "profile_not_found", "profile was not found", "run compozy profile list", http.StatusNotFound
	case errors.Is(err, profilepkg.ErrInvalidInput):
		code, message, action, status = "profile_input_invalid", err.Error(), "check the request and retry", http.StatusBadRequest
	}
	return status, contract.ProfileErrorPayload{Error: contract.ProfileError{Code: code, Message: message, Action: action}}
}

func profileErrorStatus(err *profilepkg.Error) int {
	switch err.Code {
	case "profile_not_found":
		return http.StatusNotFound
	case "profile_name_invalid", "profile_name_reserved", "profile_identity_invalid", "profile_input_invalid", "profile_selection_conflict":
		return http.StatusBadRequest
	case "profile_remote_management_forbidden":
		return http.StatusForbidden
	default:
		return http.StatusConflict
	}
}

// ProfileRemoteWriteForbidden rejects profile-state writes on paired listeners.
func (h *BaseHandlers) ProfileRemoteWriteForbidden(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusForbidden, contract.ProfileErrorPayload{Error: contract.ProfileError{
		Code:    "profile_remote_management_forbidden",
		Message: "profile management is available only on the local Compozy surface",
		Action:  "manage profiles from the local CLI or desktop app",
	}})
}
