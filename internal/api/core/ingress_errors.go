package core

import (
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/ingress"
	"github.com/gin-gonic/gin"
)

func ingressError(err error) (int, string) {
	switch {
	case errors.Is(err, ingress.ErrSubjectNotFound):
		return http.StatusNotFound, "ingress_subject_not_found"
	case errors.Is(err, ingress.ErrForbidden):
		return http.StatusForbidden, "ingress_forbidden"
	case errors.Is(err, ingress.ErrTargetUnavailable):
		return http.StatusConflict, "ingress_target_unavailable"
	case errors.Is(err, ingress.ErrRateLimited):
		return http.StatusTooManyRequests, "ingress_rate_limited"
	case errors.Is(err, ingress.ErrEndpointUnverified), errors.Is(err, ingress.ErrUnreachable):
		return http.StatusServiceUnavailable, ingress.ErrUnavailable.Error()
	default:
		return http.StatusInternalServerError, ""
	}
}

func StatusForIngressError(err error) int {
	status, _ := ingressError(err)
	return status
}

func IngressErrorCode(err error) string {
	_, code := ingressError(err)
	return code
}

func (h *BaseHandlers) respondIngressError(c *gin.Context, err error) {
	status, code := ingressError(err)
	payload := ErrorPayloadForStatus(status, err, h.MaskInternalErrors)
	payload.Code = code
	c.AbortWithStatusJSON(status, payload)
}

func (h *BaseHandlers) respondIngressRequestError(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusBadRequest, contract.ErrorPayload{
		Error: "Invalid ingress binding request.", Code: "invalid_ingress_request",
	})
}
