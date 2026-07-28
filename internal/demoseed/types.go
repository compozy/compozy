// Package demoseed prepares a coherent local workspace for product demonstrations.
package demoseed

import (
	"errors"
	"time"
)

var (
	// ErrScenarioExists reports that the Northstar Pay workspace already exists.
	ErrScenarioExists = errors.New("demo seed: Northstar Pay scenario already exists")
)

const (
	workspaceName        = "Northstar Pay Launch HQ"
	workspaceRelative    = "workspaces/northstar-pay/launch-hq"
	productLeadAgent     = "product-lead"
	providerClaude       = "claude"
	providerCodex        = "codex"
	approveReads         = "approve-reads"
	toolNetworkSend      = "compozy__network_send"
	toolTaskList         = "compozy__task_list"
	toolTaskRead         = "compozy__task_read"
	toolKindRead         = "read"
	taskStatusCompleted  = "completed"
	taskPriorityHigh     = "high"
	taskPriorityUrgent   = "urgent"
	approvalPolicyNone   = "none"
	approvalNotRequired  = "not_required"
	ownerAgentSession    = "agent_session"
	operatorRef          = "operator:pedro"
	taskSupportID        = "task_northstar_support_handoff"
	taskLaunchDecisionID = "task_northstar_launch_decision"
	networkRootMessageID = "msg_northstar_01"
	jsonTextKey          = "text"
	launchChannel        = "launch-war-room"
	launchThreadID       = "thread_checkout_launch"
	launchLoopName       = "launch-readiness"
	launchAutomationID   = "job_northstar_canary_digest"
	launchAutomationRun  = "autorun_northstar_canary_digest"
)

var scenarioSessionIDs = []string{
	"sess_northstar_partner_replay",
	"sess_northstar_compliance_copy",
	"sess_northstar_support_handoff",
	"sess_northstar_launch_decision",
}

// Options controls where and when the scenario is written.
type Options struct {
	HomeDir string
	Replace bool
	Now     time.Time
}

// Counts summarizes the durable records created by a seed run.
type Counts struct {
	Agents           int `json:"agents"`
	Sessions         int `json:"sessions"`
	Tasks            int `json:"tasks"`
	NetworkMessages  int `json:"network_messages"`
	LoopRuns         int `json:"loop_runs"`
	AutomationJobs   int `json:"automation_jobs"`
	AutomationRuns   int `json:"automation_runs"`
	TranscriptEvents int `json:"transcript_events"`
}

// Result identifies the seeded workspace and the shortest useful demo route.
type Result struct {
	HomeDir          string   `json:"home_dir"`
	DatabaseFile     string   `json:"database_file"`
	WorkspaceID      string   `json:"workspace_id"`
	WorkspaceRoot    string   `json:"workspace_root"`
	WorkspaceName    string   `json:"workspace_name"`
	SessionIDs       []string `json:"session_ids"`
	TaskIDs          []string `json:"task_ids"`
	NetworkChannel   string   `json:"network_channel"`
	NetworkThreadID  string   `json:"network_thread_id"`
	LoopName         string   `json:"loop_name"`
	LoopRunID        string   `json:"loop_run_id"`
	AutomationJobID  string   `json:"automation_job_id"`
	SuggestedWebPath []string `json:"suggested_web_path"`
	Counts           Counts   `json:"counts"`
}

type agentStory struct {
	Name        string
	Provider    string
	Permissions string
	Tools       []string
	Prompt      string
}

type sessionStory struct {
	ID         string
	Name       string
	AgentName  string
	Provider   string
	StartedAt  time.Time
	EndedAt    time.Time
	Prompt     string
	Opening    string
	ToolName   string
	ToolKind   string
	ToolInput  string
	ToolResult string
	Conclusion string
	Input      int64
	Output     int64
	CostUSD    float64
}

type taskStory struct {
	ID                string
	Identifier        string
	Title             string
	Description       string
	Priority          string
	Status            string
	ApprovalPolicy    string
	ApprovalState     string
	OwnerKind         string
	OwnerRef          string
	SessionID         string
	RunID             string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	ClosedAt          time.Time
	TokensUsed        int64
	Result            string
	DependencyTaskIDs []string
	BlockReason       string
}

type networkMessageStory struct {
	ID        string
	SessionID string
	ReplyTo   string
	Text      string
	At        time.Time
}
