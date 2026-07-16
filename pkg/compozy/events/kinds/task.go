package kinds

import "time"

// TaskParallelTaskStatus is the terminal status of one task in a parallel run.
type TaskParallelTaskStatus string

const (
	TaskParallelTaskStatusMerged    TaskParallelTaskStatus = "merged"
	TaskParallelTaskStatusRecovered TaskParallelTaskStatus = "recovered"
	TaskParallelTaskStatusFailed    TaskParallelTaskStatus = "failed"
	TaskParallelTaskStatusSkipped   TaskParallelTaskStatus = "skipped"
	TaskParallelTaskStatusCanceled  TaskParallelTaskStatus = "canceled"
)

// IsIntegrated reports whether the task content reached the integration branch.
func (s TaskParallelTaskStatus) IsIntegrated() bool {
	return s == TaskParallelTaskStatusMerged || s == TaskParallelTaskStatusRecovered
}

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
//
// ParallelLimit and the worktree fields are additive and optional. They are only
// populated for parallel-mode runs once a child worktree has been planned or
// allocated, and they stay empty for enqueued runs and for parent events emitted
// before this metadata existed. Snapshot reconstruction must treat any empty
// field as "unknown" so older parent event streams remain compatible.
//
// Completed, Recovered, and Parked are the end-of-run recovery counts and are
// populated only on EventKindTaskRunMultipleSummary.
type TaskRunMultiplePayload struct {
	RunID          string   `json:"run_id,omitempty"`
	Mode           string   `json:"mode,omitempty"`
	Slug           string   `json:"slug,omitempty"`
	Slugs          []string `json:"slugs,omitempty"`
	Index          int      `json:"index,omitempty"`
	Total          int      `json:"total,omitempty"`
	ParallelLimit  int      `json:"parallel_limit,omitempty"`
	Status         string   `json:"status,omitempty"`
	ChildRunID     string   `json:"child_run_id,omitempty"`
	Error          string   `json:"error,omitempty"`
	WorktreePath   string   `json:"worktree_path,omitempty"`
	BaseBranch     string   `json:"base_branch,omitempty"`
	BaseCommit     string   `json:"base_commit,omitempty"`
	WorktreeStatus string   `json:"worktree_status,omitempty"`
	Completed      int      `json:"completed,omitempty"`
	Recovered      int      `json:"recovered,omitempty"`
	Parked         int      `json:"parked,omitempty"`
	WorktreeReason string   `json:"worktree_reason,omitempty"`
	ResultBranch   string   `json:"result_branch,omitempty"`
}

// TaskParallelPlanPayload describes the full task DAG known before a parallel
// task run starts executing waves. It is emitted once so remote UIs can render
// every task and pending wave before the first child run streams output.
type TaskParallelPlanPayload struct {
	RunID             string                 `json:"run_id,omitempty"`
	Workflow          string                 `json:"workflow,omitempty"`
	IntegrationBranch string                 `json:"integration_branch,omitempty"`
	ParallelLimit     int                    `json:"parallel_limit,omitempty"`
	Tasks             []TaskParallelPlanTask `json:"tasks,omitempty"`
	Waves             []TaskParallelPlanWave `json:"waves,omitempty"`
}

type TaskParallelPlanTask struct {
	ID           string   `json:"id,omitempty"`
	Number       int      `json:"number,omitempty"`
	Title        string   `json:"title,omitempty"`
	File         string   `json:"file,omitempty"`
	Status       string   `json:"status,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
	WaveIndex    int      `json:"wave_index,omitempty"`
}

type TaskParallelPlanWave struct {
	Index   int      `json:"index,omitempty"`
	TaskIDs []string `json:"task_ids,omitempty"`
}

// TaskParallelPayload describes one wave/merge/conflict transition emitted by the
// ParallelExecutionOrchestrator during a parallel PRD-tasks run. Wave-level events
// (wave_completed, merge_started, phase_changed, and orchestrator settlements)
// leave task_id empty; per-task events (wave_started, task_started,
// conflict_detected, conflict_resolving, merged, and task_completed) carry
// task_id and the wave it belongs to so the TUI can group sidebar cards by
// wave. task_started also carries child_run_id so remote UIs can attach the
// real child run stream to the task row. All fields are additive and optional;
// snapshot reconstruction must treat any empty field as "unknown".
type TaskParallelPayload struct {
	RunID             string   `json:"run_id,omitempty"`
	ChildRunID        string   `json:"child_run_id,omitempty"`
	WaveIndex         int      `json:"wave_index,omitempty"`
	WaveTotal         int      `json:"wave_total,omitempty"`
	TaskID            string   `json:"task_id,omitempty"`
	Phase             string   `json:"phase,omitempty"`
	IntegrationBranch string   `json:"integration_branch,omitempty"`
	ConflictFiles     []string `json:"conflict_files,omitempty"`
	Attempt           int      `json:"attempt,omitempty"`
	MaxAttempts       int      `json:"max_attempts,omitempty"`
	WorktreePath      string   `json:"worktree_path,omitempty"`
	WorktreeStatus    string   `json:"worktree_status,omitempty"`
	WorktreeReason    string   `json:"worktree_reason,omitempty"`
	ResultBranch      string   `json:"result_branch,omitempty"`
	Status            string   `json:"status,omitempty"`
	Error             string   `json:"error,omitempty"`
}
