package core

import (
	"context"
	"fmt"
	"strings"

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

func (h *BaseHandlers) resolveProfileReadScope(c *gin.Context) (profilepkg.ReadScope, error) {
	allProfiles, err := parseBoolQuery(c, "all_profiles")
	if err != nil {
		return profilepkg.ReadScope{}, fmt.Errorf("%w: %v", profilepkg.ErrInvalidInput, err)
	}
	requested := strings.TrimSpace(c.Query("profile"))
	if allProfiles && requested != "" {
		return profilepkg.ReadScope{}, &profilepkg.Error{
			Code:    "profile_selection_conflict",
			Message: "profile and all_profiles cannot be used together",
			Action:  "choose one profile or set all_profiles=true",
			Cause:   profilepkg.ErrInvalidInput,
		}
	}
	sessionProfileID := ""
	credentials := agentCallerCredentialsFromRequest(c)
	if hasAgentCallerIdentityCredentials(credentials) {
		caller, resolveErr := h.resolveAgentCaller(c.Request.Context(), credentials, "profile.read")
		if resolveErr != nil {
			return profilepkg.ReadScope{}, resolveErr
		}
		sessionProfileID = strings.TrimSpace(caller.Session.ProfileID)
	}
	if allProfiles {
		return profilepkg.ReadScope{AllProfiles: true}, nil
	}
	if requested == "" && sessionProfileID == "" {
		requested = "default"
	}
	if h == nil || h.Profiles == nil {
		if sessionProfileID != "" && requested == "" {
			return profilepkg.ReadScope{ProfileID: sessionProfileID}, nil
		}
		if requested == "default" {
			if sessionProfileID != "" && sessionProfileID != store.DefaultProfileID {
				return profilepkg.ReadScope{}, fmt.Errorf("profile service is unavailable")
			}
			return profilepkg.ReadScope{ProfileID: store.DefaultProfileID}, nil
		}
		return profilepkg.ReadScope{}, fmt.Errorf("profile service is unavailable")
	}
	resolved, err := h.profileService().Resolve(c.Request.Context(), profilepkg.ResolveInput{
		Flag:             requested,
		SessionProfileID: sessionProfileID,
		Lens:             profilepkg.Lens{Kind: profilepkg.SelectionLensGlobal},
	})
	if err != nil {
		return profilepkg.ReadScope{}, err
	}
	scope := profilepkg.ReadScope{ProfileID: strings.TrimSpace(resolved.Profile.ID)}
	if err := scope.Validate(); err != nil {
		return profilepkg.ReadScope{}, fmt.Errorf("api: resolved profile read scope: %w", err)
	}
	return scope, nil
}

func (h *BaseHandlers) resolveProfileMutationScope(c *gin.Context) (profilepkg.ReadScope, error) {
	scope, err := h.resolveProfileReadScope(c)
	if err != nil {
		return profilepkg.ReadScope{}, err
	}
	if scope.AllProfiles {
		return profilepkg.ReadScope{}, &profilepkg.Error{
			Code:    "profile_selection_conflict",
			Message: "all_profiles is only available for reads",
			Action:  "choose the profile that owns the mutation",
			Cause:   profilepkg.ErrInvalidInput,
		}
	}
	return scope, nil
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
			store.DefaultProfileID: {ID: store.DefaultProfileID, Name: "default"},
			"":                     {ID: store.DefaultProfileID, Name: "default"},
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
