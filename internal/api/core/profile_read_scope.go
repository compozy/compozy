package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/agentidentity"
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/store"
	"github.com/gin-gonic/gin"
)

type profileOwnerIdentity struct {
	ID       string
	Name     string
	Color    string
	Icon     string
	Emoji    string
	Archived bool
}

type resolvedProfileReadScope struct {
	Scope       profilepkg.ReadScope
	ProfileName string
}

func (h *BaseHandlers) resolveProfileReadScope(c *gin.Context) (profilepkg.ReadScope, error) {
	resolved, err := h.resolveProfileReadSelection(c)
	return resolved.Scope, err
}

func (h *BaseHandlers) resolveProfileReadSelection(c *gin.Context) (resolvedProfileReadScope, error) {
	return h.resolveProfileReadSelectionForWorkspace(c, "")
}

func (h *BaseHandlers) resolveProfileReadSelectionForWorkspace(
	c *gin.Context,
	workspaceID string,
) (resolvedProfileReadScope, error) {
	allProfiles, err := parseBoolQuery(c, "all_profiles")
	if err != nil {
		return resolvedProfileReadScope{}, fmt.Errorf("%w: %v", profilepkg.ErrInvalidInput, err)
	}
	requested := strings.TrimSpace(c.Query("profile"))
	if allProfiles && requested != "" {
		return resolvedProfileReadScope{}, &profilepkg.Error{
			Code:    profileSelectionConflictCode,
			Message: "profile and all_profiles cannot be used together",
			Action:  "choose one profile or set all_profiles=true",
			Cause:   profilepkg.ErrInvalidInput,
		}
	}
	sessionProfileID := ""
	credentials := agentCallerCredentialsFromRequest(c)
	if hasAgentCallerIdentityCredentials(credentials) {
		var caller agentidentity.Caller
		var resolveErr error
		if strings.TrimSpace(workspaceID) == "" {
			caller, resolveErr = h.resolveAgentCaller(c.Request.Context(), credentials, "profile.read")
		} else {
			caller, resolveErr = h.resolveAgentCallerForWorkspace(
				c.Request.Context(), credentials, "terminal.read", workspaceID,
			)
		}
		if resolveErr != nil {
			return resolvedProfileReadScope{}, resolveErr
		}
		sessionProfileID = strings.TrimSpace(caller.Session.ProfileID)
	}
	if allProfiles {
		if sessionProfileID != "" {
			return resolvedProfileReadScope{}, &profilepkg.Error{
				Code:    profileSelectionConflictCode,
				Message: "all_profiles is only available to operators",
				Action:  "read the agent profile selected by the authenticated session",
				Cause:   profilepkg.ErrInvalidInput,
			}
		}
		return resolvedProfileReadScope{Scope: profilepkg.ReadScope{AllProfiles: true}}, nil
	}
	if requested == "" && sessionProfileID == "" {
		requested = profileDefaultName
	}
	if h == nil || h.Profiles == nil {
		if sessionProfileID != "" && requested == "" {
			return resolvedProfileReadScope{Scope: profilepkg.ReadScope{ProfileID: sessionProfileID}}, nil
		}
		if requested == profileDefaultName {
			if sessionProfileID != "" && sessionProfileID != store.DefaultProfileID {
				return resolvedProfileReadScope{}, fmt.Errorf("profile service is unavailable")
			}
			return resolvedProfileReadScope{
				Scope:       profilepkg.ReadScope{ProfileID: store.DefaultProfileID},
				ProfileName: profileDefaultName,
			}, nil
		}
		return resolvedProfileReadScope{}, fmt.Errorf("profile service is unavailable")
	}
	resolved, err := h.profileService().Resolve(c.Request.Context(), profilepkg.ResolveInput{
		Flag:             requested,
		SessionProfileID: sessionProfileID,
		Lens:             profilepkg.Lens{Kind: profilepkg.SelectionLensGlobal},
	})
	if err != nil {
		return resolvedProfileReadScope{}, err
	}
	scope := profilepkg.ReadScope{ProfileID: strings.TrimSpace(resolved.Profile.ID)}
	if err := scope.Validate(); err != nil {
		return resolvedProfileReadScope{}, fmt.Errorf("api: resolved profile read scope: %w", err)
	}
	return resolvedProfileReadScope{Scope: scope, ProfileName: strings.TrimSpace(resolved.Profile.Name)}, nil
}

func (h *BaseHandlers) resolveProfileMutationScope(c *gin.Context) (profilepkg.ReadScope, error) {
	resolved, err := h.resolveProfileMutationSelection(c)
	return resolved.Scope, err
}

func (h *BaseHandlers) resolveProfileMutationSelection(c *gin.Context) (resolvedProfileReadScope, error) {
	return h.resolveProfileMutationSelectionForWorkspace(c, "")
}

func (h *BaseHandlers) resolveProfileMutationSelectionForWorkspace(
	c *gin.Context,
	workspaceID string,
) (resolvedProfileReadScope, error) {
	resolved, err := h.resolveProfileReadSelectionForWorkspace(c, workspaceID)
	if err != nil {
		return resolvedProfileReadScope{}, err
	}
	if resolved.Scope.AllProfiles {
		return resolvedProfileReadScope{}, &profilepkg.Error{
			Code:    profileSelectionConflictCode,
			Message: "all_profiles is only available for reads",
			Action:  "choose the profile that owns the mutation",
			Cause:   profilepkg.ErrInvalidInput,
		}
	}
	return resolved, nil
}

func (h *BaseHandlers) respondProfileReadScopeError(c *gin.Context, err error) {
	if isProfileDomainError(err) {
		respondProfileError(c, err)
		return
	}
	h.respondError(c, StatusForAgentIdentityError(err), err)
}

func (h *BaseHandlers) profileOwnerIdentities(ctx context.Context) (map[string]profileOwnerIdentity, error) {
	if h == nil || h.Profiles == nil {
		return map[string]profileOwnerIdentity{
			store.DefaultProfileID: {ID: store.DefaultProfileID, Name: profileDefaultName},
			"":                     {ID: store.DefaultProfileID, Name: profileDefaultName},
		}, nil
	}
	profiles, err := h.profileService().List(ctx)
	if err != nil {
		return nil, err
	}
	result := make(map[string]profileOwnerIdentity, len(profiles))
	for _, item := range profiles {
		result[item.ID] = profileOwnerIdentity{
			ID: strings.TrimSpace(item.ID), Name: strings.TrimSpace(item.Name),
			Color: strings.TrimSpace(item.Color), Icon: strings.TrimSpace(item.Icon),
			Emoji: strings.TrimSpace(item.Emoji), Archived: item.State == profilepkg.StateArchived,
		}
	}
	return result, nil
}
