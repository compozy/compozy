package core

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/ingress"
	"github.com/gin-gonic/gin"
)

const ingressAction = "ingress"

func ingressForbidden(err error) error {
	return errors.Join(ingress.ErrForbidden, err)
}

func (h *BaseHandlers) BindIngress(c *gin.Context) {
	var request contract.IngressBindRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.respondIngressRequestError(c)
		return
	}
	ref := ingress.SubjectRef{
		Kind: ingress.SubjectKind(request.SubjectKind), ID: strings.TrimSpace(request.SubjectID),
	}
	if err := ref.Validate(); err != nil || !request.Confirmed {
		h.respondIngressRequestError(c)
		return
	}
	caller, err := h.ingressCaller(c)
	if err != nil {
		h.respondIngressError(c, ingressForbidden(err))
		return
	}
	if h.Ingress == nil {
		h.respondIngressError(c, ingress.ErrSubjectNotFound)
		return
	}
	binding, err := h.Ingress.Bind(c.Request.Context(), ingress.BindRequest{
		Subject: ref, Confirmed: request.Confirmed, Caller: caller,
	})
	if err != nil {
		if errors.Is(err, ingress.ErrUnreachable) || errors.Is(err, ingress.ErrEndpointUnverified) {
			c.JSON(http.StatusServiceUnavailable, contract.ErrorPayload{
				Error: "Webhook ingress is unavailable.", Code: ingress.ErrUnavailable.Error(), Retryable: true,
			})
			return
		}
		h.respondIngressError(c, err)
		return
	}
	projection, err := h.Ingress.Project(c.Request.Context(), ref)
	if err != nil {
		h.respondIngressError(c, err)
		return
	}
	status := http.StatusOK
	if binding.Changed {
		status = http.StatusCreated
	}
	c.JSON(status, contract.IngressBindingResponse{
		Binding: IngressPayload(projection), Changed: binding.Changed,
	})
}

func (h *BaseHandlers) UnbindIngress(c *gin.Context) {
	ref := ingress.SubjectRef{
		Kind: ingress.SubjectKind(c.Param("subject_kind")),
		ID:   strings.TrimSpace(c.Param("subject_id")),
	}
	if err := ref.Validate(); err != nil {
		h.respondIngressRequestError(c)
		return
	}
	caller, err := h.ingressCaller(c)
	if err != nil {
		h.respondIngressError(c, ingressForbidden(err))
		return
	}
	if h.Ingress == nil {
		h.respondIngressError(c, ingress.ErrSubjectNotFound)
		return
	}
	_, err = h.Ingress.Unbind(c.Request.Context(), ingress.UnbindRequest{
		Subject: ref, Caller: caller,
	})
	if err != nil {
		h.respondIngressError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *BaseHandlers) ingressCaller(c *gin.Context) (*ingress.Caller, error) {
	credentials := agentCallerCredentialsFromRequest(c)
	if !hasAgentCallerIdentityCredentials(credentials) {
		return nil, nil
	}
	caller, err := h.resolveAgentCaller(c.Request.Context(), credentials, ingressAction)
	if err != nil {
		return nil, err
	}
	return &ingress.Caller{
		SessionID: caller.Session.ID, WorkspaceID: caller.Session.WorkspaceID,
		AgentName: caller.Session.AgentName,
	}, nil
}

func (h *BaseHandlers) ingressReadContext(c *gin.Context, action string) (context.Context, error) {
	return h.ingressReadContextForWorkspace(c, action, "")
}

func (h *BaseHandlers) ingressReadContextForWorkspace(
	c *gin.Context,
	action string,
	workspaceRef string,
) (context.Context, error) {
	ctx := c.Request.Context()
	credentials := agentCallerCredentialsFromRequest(c)
	if !hasAgentCallerIdentityCredentials(credentials) {
		return ctx, nil
	}
	caller, err := h.resolveAgentCallerForWorkspace(ctx, credentials, action, workspaceRef)
	if err != nil {
		return nil, err
	}
	return ingress.WithReadWorkspace(ctx, caller.Session.WorkspaceID), nil
}

func (h *BaseHandlers) requireIngressReadContext(
	c *gin.Context,
	action string,
) (context.Context, bool) {
	return h.requireIngressReadContextForWorkspace(c, action, "")
}

func (h *BaseHandlers) requireIngressReadContextForWorkspace(
	c *gin.Context,
	action string,
	workspaceRef string,
) (context.Context, bool) {
	ctx, err := h.ingressReadContextForWorkspace(c, action, workspaceRef)
	if err != nil {
		h.respondIngressError(c, ingressForbidden(err))
		return nil, false
	}
	return ctx, true
}

func IngressPayload(projection ingress.Projection) contract.IngressPayload {
	payload := contract.IngressPayload{
		SubjectKind:                 string(projection.Subject.Kind),
		SubjectID:                   diagnostics.RedactAndBound(projection.Subject.ID, maxDiagnosticPayloadBytes),
		ScopeKind:                   string(projection.Scope),
		WorkspaceID:                 diagnostics.RedactAndBound(projection.WorkspaceID, maxDiagnosticPayloadBytes),
		URL:                         diagnostics.RedactAndBound(projection.URL, maxDiagnosticPayloadBytes),
		Reachability:                string(projection.Reachability),
		EndpointGeneration:          projection.EndpointGeneration,
		ConfirmedEndpointGeneration: projection.ConfirmedEndpointGeneration,
		EnablePath:                  diagnostics.RedactAndBound(projection.EnablePath, maxDiagnosticPayloadBytes),
	}
	if !projection.ConfirmedAt.IsZero() {
		confirmedAt := projection.ConfirmedAt
		payload.ConfirmedAt = &confirmedAt
	}
	return payload
}
