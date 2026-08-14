package contract

type WorktreeExitAction string

type WorktreeExitActionPlan struct {
	Action        WorktreeExitAction `json:"action"`
	Label         string             `json:"label"`
	Enabled       bool               `json:"enabled"`
	BlockedReason string             `json:"blocked_reason,omitempty"`
	Publish       bool               `json:"publish,omitempty"`
	URL           string             `json:"url,omitempty"`
	PRNumber      int                `json:"pr_number,omitempty"`
}

type WorktreeExitCommitScope struct {
	ChangedFiles       int      `json:"changed_files"`
	Insertions         int      `json:"insertions"`
	Deletions          int      `json:"deletions"`
	UntrackedFiles     []string `json:"untracked_files"`
	UntrackedTotal     int      `json:"untracked_total"`
	UntrackedTruncated bool     `json:"untracked_truncated,omitempty"`
}

type WorktreeExitCleanupEvidence struct {
	ForgeState string `json:"forge_state,omitempty"`
	Stale      bool   `json:"stale,omitempty"`
	Safe       bool   `json:"safe"`
	Source     string `json:"source,omitempty"`
	Summary    string `json:"summary,omitempty"`
	Blocker    string `json:"blocker,omitempty"`
	Downgraded bool   `json:"downgraded,omitempty"`
}

type WorktreeExitPRPrefill struct {
	Title string `json:"title,omitempty"`
	Body  string `json:"body,omitempty"`
}

type WorktreeExitForgeCapabilities struct {
	Provider           string   `json:"provider"`
	RequestNoun        string   `json:"request_noun"`
	OpenActionLabel    string   `json:"open_action_label"`
	ViewActionLabel    string   `json:"view_action_label"`
	SupportsDraft      bool     `json:"supports_draft"`
	CompareURLTemplate string   `json:"compare_url_template,omitempty"`
	CredentialSource   string   `json:"credential_source,omitempty"`
	DefaultBranch      string   `json:"default_branch,omitempty"`
	TemplatePaths      []string `json:"template_paths,omitempty"`
}

type WorktreeExitPlanResponse struct {
	WorktreeID       string                         `json:"worktree_id"`
	Primary          WorktreeExitAction             `json:"primary,omitempty"`
	Actions          []WorktreeExitActionPlan       `json:"actions"`
	GlobalPauseCause string                         `json:"global_pause_cause,omitempty"`
	CommitScope      WorktreeExitCommitScope        `json:"commit_scope"`
	BrowserURL       string                         `json:"browser_url,omitempty"`
	Forge            *WorktreeExitForgeCapabilities `json:"forge,omitempty"`
	ForgeStatus      *WorktreeForgeStatusPayload    `json:"forge_status,omitempty"`
	PRPrefill        *WorktreeExitPRPrefill         `json:"pr_prefill,omitempty"`
	Cleanup          WorktreeExitCleanupEvidence    `json:"cleanup"`
	Base             string                         `json:"base,omitempty"`
}

type RunWorktreeExitActionRequest struct {
	Action  WorktreeExitAction `json:"action"`
	Message string             `json:"message,omitempty"`
	Title   string             `json:"title,omitempty"`
	Body    string             `json:"body,omitempty"`
	Draft   bool               `json:"draft,omitempty"`
	Base    string             `json:"base,omitempty"`
}

type WorktreeExitOperationResponse struct {
	OperationID string `json:"op_id"`
}

type CancelWorktreeExitActionRequest struct {
	OperationID string `json:"op_id"`
}
