package core

import (
	"context"
	"strings"

	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/gin-gonic/gin"
)

// resolveTerminalProfileReadSelection resolves a terminal read. Terminals and their retained
// recordings outlive their profile's lifecycle, so a read of one profile resolves it
// historically: an archived profile, or one held by an unfinished lifecycle operation, still
// answers with its terminals. The all-profiles aggregate and an agent session that names
// another profile go through the shared profile read policy, which owns their authorization.
func (h *BaseHandlers) resolveTerminalProfileReadSelection(
	c *gin.Context,
	workspaceID string,
) (resolvedProfileReadScope, error) {
	allProfiles, err := parseBoolQuery(c, "all_profiles")
	if err != nil || allProfiles || h == nil || h.Profiles == nil {
		return h.resolveProfileReadSelectionForWorkspace(c, workspaceID)
	}
	caller, err := h.resolveProfileReadCaller(c, workspaceID)
	if err != nil {
		return resolvedProfileReadScope{}, err
	}
	requested := strings.TrimSpace(c.Query("profile"))
	profile, err := h.historicalTerminalProfile(c.Request.Context(), requested, caller.sessionProfileID)
	if caller.sessionProfileID != "" && (err != nil || strings.TrimSpace(profile.ID) != caller.sessionProfileID) {
		return h.resolveProfileReadSelectionForWorkspace(c, workspaceID)
	}
	if err != nil {
		return resolvedProfileReadScope{}, err
	}
	return resolvedProfileReadScope{
		Scope:       profilepkg.ReadScope{ProfileID: strings.TrimSpace(profile.ID)},
		ProfileName: strings.TrimSpace(profile.Name),
	}, nil
}

// historicalTerminalProfile finds the named profile, or the session's own profile when no name
// is given, without requiring it to be available.
func (h *BaseHandlers) historicalTerminalProfile(
	ctx context.Context,
	requested string,
	sessionProfileID string,
) (profilepkg.Profile, error) {
	if sessionProfileID == "" {
		if requested == "" {
			requested = profileDefaultName
		}
		return h.profileService().GetByName(ctx, requested)
	}
	profiles, err := h.profileService().List(ctx)
	if err != nil {
		return profilepkg.Profile{}, err
	}
	for _, candidate := range profiles {
		if (requested == "" && strings.TrimSpace(candidate.ID) == sessionProfileID) ||
			(requested != "" && candidate.Name == requested) {
			return candidate.Profile, nil
		}
	}
	return profilepkg.Profile{}, profilepkg.ErrNotFound
}
