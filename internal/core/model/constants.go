package model

import "time"

const (
	UnknownFileName        = "unknown"
	IDECodex               = "codex"
	IDEClaude              = "claude"
	IDEDroid               = "droid"
	IDECursor              = "cursor-agent"
	IDEOpenCode            = "opencode"
	IDEPi                  = "pi"
	IDEOMP                 = "omp"
	IDEGemini              = "gemini"
	IDECopilot             = "copilot"
	IDEKiro                = "kiro"
	IDEDevin               = "devin"
	DefaultCodexModel      = "gpt-5.6-sol"
	DefaultClaudeModel     = "opus"
	DefaultCursorModel     = "composer-1"
	DefaultOpenCodeModel   = "anthropic/claude-opus-4-6"
	DefaultPiModel         = "anthropic/claude-opus-4-6"
	DefaultOMPModel        = "auto"
	DefaultGeminiModel     = "gemini-2.5-pro"
	DefaultCopilotModel    = "claude-sonnet-4.6"
	DefaultKiroModel       = "claude-opus-4.6"
	DefaultDevinModel      = "anthropic/claude-opus-4-6"
	DefaultActivityTimeout = 10 * time.Minute
	// DefaultStallIdleTimeout is the default per-attempt idle window; any session
	// update resets it. Applied by RuntimeConfig.ApplyDefaults.
	DefaultStallIdleTimeout = 3 * time.Minute
	// DefaultStallChildTimeout is the default daemon per-child backstop budget.
	// It must stay strictly greater than DefaultStallIdleTimeout so the fast
	// in-attempt watchdog gets the first chance to self-heal.
	DefaultStallChildTimeout = 6 * time.Minute
	// DefaultStallTerminalCap is the default absolute per-command wall-clock cap;
	// a generous last-resort backstop for a runaway terminal command.
	DefaultStallTerminalCap = 45 * time.Minute
	// DefaultStallRetries is the default number of clean-state stall retries
	// performed before a job is parked.
	DefaultStallRetries      = 1
	WorkflowRootDirName      = ".compozy"
	WorkflowConfigFileName   = "config.toml"
	WorkflowTasksDirName     = "tasks"
	WorkflowRunsDirName      = "runs"
	ArchivedWorkflowDirName  = "_archived"
	ModeCodeReview           = "pr-review"
	ModePRDTasks             = "prd-tasks"
	ModeExec                 = "exec"
	AccessModeDefault        = "default"
	AccessModeFull           = "full"
	OutputFormatTextValue    = "text"
	OutputFormatJSONValue    = "json"
	OutputFormatRawJSONValue = "raw-json"
)

type ExecutionMode string

const (
	ExecutionModePRReview ExecutionMode = ModeCodeReview
	ExecutionModePRDTasks ExecutionMode = ModePRDTasks
	ExecutionModeExec     ExecutionMode = ModeExec
)

type OutputFormat string

const (
	OutputFormatText    OutputFormat = OutputFormatTextValue
	OutputFormatJSON    OutputFormat = OutputFormatJSONValue
	OutputFormatRawJSON OutputFormat = OutputFormatRawJSONValue
)
