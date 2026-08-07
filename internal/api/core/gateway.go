package core

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/gin-gonic/gin"
)

var (
	errGatewayInvalidRequest     = errors.New("gateway request is invalid")
	errGatewayServiceUnavailable = errors.New("gateway service is not configured")
)

func (h *BaseHandlers) GetGatewayStatus(c *gin.Context) {
	ctx, err := h.gatewayStatusReadContext(c)
	if err != nil {
		h.respondGatewayError(c, errors.Join(gateway.ErrIngressForbidden, err))
		return
	}
	payload, err := h.GatewayStatus(ctx)
	if err != nil {
		h.respondGatewayError(c, err)
		return
	}
	c.JSON(http.StatusOK, payload)
}

func (h *BaseHandlers) GetGatewayAudit(c *gin.Context) {
	ctx, err := h.gatewayIngressReadContext(c, "gateway.audit")
	if err != nil {
		h.respondGatewayError(c, errors.Join(gateway.ErrIngressForbidden, err))
		return
	}
	if h == nil || h.Gateway == nil {
		h.respondGatewayError(c, errGatewayServiceUnavailable)
		return
	}
	report, err := h.Gateway.Audit(ctx)
	if err != nil {
		h.respondGatewayError(c, err)
		return
	}
	c.JSON(http.StatusOK, GatewayAuditPayload(report))
}

func (h *BaseHandlers) gatewayStatusReadContext(c *gin.Context) (context.Context, error) {
	return h.gatewayIngressReadContext(c, "gateway.status")
}

func (h *BaseHandlers) gatewayIngressReadContext(c *gin.Context, action string) (context.Context, error) {
	ctx := c.Request.Context()
	credentials := agentCallerCredentialsFromRequest(c)
	if !hasAgentCallerIdentityCredentials(credentials) {
		return ctx, nil
	}
	caller, err := h.resolveAgentCaller(ctx, credentials, action)
	if err != nil {
		return nil, err
	}
	return gateway.WithIngressReadWorkspace(ctx, caller.Session.WorkspaceID), nil
}

func (h *BaseHandlers) GatewayStatus(ctx context.Context) (contract.GatewayStatusPayload, error) {
	if h == nil || h.Gateway == nil {
		return contract.GatewayStatusPayload{}, errGatewayServiceUnavailable
	}
	status, err := h.Gateway.Status(ctx)
	if err != nil {
		return contract.GatewayStatusPayload{}, err
	}
	return GatewayStatusPayload(status), nil
}

func (h *BaseHandlers) SetGatewaySurface(c *gin.Context) {
	var request contract.GatewaySurfaceRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.respondGatewayRequestError(c)
		return
	}
	if !validGatewaySurfaceRequest(request) {
		h.respondGatewayRequestError(c)
		return
	}
	status, err := h.Gateway.SetSurfaceExposure(c.Request.Context(), gateway.SurfaceExposureRequest{
		Surface: gateway.Surface(request.Surface), Tier: gateway.Tier(request.Tier),
		Desired: gateway.DesiredState(request.Desired), ExpectedGeneration: request.ExpectedGeneration,
		Consent: request.Consent,
	})
	if err != nil {
		h.respondGatewayError(c, err)
		return
	}
	c.JSON(http.StatusOK, GatewayStatusPayload(status))
}

func (h *BaseHandlers) EnableGatewayProvider(c *gin.Context) {
	var request contract.GatewayProviderEnableRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.respondGatewayRequestError(c)
		return
	}
	if !validGatewayProviderEnableRequest(c.Param("name"), request) {
		h.respondGatewayRequestError(c)
		return
	}
	status, err := h.Gateway.EnableProvider(c.Request.Context(), gateway.ProviderEnableRequest{
		Tier: gateway.Tier(request.Tier), ExpectedGeneration: request.ExpectedGeneration,
		Provider: gateway.ProviderIdentity{
			Name: c.Param("name"), InstallSource: request.InstallSource,
			DigestConfirmed: request.DigestConfirmed,
		},
	})
	if err != nil {
		h.respondGatewayError(c, err)
		return
	}
	c.JSON(http.StatusOK, GatewayStatusPayload(status))
}

func (h *BaseHandlers) DisableGatewayProvider(c *gin.Context) {
	tier := gateway.Tier(c.Query("tier"))
	if tier.Validate() != nil || strings.TrimSpace(c.Param("name")) == "" {
		h.respondGatewayRequestError(c)
		return
	}
	status, err := h.Gateway.DisableProvider(
		c.Request.Context(),
		tier,
		c.Param("name"),
	)
	if err != nil {
		h.respondGatewayError(c, err)
		return
	}
	c.JSON(http.StatusOK, GatewayStatusPayload(status))
}

func (h *BaseHandlers) MintGatewayPairing(c *gin.Context) {
	artifact, err := h.Gateway.MintPairing(c.Request.Context(), gateway.PairingRequest{
		Source: gateway.PairingSource(h.GatewayPairingSource),
	})
	if err != nil {
		h.respondGatewayError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	c.JSON(http.StatusCreated, GatewayPairingArtifactPayload(artifact))
}

func (h *BaseHandlers) GatewayRedeem(
	ctx context.Context,
	request contract.GatewayPairingRedeemRequest,
) (contract.GatewayIssuedCredentialPayload, error) {
	if h == nil || h.Gateway == nil {
		return contract.GatewayIssuedCredentialPayload{}, errGatewayServiceUnavailable
	}
	if !validGatewayPairingRedeemRequest(request) {
		return contract.GatewayIssuedCredentialPayload{}, errGatewayInvalidRequest
	}
	issued, err := h.Gateway.RedeemPairing(ctx, gateway.RedeemRequest{
		Artifact: request.Artifact, Name: request.Name, Kind: gateway.ActorKind(request.ActorKind),
		Source: gateway.PairingSource(h.GatewayPairingSource), Credential: request.Credential,
	})
	if err != nil {
		return contract.GatewayIssuedCredentialPayload{}, err
	}
	return GatewayIssuedCredentialPayload(issued), nil
}

func (h *BaseHandlers) RedeemGatewayPairing(c *gin.Context) {
	var request contract.GatewayPairingRedeemRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.respondGatewayRequestError(c)
		return
	}
	if !validGatewayPairingRedeemRequest(request) {
		h.respondGatewayRequestError(c)
		return
	}
	payload, err := h.GatewayRedeem(c.Request.Context(), request)
	if err != nil {
		h.respondGatewayError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	c.JSON(http.StatusOK, payload)
}

func (h *BaseHandlers) ListGatewayDevices(c *gin.Context) {
	devices, err := h.Gateway.ListDevices(c.Request.Context())
	if err != nil {
		h.respondGatewayError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.GatewayDevicesResponse{Devices: GatewayDevicePayloads(devices)})
}

func (h *BaseHandlers) RenameGatewayDevice(c *gin.Context) {
	var request contract.GatewayDeviceRenameRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.respondGatewayRequestError(c)
		return
	}
	if strings.TrimSpace(request.Name) == "" {
		h.respondGatewayRequestError(c)
		return
	}
	device, err := h.Gateway.RenameDevice(c.Request.Context(), c.Param("id"), request.Name)
	if err != nil {
		h.respondGatewayError(c, err)
		return
	}
	c.JSON(http.StatusOK, GatewayDevicePayload(device))
}

func (h *BaseHandlers) RevokeGatewayDevice(c *gin.Context) {
	result, err := h.Gateway.RevokeDevice(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.respondGatewayError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.GatewayRevokePayload{
		Device: GatewayDevicePayload(result.Device), Changed: result.Changed, Canceled: result.Canceled,
	})
}

func (h *BaseHandlers) MintGatewayStreamTicket(c *gin.Context) {
	device, ok := gateway.DeviceFromContext(c.Request.Context())
	if !ok {
		h.respondGatewayError(c, gateway.ErrDeviceUnauthenticated)
		return
	}
	ticket, err := h.Gateway.MintStreamTicket(c.Request.Context(), device.ID)
	if err != nil {
		h.respondGatewayError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	c.JSON(http.StatusCreated, GatewayStreamTicketPayload(ticket))
}

func (h *BaseHandlers) respondGatewayError(c *gin.Context, err error) {
	status := StatusForGatewayError(err)
	payload := ErrorPayloadForStatus(status, err, h.MaskInternalErrors)
	payload.Code = GatewayErrorCode(err)
	c.AbortWithStatusJSON(status, payload)
}

func (h *BaseHandlers) respondGatewayRequestError(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusBadRequest, contract.ErrorPayload{
		Error: "invalid gateway request", Code: GatewayInvalidRequestCode,
	})
}

func validGatewaySurfaceRequest(request contract.GatewaySurfaceRequest) bool {
	return gateway.Surface(request.Surface).Validate() == nil &&
		gateway.Tier(request.Tier).Validate() == nil &&
		gateway.DesiredState(request.Desired).Validate() == nil
}

func validGatewayProviderEnableRequest(name string, request contract.GatewayProviderEnableRequest) bool {
	return strings.TrimSpace(name) != "" && strings.TrimSpace(request.InstallSource) != "" &&
		gateway.Tier(request.Tier).Validate() == nil
}

func validGatewayPairingRedeemRequest(request contract.GatewayPairingRedeemRequest) bool {
	kind := gateway.ActorKind(request.ActorKind)
	return strings.TrimSpace(request.Artifact) != "" && strings.TrimSpace(request.Name) != "" &&
		(kind == gateway.ActorKindOperatorDevice || kind == gateway.ActorKindCLIProfile) &&
		(strings.TrimSpace(request.Credential) == "" || gateway.IsDeviceCredential(request.Credential))
}
