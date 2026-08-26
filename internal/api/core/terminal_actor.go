package core

import (
	"github.com/compozy/compozy/internal/agentidentity"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) terminalActor(
	c *gin.Context,
	workspaceID, profileID, action string,
) (terminalpkg.Actor, bool) {
	credentials := agentCallerCredentialsFromRequest(c)
	if !hasAgentCallerIdentityCredentials(credentials) {
		return terminalpkg.Actor{Kind: terminalpkg.ActorKindHuman, ID: terminalpkg.OperatorActorID, ProfileID: profileID}, true
	}
	caller, err := h.resolveAgentCallerForWorkspace(c.Request.Context(), credentials, action, workspaceID)
	if err != nil {
		h.respondError(c, StatusForAgentIdentityError(err), err)
		return terminalpkg.Actor{}, false
	}
	info, err := h.Sessions.Status(c.Request.Context(), caller.Session.ID)
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return terminalpkg.Actor{}, false
	}
	if info == nil {
		h.respondError(c, StatusForAgentIdentityError(agentidentity.ErrIdentityStale), agentidentity.ErrIdentityStale)
		return terminalpkg.Actor{}, false
	}
	return terminalpkg.Actor{
		Kind: terminalpkg.ActorKindAgent, ID: caller.Session.AgentName, ProfileID: profileID,
		SessionID: caller.Session.ID, Generation: info.RuntimeGeneration,
	}, true
}
