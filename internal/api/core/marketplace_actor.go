package core

import (
	"errors"
	"fmt"
	"strings"

	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) marketplaceReadActorContext(
	c *gin.Context,
	action string,
) (*taskpkg.ActorContext, bool) {
	scope, err := parseMarketplaceReadScope(c.Query("scope"), c.Query("workspace_id"), c.Query("profile"))
	if err != nil {
		h.respondMarketplaceError(c, err)
		return nil, false
	}
	if scope.scope == settingspkg.ScopeUser {
		return nil, true
	}

	action = "marketplace." + strings.TrimSpace(action)
	var actor taskpkg.ActorContext
	if h.TaskActorContextResolver != nil {
		actor, err = h.TaskActorContextResolver(c, action)
	} else if credentials := agentCallerCredentialsFromRequest(c); hasAgentCallerIdentityCredentials(credentials) {
		caller, resolveErr := h.resolveAgentCallerForWorkspace(
			c.Request.Context(),
			credentials,
			action,
			scope.workspaceID,
		)
		err = resolveErr
		actor = caller.Actor
	} else {
		actor, err = taskpkg.DeriveHumanActorContextForWorkspace(
			defaultTaskActorRef,
			scope.workspaceID,
			taskOriginKindForTransport(h.transportName()),
			action,
		)
	}
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return nil, false
	}
	if scope.scope != settingspkg.ScopeUser {
		readScope, resolveErr := h.resolveProfileMutationScope(c)
		if resolveErr != nil {
			h.respondProfileReadScopeError(c, resolveErr)
			return nil, false
		}
		actor.ReadScope = store.ReadScope{ProfileID: readScope.ProfileID}
	}
	if err = validateMarketplaceReadActor(scope, &actor); err != nil {
		h.respondMarketplaceError(c, err)
		return nil, false
	}
	return &actor, true
}

func validateMarketplaceReadActor(
	scope marketplaceReadScope,
	actor *taskpkg.ActorContext,
) error {
	if scope.scope == settingspkg.ScopeUser {
		return nil
	}
	if actor == nil {
		return errors.Join(
			ErrMarketplaceUnavailable,
			errors.New("workspace marketplace actor is not configured"),
		)
	}
	if err := actor.Validate(); err != nil {
		return fmt.Errorf("validate workspace marketplace actor: %w", err)
	}
	if !actor.Authority.Read {
		return errors.Join(taskpkg.ErrPermissionDenied, errors.New("marketplace actor cannot read"))
	}
	if scope.scope == settingspkg.ScopeProfile &&
		(strings.TrimSpace(actor.ReadScope.ProfileID) == "" || actor.ReadScope.AllProfiles) {
		return errors.Join(
			taskpkg.ErrPermissionDenied,
			errors.New("profile marketplace actor does not match the requested profile"),
		)
	}
	if scope.scope == settingspkg.ScopeWorkspace && strings.TrimSpace(actor.Scope.WorkspaceID) != scope.workspaceID {
		return errors.Join(
			taskpkg.ErrPermissionDenied,
			errors.New("workspace marketplace actor does not match the requested workspace"),
		)
	}
	return nil
}
