package core

import (
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/ingress"
	"github.com/gin-gonic/gin"
)

const gatewayIngressAction = "gateway.ingress"

func gatewayIngressForbidden(err error) error {
	return errors.Join(ingress.ErrForbidden, err)
}

func (h *BaseHandlers) BindGatewayIngress(c *gin.Context) { h.bindIngress(c, false) }

func (h *BaseHandlers) BindIngress(c *gin.Context) { h.bindIngress(c, true) }

func (h *BaseHandlers) bindIngress(c *gin.Context, remote bool) {
	var request contract.GatewayIngressBindRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.respondGatewayRequestError(c)
		return
	}
	ref := ingress.SubjectRef{
		Kind: ingress.SubjectKind(request.SubjectKind), ID: strings.TrimSpace(request.SubjectID),
	}
	if err := ref.Validate(); err != nil || !request.Confirmed {
		h.respondGatewayRequestError(c)
		return
	}
	caller, err := h.gatewayIngressCaller(c)
	if err != nil {
		h.respondGatewayError(c, gatewayIngressForbidden(err))
		return
	}
	if h.Ingress == nil {
		h.respondGatewayError(c, ingress.ErrSubjectNotFound)
		return
	}
	binding, err := h.Ingress.Bind(c.Request.Context(), ingress.BindRequest{
		Subject: ref, Confirmed: request.Confirmed, Caller: caller,
	})
	if err != nil {
		if remote && errors.Is(err, ingress.ErrUnreachable) {
			c.JSON(http.StatusServiceUnavailable, contract.ErrorPayload{
				Error: "Webhook ingress is unavailable.", Code: ingress.ErrUnavailable.Error(), Retryable: true,
			})
			return
		}
		h.respondGatewayError(c, err)
		return
	}
	projection, err := h.Ingress.Project(c.Request.Context(), ref)
	if err != nil {
		h.respondGatewayError(c, err)
		return
	}
	status := http.StatusOK
	if binding.Changed {
		status = http.StatusCreated
	}
	c.JSON(status, contract.GatewayIngressBindingResponse{
		Binding: GatewayIngressPayload(projection), Changed: binding.Changed,
	})
}

func (h *BaseHandlers) UnbindGatewayIngress(c *gin.Context) { h.unbindIngress(c, false) }

func (h *BaseHandlers) UnbindIngress(c *gin.Context) { h.unbindIngress(c, true) }

func (h *BaseHandlers) unbindIngress(c *gin.Context, noContent bool) {
	ref := ingress.SubjectRef{
		Kind: ingress.SubjectKind(c.Param("subject_kind")),
		ID:   strings.TrimSpace(c.Param("subject_id")),
	}
	if err := ref.Validate(); err != nil {
		h.respondGatewayRequestError(c)
		return
	}
	caller, err := h.gatewayIngressCaller(c)
	if err != nil {
		h.respondGatewayError(c, gatewayIngressForbidden(err))
		return
	}
	if h.Ingress == nil {
		h.respondGatewayError(c, ingress.ErrSubjectNotFound)
		return
	}
	result, err := h.Ingress.Unbind(c.Request.Context(), ingress.UnbindRequest{
		Subject: ref, Caller: caller,
	})
	if err != nil {
		h.respondGatewayError(c, err)
		return
	}
	if noContent {
		c.Status(http.StatusNoContent)
		return
	}
	c.JSON(http.StatusOK, GatewayIngressUnbindPayload(result))
}

func (h *BaseHandlers) gatewayIngressCaller(c *gin.Context) (*ingress.Caller, error) {
	credentials := agentCallerCredentialsFromRequest(c)
	if !hasAgentCallerIdentityCredentials(credentials) {
		return nil, nil
	}
	caller, err := h.resolveAgentCaller(c.Request.Context(), credentials, gatewayIngressAction)
	if err != nil {
		return nil, err
	}
	return &ingress.Caller{
		SessionID: caller.Session.ID, WorkspaceID: caller.Session.WorkspaceID,
		AgentName: caller.Session.AgentName,
	}, nil
}
