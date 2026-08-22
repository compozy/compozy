package core

import (
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/gin-gonic/gin"
)

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

func (h *BaseHandlers) GetProfile(c *gin.Context) {
	value, err := h.profileService().GetByName(c.Request.Context(), c.Param("name"))
	if err != nil {
		respondProfileError(c, err)
		return
	}
	profiles, err := h.profileService().List(c.Request.Context())
	if err != nil {
		respondProfileError(c, err)
		return
	}
	for _, item := range profiles {
		if item.ID == value.ID {
			c.JSON(http.StatusOK, profileContract(
				value, item.WorkItems, item.NeedsSetup, item.CredentialRequirements,
			))
			return
		}
	}
	c.JSON(http.StatusOK, profileContract(value, 0, false, nil))
}

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

func (h *BaseHandlers) PrepareProfileRename(c *gin.Context) {
	plan, err := h.profileService().PrepareRename(c.Request.Context(), c.Param("name"), c.Query("new_name"))
	if err != nil {
		respondProfileError(c, err)
		return
	}
	c.JSON(http.StatusOK, renamePlanContract(plan))
}

func (h *BaseHandlers) PrepareProfileArchive(c *gin.Context) {
	plan, err := h.profileService().PrepareArchive(c.Request.Context(), c.Param("name"))
	if err != nil {
		respondProfileError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.ArchiveProfilePlan{
		Revision: plan.Revision, RunningSessions: plan.RunningSessions, ApprovalBlockers: plan.ApprovalBlockers,
		LeasedRuns: plan.LeasedRuns, QueuedRunsToFreeze: plan.QueuedRunsToFreeze, AutomationsToPause: plan.AutomationsToPause,
	})
}

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

func (h *BaseHandlers) DeleteProfile(c *gin.Context) {
	result, err := h.profileService().Delete(c.Request.Context(), c.Param("name"), c.Query("plan_revision"))
	if err != nil {
		respondProfileError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.DeleteProfileResponse{Deleted: true, Removed: removalContract(result.Removed)})
}

func (h *BaseHandlers) ListProfileOperations(c *gin.Context) {
	operations, err := h.profileService().ListOps(c.Request.Context())
	if err != nil {
		respondProfileError(c, err)
		return
	}
	c.JSON(http.StatusOK, operationContracts(operations))
}

func (h *BaseHandlers) RetryProfileOperation(c *gin.Context) {
	operation, err := h.profileService().RetryOp(c.Request.Context(), c.Param("op_id"))
	if err != nil {
		respondProfileError(c, err)
		return
	}
	c.JSON(http.StatusOK, operationContract(operation))
}

func (h *BaseHandlers) GetProfileSelections(c *gin.Context) {
	selections, profiles, err := h.profileSelections(c)
	if err != nil {
		respondProfileError(c, err)
		return
	}
	contracts := selectionContracts(selections, profiles)
	if scope, workspaceID := strings.TrimSpace(c.Query("scope")), strings.TrimSpace(c.Query("workspace_id")); scope != "" || workspaceID != "" {
		if scope == "" {
			scope = string(profilepkg.SelectionLensWorkspace)
		}
		lens := profilepkg.Lens{Kind: profilepkg.SelectionLens(scope), WorkspaceID: workspaceID}
		if err := lens.Validate(); err != nil {
			respondProfileError(c, err)
			return
		}
		for _, selection := range contracts {
			if selection.Scope == string(lens.Kind) && selection.WorkspaceID == workspaceID {
				c.JSON(http.StatusOK, selection)
				return
			}
		}
		c.JSON(http.StatusOK, contract.ProfileSelection{Scope: string(lens.Kind), WorkspaceID: workspaceID, Profile: "default"})
		return
	}
	c.JSON(http.StatusOK, contracts)
}

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
		Scope: string(profilepkg.SelectionLens(request.Scope)), WorkspaceID: strings.TrimSpace(request.WorkspaceID), Profile: selected.Name,
	})
}

func (h *BaseHandlers) profileService() ProfileService {
	if h == nil || h.Profiles == nil {
		return unavailableProfileService{}
	}
	return h.Profiles
}
