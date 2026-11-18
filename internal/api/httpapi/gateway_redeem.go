package httpapi

import (
	"errors"
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
	source := gatewayAuthSource(c.Request)
	localOperator := h.GatewayPairingSource == string(gateway.PairingSourceLocal)
	if h.pairingLimiter != nil && !h.pairingLimiter.Allow(source, localOperator) {
		abortGatewayError(c, gateway.ErrAuthRateLimited)
		return
	}
	payload, err := h.GatewayRedeem(c.Request.Context(), request)
	if err != nil {
		h.completeGatewayRedeemAttempt(source, localOperator, err)
		abortGatewayError(c, err)
		return
	}
	if h.pairingLimiter != nil {
		h.pairingLimiter.Success(source)
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	if actorKind == gateway.ActorKindOperatorDevice {
		http.SetCookie(c.Writer, &http.Cookie{
			Name: gatewayDeviceCookieName, Value: payload.Credential, Path: "/",
			Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode,
		})
		payload.Credential = ""
	}
	c.JSON(http.StatusOK, payload)
}

// completeGatewayRedeemAttempt charges the pairing budget only for a wrong or unknown code.
func (h *Handlers) completeGatewayRedeemAttempt(source string, localOperator bool, err error) {
	if h.pairingLimiter == nil {
		return
	}
	if errors.Is(err, gateway.ErrPairingInvalid) {
		h.pairingLimiter.Failure(source, localOperator)
		return
	}
	h.pairingLimiter.Complete(source)
}

func abortGatewayRequest(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusBadRequest, contract.ErrorPayload{
		Error: "invalid gateway request", Code: core.GatewayInvalidRequestCode,
	})
}
