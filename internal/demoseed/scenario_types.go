package demoseed

import (
	"time"

	"github.com/compozy/compozy/internal/task"
)

type workspaceStory struct {
	Key          string
	Name         string
	Relative     string
	DefaultAgent string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type agentStory struct {
	Name         string
	WorkspaceKey string
	Provider     string
	Model        string
	Permissions  string
	Tools        []string
	CategoryPath []string
	Prompt       string
	// Always-on identity: a face, a description, and the check-in policy beside AGENT.md.
	DisplayName string
	Description string
	Kind        string
	Appearance  map[string]any
	Heartbeat   string
	// Scripts are check-in source scripts written to the agent's sources folder.
	Scripts map[string]string
}

type transcriptStepKind string

const (
	stepUser     transcriptStepKind = "user"
	stepThinking transcriptStepKind = "thinking"
	stepAgent    transcriptStepKind = "agent"
	stepTool     transcriptStepKind = "tool"
)

// transcriptStep is one authored beat of a session transcript. A stepTool beat
// expands into the paired tool_call and tool_result events the renderer needs.
type transcriptStep struct {
	Kind       transcriptStepKind
	Text       string
	ToolName   string
	ToolKind   string
	ToolInput  string
	ToolResult string
}

type sessionFailureStory struct {
	Kind    string
	Summary string
}

type sessionStory struct {
	ID           string
	Name         string
	WorkspaceKey string
	AgentName    string
	Provider     string
	Model        string
	SessionType  string
	ParentID     string
	SpawnRole    string
	// InitiatorKind/InitiatorID name the non-operator actor that started the session, such as an automation.
	InitiatorKind string
	InitiatorID   string
	StartedAt     time.Time
	EndedAt       time.Time
	StopReason    string
	StopDetail    string
	Failure       *sessionFailureStory
	Steps         []transcriptStep
	Input         int64
	Output        int64
	CostUSD       float64
}

type taskStory struct {
	ID                string
	WorkspaceKey      string
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
	Initiative        string
	DependencyTaskIDs []string
	BlockReason       string
	BlockDetails      string
	History           []taskRunStory
}

// taskRunStory is one closed historical run behind a task, used to give the
// 14-day outcomes chart real shape.
type taskRunStory struct {
	ID         string
	Attempt    int32
	Status     task.RunStatus
	SessionID  string
	StartedAt  time.Time
	EndedAt    time.Time
	TokensUsed int64
	Result     string
	Error      string
}

type eventSummaryStory struct {
	ID           string
	WorkspaceKey string
	SessionID    string
	AgentName    string
	Type         string
	Summary      string
	Provider     string
	Outcome      string
	HookEvent    string
	HookName     string
	At           time.Time
}

type tokenUsageStory struct {
	DaysBack     int
	WorkspaceKey string
	AgentName    string
	Input        int64
	Output       int64
	CostUSD      float64
}
