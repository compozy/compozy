package httpapi

import (
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/gin-gonic/gin"
)

func (h *Handlers) redeemGatewayPairing(c *gin.Context) {
	var request contract.GatewayPairingRedeemRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		abortGatewayRequest(c)
		return
	}
	if hasBrowserRequestMetadata(c.Request) && gateway.ActorKind(request.ActorKind) == gateway.ActorKindCLIProfile {
		abortGatewayRequest(c)
		return
	}
	payload, err := h.GatewayRedeem(c.Request.Context(), request)
	if err != nil {
		status := core.StatusForGatewayError(err)
		errorPayload := core.ErrorPayloadForStatus(status, err, true)
		errorPayload.Code = core.GatewayErrorCode(err)
		c.AbortWithStatusJSON(status, errorPayload)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	if gateway.ActorKind(request.ActorKind) == gateway.ActorKindOperatorDevice {
		http.SetCookie(c.Writer, &http.Cookie{
			Name: gatewayDeviceCookieName, Value: payload.Credential, Path: "/",
			Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode,
		})
		payload.Credential = ""
	}
	c.JSON(http.StatusOK, payload)
}

func abortGatewayRequest(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusBadRequest, contract.ErrorPayload{
		Error: "invalid gateway request", Code: core.GatewayInvalidRequestCode,
	})
}
