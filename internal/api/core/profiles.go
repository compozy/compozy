package core

import (
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/gin-gonic/gin"
)

// ListProfiles returns every profile and its ownership summary.
func (h *BaseHandlers) ListProfiles(c *gin.Context) {
	profiles, err := h.profileService().List(c.Request.Context())
	if err != nil {
		respondProfileError(c, err)
		return
	}
	result := make([]contract.Profile, 0, len(profiles))
	for _, item := range profiles {
		result = append(result, profileContract(
			item.Profile, item.WorkItems, item.NeedsSetup, item.CredentialRequirements,
		))
	}
	c.JSON(http.StatusOK, result)
}

// GetProfile returns one profile by its public name.
func (h *BaseHandlers) GetProfile(c *gin.Context) {
	value, err := h.profileService().GetWithCounts(c.Request.Context(), c.Param("name"))
	if err != nil {
		respondProfileError(c, err)
		return
	}
	c.JSON(http.StatusOK, profileContract(
		value.Profile,
		value.WorkItems,
		value.NeedsSetup,
		value.CredentialRequirements,
	))
}

// CreateProfile creates a profile and optionally activates it for a selection lens.
func (h *BaseHandlers) CreateProfile(c *gin.Context) {
	var request contract.CreateProfileRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondProfileBindingError(c, err)
		return
	}
	input := profilepkg.CreateInput{Name: request.Name, Color: request.Color, Icon: request.Icon, Emoji: request.Emoji}
	if request.Activate != nil {
		input.Activate = &profilepkg.Lens{
			Kind: profilepkg.SelectionLens(request.Activate.Scope), WorkspaceID: request.Activate.WorkspaceID,
		}
	}
	created, err := h.profileService().Create(c.Request.Context(), input)
	if err != nil {
		respondProfileError(c, err)
		return
	}
	c.JSON(http.StatusCreated, profileContract(created, 0, false, nil))
}

// UpdateProfile updates the identity fields of one profile.
func (h *BaseHandlers) UpdateProfile(c *gin.Context) {
	var request contract.UpdateProfileRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondProfileBindingError(c, err)
		return
	}
	updated, err := h.profileService().UpdateIdentity(c.Request.Context(), c.Param("name"), profilepkg.IdentityPatch{
		Color: request.Color, Icon: request.Icon, Emoji: request.Emoji,
	})
	if err != nil {
		respondProfileError(c, err)
		return
	}
	c.JSON(http.StatusOK, profileContract(updated, 0, false, nil))
}

// PrepareProfileRename previews the effects of renaming one profile.
func (h *BaseHandlers) PrepareProfileRename(c *gin.Context) {
	plan, err := h.profileService().PrepareRename(c.Request.Context(), c.Param("name"), c.Query("new_name"))
	if err != nil {
		respondProfileError(c, err)
		return
	}
	c.JSON(http.StatusOK, renamePlanContract(plan))
}

// PrepareProfileArchive previews the blockers and work affected by archiving a profile.
func (h *BaseHandlers) PrepareProfileArchive(c *gin.Context) {
	plan, err := h.profileService().PrepareArchive(c.Request.Context(), c.Param("name"))
	if err != nil {
		respondProfileError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.ArchiveProfilePlan{
		Revision:           plan.Revision,
		RunningSessions:    plan.RunningSessions,
		ApprovalBlockers:   plan.ApprovalBlockers,
		LeasedRuns:         plan.LeasedRuns,
		QueuedRunsToFreeze: plan.QueuedRunsToFreeze,
		AutomationsToPause: plan.AutomationsToPause,
	})
}

// PrepareProfileDelete previews the resources that deleting a profile will remove.
func (h *BaseHandlers) PrepareProfileDelete(c *gin.Context) {
	plan, err := h.profileService().PrepareDelete(c.Request.Context(), c.Param("name"))
	if err != nil {
		respondProfileError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.DeleteProfilePlan{
		Revision: plan.Revision, Removed: removalContract(plan.Removed),
		SelectionsToSweep: plan.SelectionsToSweep, ApprovalBlockers: plan.ApprovalBlockers,
	})
}

// RenameProfile applies a previously reviewed profile rename request.
func (h *BaseHandlers) RenameProfile(c *gin.Context) {
	var request contract.RenameProfileRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondProfileBindingError(c, err)
		return
	}
	result, err := h.profileService().Rename(c.Request.Context(), c.Param("name"), profilepkg.RenameOptions{
		NewName: request.NewName, Repos: repoChoice(request.Repos), PlanRevision: request.PlanRevision,
	})
	if err != nil {
		respondProfileError(c, err)
		return
	}
	c.JSON(http.StatusOK, renameResultContract(result))
}

// ArchiveProfile archives a profile after validating the supplied plan revision.
func (h *BaseHandlers) ArchiveProfile(c *gin.Context) {
	var request contract.ProfilePlanRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondProfileBindingError(c, err)
		return
	}
	result, err := h.profileService().Archive(c.Request.Context(), c.Param("name"), request.PlanRevision)
	if err != nil {
		respondProfileError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.ArchiveProfileResponse{
		State: string(profilepkg.StateArchived), PausedAutomations: result.PausedAutomations,
		FrozenQueuedRuns: result.FrozenQueuedRuns,
	})
}

// UnarchiveProfile restores an archived profile to the active catalog.
func (h *BaseHandlers) UnarchiveProfile(c *gin.Context) {
	result, err := h.profileService().Unarchive(c.Request.Context(), c.Param("name"))
	if err != nil {
		respondProfileError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.UnarchiveProfileResponse{
		State: string(result.Profile.State), PausedAutomations: result.PausedAutomations,
	})
}

// DeleteProfile removes a profile after validating the supplied plan revision.
func (h *BaseHandlers) DeleteProfile(c *gin.Context) {
	result, err := h.profileService().Delete(c.Request.Context(), c.Param("name"), c.Query("plan_revision"))
	if err != nil {
		respondProfileError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.DeleteProfileResponse{Deleted: true, Removed: removalContract(result.Removed)})
}

// ListProfileOperations returns lifecycle operations recorded by the profile service.
func (h *BaseHandlers) ListProfileOperations(c *gin.Context) {
	operations, err := h.profileService().ListOps(c.Request.Context())
	if err != nil {
		respondProfileError(c, err)
		return
	}
	c.JSON(http.StatusOK, operationContracts(operations))
}

// RetryProfileOperation retries one failed profile lifecycle operation.
func (h *BaseHandlers) RetryProfileOperation(c *gin.Context) {
	operation, err := h.profileService().RetryOp(c.Request.Context(), c.Param("op_id"))
	if err != nil {
		respondProfileError(c, err)
		return
	}
	c.JSON(http.StatusOK, operationContract(operation))
}

// GetProfileSelections returns remembered profile selections, optionally filtered to one lens.
func (h *BaseHandlers) GetProfileSelections(c *gin.Context) {
	selections, profiles, err := h.profileSelections(c)
	if err != nil {
		respondProfileError(c, err)
		return
	}
	contracts := selectionContracts(selections, profiles)
	if scope, workspaceID := strings.TrimSpace(
		c.Query("scope"),
	), strings.TrimSpace(
		c.Query("workspace_id"),
	); scope != "" ||
		workspaceID != "" {
		if scope == "" {
			scope = string(profilepkg.SelectionLensWorkspace)
		}
		lens := profilepkg.Lens{Kind: profilepkg.SelectionLens(scope), WorkspaceID: workspaceID}
		if err := lens.Validate(); err != nil {
			respondProfileError(c, err)
			return
		}
		for _, selection := range contracts {
			if string(selection.Scope) == string(lens.Kind) && selection.WorkspaceID == workspaceID {
				c.JSON(http.StatusOK, []contract.ProfileSelection{selection})
				return
			}
		}
		c.JSON(http.StatusOK, []contract.ProfileSelection{{
			Scope: contract.ProfileSelectionScope(lens.Kind), WorkspaceID: workspaceID, Profile: profileDefaultName,
		}})
		return
	}
	c.JSON(http.StatusOK, contracts)
}

// PutProfileSelection stores the selected profile for one selection lens.
func (h *BaseHandlers) PutProfileSelection(c *gin.Context) {
	var request contract.ProfileSelection
	if err := c.ShouldBindJSON(&request); err != nil {
		respondProfileBindingError(c, err)
		return
	}
	selected, err := h.profileService().GetByName(c.Request.Context(), request.Profile)
	if err == nil {
		err = h.profileService().PutSelection(c.Request.Context(), profilepkg.Selection{
			Lens: profilepkg.SelectionLens(request.Scope), WorkspaceID: request.WorkspaceID, ProfileID: selected.ID,
		})
	}
	if err != nil {
		respondProfileError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.ProfileSelection{
		Scope: contract.ProfileSelectionScope(
			profilepkg.SelectionLens(request.Scope),
		),
		WorkspaceID: strings.TrimSpace(request.WorkspaceID),
		Profile:     selected.Name,
	})
}

func (h *BaseHandlers) profileService() ProfileService {
	if h == nil || h.Profiles == nil {
		return unavailableProfileService{}
	}
	return h.Profiles
}
