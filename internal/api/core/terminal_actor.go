package core

import (
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/session"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	"github.com/compozy/compozy/internal/windowmanager"
	"github.com/gin-gonic/gin"
)

const terminalClientAttachmentHeader = "X-Compozy-Client-Token"

func (h *BaseHandlers) terminalActor(
	c *gin.Context,
	workspaceID, profileID, action string,
) (terminalpkg.Actor, bool) {
	credentials := agentCallerCredentialsFromRequest(c)
	if !hasAgentCallerIdentityCredentials(credentials) {
		return terminalpkg.Actor{
			Kind:      terminalpkg.ActorKindHuman,
			ID:        terminalpkg.OperatorActorID,
			ProfileID: profileID,
		}, true
	}
	caller, err := h.resolveAgentCallerForWorkspace(c.Request.Context(), credentials, action, workspaceID)
	if err != nil {
		h.respondTerminalMappedError(c, StatusForAgentIdentityError(err), err)
		return terminalpkg.Actor{}, false
	}
	if h.Sessions == nil {
		h.respondTerminalMappedError(
			c,
			StatusForAgentIdentityError(agentidentity.ErrIdentityLookupUnavailable),
			agentidentity.ErrIdentityLookupUnavailable,
		)
		return terminalpkg.Actor{}, false
	}
	run, err := h.Sessions.ActivePromptRun(c.Request.Context(), caller.Session.ID)
	if err != nil {
		if !errors.Is(err, session.ErrPromptNotActive) {
			h.respondTerminalMappedError(c, 500, err)
			return terminalpkg.Actor{}, false
		}
		h.respondTerminalMappedError(
			c,
			StatusForAgentIdentityError(agentidentity.ErrIdentityStale),
			errors.Join(agentidentity.ErrIdentityStale, err),
		)
		return terminalpkg.Actor{}, false
	}
	if run.SessionID != caller.Session.ID || run.WorkspaceID != workspaceID ||
		run.ProfileID != profileID || strings.TrimSpace(run.RunID) == "" || run.Generation <= 0 {
		h.respondTerminalMappedError(
			c,
			StatusForAgentIdentityError(agentidentity.ErrIdentityStale),
			agentidentity.ErrIdentityStale,
		)
		return terminalpkg.Actor{}, false
	}
	return terminalpkg.Actor{
		Kind: terminalpkg.ActorKindAgent, ID: caller.Session.AgentName, ProfileID: profileID,
		SessionID: caller.Session.ID, RunID: run.RunID, Generation: run.Generation,
	}, true
}

func (h *BaseHandlers) bindTerminalHumanClient(
	c *gin.Context,
	workspaceID, profileID, clientID string,
	actor terminalpkg.Actor,
) (terminalpkg.Actor, error) {
	if actor.Kind != terminalpkg.ActorKindHuman {
		return actor, nil
	}
	clientID = strings.TrimSpace(clientID)
	token := strings.TrimSpace(c.GetHeader(terminalClientAttachmentHeader))
	if clientID == "" && token == "" {
		return actor, nil
	}
	if clientID == "" || token == "" || h.WindowManager == nil {
		return terminalpkg.Actor{}, fmt.Errorf(
			"terminal browser identity is incomplete: %w",
			windowmanager.ErrClientUnauthorized,
		)
	}
	service, err := h.WindowManager.WindowManagerFor(profileID)
	if err != nil {
		return terminalpkg.Actor{}, fmt.Errorf("resolve terminal browser identity: %w", err)
	}
	if err := service.AuthorizeClient(
		c.Request.Context(),
		windowmanager.WorkspaceID(workspaceID),
		windowmanager.ClientID(clientID),
		token,
	); err != nil {
		return terminalpkg.Actor{}, fmt.Errorf("authorize terminal browser identity: %w", err)
	}
	actor.ID = clientID
	return actor, nil
}
