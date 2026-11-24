package core

import (
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/apptoken"
	"github.com/compozy/compozy/internal/remotehost"
	"github.com/gin-gonic/gin"
)

func RespondAppTokenError(c *gin.Context, err error, authentication bool) {
	status, code := AppTokenErrorStatus(err, authentication)
	payload := ErrorPayloadForStatus(status, err, true)
	payload.Code = code
	c.AbortWithStatusJSON(status, payload)
}

func AppTokenErrorStatus(err error, authentication bool) (int, string) {
	switch {
	case errors.Is(err, apptoken.ErrNameInvalid):
		return http.StatusBadRequest, "app_token_name_invalid"
	case errors.Is(err, apptoken.ErrOriginInvalid):
		return http.StatusBadRequest, "app_token_origin_invalid"
	case errors.Is(err, apptoken.ErrOriginLimit):
		return http.StatusBadRequest, "app_token_origin_limit"
	case errors.Is(err, apptoken.ErrStateInvalid):
		return http.StatusBadRequest, "app_token_state_invalid"
	case errors.Is(err, apptoken.ErrLimit):
		return http.StatusConflict, "app_token_limit"
	case errors.Is(err, apptoken.ErrNotFound):
		return http.StatusNotFound, "app_token_not_found"
	case errors.Is(err, apptoken.ErrUnauthenticated):
		return http.StatusUnauthorized, "app_token_unauthenticated"
	case errors.Is(err, apptoken.ErrRequired):
		return http.StatusUnauthorized, "app_token_required"
	case errors.Is(err, apptoken.ErrRateLimited):
		return http.StatusTooManyRequests, "app_token_rate_limited"
	case errors.Is(err, apptoken.ErrOriginNotAllowed):
		return http.StatusForbidden, "app_origin_not_allowed"
	case errors.Is(err, apptoken.ErrRouteForbidden):
		return http.StatusForbidden, "app_token_route_forbidden"
	case errors.Is(err, apptoken.ErrLocalOnly):
		return http.StatusForbidden, "app_token_local_only"
	case errors.Is(err, apptoken.ErrRevoked), errors.Is(err, remotehost.ErrDeviceRevoked):
		if authentication {
			return http.StatusUnauthorized, "app_token_revoked"
		}
		return http.StatusConflict, "app_token_revoked"
	default:
		return http.StatusInternalServerError, "internal_error"
	}
}
