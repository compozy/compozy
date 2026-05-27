package kinds

import "time"

// TaskFileUpdatedPayload describes a rewritten task file.
type TaskFileUpdatedPayload struct {
	TasksDir  string `json:"tasks_dir"`
	TaskName  string `json:"task_name"`
	FilePath  string `json:"file_path"`
	OldStatus string `json:"old_status,omitempty"`
	NewStatus string `json:"new_status,omitempty"`
}

// TaskFileSkippedReason categorizes why a task completion was suppressed.
type TaskFileSkippedReason string

const (
	// TaskFileSkippedReasonNoWorkspaceChanges is emitted when the agent
	// session ended cleanly but did not modify any file in the workspace.
	// The task file is left at its prior status and will be re-dispatched
	// on the next run.
	TaskFileSkippedReasonNoWorkspaceChanges TaskFileSkippedReason = "no_workspace_changes"
)

// TaskFileSkippedPayload describes a task completion that was deliberately
// suppressed because no positive evidence of progress was observed.
type TaskFileSkippedPayload struct {
	TasksDir        string                `json:"tasks_dir"`
	TaskName        string                `json:"task_name"`
	FilePath        string                `json:"file_path"`
	PreservedStatus string                `json:"preserved_status,omitempty"`
	Reason          TaskFileSkippedReason `json:"reason"`
}

// TaskMetadataRefreshedPayload describes refreshed task workflow metadata.
type TaskMetadataRefreshedPayload struct {
	TasksDir  string    `json:"tasks_dir"`
	CreatedAt time.Time `json:"created_at,omitzero"`
	UpdatedAt time.Time `json:"updated_at,omitzero"`
	Total     int       `json:"total,omitempty"`
	Completed int       `json:"completed,omitempty"`
	Pending   int       `json:"pending,omitempty"`
}

// TaskRunMultiplePayload describes daemon-owned multi-task queue lifecycle events.
type TaskRunMultiplePayload struct {
	RunID      string   `json:"run_id,omitempty"`
	Mode       string   `json:"mode,omitempty"`
	Slug       string   `json:"slug,omitempty"`
	Slugs      []string `json:"slugs,omitempty"`
	Index      int      `json:"index,omitempty"`
	Total      int      `json:"total,omitempty"`
	Status     string   `json:"status,omitempty"`
	ChildRunID string   `json:"child_run_id,omitempty"`
	Error      string   `json:"error,omitempty"`
}
