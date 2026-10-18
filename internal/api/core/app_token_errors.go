package core

import (
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/gateway"
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
	case errors.Is(err, gateway.ErrAppTokenNameInvalid):
		return http.StatusBadRequest, "app_token_name_invalid"
	case errors.Is(err, gateway.ErrAppTokenOriginInvalid):
		return http.StatusBadRequest, "app_token_origin_invalid"
	case errors.Is(err, gateway.ErrAppTokenOriginLimit):
		return http.StatusBadRequest, "app_token_origin_limit"
	case errors.Is(err, gateway.ErrAppTokenStateInvalid):
		return http.StatusBadRequest, "app_token_state_invalid"
	case errors.Is(err, gateway.ErrAppTokenLimit):
		return http.StatusConflict, "app_token_limit"
	case errors.Is(err, gateway.ErrAppTokenNotFound):
		return http.StatusNotFound, "app_token_not_found"
	case errors.Is(err, gateway.ErrAppTokenUnauthenticated):
		return http.StatusUnauthorized, "app_token_unauthenticated"
	case errors.Is(err, gateway.ErrDeviceUnauthenticated):
		return http.StatusUnauthorized, "app_token_unauthenticated"
	case errors.Is(err, gateway.ErrAppTokenRequired):
		return http.StatusUnauthorized, "app_token_required"
	case errors.Is(err, gateway.ErrAppTokenRateLimited):
		return http.StatusTooManyRequests, "app_token_rate_limited"
	case errors.Is(err, gateway.ErrAppOriginNotAllowed):
		return http.StatusForbidden, "app_origin_not_allowed"
	case errors.Is(err, gateway.ErrAppTokenRouteForbidden):
		return http.StatusForbidden, "app_token_route_forbidden"
	case errors.Is(err, gateway.ErrAppTokenLocalOnly):
		return http.StatusForbidden, "gateway_app_token_local_only"
	case errors.Is(err, gateway.ErrAppTokenRevoked), errors.Is(err, gateway.ErrDeviceRevoked):
		if authentication {
			return http.StatusUnauthorized, "app_token_revoked"
		}
		return http.StatusConflict, "app_token_revoked"
	default:
		return http.StatusInternalServerError, "internal_error"
	}
}
