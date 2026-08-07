package httpapi

import (
	"github.com/compozy/compozy/internal/gateway"
	"github.com/gin-gonic/gin"
)

func (h *Handlers) gatewayAdmissionMiddleware(tier gateway.Tier, surface gateway.Surface) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h == nil || h.gatewayAdmission == nil {
			abortGatewayAuth(c, gateway.ErrExposureRefused)
			return
		}
		release, err := h.gatewayAdmission.Acquire(tier, surface)
		if err != nil {
			abortGatewayAuth(c, err)
			return
		}
		if release == nil {
			abortGatewayAuth(c, gateway.ErrExposureRefused)
			return
		}
		defer release()
		c.Next()
	}
}
