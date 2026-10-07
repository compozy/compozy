package worktree

type ExitAction string

const (
	ExitActionDeliver    ExitAction = "deliver"
	ExitActionCommit     ExitAction = "commit"
	ExitActionCommitPush ExitAction = "commit_push"
	ExitActionPush       ExitAction = "push"
	ExitActionOpenPR     ExitAction = "open_pr"
	ExitActionViewPR     ExitAction = "view_pr"
)

type ExitActionPlan struct {
	Action        ExitAction `json:"action"`
	Label         string     `json:"label"`
	Enabled       bool       `json:"enabled"`
	BlockedReason string     `json:"blocked_reason,omitempty"`
	Publish       bool       `json:"publish,omitzero"`
	URL           string     `json:"url,omitempty"`
	PRNumber      int        `json:"pr_number,omitzero"`
}

type ExitCommitScope struct {
	IncludePaths       []string `json:"include_paths,omitempty"`
	Fingerprint        string   `json:"fingerprint,omitempty"`
	Complete           bool     `json:"complete,omitzero"`
	ChangedFiles       int      `json:"changed_files"`
	Insertions         int      `json:"insertions"`
	Deletions          int      `json:"deletions"`
	UntrackedFiles     []string `json:"untracked_files"`
	UntrackedTotal     int      `json:"untracked_total"`
	UntrackedTruncated bool     `json:"untracked_truncated,omitzero"`
}

type ExitCleanupEvidence struct {
	ForgeState string `json:"forge_state,omitempty"`
	Stale      bool   `json:"stale,omitzero"`
	Safe       bool   `json:"safe"`
	Source     string `json:"source,omitempty"`
	Summary    string `json:"summary,omitempty"`
	Blocker    string `json:"blocker,omitempty"`
	Downgraded bool   `json:"downgraded,omitzero"`
}

type ExitPRPrefill struct {
	Title string `json:"title,omitempty"`
	Body  string `json:"body,omitempty"`
}

type ExitPlan struct {
	WorktreeID       string              `json:"worktree_id"`
	Primary          ExitAction          `json:"primary,omitempty"`
	Actions          []ExitActionPlan    `json:"actions"`
	GlobalPauseCause string              `json:"global_pause_cause,omitempty"`
	CommitScope      ExitCommitScope     `json:"commit_scope"`
	BrowserURL       string              `json:"browser_url,omitempty"`
	Forge            *ForgeCapabilities  `json:"forge,omitzero"`
	ForgeStatus      *ForgeStatus        `json:"forge_status,omitzero"`
	PRPrefill        *ExitPRPrefill      `json:"pr_prefill,omitzero"`
	Cleanup          ExitCleanupEvidence `json:"cleanup"`
	Base             string              `json:"base,omitempty"`
	RemoteURLs       []string            `json:"remote_urls,omitempty"`
}
