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
	actorKind := gateway.ActorKind(request.ActorKind)
	if hasBrowserRequestMetadata(c.Request) && actorKind == gateway.ActorKindCLIProfile {
		abortGatewayRequest(c)
		return
	}
	payload, err := h.GatewayRedeem(c.Request.Context(), request)
	if err != nil {
		abortGatewayError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	if actorKind == gateway.ActorKindOperatorDevice {
		// #nosec G124 -- HTTP mode needs a non-Secure cookie; HTTPS remains Secure.
		http.SetCookie(c.Writer, &http.Cookie{
			Name: gatewayDeviceCookieName, Value: payload.Credential, Path: "/",
			Secure: c.Request.TLS != nil, HttpOnly: true, SameSite: http.SameSiteLaxMode,
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
