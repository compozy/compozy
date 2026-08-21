package core

import (
	"github.com/compozy/compozy/internal/api/contract"
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/gin-gonic/gin"
)

func profileContract(
	value profilepkg.Profile,
	workItems int,
	needsSetup bool,
	requirements []profilepkg.CredentialRequirement,
) contract.Profile {
	return contract.Profile{
		ID: value.ID, Name: value.Name, Color: value.Color, Icon: optionalString(value.Icon),
		Emoji: optionalString(value.Emoji), State: string(value.State), CreatedAt: value.CreatedAt,
		ArchivedAt: value.ArchivedAt, WorkItems: workItems, NeedsSetup: needsSetup,
		CredentialRequirements: credentialRequirementContracts(requirements),
	}
}

func credentialRequirementContracts(
	values []profilepkg.CredentialRequirement,
) []contract.ProfileCredentialRequirement {
	result := make([]contract.ProfileCredentialRequirement, 0, len(values))
	for _, value := range values {
		result = append(result, contract.ProfileCredentialRequirement{
			Provider: value.Provider, Slot: value.Slot,
			SourceExtension: value.SourceExtension, Missing: value.Missing,
		})
	}
	return result
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func repoChoice(values []string) profilepkg.RepoChoice {
	choice := profilepkg.RepoChoice{}
	if len(values) == 0 {
		choice.None = true
		return choice
	}
	if len(values) == 1 && values[0] == "all" {
		choice.All = true
		return choice
	}
	if len(values) == 1 && values[0] == "none" {
		choice.None = true
		return choice
	}
	choice.WorkspaceIDs = append([]string(nil), values...)
	return choice
}

func renamePlanContract(plan profilepkg.RenamePlan) contract.RenameProfilePlan {
	repos := make([]contract.ProfileFolderRef, 0, len(plan.RepoCandidates))
	for _, item := range plan.RepoCandidates {
		repos = append(repos, contract.ProfileFolderRef{
			WorkspaceID: item.WorkspaceID, WorkspaceName: item.WorkspaceName, Path: item.Path,
		})
	}
	return contract.RenameProfilePlan{
		Revision: plan.Revision, MachineFolders: plan.MachineFolders, RepoCandidates: repos,
		DormantPlacements: placementContracts(plan.DormantPlacements), VaultRefRewrites: plan.VaultRefRewrites,
	}
}

func renameResultContract(result profilepkg.RenameResult) contract.RenameProfileResponse {
	repos := make([]contract.ProfileRepoRenameOutcome, 0, len(result.RepoResults))
	for _, item := range result.RepoResults {
		repos = append(repos, contract.ProfileRepoRenameOutcome{
			WorkspaceID: item.WorkspaceID, Renamed: item.Renamed, Reason: item.Reason,
		})
	}
	return contract.RenameProfileResponse{
		Renamed: true, RepoResults: repos, DormantPlacements: placementContracts(result.DormantPlacements),
	}
}

func placementContracts(values []profilepkg.PlacementRef) []contract.ProfilePlacement {
	result := make([]contract.ProfilePlacement, 0, len(values))
	for _, value := range values {
		result = append(result, contract.ProfilePlacement{
			Extension: value.Extension, Resource: value.Resource, Profile: value.ProfileName,
		})
	}
	return result
}

func removalContract(value profilepkg.RemovalSummary) contract.ProfileRemovalSummary {
	return contract.ProfileRemovalSummary{
		Agents: value.Agents, Skills: value.Skills, Loops: value.Loops, MCPServers: value.MCPServers,
		ConfigKeys: value.ConfigKeys, CredentialOverrides: value.CredentialOverrides,
		MemoryEntries: value.MemoryEntries, DesktopPartitions: value.DesktopPartitions,
		PaletteUsage: value.PaletteUsage, PaletteQueryHits: value.PaletteQueryHits,
		PalettePins: value.PalettePins, TerminalApprovals: value.TerminalApprovals,
		EventSummaries: value.EventSummaries,
	}
}

func operationContracts(values []profilepkg.LifecycleOp) []contract.ProfileOperation {
	result := make([]contract.ProfileOperation, 0, len(values))
	for _, value := range values {
		result = append(result, operationContract(value))
	}
	return result
}

func operationContract(value profilepkg.LifecycleOp) contract.ProfileOperation {
	return contract.ProfileOperation{
		ID:      value.ID,
		Kind:    value.Kind,
		Profile: value.Profile,
		Status:  value.Status,
		Step:    value.Step,
		Error:   value.Error,
	}
}

func (h *BaseHandlers) profileSelections(c *gin.Context) ([]profilepkg.Selection, map[string]string, error) {
	selections, err := h.profileService().ListSelections(c.Request.Context())
	if err != nil {
		return nil, nil, err
	}
	profiles, err := h.profileService().List(c.Request.Context())
	if err != nil {
		return nil, nil, err
	}
	names := make(map[string]string, len(profiles))
	for _, item := range profiles {
		names[item.ID] = item.Name
	}
	return selections, names, nil
}

func selectionContracts(values []profilepkg.Selection, names map[string]string) []contract.ProfileSelection {
	result := make([]contract.ProfileSelection, 0, len(values))
	for _, value := range values {
		name, found := names[value.ProfileID]
		if !found {
			continue
		}
		result = append(result, contract.ProfileSelection{
			Scope: contract.ProfileSelectionScope(value.Lens), WorkspaceID: value.WorkspaceID, Profile: name,
		})
	}
	return result
}
