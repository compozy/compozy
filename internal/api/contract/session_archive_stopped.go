package contract

// SessionArchiveStoppedRequest archives every eligible stopped thread of the
// selected profile in one workspace or across all of its unarchived workspaces.
type SessionArchiveStoppedRequest struct {
	// WorkspaceID is a workspace id, path, or name; omit it with all_workspaces.
	WorkspaceID   string `json:"workspace_id,omitempty"`
	AllWorkspaces bool   `json:"all_workspaces,omitempty"`
	// DryRun reports what the action would archive and archives nothing.
	DryRun bool `json:"dry_run,omitempty"`
}

// SessionArchiveStoppedThread is one archived thread, or one a dry run would archive.
type SessionArchiveStoppedThread struct {
	RootID      string `json:"root_id"`
	WorkspaceID string `json:"workspace_id"`
	Title       string `json:"title"`
	// SessionIDs are every session archived with the thread, root first.
	SessionIDs []string `json:"session_ids"`
	// RestoreIDs are SessionIDs without subagents, root first; unarchiving them
	// one by one in this order undoes the action, as subagents restore with their parent.
	RestoreIDs []string `json:"restore_ids"`
}

// SessionArchiveStoppedSkip is one thread the action left alone. Reason is one of
// instance_main, running, waiting_on_you, queued_input, pinned, later, caller.
type SessionArchiveStoppedSkip struct {
	RootID      string `json:"root_id"`
	WorkspaceID string `json:"workspace_id"`
	Title       string `json:"title"`
	Reason      string `json:"reason"`
}

// SessionArchiveStoppedFailure is one thread whose archive failed; code is archive_failed.
type SessionArchiveStoppedFailure struct {
	RootID      string `json:"root_id"`
	WorkspaceID string `json:"workspace_id"`
	Title       string `json:"title"`
	Code        string `json:"code"`
	Message     string `json:"message"`
}

// SessionArchiveStoppedResponse reports one Archive stopped action. Archived is
// complete; skipped and failures list the first 100 threads, while skip_counts
// and failure_count count every skipped and failed thread.
type SessionArchiveStoppedResponse struct {
	DryRun               bool                           `json:"dry_run"`
	Archived             []SessionArchiveStoppedThread  `json:"archived"`
	ArchivedThreadCount  int                            `json:"archived_thread_count"`
	ArchivedSessionCount int                            `json:"archived_session_count"`
	SkipCounts           map[string]int                 `json:"skip_counts"`
	Skipped              []SessionArchiveStoppedSkip    `json:"skipped"`
	SkippedTruncated     bool                           `json:"skipped_truncated"`
	Failures             []SessionArchiveStoppedFailure `json:"failures"`
	FailuresTruncated    bool                           `json:"failures_truncated"`
	// FailureCount counts every failed thread, listed in failures or not.
	FailureCount int `json:"failure_count"`
}
