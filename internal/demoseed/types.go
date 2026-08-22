// Package demoseed prepares a coherent local workspace for product demonstrations.
package demoseed

import (
	"errors"
	"time"
)

// ErrScenarioExists reports that a Northstar Pay workspace already exists.
var ErrScenarioExists = errors.New("demo seed: Northstar Pay scenario already exists")

// Options controls where and when the scenario is written.
type Options struct {
	HomeDir string
	Replace bool
	Now     time.Time
}

// Counts summarizes the durable records created by a seed run.
type Counts struct {
	Workspaces          int `json:"workspaces"`
	Agents              int `json:"agents"`
	Sessions            int `json:"sessions"`
	Tasks               int `json:"tasks"`
	TaskRuns            int `json:"task_runs"`
	NetworkMessages     int `json:"network_messages"`
	LoopDefinitions     int `json:"loop_definitions"`
	LoopRuns            int `json:"loop_runs"`
	LoopGenerations     int `json:"loop_generations"`
	LoopRunEvents       int `json:"loop_run_events"`
	GoalTurns           int `json:"goal_turns"`
	Memories            int `json:"memories"`
	EventSummaries      int `json:"event_summaries"`
	TokenUsageDays      int `json:"token_usage_days"`
	Worktrees           int `json:"worktrees"`
	NotificationPresets int `json:"notification_presets"`
	AutomationJobs      int `json:"automation_jobs"`
	AutomationRuns      int `json:"automation_runs"`
	TranscriptEvents    int `json:"transcript_events"`
}

// Result identifies the seeded workspaces and the shortest useful demo routes.
type Result struct {
	HomeDir          string   `json:"home_dir"`
	DatabaseFile     string   `json:"database_file"`
	WorkspaceID      string   `json:"workspace_id"`
	WorkspaceRoot    string   `json:"workspace_root"`
	WorkspaceName    string   `json:"workspace_name"`
	WorkspaceIDs     []string `json:"workspace_ids"`
	SessionIDs       []string `json:"session_ids"`
	TaskIDs          []string `json:"task_ids"`
	NetworkChannel   string   `json:"network_channel"`
	NetworkThreadID  string   `json:"network_thread_id"`
	LoopName         string   `json:"loop_name"`
	LoopNames        []string `json:"loop_names"`
	LoopRunID        string   `json:"loop_run_id"`
	LoopRunIDs       []string `json:"loop_run_ids"`
	AutomationJobID  string   `json:"automation_job_id"`
	SuggestedWebPath []string `json:"suggested_web_path"`
	Counts           Counts   `json:"counts"`
}
