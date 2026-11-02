package contract

type SessionWorkLinkRequest struct {
	Repository string `json:"repository,omitempty"`
	Branch     string `json:"branch"`
	PRNumber   int    `json:"pr_number,omitempty"`
}

type SessionLinkedWorkItem struct {
	Repository         string                           `json:"repository"`
	Branch             string                           `json:"branch"`
	PRNumber           int                              `json:"pr_number,omitempty"`
	WorktreeID         string                           `json:"worktree_id,omitempty"`
	WorktreeName       string                           `json:"worktree_name,omitempty"`
	WorktreeRemoved    bool                             `json:"worktree_removed"`
	Provenance         string                           `json:"provenance"`
	RunsHere           bool                             `json:"runs_here"`
	SourceSessionID    string                           `json:"source_session_id,omitempty"`
	SourceSessionTitle string                           `json:"source_session_title,omitempty"`
	PullRequest        *SessionPullRequestPayload       `json:"pull_request"`
	PullRequestLookup  *SessionPullRequestLookupPayload `json:"pull_request_lookup"`
}

type SessionLinkedWorkSummary struct {
	Total    int                    `json:"total"`
	NeedsYou int                    `json:"needs_you"`
	Lead     *SessionLinkedWorkItem `json:"lead,omitempty"`
}

type SessionLinkedWorkResponse struct {
	Items []SessionLinkedWorkItem `json:"items"`
}
