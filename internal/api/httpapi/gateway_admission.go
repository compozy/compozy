package httpapi

import (
	"github.com/compozy/compozy/internal/gateway"
	"github.com/gin-gonic/gin"
)

const gatewayTierHeader = "X-Compozy-Gateway-Tier"

func localListenerTierMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header(gatewayTierHeader, string(SurfaceSetLocal))
		c.Next()
	}
}

func (h *Handlers) gatewayAdmissionMiddleware(tier gateway.Tier, surface gateway.Surface) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header(gatewayTierHeader, string(tier))
		if h == nil || h.gatewayAdmission == nil {
			abortGatewayError(c, gateway.ErrExposureRefused)
			return
		}
		release, err := h.gatewayAdmission.Acquire(tier, surface)
		if err != nil {
			abortGatewayError(c, err)
			return
		}
		if release == nil {
			abortGatewayError(c, gateway.ErrExposureRefused)
			return
		}
		defer release()
		c.Next()
	}
}
