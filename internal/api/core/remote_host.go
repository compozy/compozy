package core

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/account"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/remotehost"
	"github.com/compozy/compozy/internal/remoteproto"
)

type RemoteHostService interface {
	Status() remotehost.Status
	Patch(context.Context, remotehost.Patch) (remotehost.Status, error)
	Link(context.Context, string) (remotehost.LinkResult, error)
	Unlink(context.Context) error
	Devices(context.Context) ([]remotehost.Device, error)
	PatchDevice(context.Context, remoteproto.DeviceID, remotehost.DevicePatch) (remotehost.Device, error)
	RevokeDevice(context.Context, remoteproto.DeviceID) error
}

var _ RemoteHostService = (*remotehost.Host)(nil)

func (h *BaseHandlers) RemoteStatus(c *gin.Context) {
	if h.remoteReady(c) {
		c.JSON(http.StatusOK, contract.RemoteStatusResponse(h.Remote.Status()))
	}
}

func (h *BaseHandlers) PatchRemoteStatus(c *gin.Context) {
	var input contract.RemotePatchRequest
	if !h.remoteReady(c) || !decodeRemoteRequest(c, &input) {
		return
	}
	result, err := h.Remote.Patch(c.Request.Context(), remotehost.Patch(input))
	remoteResult(c, http.StatusOK, contract.RemoteStatusResponse(result), err)
}

func (h *BaseHandlers) LinkRemote(c *gin.Context) {
	var input contract.RemoteLinkRequest
	if !h.remoteReady(c) || !decodeRemoteRequest(c, &input) {
		return
	}
	if h.Account != nil {
		current := h.Account.Status()
		if current.Source == account.SourceToken {
			RespondRemoteError(c, account.ErrTokenLocalOnly)
			return
		}
		if current.Gate != account.GateOff && current.State != account.SignedIn && input.LinkToken == "" {
			RespondRemoteError(c, account.ErrSignInRequired)
			return
		}
	}
	link, err := h.Remote.Link(c.Request.Context(), input.LinkToken)
	result := contract.RemoteLinkResponse{ComputerID: link.ComputerID, Name: link.Name, Enabled: link.Enabled}
	if login := link.Login; login != nil {
		result.AttemptID = login.AttemptID
		result.VerificationURI, result.VerificationURIComplete = login.VerificationURI, login.VerificationURIComplete
		result.UserCode, result.ExpiresAt = login.UserCode, login.ExpiresAt
	}
	remoteResult(c, http.StatusOK, result, err)
}

func (h *BaseHandlers) UnlinkRemote(c *gin.Context) {
	if h.remoteReady(c) {
		remoteResult(c, http.StatusNoContent, nil, h.Remote.Unlink(c.Request.Context()))
	}
}

func (h *BaseHandlers) ListRemoteDevices(c *gin.Context) {
	if !h.remoteReady(c) {
		return
	}
	list, err := h.Remote.Devices(c.Request.Context())
	devices := make([]contract.RemoteDeviceResponse, 0, len(list))
	for _, device := range list {
		devices = append(devices, contract.RemoteDeviceResponse(device))
	}
	remoteResult(c, http.StatusOK, contract.RemoteDevicesResponse{Devices: devices}, err)
}

func (h *BaseHandlers) PatchRemoteDevice(c *gin.Context) {
	var input contract.RemoteDevicePatchRequest
	if !h.remoteReady(c) || !decodeRemoteRequest(c, &input) {
		return
	}
	if _, remote := remotehost.PrincipalFromContext(c.Request.Context()); remote && input.Approved != nil {
		RespondRemoteError(c, remotehost.ErrApprovalLocalOnly)
		return
	}
	result, err := h.Remote.PatchDevice(
		c.Request.Context(),
		remoteproto.DeviceID(c.Param("id")),
		remotehost.DevicePatch(input),
	)
	remoteResult(c, http.StatusOK, contract.RemoteDeviceResponse(result), err)
}

func (h *BaseHandlers) RevokeRemoteDevice(c *gin.Context) {
	if h.remoteReady(c) {
		remoteResult(
			c,
			http.StatusNoContent,
			nil,
			h.Remote.RevokeDevice(c.Request.Context(), remoteproto.DeviceID(c.Param("id"))),
		)
	}
}

func (h *BaseHandlers) remoteReady(c *gin.Context) bool {
	if h.Remote != nil {
		return true
	}
	RespondRemoteError(c, &remotehost.Failure{Status: http.StatusServiceUnavailable, Code: "remote_unavailable",
		Message: "Remote access is unavailable."})
	return false
}

func decodeRemoteRequest(c *gin.Context, input any) bool {
	if err := decodeStrictJSONBody(c, input); err != nil {
		RespondRemoteError(c, &remotehost.Failure{Status: http.StatusBadRequest, Code: "invalid_remote_request",
			Message: "The remote access request is invalid."})
		return false
	}
	return true
}

func remoteResult(c *gin.Context, status int, value any, err error) {
	switch {
	case err != nil:
		RespondRemoteError(c, err)
	case status == http.StatusNoContent:
		c.Status(status)
	default:
		c.JSON(status, value)
	}
}

func RespondRemoteError(c *gin.Context, err error) {
	if failure, ok := errors.AsType[*cloudclient.Failure](err); ok {
		if !failure.RetryAt.IsZero() {
			c.Header("Retry-After", failure.RetryAt.UTC().Format(http.TimeFormat))
		}
		c.AbortWithStatusJSON(failure.Status, contract.ErrorPayload{
			Error: failure.Message, Code: failure.Code, Retryable: failure.Temporary(),
		})
		return
	}
	failure := remotehost.DescribeError(err)
	c.AbortWithStatusJSON(failure.Status, contract.ErrorPayload{
		Error: failure.Message, Code: failure.Code, Details: failure.Details,
		Retryable: failure.Status == http.StatusServiceUnavailable,
	})
}

func remoteRequestError(ctx context.Context, err error) error {
	cause := context.Cause(ctx)
	if errors.Is(cause, remotehost.ErrDeviceRevoked) || errors.Is(cause, remotehost.ErrEntitlement) ||
		errors.Is(cause, remotehost.ErrPlanRequired) || errors.Is(cause, remotehost.ErrSessionExpired) {
		return cause
	}
	return err
}
