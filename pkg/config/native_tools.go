package config

import "time"

const (
	DefaultCallAgentsMaxConcurrent    = 10
	DefaultCallTasksMaxConcurrent     = 10
	DefaultCallWorkflowsMaxConcurrent = 10
)

// NativeToolsConfig controls cp__ builtin enablement and sandbox settings.
type NativeToolsConfig struct {
	Enabled         bool              `koanf:"enabled"          json:"enabled"          yaml:"enabled"          mapstructure:"enabled"`
	RootDir         string            `koanf:"root_dir"         json:"root_dir"         yaml:"root_dir"         mapstructure:"root_dir"`
	AdditionalRoots []string          `koanf:"additional_roots" json:"additional_roots" yaml:"additional_roots" mapstructure:"additional_roots"`
	Exec            NativeExecConfig  `koanf:"exec"             json:"exec"             yaml:"exec"             mapstructure:"exec"`
	Fetch           NativeFetchConfig `koanf:"fetch"            json:"fetch"            yaml:"fetch"            mapstructure:"fetch"`
	// CallAgent configures single agent execution through cp__call_agent.
	CallAgent NativeCallAgentConfig `koanf:"call_agent"       json:"call_agent"       yaml:"call_agent"       mapstructure:"call_agent"`
	// CallAgents governs multi-agent orchestration for cp__call_agents.
	CallAgents NativeCallAgentsConfig `koanf:"call_agents"      json:"call_agents"      yaml:"call_agents"      mapstructure:"call_agents"`
	// CallTask configures single task execution through cp__call_task.
	CallTask NativeCallTaskConfig `koanf:"call_task"        json:"call_task"        yaml:"call_task"        mapstructure:"call_task"`
	// CallTasks governs parallel task execution for cp__call_tasks.
	CallTasks NativeCallTasksConfig `koanf:"call_tasks"       json:"call_tasks"       yaml:"call_tasks"       mapstructure:"call_tasks"`
	// CallWorkflow configures single workflow execution via cp__call_workflow.
	CallWorkflow NativeCallWorkflowConfig `koanf:"call_workflow"    json:"call_workflow"    yaml:"call_workflow"    mapstructure:"call_workflow"`
	// CallWorkflows governs parallel workflow execution via cp__call_workflows.
	CallWorkflows NativeCallWorkflowsConfig `koanf:"call_workflows"   json:"call_workflows"   yaml:"call_workflows"   mapstructure:"call_workflows"`
}

// NativeExecConfig holds cp__exec configuration knobs.
type NativeExecConfig struct {
	Timeout        time.Duration             `koanf:"timeout"          json:"timeout"          yaml:"timeout"          mapstructure:"timeout"`
	MaxStdoutBytes int64                     `koanf:"max_stdout_bytes" json:"max_stdout_bytes" yaml:"max_stdout_bytes" mapstructure:"max_stdout_bytes"`
	MaxStderrBytes int64                     `koanf:"max_stderr_bytes" json:"max_stderr_bytes" yaml:"max_stderr_bytes" mapstructure:"max_stderr_bytes"`
	Allowlist      []NativeExecCommandConfig `koanf:"allowlist"        json:"allowlist"        yaml:"allowlist"        mapstructure:"allowlist"`
}

// NativeExecCommandConfig defines per-command execution policies.
type NativeExecCommandConfig struct {
	Path            string                     `koanf:"path"             json:"path"             yaml:"path"             mapstructure:"path"`
	Description     string                     `koanf:"description"      json:"description"      yaml:"description"      mapstructure:"description"`
	Timeout         time.Duration              `koanf:"timeout"          json:"timeout"          yaml:"timeout"          mapstructure:"timeout"`
	MaxArgs         int                        `koanf:"max_args"         json:"max_args"         yaml:"max_args"         mapstructure:"max_args"`
	AllowAdditional bool                       `koanf:"allow_additional" json:"allow_additional" yaml:"allow_additional" mapstructure:"allow_additional"`
	Arguments       []NativeExecArgumentConfig `koanf:"arguments"        json:"arguments"        yaml:"arguments"        mapstructure:"arguments"`
}

// NativeExecArgumentConfig enforces validation for a single argument position.
type NativeExecArgumentConfig struct {
	Index    int      `koanf:"index"    json:"index"    yaml:"index"    mapstructure:"index"`
	Pattern  string   `koanf:"pattern"  json:"pattern"  yaml:"pattern"  mapstructure:"pattern"`
	Enum     []string `koanf:"enum"     json:"enum"     yaml:"enum"     mapstructure:"enum"`
	Optional bool     `koanf:"optional" json:"optional" yaml:"optional" mapstructure:"optional"`
}

// NativeFetchConfig holds cp__fetch configuration knobs.
type NativeFetchConfig struct {
	Timeout        time.Duration `koanf:"timeout"         json:"timeout"         yaml:"timeout"         mapstructure:"timeout"`
	MaxBodyBytes   int64         `koanf:"max_body_bytes"  json:"max_body_bytes"  yaml:"max_body_bytes"  mapstructure:"max_body_bytes"`
	MaxRedirects   int           `koanf:"max_redirects"   json:"max_redirects"   yaml:"max_redirects"   mapstructure:"max_redirects"`
	AllowedMethods []string      `koanf:"allowed_methods" json:"allowed_methods" yaml:"allowed_methods" mapstructure:"allowed_methods"`
}

// NativeCallAgentConfig configures cp__call_agent behavior.
type NativeCallAgentConfig struct {
	Enabled        bool          `koanf:"enabled"         json:"enabled"         yaml:"enabled"         mapstructure:"enabled"`
	DefaultTimeout time.Duration `koanf:"default_timeout" json:"default_timeout" yaml:"default_timeout" mapstructure:"default_timeout"`
}

// NativeCallAgentsConfig configures cp__call_agents behavior.
// MaxConcurrent values of 0 or less fall back to sequential execution.
type NativeCallAgentsConfig struct {
	Enabled        bool          `koanf:"enabled"         json:"enabled"         yaml:"enabled"         mapstructure:"enabled"`
	DefaultTimeout time.Duration `koanf:"default_timeout" json:"default_timeout" yaml:"default_timeout" mapstructure:"default_timeout"`
	// MaxConcurrent limits concurrent agent executions; 0 selects sequential execution, negative values are invalid.
	MaxConcurrent int `koanf:"max_concurrent"  json:"max_concurrent"  yaml:"max_concurrent"  mapstructure:"max_concurrent"  validate:"min=0"`
}

// NativeCallTaskConfig configures cp__call_task behavior.
type NativeCallTaskConfig struct {
	Enabled        bool          `koanf:"enabled"         json:"enabled"         yaml:"enabled"         mapstructure:"enabled"`
	DefaultTimeout time.Duration `koanf:"default_timeout" json:"default_timeout" yaml:"default_timeout" mapstructure:"default_timeout"`
}

// NativeCallTasksConfig configures cp__call_tasks behavior.
// MaxConcurrent values of 0 or less fall back to sequential execution.
type NativeCallTasksConfig struct {
	Enabled        bool          `koanf:"enabled"         json:"enabled"         yaml:"enabled"         mapstructure:"enabled"`
	DefaultTimeout time.Duration `koanf:"default_timeout" json:"default_timeout" yaml:"default_timeout" mapstructure:"default_timeout"`
	MaxConcurrent  int           `koanf:"max_concurrent"  json:"max_concurrent"  yaml:"max_concurrent"  mapstructure:"max_concurrent"  validate:"min=0"`
}

// NativeCallWorkflowConfig configures cp__call_workflow behavior.
type NativeCallWorkflowConfig struct {
	Enabled        bool          `koanf:"enabled"         json:"enabled"         yaml:"enabled"         mapstructure:"enabled"`
	DefaultTimeout time.Duration `koanf:"default_timeout" json:"default_timeout" yaml:"default_timeout" mapstructure:"default_timeout"`
}

// NativeCallWorkflowsConfig configures cp__call_workflows behavior.
// MaxConcurrent values of 0 or less fall back to sequential execution.
type NativeCallWorkflowsConfig struct {
	Enabled        bool          `koanf:"enabled"         json:"enabled"         yaml:"enabled"         mapstructure:"enabled"`
	DefaultTimeout time.Duration `koanf:"default_timeout" json:"default_timeout" yaml:"default_timeout" mapstructure:"default_timeout"`
	MaxConcurrent  int           `koanf:"max_concurrent"  json:"max_concurrent"  yaml:"max_concurrent"  mapstructure:"max_concurrent"  validate:"min=0"`
}

// DefaultNativeToolsConfig returns safe defaults for native tool execution.
func DefaultNativeToolsConfig() NativeToolsConfig {
	return NativeToolsConfig{
		Enabled:         true,
		RootDir:         ".",
		AdditionalRoots: nil,
		Exec: NativeExecConfig{
			Timeout:        30 * time.Second,
			MaxStdoutBytes: 2 << 20, // 2 MiB
			MaxStderrBytes: 1 << 10, // 1 KiB
			Allowlist:      nil,
		},
		Fetch: NativeFetchConfig{
			Timeout:        5 * time.Second,
			MaxBodyBytes:   2 << 20, // 2 MiB
			MaxRedirects:   5,
			AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD"},
		},
		CallAgent: NativeCallAgentConfig{
			Enabled:        true,
			DefaultTimeout: 60 * time.Second,
		},
		CallAgents: NativeCallAgentsConfig{
			Enabled:        true,
			DefaultTimeout: 60 * time.Second,
			MaxConcurrent:  DefaultCallAgentsMaxConcurrent,
		},
		CallTask: NativeCallTaskConfig{
			Enabled:        true,
			DefaultTimeout: 60 * time.Second,
		},
		CallTasks: NativeCallTasksConfig{
			Enabled:        true,
			DefaultTimeout: 60 * time.Second,
			MaxConcurrent:  DefaultCallTasksMaxConcurrent,
		},
		CallWorkflow: NativeCallWorkflowConfig{
			Enabled:        true,
			DefaultTimeout: 300 * time.Second,
		},
		CallWorkflows: NativeCallWorkflowsConfig{
			Enabled:        true,
			DefaultTimeout: 300 * time.Second,
			MaxConcurrent:  DefaultCallWorkflowsMaxConcurrent,
		},
	}
}
