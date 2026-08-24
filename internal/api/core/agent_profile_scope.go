package core

import (
	"context"
	"fmt"
	"strings"

	profilepkg "github.com/compozy/compozy/internal/profile"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) agentResourceProfileName(c *gin.Context) (string, error) {
	_, name, err := h.agentResourceProfile(c)
	return name, err
}

func (h *BaseHandlers) agentResourceProfile(
	c *gin.Context,
) (profilepkg.ReadScope, string, error) {
	scope, err := h.resolveProfileReadScope(c)
	if err != nil {
		return profilepkg.ReadScope{}, "", err
	}
	name, err := h.agentResourceProfileNameForScope(c.Request.Context(), scope)
	return scope, name, err
}

func (h *BaseHandlers) agentResourceProfileNameForScope(
	ctx context.Context,
	scope profilepkg.ReadScope,
) (string, error) {
	if scope.AllProfiles {
		return "", &profilepkg.Error{
			Code:    profileSelectionConflictCode,
			Message: "agent resources require one profile",
			Action:  "choose a profile instead of all profiles",
			Cause:   profilepkg.ErrInvalidInput,
		}
	}
	identities, err := h.profileOwnerIdentities(ctx)
	if err != nil {
		return "", fmt.Errorf("api: list profile identities for agent resources: %w", err)
	}
	identity, ok := identities[strings.TrimSpace(scope.ProfileID)]
	if !ok || strings.TrimSpace(identity.Name) == "" {
		return "", fmt.Errorf("api: resolved agent resource profile is unavailable")
	}
	return strings.TrimSpace(identity.Name), nil
}

func resolveWorkspaceAgentProfile(
	ctx context.Context,
	resolver workspacepkg.RuntimeResolver,
	workspaceRef string,
	profileName string,
) (workspacepkg.ResolvedWorkspace, error) {
	if profileResolver, ok := resolver.(workspacepkg.ProfileRuntimeResolver); ok {
		return profileResolver.ResolveForProfile(ctx, workspaceRef, profileName)
	}
	if strings.TrimSpace(profileName) != "" {
		return workspacepkg.ResolvedWorkspace{}, fmt.Errorf(
			"api: profile-aware workspace resolver is unavailable for profile %q",
			strings.TrimSpace(profileName),
		)
	}
	return resolver.Resolve(ctx, workspaceRef)
}
