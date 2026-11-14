package contract

import "time"

// SessionHistoryCleanupSelection is what one cleanup deletes. Everything
// deletes each picked archived thread whole and includes the other two;
// without it, children and subagents delete those sessions and keep the main
// sessions.
type SessionHistoryCleanupSelection struct {
	Everything bool `json:"everything"`
	Children   bool `json:"children"`
	Subagents  bool `json:"subagents"`
}

// SessionHistoryCleanupRequest plans or starts a cleanup of the selected
// profile's archived sessions, in one workspace or across all of them.
type SessionHistoryCleanupRequest struct {
	// WorkspaceID is a workspace id, path, name, or "@user"; omit it with all_workspaces.
	WorkspaceID   string                         `json:"workspace_id,omitempty"`
	AllWorkspaces bool                           `json:"all_workspaces,omitempty"`
	Delete        SessionHistoryCleanupSelection `json:"delete"`
	// OlderThanDays picks threads archived more than this many days ago.
	OlderThanDays *int `json:"older_than_days,omitempty"`
	// ArchivedBefore picks threads archived strictly before this instant.
	// At most one of older_than_days and archived_before; neither picks every archived thread.
	ArchivedBefore *time.Time `json:"archived_before,omitempty"`
	// SessionIDs narrows the picked threads to these archived main sessions, as a retry does.
	SessionIDs []string `json:"session_ids,omitempty"`
	// DryRun previews the cleanup and deletes nothing.
	DryRun bool `json:"dry_run,omitempty"`
}

// SessionHistoryCleanupCounts counts sessions by kind; total is their sum.
type SessionHistoryCleanupCounts struct {
	Sessions  int `json:"sessions"`
	Children  int `json:"children"`
	Subagents int `json:"subagents"`
	Total     int `json:"total"`
}

// SessionHistoryCleanupChoices are the totals each check alone would delete.
type SessionHistoryCleanupChoices struct {
	Everything int `json:"everything"`
	Children   int `json:"children"`
	Subagents  int `json:"subagents"`
}

// SessionHistoryCleanupWorkspace is how many sessions a cleanup deletes in one workspace.
type SessionHistoryCleanupWorkspace struct {
	WorkspaceID string `json:"workspace_id"`
	Total       int    `json:"total"`
}

// SessionHistoryCleanupPlan is the dry run of one cleanup. Every count is a
// number of sessions: a child session and a subagent are sessions too.
type SessionHistoryCleanupPlan struct {
	ArchivedBefore *time.Time                  `json:"archived_before,omitempty"`
	Counts         SessionHistoryCleanupCounts `json:"counts"`
	// KeptLive counts child sessions still in the session list or in Later that
	// the selection would otherwise delete; they and everything under them stay.
	KeptLive   int                              `json:"kept_live"`
	Choices    SessionHistoryCleanupChoices     `json:"choices"`
	Workspaces []SessionHistoryCleanupWorkspace `json:"workspaces"`
	// Threads is how many archived threads lose at least one session.
	Threads int `json:"threads"`
}

// SessionHistoryCleanupFailure is one thread that kept sessions the run meant
// to delete; retry it with session_ids set to its session_id.
type SessionHistoryCleanupFailure struct {
	SessionID   string `json:"session_id"`
	WorkspaceID string `json:"workspace_id"`
	Title       string `json:"title"`
	Code        string `json:"code"`
	Message     string `json:"message"`
}

// SessionHistoryCleanupRun is one background cleanup. State is running,
// completed, partial, failed, or stopped.
type SessionHistoryCleanupRun struct {
	ID             string                         `json:"id"`
	ProfileID      string                         `json:"profile_id"`
	WorkspaceID    string                         `json:"workspace_id,omitempty"`
	AllWorkspaces  bool                           `json:"all_workspaces,omitempty"`
	Delete         SessionHistoryCleanupSelection `json:"delete"`
	ArchivedBefore *time.Time                     `json:"archived_before,omitempty"`
	State          string                         `json:"state"`
	// Total is every session the run planned to delete; done is how many it settled.
	Total   int `json:"total"`
	Done    int `json:"done"`
	Deleted int `json:"deleted"`
	// Skipped counts sessions restored or moved after the plan, so kept.
	Skipped       int                            `json:"skipped"`
	FailedThreads int                            `json:"failed_threads"`
	Failures      []SessionHistoryCleanupFailure `json:"failures"`
	StartedAt     time.Time                      `json:"started_at"`
	FinishedAt    *time.Time                     `json:"finished_at,omitempty"`
}

// SessionHistoryCleanupPreviewResponse answers a dry run.
type SessionHistoryCleanupPreviewResponse struct {
	Plan SessionHistoryCleanupPlan `json:"plan"`
}

// SessionHistoryCleanupStartResponse answers an accepted cleanup with its plan.
type SessionHistoryCleanupStartResponse struct {
	Plan SessionHistoryCleanupPlan `json:"plan"`
	Run  SessionHistoryCleanupRun  `json:"run"`
}

// SessionHistoryCleanupRunResponse carries the profile's running or most
// recent cleanup run; run is null when the daemon has run none since it started.
type SessionHistoryCleanupRunResponse struct {
	Run *SessionHistoryCleanupRun `json:"run"`
}
