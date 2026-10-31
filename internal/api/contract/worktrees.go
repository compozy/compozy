package contract

import "time"

type CreateWorktreeRequest struct {
	Name           string `json:"name,omitempty"`
	Branch         string `json:"branch,omitempty"`
	BaseRef        string `json:"base_ref,omitempty"`
	ExistingBranch string `json:"existing_branch,omitempty"`
	Path           string `json:"path,omitempty"`
}

type AdoptWorktreeRequest struct {
	Path string `json:"path"`
}

// RemoveDiscoveredWorktreeRequest names a Git-only linked worktree by the path
// exactly as the listing reports it. Force acknowledges dirty or unpushed loss.
// ExpectStale confirms only clearing a stale Git entry; a checkout that is
// present again is refused with worktree_discovered_changed.
type RemoveDiscoveredWorktreeRequest struct {
	Path        string `json:"path"`
	Force       bool   `json:"force,omitempty"`
	ExpectStale bool   `json:"expect_stale,omitempty"`
}

type WorktreeStatusPayload struct {
	Branch      *string    `json:"branch"`
	Detached    *bool      `json:"detached"`
	HeadSHA     *string    `json:"head_sha"`
	DirtyFiles  *int       `json:"dirty_files"`
	Insertions  *int       `json:"insertions"`
	Deletions   *int       `json:"deletions"`
	HasUpstream *bool      `json:"has_upstream"`
	Ahead       *int       `json:"ahead"`
	AheadOfBase *int       `json:"ahead_of_base"`
	Behind      *int       `json:"behind"`
	ReadError   string     `json:"read_error,omitempty"`
	RefreshedAt *time.Time `json:"refreshed_at"`
}

type WorktreeForgeStatusPayload struct {
	Provider  string     `json:"provider"`
	PRNumber  *int       `json:"pr_number"`
	PRState   *string    `json:"pr_state"`
	PRURL     string     `json:"pr_url,omitempty"`
	Merged    *bool      `json:"merged"`
	FetchedAt *time.Time `json:"fetched_at"`
}

type WorktreePayload struct {
	ID              string    `json:"id"`
	ProfileID       string    `json:"profile_id"`
	ProfileName     string    `json:"profile_name"`
	ProfileColor    string    `json:"profile_color,omitempty"`
	ProfileIcon     string    `json:"profile_icon,omitempty"`
	ProfileEmoji    string    `json:"profile_emoji,omitempty"`
	ProfileArchived bool      `json:"profile_archived"`
	WorkspaceID     string    `json:"workspace_id"`
	Name            string    `json:"name"`
	Branch          string    `json:"branch"`
	Path            string    `json:"path"`
	State           string    `json:"state"`
	PendingPhase    string    `json:"pending_phase,omitempty"`
	Origin          string    `json:"origin"`
	SetupState      string    `json:"setup_state"`
	SetupError      string    `json:"setup_error,omitempty"`
	CreateError     string    `json:"create_error,omitempty"`
	BaseRef         string    `json:"base_ref,omitempty"`
	CreatedBranch   bool      `json:"created_branch"`
	RunNamespace    string    `json:"run_namespace,omitempty"`
	CreatedHead     string    `json:"created_head,omitempty"`
	RunID           string    `json:"run_id,omitempty"`
	Dirty           *bool     `json:"dirty"`
	Ahead           *int      `json:"ahead"`
	Behind          *int      `json:"behind"`
	AgentActivity   string    `json:"agent_activity"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type DiscoveredWorktreePayload struct {
	Name        string `json:"name"`
	Branch      string `json:"branch"`
	Path        string `json:"path"`
	Detached    bool   `json:"detached"`
	Stale       bool   `json:"stale"`
	Selectable  bool   `json:"selectable"`
	Unavailable bool   `json:"unavailable"`
	Reason      string `json:"reason,omitempty"`
}

type WorktreeRepoPayload struct {
	GitBacked    bool   `json:"git_backed"`
	GitAvailable bool   `json:"git_available"`
	Diagnostic   string `json:"diagnostic,omitempty"`
}

type WorktreeBindingsPayload struct {
	AgentActivity string `json:"agent_activity"`
}

type WorktreesResponse struct {
	Worktrees  []WorktreePayload           `json:"worktrees"`
	Discovered []DiscoveredWorktreePayload `json:"discovered"`
	Repo       WorktreeRepoPayload         `json:"repo"`
}

type WorktreeResponse struct {
	Worktree WorktreePayload `json:"worktree"`
}

type WorktreeInspectionResponse struct {
	Worktree WorktreePayload             `json:"worktree"`
	Status   *WorktreeStatusPayload      `json:"status"`
	Forge    *WorktreeForgeStatusPayload `json:"forge"`
	Repo     WorktreeRepoPayload         `json:"repo"`
	Bindings WorktreeBindingsPayload     `json:"bindings"`
}

type WorktreeStatusResponse struct {
	WorktreeID string                      `json:"worktree_id"`
	Status     WorktreeStatusPayload       `json:"status"`
	Forge      *WorktreeForgeStatusPayload `json:"forge"`
}

type WorktreeRemovalRiskPayload struct {
	ChangedFiles    int  `json:"changed_files"`
	Insertions      int  `json:"insertions"`
	Deletions       int  `json:"deletions"`
	UnpushedCommits int  `json:"unpushed_commits"`
	ExistsOnRemote  bool `json:"exists_on_remote"`
	// LeftoverFiles counts the files left in a worktree_checkout_orphaned
	// checkout; Git cannot tell which of them hold changes.
	LeftoverFiles int `json:"leftover_files"`
}

type WorktreeRemovalRefusalPayload struct {
	Code string `json:"code"`
	// Error explains the refusal and its force consequence in one sentence.
	Error     string                     `json:"error,omitempty"`
	Risk      WorktreeRemovalRiskPayload `json:"risk"`
	Downgrade bool                       `json:"downgrade"`
}

// WorktreeBranchOutcomePayload reports an opted-in local branch deletion. A kept
// branch names a stable reason: no_branch, branch_absent, checkout_present,
// root_checkout_branch, default_branch, registration_present,
// checked_out_elsewhere, not_fully_merged, or delete_failed.
type WorktreeBranchOutcomePayload struct {
	Name    string `json:"name"`
	Deleted bool   `json:"deleted"`
	Reason  string `json:"reason,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

// WorktreeCleanupResponse answers a removal or dismissal that requested
// delete_branch; requests without it keep their 204 response, which clients
// decode as an absent (nil) branch outcome.
type WorktreeCleanupResponse struct {
	Branch *WorktreeBranchOutcomePayload `json:"branch,omitempty"`
}

type WorktreeCatalogEventPayload struct {
	Kind        string `json:"kind"`
	WorkspaceID string `json:"workspace_id"`
	WorktreeID  string `json:"worktree_id"`
	Error       string `json:"error,omitempty"`
}
