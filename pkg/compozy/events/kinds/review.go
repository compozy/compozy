package kinds

import "time"

// ReviewStatusFinalizedPayload describes finalized review issue statuses.
type ReviewStatusFinalizedPayload struct {
	ReviewsDir string   `json:"reviews_dir"`
	IssueIDs   []string `json:"issue_ids,omitempty"`
}

// ReviewRoundRefreshedPayload describes refreshed review round metadata.
type ReviewRoundRefreshedPayload struct {
	ReviewsDir string    `json:"reviews_dir"`
	Provider   string    `json:"provider,omitempty"`
	PR         string    `json:"pr,omitempty"`
	Round      int       `json:"round,omitempty"`
	CreatedAt  time.Time `json:"created_at,omitzero"`
	Total      int       `json:"total,omitempty"`
	Resolved   int       `json:"resolved,omitempty"`
	Unresolved int       `json:"unresolved,omitempty"`
}

// ReviewIssueResolvedPayload describes a resolved provider-backed issue.
type ReviewIssueResolvedPayload struct {
	ReviewsDir     string    `json:"reviews_dir"`
	IssueID        string    `json:"issue_id"`
	FilePath       string    `json:"file_path,omitempty"`
	Provider       string    `json:"provider,omitempty"`
	PR             string    `json:"pr,omitempty"`
	ProviderRef    string    `json:"provider_ref,omitempty"`
	ProviderPosted bool      `json:"provider_posted,omitempty"`
	PostedAt       time.Time `json:"posted_at,omitzero"`
}

// ReviewWatchPayload describes daemon review-watch lifecycle events.
type ReviewWatchPayload struct {
	Provider        string `json:"provider,omitempty"`
	PR              string `json:"pr,omitempty"`
	Workflow        string `json:"workflow,omitempty"`
	Round           int    `json:"round,omitempty"`
	RunID           string `json:"run_id,omitempty"`
	ChildRunID      string `json:"child_run_id,omitempty"`
	HeadSHA         string `json:"head_sha,omitempty"`
	ReviewID        string `json:"review_id,omitempty"`
	ReviewState     string `json:"review_state,omitempty"`
	Status          string `json:"status,omitempty"`
	Remote          string `json:"remote,omitempty"`
	Branch          string `json:"branch,omitempty"`
	Total           int    `json:"total,omitempty"`
	Resolved        int    `json:"resolved,omitempty"`
	Unresolved      int    `json:"unresolved,omitempty"`
	Dirty           bool   `json:"dirty,omitempty"`
	UnpushedCommits int    `json:"unpushed_commits,omitempty"`
	Error           string `json:"error,omitempty"`
}
