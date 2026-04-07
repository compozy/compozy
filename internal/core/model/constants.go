package model

import "time"

const (
	UnknownFileName          = "unknown"
	IDECodex                 = "codex"
	IDEClaude                = "claude"
	IDEDroid                 = "droid"
	IDECursor                = "cursor-agent"
	IDEOpenCode              = "opencode"
	IDEPi                    = "pi"
	IDEGemini                = "gemini"
	IDECopilot               = "copilot"
	DefaultCodexModel        = "gpt-5.4"
	DefaultClaudeModel       = "opus"
	DefaultCursorModel       = "composer-1"
	DefaultOpenCodeModel     = "anthropic/claude-opus-4-6"
	DefaultPiModel           = "anthropic/claude-opus-4-6"
	DefaultGeminiModel       = "gemini-2.5-pro"
	DefaultCopilotModel      = "claude-sonnet-4.6"
	DefaultActivityTimeout   = 10 * time.Minute
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
