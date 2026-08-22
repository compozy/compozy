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

type ProfileSelection struct {
	Scope       string `json:"scope"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	Profile     string `json:"profile"`
}

type CreateProfileRequest struct {
	Name     string            `json:"name" binding:"required"`
	Color    string            `json:"color,omitempty"`
	Icon     string            `json:"icon,omitempty"`
	Emoji    string            `json:"emoji,omitempty"`
	Activate *ProfileSelection `json:"activate,omitempty"`
}

type UpdateProfileRequest struct {
	Color *string `json:"color,omitempty"`
	Icon  *string `json:"icon,omitempty"`
	Emoji *string `json:"emoji,omitempty"`
}

type RenameProfileRequest struct {
	NewName      string   `json:"new_name" binding:"required"`
	Repos        []string `json:"repos,omitempty"`
	PlanRevision string   `json:"plan_revision" binding:"required"`
}

type ProfilePlanRequest struct {
	PlanRevision string `json:"plan_revision" binding:"required"`
}

type RenameProfilePlan struct {
	Revision          string             `json:"revision"`
	MachineFolders    []string           `json:"machine_folders"`
	RepoCandidates    []ProfileFolderRef `json:"repo_candidates"`
	DormantPlacements []ProfilePlacement `json:"dormant_placements"`
	VaultRefRewrites  int                `json:"vault_ref_rewrites"`
}

type ArchiveProfilePlan struct {
	Revision           string   `json:"revision"`
	RunningSessions    []string `json:"running_sessions"`
	ApprovalBlockers   []string `json:"approval_blockers"`
	LeasedRuns         int      `json:"leased_runs"`
	QueuedRunsToFreeze int      `json:"queued_runs_to_freeze"`
	AutomationsToPause []string `json:"automations_to_pause"`
}

type DeleteProfilePlan struct {
	Revision          string                `json:"revision"`
	Removed           ProfileRemovalSummary `json:"removed"`
	SelectionsToSweep int                   `json:"selections_to_sweep"`
	ApprovalBlockers  []string              `json:"approval_blockers"`
}

type ProfileFolderRef struct {
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceName string `json:"workspace"`
	Path          string `json:"path"`
}

type ProfilePlacement struct {
	Extension string `json:"extension"`
	Resource  string `json:"resource"`
	Profile   string `json:"profile"`
}

type ProfileRepoRenameOutcome struct {
	WorkspaceID string `json:"workspace_id"`
	Renamed     bool   `json:"renamed"`
	Reason      string `json:"reason,omitempty"`
}

type RenameProfileResponse struct {
	Renamed           bool                       `json:"renamed"`
	RepoResults       []ProfileRepoRenameOutcome `json:"repo_results"`
	DormantPlacements []ProfilePlacement         `json:"dormant_placements"`
}

type ArchiveProfileResponse struct {
	State             string   `json:"state"`
	PausedAutomations []string `json:"paused_automations"`
	FrozenQueuedRuns  int      `json:"frozen_queued_runs"`
}

type UnarchiveProfileResponse struct {
	State             string   `json:"state"`
	PausedAutomations []string `json:"paused_automations"`
}

type DeleteProfileResponse struct {
	Deleted bool                  `json:"deleted"`
	Removed ProfileRemovalSummary `json:"removed"`
}

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
}

type ProfileOperation struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Profile string `json:"profile"`
	Status  string `json:"status"`
	Step    string `json:"step"`
	Error   string `json:"error,omitempty"`
}

type ProfileErrorPayload struct {
	Error ProfileError `json:"error"`
}

type ProfileError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Action  string `json:"action"`
}
