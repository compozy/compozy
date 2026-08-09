package core

import (
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/gin-gonic/gin"
)

const gatewayIngressAction = "gateway.ingress"

func gatewayIngressForbidden(err error) error {
	return errors.Join(gateway.ErrIngressForbidden, err)
}

func (h *BaseHandlers) BindGatewayIngress(c *gin.Context) {
	var request contract.GatewayIngressBindRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.respondGatewayRequestError(c)
		return
	}
	ref := gateway.IngressSubjectRef{
		Kind: gateway.IngressSubjectKind(request.SubjectKind), ID: strings.TrimSpace(request.SubjectID),
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
	binding, err := h.Gateway.BindIngress(c.Request.Context(), gateway.IngressBindRequest{
		Subject: ref, Confirmed: request.Confirmed, Caller: caller,
	})
	if err != nil {
		h.respondGatewayError(c, err)
		return
	}
	projection, err := h.Gateway.ProjectIngress(c.Request.Context(), ref)
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

func (h *BaseHandlers) UnbindGatewayIngress(c *gin.Context) {
	ref := gateway.IngressSubjectRef{
		Kind: gateway.IngressSubjectKind(c.Param("subject_kind")),
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
	result, err := h.Gateway.UnbindIngress(c.Request.Context(), gateway.IngressUnbindRequest{
		Subject: ref, Caller: caller,
	})
	if err != nil {
		h.respondGatewayError(c, err)
		return
	}
	c.JSON(http.StatusOK, GatewayIngressUnbindPayload(result))
}

func (h *BaseHandlers) gatewayIngressCaller(c *gin.Context) (*gateway.IngressCaller, error) {
	credentials := agentCallerCredentialsFromRequest(c)
	if !hasAgentCallerIdentityCredentials(credentials) {
		return nil, nil
	}
	caller, err := h.resolveAgentCaller(c.Request.Context(), credentials, gatewayIngressAction)
	if err != nil {
		return nil, err
	}
	return &gateway.IngressCaller{
		SessionID: caller.Session.ID, WorkspaceID: caller.Session.WorkspaceID,
		AgentName: caller.Session.AgentName,
	}, nil
}
