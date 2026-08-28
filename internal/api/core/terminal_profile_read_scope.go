package core

import (
	"fmt"
	"strings"

	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/store"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) resolveTerminalProfileReadSelection(
	c *gin.Context,
	workspaceID string,
) (resolvedProfileReadScope, error) {
	allProfiles, err := parseBoolQuery(c, "all_profiles")
	if err != nil {
		return resolvedProfileReadScope{}, fmt.Errorf("%w: %v", profilepkg.ErrInvalidInput, err)
	}
	requested := strings.TrimSpace(c.Query("profile"))
	if allProfiles && requested != "" {
		return resolvedProfileReadScope{}, terminalProfileSelectionConflict(
			"profile and all_profiles cannot be used together",
			"choose one profile or set all_profiles=true",
		)
	}
	sessionProfileID := ""
	credentials := agentCallerCredentialsFromRequest(c)
	if hasAgentCallerIdentityCredentials(credentials) {
		caller, resolveErr := h.resolveAgentCallerForWorkspace(
			c.Request.Context(), credentials, "terminal.read", workspaceID,
		)
		if resolveErr != nil {
			return resolvedProfileReadScope{}, resolveErr
		}
		sessionProfileID = strings.TrimSpace(caller.Session.ProfileID)
		if allProfiles {
			return resolvedProfileReadScope{}, terminalProfileSelectionConflict(
				"all_profiles is only available to operators",
				"read the agent profile selected by the authenticated session",
			)
		}
	}
	if allProfiles {
		return resolvedProfileReadScope{Scope: profilepkg.ReadScope{AllProfiles: true}}, nil
	}
	profile, err := h.historicalTerminalProfile(c, requested, sessionProfileID)
	if err != nil {
		return resolvedProfileReadScope{}, err
	}
	return resolvedProfileReadScope{
		Scope: profilepkg.ReadScope{ProfileID: strings.TrimSpace(profile.ID)}, ProfileName: profile.Name,
	}, nil
}

func (h *BaseHandlers) historicalTerminalProfile(
	c *gin.Context,
	requested string,
	sessionProfileID string,
) (profilepkg.Profile, error) {
	if h == nil || h.Profiles == nil {
		if sessionProfileID == "" || sessionProfileID == store.DefaultProfileID {
			return profilepkg.Profile{ID: store.DefaultProfileID, Name: profileDefaultName}, nil
		}
		return profilepkg.Profile{}, fmt.Errorf("profile service is unavailable")
	}
	if sessionProfileID == "" {
		if requested == "" {
			requested = profileDefaultName
		}
		return h.profileService().GetByName(c.Request.Context(), requested)
	}
	profiles, err := h.profileService().List(c.Request.Context())
	if err != nil {
		return profilepkg.Profile{}, err
	}
	for _, candidate := range profiles {
		if strings.TrimSpace(candidate.ID) != sessionProfileID {
			continue
		}
		if requested != "" && requested != candidate.Name {
			return profilepkg.Profile{}, &profilepkg.Error{
				Code:    "profile_session_conflict",
				Message: fmt.Sprintf("session is bound to profile %q", candidate.Name),
				Action:  "drop the profile selector; the session decides",
				Cause:   profilepkg.ErrSessionConflict,
			}
		}
		return candidate.Profile, nil
	}
	return profilepkg.Profile{}, profilepkg.ErrNotFound
}

func terminalProfileSelectionConflict(message string, action string) error {
	return &profilepkg.Error{
		Code: profileSelectionConflictCode, Message: message, Action: action, Cause: profilepkg.ErrInvalidInput,
	}
}
