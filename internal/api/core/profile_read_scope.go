package core

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/agentidentity"
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/workspaceaccess"
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
	return h.resolveProfileReadSelectionWithSelector(c, workspaceID, c.Query("profile"))
}

func (h *BaseHandlers) resolveProfileReadSelectionWithSelector(
	c *gin.Context,
	workspaceID, requested string,
) (resolvedProfileReadScope, error) {
	allProfiles, err := parseBoolQuery(c, "all_profiles")
	if err != nil {
		return resolvedProfileReadScope{}, fmt.Errorf("%w: %v", profilepkg.ErrInvalidInput, err)
	}
	requested = strings.TrimSpace(requested)
	if allProfiles && requested != "" {
		return resolvedProfileReadScope{}, &profilepkg.Error{
			Code:    profileSelectionConflictCode,
			Message: "profile and all_profiles cannot be used together",
			Action:  "choose one profile or set all_profiles=true",
			Cause:   profilepkg.ErrInvalidInput,
		}
	}
	caller, err := h.resolveProfileReadCaller(c, workspaceID)
	if err != nil {
		return resolvedProfileReadScope{}, err
	}
	sessionProfileID := caller.sessionProfileID
	if allProfiles {
		return h.allProfilesReadScope(c, caller, workspaceID)
	}
	if requested == "" && sessionProfileID == "" {
		requested = profileDefaultName
	}
	if h == nil || h.Profiles == nil {
		return profileReadScopeWithoutService(sessionProfileID, requested)
	}
	// An agent session that names another profile is decided by the workspace-access policy
	// (grant, permission mode, or consent). A denied caller gets the session-bound resolution's
	// answer, exactly as before the policy could admit it.
	boundProfileID := sessionProfileID
	if sessionProfileID != "" && requested != "" {
		boundProfileID = ""
	}
	resolved, err := h.resolveRequestedProfile(c, caller, requested, boundProfileID)
	if err != nil {
		return resolvedProfileReadScope{}, err
	}
	scope := profilepkg.ReadScope{ProfileID: strings.TrimSpace(resolved.Profile.ID)}
	if err := scope.Validate(); err != nil {
		return resolvedProfileReadScope{}, fmt.Errorf("api: resolved profile read scope: %w", err)
	}
	if sessionProfileID != "" && scope.ProfileID != sessionProfileID {
		if err := h.authorizeGrantedProfile(c, caller.caller, workspaceID, scope.ProfileID, false); err != nil {
			if !caller.granted {
				return h.sessionBoundProfileReadScope(c, requested, sessionProfileID, err)
			}
			return resolvedProfileReadScope{}, err
		}
	}
	return resolvedProfileReadScope{Scope: scope, ProfileName: strings.TrimSpace(resolved.Profile.Name)}, nil
}

// resolveRequestedProfile resolves the selection. For an agent session without a grant, any
// failure (missing, archived, or unavailable) answers like an existing foreign profile, so the
// status does not reveal which profiles exist or what state they are in. A name that is the
// session's own profile still gets its real error from the session-bound resolution.
func (h *BaseHandlers) resolveRequestedProfile(
	c *gin.Context,
	caller profileReadCaller,
	requested, boundProfileID string,
) (profilepkg.Resolution, error) {
	resolved, err := h.profileService().Resolve(c.Request.Context(), profilepkg.ResolveInput{
		Flag:             requested,
		SessionProfileID: boundProfileID,
		Lens:             profilepkg.Lens{Kind: profilepkg.SelectionLensGlobal},
	})
	if err != nil && boundProfileID == "" && caller.sessionProfileID != "" && !caller.granted {
		_, err = h.sessionBoundProfileReadScope(c, requested, caller.sessionProfileID, err)
	}
	return resolved, err
}

// sessionBoundProfileReadScope answers a denied cross-profile selection with the error the
// session-bound resolution reports, keeping the public error code of agents without reach.
func (h *BaseHandlers) sessionBoundProfileReadScope(
	c *gin.Context,
	requested, sessionProfileID string,
	denied error,
) (resolvedProfileReadScope, error) {
	_, err := h.profileService().Resolve(c.Request.Context(), profilepkg.ResolveInput{
		Flag:             requested,
		SessionProfileID: sessionProfileID,
		Lens:             profilepkg.Lens{Kind: profilepkg.SelectionLensGlobal},
	})
	if err != nil {
		return resolvedProfileReadScope{}, err
	}
	return resolvedProfileReadScope{}, denied
}

// profileReadCaller is the authenticated agent session behind a profile read, when there is one.
type profileReadCaller struct {
	caller           agentidentity.Caller
	sessionProfileID string
	granted          bool
}

// resolveProfileReadCaller authenticates an agent caller and binds the request's access origin:
// the operator origin, or the agent session's effective grant.
func (h *BaseHandlers) resolveProfileReadCaller(c *gin.Context, workspaceID string) (profileReadCaller, error) {
	origin := requestOperatorOrigin(c)
	var result profileReadCaller
	credentials := agentCallerCredentialsFromRequest(c)
	if hasAgentCallerIdentityCredentials(credentials) {
		var caller agentidentity.Caller
		var err error
		if strings.TrimSpace(workspaceID) == "" {
			caller, err = h.resolveAgentCaller(c.Request.Context(), credentials, "profile.read")
		} else {
			caller, err = h.resolveAgentCallerForWorkspace(
				c.Request.Context(),
				credentials,
				"terminal.read",
				workspaceID,
			)
		}
		if err != nil {
			return profileReadCaller{}, err
		}
		origin = workspaceaccess.Origin{
			Kind:    workspaceaccess.ActorAgentSession,
			ID:      caller.Session.ID,
			Ceiling: caller.Session.EffectiveWorkspaceAccess,
		}
		result = profileReadCaller{
			caller:           caller,
			sessionProfileID: strings.TrimSpace(caller.Session.ProfileID),
			granted:          caller.Session.EffectiveWorkspaceAccess == workspaceaccess.Unrestricted,
		}
	}
	c.Request = c.Request.WithContext(workspaceaccess.WithOrigin(c.Request.Context(), origin))
	return result, nil
}

// allProfilesReadScope admits the machine-wide read for operators and for agent sessions the
// workspace-access policy admits: the unrestricted grant, approve-all, or approve-reads consent.
func (h *BaseHandlers) allProfilesReadScope(
	c *gin.Context,
	caller profileReadCaller,
	workspaceID string,
) (resolvedProfileReadScope, error) {
	if caller.sessionProfileID != "" {
		if err := h.authorizeGrantedProfile(c, caller.caller, workspaceID, "", true); err != nil {
			if caller.granted {
				return resolvedProfileReadScope{}, err
			}
			return resolvedProfileReadScope{}, &profilepkg.Error{
				Code: profileSelectionConflictCode,
				Message: "all_profiles needs an operator, an agent session with permissions.mode " +
					"approve-all, or workspace_access: unrestricted",
				Action: "read the agent profile selected by the authenticated session",
				Cause:  profilepkg.ErrInvalidInput,
			}
		}
	}
	return resolvedProfileReadScope{Scope: profilepkg.ReadScope{AllProfiles: true}}, nil
}

// profileReadScopeWithoutService resolves only the session's or the default profile when no
// profile service is wired.
func profileReadScopeWithoutService(sessionProfileID, requested string) (resolvedProfileReadScope, error) {
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

// profileOwnerIdentities loads current owner labels through the configured profile service.
func (h *BaseHandlers) profileOwnerIdentities(ctx context.Context) (map[string]profileOwnerIdentity, error) {
	if h == nil {
		return profileOwnerIdentities(ctx, nil)
	}
	return profileOwnerIdentities(ctx, h.Profiles)
}

// profileOwnerIdentities indexes metadata by stable ID, retaining the default owner when no service is configured.
func profileOwnerIdentities(ctx context.Context, profiles ProfileOwnerReader) (map[string]profileOwnerIdentity, error) {
	if profiles == nil {
		return map[string]profileOwnerIdentity{
			store.DefaultProfileID: {ID: store.DefaultProfileID, Name: profileDefaultName},
			"":                     {ID: store.DefaultProfileID, Name: profileDefaultName},
		}, nil
	}
	items, err := profiles.ListIdentities(ctx)
	if err != nil {
		return nil, err
	}
	result := make(map[string]profileOwnerIdentity, len(items))
	for _, item := range items {
		result[item.ID] = profileOwnerIdentity{
			ID: strings.TrimSpace(item.ID), Name: strings.TrimSpace(item.Name),
			Color: strings.TrimSpace(item.Color), Icon: strings.TrimSpace(item.Icon),
			Emoji: strings.TrimSpace(item.Emoji), Archived: item.State == profilepkg.StateArchived,
		}
	}
	return result, nil
}

func (h *BaseHandlers) authorizeGrantedProfile(
	c *gin.Context,
	caller agentidentity.Caller,
	workspaceID, profileID string,
	all bool,
) error {
	if workspaceID == "" {
		workspaceID = caller.Session.WorkspaceID
	}
	target := workspaceaccess.ScopeTarget(workspaceID, profileID)
	target.AllProfiles = all
	return agentidentity.ValidateWorkspaceAccess(c.Request.Context(), h.WorkspaceAccess, workspaceaccess.Request{
		Actor: workspaceaccess.ActorRef{
			Kind:        workspaceaccess.ActorAgentSession,
			SessionID:   caller.Session.ID,
			WorkspaceID: caller.Session.WorkspaceID,
			AgentName:   caller.Session.AgentName,
		},
		Target:   target,
		ReadOnly: c.Request.Method == http.MethodGet,
		Seam:     workspaceaccess.SeamIdentity,
	})
}
