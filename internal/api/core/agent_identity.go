package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/agh/internal/agentidentity"
	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/gin-gonic/gin"
)

const (
	agentActionMe              = "agent.me"
	agentActionCoordinatorRole = "agent.coordinator.config"
)

var (
	errAgentIdentityUnavailable = errors.New("api: session service is not configured")
	errCoordinatorRoleMissing   = errors.New("api: coordinator role resolver is not configured")
)

// StatusForAgentIdentityError maps agent identity failures to transport statuses.
func StatusForAgentIdentityError(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, errAgentIdentityUnavailable),
		errors.Is(err, agentidentity.ErrIdentityLookupUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, agentidentity.ErrIdentityUnauthorized):
		return http.StatusForbidden
	case errors.Is(err, agentidentity.ErrIdentityRequired),
		errors.Is(err, agentidentity.ErrIdentityMismatch),
		errors.Is(err, agentidentity.ErrIdentityStale):
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}

// AgentMe returns the daemon-validated caller identity for agent-facing UDS operations.
func (h *BaseHandlers) AgentMe(c *gin.Context) {
	caller, ok := h.requireAgentCaller(c, agentActionMe)
	if !ok {
		return
	}
	payload := agentMePayloadFromCaller(caller)
	h.enrichAgentMePayload(c.Request.Context(), caller, &payload)
	c.JSON(http.StatusOK, contract.AgentMeResponse{Me: contract.NormalizeAgentMePayload(payload)})
}

// AgentCoordinatorRole returns the resolved coordinator policy for the caller workspace.
func (h *BaseHandlers) AgentCoordinatorRole(c *gin.Context) {
	caller, ok := h.requireAgentCaller(c, agentActionCoordinatorRole)
	if !ok {
		return
	}
	payload, err := h.agentCoordinatorConfigPayload(c.Request.Context(), caller.Session.WorkspaceID)
	if err != nil {
		h.respondError(c, statusForCoordinatorRoleError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.AgentCoordinatorConfigResponse{Coordinator: payload})
}

func (h *BaseHandlers) requireAgentCaller(
	c *gin.Context,
	action string,
) (agentidentity.Caller, bool) {
	caller, err := h.resolveAgentCaller(
		c.Request.Context(),
		agentCallerCredentialsFromRequest(c),
		action,
	)
	if err != nil {
		h.respondError(c, StatusForAgentIdentityError(err), err)
		return agentidentity.Caller{}, false
	}
	return caller, true
}

func (h *BaseHandlers) resolveAgentCaller(
	ctx context.Context,
	credentials agentidentity.Credentials,
	action string,
) (agentidentity.Caller, error) {
	return h.resolveAgentCallerForWorkspace(ctx, credentials, action, "")
}

func (h *BaseHandlers) resolveAgentCallerForWorkspace(
	ctx context.Context,
	credentials agentidentity.Credentials,
	action string,
	expectedWorkspaceID string,
) (agentidentity.Caller, error) {
	if h == nil || h.Sessions == nil {
		return agentidentity.Caller{}, errAgentIdentityUnavailable
	}
	return agentidentity.Resolve(ctx, agentidentity.ResolveOptions{
		Credentials:         credentials,
		Lookup:              h.agentSessionLookup,
		ExpectedWorkspaceID: strings.TrimSpace(expectedWorkspaceID),
		OriginKind:          taskOriginKindForTransport(h.transportName()),
		OriginRef:           strings.TrimSpace(action),
	})
}

func (h *BaseHandlers) agentSessionLookup(
	ctx context.Context,
	sessionID string,
) (agentidentity.SessionSnapshot, error) {
	info, err := h.Sessions.Status(ctx, sessionID)
	if err != nil {
		return agentidentity.SessionSnapshot{}, err
	}
	return agentidentity.SessionSnapshotFromInfo(info), nil
}

func agentCallerCredentialsFromRequest(c *gin.Context) agentidentity.Credentials {
	if c == nil || c.Request == nil {
		return agentidentity.Credentials{}
	}
	return agentidentity.Credentials{
		SessionID:   c.GetHeader(agentidentity.HeaderSessionID),
		AgentName:   c.GetHeader(agentidentity.HeaderAgent),
		WorkspaceID: c.GetHeader(agentidentity.HeaderWorkspaceID),
	}
}

func hasAgentCallerIdentityCredentials(credentials agentidentity.Credentials) bool {
	return strings.TrimSpace(credentials.SessionID) != "" || strings.TrimSpace(credentials.AgentName) != ""
}

func agentMePayloadFromCaller(caller agentidentity.Caller) contract.AgentMePayload {
	payload := contract.AgentMePayload{
		Self: contract.AgentIdentityPayload{
			SessionID: caller.Session.ID,
			AgentName: caller.Session.AgentName,
			Provider:  caller.Session.Provider,
			Model:     caller.Session.Model,
		},
		Workspace: contract.AgentWorkspacePayload{
			ID:      caller.Session.WorkspaceID,
			RootDir: caller.Session.WorkspacePath,
		},
		Session: contract.AgentSessionPayload{
			ID:                           caller.Session.ID,
			Name:                         caller.Session.Name,
			Type:                         caller.Session.Type,
			State:                        caller.Session.State,
			ResolvedNetworkParticipation: participation.CloneSpec(caller.Session.NetworkSpecSnapshot()),
			Lineage:                      contract.SessionLineagePayloadFromStore(caller.Session.Lineage),
			CreatedAt:                    caller.Session.CreatedAt,
			UpdatedAt:                    caller.Session.UpdatedAt,
		},
	}
	return contract.NormalizeAgentMePayload(payload)
}

func (h *BaseHandlers) agentCoordinatorConfigPayload(
	ctx context.Context,
	workspaceID string,
) (contract.CoordinatorConfigPayload, error) {
	if h == nil || h.CoordinatorRole == nil {
		return contract.CoordinatorConfigPayload{}, errCoordinatorRoleMissing
	}
	trimmedWorkspaceID := strings.TrimSpace(workspaceID)
	cfg, err := h.CoordinatorRole.ResolveCoordinatorRole(ctx, trimmedWorkspaceID)
	if err != nil {
		return contract.CoordinatorConfigPayload{}, fmt.Errorf("resolve coordinator config: %w", err)
	}
	source := contract.CoordinatorConfigSourceGlobal
	if trimmedWorkspaceID != "" {
		source = contract.CoordinatorConfigSourceWorkspace
	}
	return CoordinatorConfigPayloadFromConfig(cfg, source, trimmedWorkspaceID), nil
}

func statusForCoordinatorRoleError(err error) int {
	if errors.Is(err, errCoordinatorRoleMissing) {
		return http.StatusServiceUnavailable
	}
	return http.StatusInternalServerError
}
