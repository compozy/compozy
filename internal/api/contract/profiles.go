package contract

import "time"

// Profile is the public identity record shared by the profile API surfaces.
type Profile struct {
	ID                     string                         `json:"id"`
	Name                   string                         `json:"name"`
	Color                  string                         `json:"color"`
	Icon                   *string                        `json:"icon"`
	Emoji                  *string                        `json:"emoji"`
	State                  string                         `json:"state"`
	CreatedAt              time.Time                      `json:"created_at"`
	ArchivedAt             *time.Time                     `json:"archived_at,omitempty"`
	WorkItems              int                            `json:"work_items,omitempty"`
	NeedsSetup             bool                           `json:"needs_setup,omitempty"`
	CredentialRequirements []ProfileCredentialRequirement `json:"credential_requirements,omitempty"`
}

// ProfileCredentialRequirement is one missing vault-backed setup item.
type ProfileCredentialRequirement struct {
	Provider        string `json:"provider"`
	Slot            string `json:"slot"`
	SourceExtension string `json:"source_extension"`
	Missing         bool   `json:"missing"`
}

// ProfileSelectionScope identifies the lens whose profile selection is stored.
type ProfileSelectionScope string

const (
	ProfileSelectionScopeGlobal    ProfileSelectionScope = "global"
	ProfileSelectionScopeWorkspace ProfileSelectionScope = "workspace"
)

// ProfileSelection describes one remembered profile selection for a lens.
type ProfileSelection struct {
	Scope       ProfileSelectionScope `json:"scope"`
	WorkspaceID string                `json:"workspace_id,omitempty"`
	Profile     string                `json:"profile"`
}

// CreateProfileRequest creates one profile and may activate it immediately.
type CreateProfileRequest struct {
	Name     string            `json:"name"               binding:"required"`
	Color    string            `json:"color,omitempty"`
	Icon     string            `json:"icon,omitempty"`
	Emoji    string            `json:"emoji,omitempty"`
	Activate *ProfileSelection `json:"activate,omitempty"`
}

// UpdateProfileRequest changes mutable profile identity fields.
type UpdateProfileRequest struct {
	Color *string `json:"color,omitempty"`
	Icon  *string `json:"icon,omitempty"`
	Emoji *string `json:"emoji,omitempty"`
}

// RenameProfileRequest requests a profile rename using a prepared plan.
type RenameProfileRequest struct {
	NewName      string   `json:"new_name"        binding:"required"`
	Repos        []string `json:"repos,omitempty"`
	PlanRevision string   `json:"plan_revision"   binding:"required"`
}

// ProfilePlanRequest carries the revision that authorizes a lifecycle action.
type ProfilePlanRequest struct {
	PlanRevision string `json:"plan_revision" binding:"required"`
}

// RenameProfilePlan previews the folders and references affected by a rename.
type RenameProfilePlan struct {
	Revision          string             `json:"revision"`
	MachineFolders    []string           `json:"machine_folders"`
	RepoCandidates    []ProfileFolderRef `json:"repo_candidates"`
	DormantPlacements []ProfilePlacement `json:"dormant_placements"`
	VaultRefRewrites  int                `json:"vault_ref_rewrites"`
}

// ArchiveProfilePlan previews the resources affected by archiving a profile.
type ArchiveProfilePlan struct {
	Revision           string   `json:"revision"`
	RunningSessions    []string `json:"running_sessions"`
	ApprovalBlockers   []string `json:"approval_blockers"`
	LeasedRuns         int      `json:"leased_runs"`
	QueuedRunsToFreeze int      `json:"queued_runs_to_freeze"`
	AutomationsToPause []string `json:"automations_to_pause"`
}

// DeleteProfilePlan previews the resources removed with a profile.
type DeleteProfilePlan struct {
	Revision          string                `json:"revision"`
	Removed           ProfileRemovalSummary `json:"removed"`
	SelectionsToSweep int                   `json:"selections_to_sweep"`
	ApprovalBlockers  []string              `json:"approval_blockers"`
}

// ProfileFolderRef identifies one repository candidate for profile migration.
type ProfileFolderRef struct {
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceName string `json:"workspace"`
	Path          string `json:"path"`
}

// ProfilePlacement identifies one dormant profile-scoped resource.
type ProfilePlacement struct {
	Extension string `json:"extension"`
	Resource  string `json:"resource"`
	Profile   string `json:"profile"`
}

// ProfileRepoRenameOutcome reports one repository rename result.
type ProfileRepoRenameOutcome struct {
	WorkspaceID string `json:"workspace_id"`
	Renamed     bool   `json:"renamed"`
	Reason      string `json:"reason,omitempty"`
}

// RenameProfileResponse reports the outcome of a profile rename.
type RenameProfileResponse struct {
	Renamed           bool                       `json:"renamed"`
	RepoResults       []ProfileRepoRenameOutcome `json:"repo_results"`
	DormantPlacements []ProfilePlacement         `json:"dormant_placements"`
}

// ArchiveProfileResponse reports the outcome of archiving a profile.
type ArchiveProfileResponse struct {
	State             string   `json:"state"`
	PausedAutomations []string `json:"paused_automations"`
	FrozenQueuedRuns  int      `json:"frozen_queued_runs"`
}

// UnarchiveProfileResponse reports the outcome of restoring a profile.
type UnarchiveProfileResponse struct {
	State             string   `json:"state"`
	PausedAutomations []string `json:"paused_automations"`
}

// DeleteProfileResponse reports the outcome of deleting a profile.
type DeleteProfileResponse struct {
	Deleted bool                  `json:"deleted"`
	Removed ProfileRemovalSummary `json:"removed"`
}

// ProfileRemovalSummary counts resources removed with a profile.
type ProfileRemovalSummary struct {
	Agents              int `json:"agents"`
	Skills              int `json:"skills"`
	Loops               int `json:"loops"`
	MCPServers          int `json:"mcp_servers"`
	ConfigKeys          int `json:"config_keys"`
	CredentialOverrides int `json:"credential_overrides"`
	MemoryEntries       int `json:"memory_entries"`
	DesktopPartitions   int `json:"desktop_partitions"`
	PaletteUsage        int `json:"palette_usage"`
	PaletteQueryHits    int `json:"palette_query_hits"`
	PalettePins         int `json:"palette_pins"`
	TerminalApprovals   int `json:"terminal_approvals"`
	EventSummaries      int `json:"event_summaries"`
}

// ProfileOperation describes one asynchronous profile lifecycle operation.
type ProfileOperation struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Profile string `json:"profile"`
	Status  string `json:"status"`
	Step    string `json:"step"`
	Error   string `json:"error,omitempty"`
}

// ProfileErrorPayload wraps one public profile error.
type ProfileErrorPayload struct {
	Error ProfileError `json:"error"`
}

// ProfileError is the stable profile error contract.
type ProfileError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Action  string `json:"action"`
}
