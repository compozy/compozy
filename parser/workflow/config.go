package workflow

import (
	"os"
	"path/filepath"

	"dario.cat/mergo"
	"gopkg.in/yaml.v3"

	"github.com/compozy/compozy/parser/agent"
	"github.com/compozy/compozy/parser/author"
	"github.com/compozy/compozy/parser/common"
	"github.com/compozy/compozy/parser/package_ref"
	"github.com/compozy/compozy/parser/task"
	"github.com/compozy/compozy/parser/tool"
	"github.com/compozy/compozy/parser/trigger"
)

// TestMode is used to skip file existence checks during testing
var TestMode bool

// WorkflowError represents errors that can occur during workflow configuration
type WorkflowError struct {
	Message string
	Code    string
}

func (e *WorkflowError) Error() string {
	return e.Message
}

// WorkflowConfig represents a workflow configuration
type WorkflowConfig struct {
	ID          WorkflowID            `json:"id" yaml:"id"`
	Tasks       []task.TaskConfig     `json:"tasks" yaml:"tasks"`
	Trigger     trigger.TriggerConfig `json:"trigger" yaml:"trigger"`
	Version     *WorkflowVersion      `json:"version,omitempty" yaml:"version,omitempty"`
	Description *WorkflowDescription  `json:"description,omitempty" yaml:"description,omitempty"`
	Author      *author.Author        `json:"author,omitempty" yaml:"author,omitempty"`
	Tools       []tool.ToolConfig     `json:"tools,omitempty" yaml:"tools,omitempty"`
	Agents      []agent.AgentConfig   `json:"agents,omitempty" yaml:"agents,omitempty"`
	Env         common.EnvMap         `json:"env,omitempty" yaml:"env,omitempty"`

	cwd *common.CWD // internal field for current working directory
}

// SetCWD sets the current working directory for the workflow
func (w *WorkflowConfig) SetCWD(path string) {
	if w.cwd == nil {
		w.cwd = common.NewCWD(path)
	} else {
		w.cwd.Set(path)
	}
}

// GetCWD returns the current working directory
func (w *WorkflowConfig) GetCWD() string {
	if w.cwd == nil {
		return ""
	}
	return w.cwd.Get()
}

// Load loads a workflow configuration from a file
func Load(path string) (*WorkflowConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, &WorkflowError{
			Message: "Failed to open workflow config file: " + err.Error(),
			Code:    "FILE_OPEN_ERROR",
		}
	}
	defer file.Close()

	var config WorkflowConfig
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, &WorkflowError{
			Message: "Failed to decode workflow config: " + err.Error(),
			Code:    "DECODE_ERROR",
		}
	}

	config.SetCWD(filepath.Dir(path))

	// Set CWD for all components
	for i := range config.Tasks {
		config.Tasks[i].SetCWD(config.GetCWD())
	}
	for i := range config.Tools {
		config.Tools[i].SetCWD(config.GetCWD())
	}
	for i := range config.Agents {
		config.Agents[i].SetCWD(config.GetCWD())
	}

	return &config, nil
}

// AgentByRef finds an agent configuration by its package reference
func (w *WorkflowConfig) AgentByRef(ref *package_ref.PackageRef) (*agent.AgentConfig, error) {
	if !ref.Component.IsAgent() {
		return nil, &WorkflowError{
			Message: "Invalid component for agent reference",
			Code:    "INVALID_COMPONENT",
		}
	}

	switch ref.Type.Type {
	case "id":
		if w.Agents == nil {
			return nil, &WorkflowError{
				Message: "There's no agents defined in your workflow",
				Code:    "NO_AGENTS_DEFINED",
			}
		}

		agentID := agent.AgentID(ref.Type.Value)
		for _, a := range w.Agents {
			if a.ID != nil && *a.ID == agentID {
				a.SetCWD(w.cwd.Get())
				return &a, nil
			}
		}

		return nil, &WorkflowError{
			Message: "Agent not found with reference: " + ref.Type.Value,
			Code:    "AGENT_NOT_FOUND",
		}

	case "file":
		return agent.Load(w.cwd.Join(ref.Type.Value))

	case "dep":
		return nil, &WorkflowError{
			Message: "Not implemented yet",
			Code:    "NOT_IMPLEMENTED",
		}

	default:
		return nil, &WorkflowError{
			Message: "Invalid reference type for agent",
			Code:    "INVALID_REF_TYPE",
		}
	}
}

// ToolByRef finds a tool configuration by its package reference
func (w *WorkflowConfig) ToolByRef(ref *package_ref.PackageRef) (*tool.ToolConfig, error) {
	if !ref.Component.IsTool() {
		return nil, &WorkflowError{
			Message: "Invalid component for tool reference",
			Code:    "INVALID_COMPONENT",
		}
	}

	switch ref.Type.Type {
	case "id":
		if w.Tools == nil {
			return nil, &WorkflowError{
				Message: "There's no tools defined in your workflow",
				Code:    "NO_TOOLS_DEFINED",
			}
		}

		toolID := tool.ToolID(ref.Type.Value)
		for _, t := range w.Tools {
			if t.ID != nil && *t.ID == toolID {
				t.SetCWD(w.cwd.Get())
				return &t, nil
			}
		}

		return nil, &WorkflowError{
			Message: "Tool not found with reference: " + ref.Type.Value,
			Code:    "TOOL_NOT_FOUND",
		}

	case "file":
		return tool.Load(w.cwd.Join(ref.Type.Value))

	case "dep":
		return nil, &WorkflowError{
			Message: "Not implemented yet",
			Code:    "NOT_IMPLEMENTED",
		}

	default:
		return nil, &WorkflowError{
			Message: "Invalid reference type for tool",
			Code:    "INVALID_REF_TYPE",
		}
	}
}

// TaskByRef finds a task configuration by its package reference
func (w *WorkflowConfig) TaskByRef(ref *package_ref.PackageRef) (*task.TaskConfig, error) {
	if !ref.Component.IsTask() {
		return nil, &WorkflowError{
			Message: "Invalid component for task reference",
			Code:    "INVALID_COMPONENT",
		}
	}

	switch ref.Type.Type {
	case "id":
		taskID := task.TaskID(ref.Type.Value)
		for _, t := range w.Tasks {
			if t.ID != nil && *t.ID == taskID {
				t.SetCWD(w.cwd.Get())
				return &t, nil
			}
		}

		return nil, &WorkflowError{
			Message: "Task not found with reference: " + ref.Type.Value,
			Code:    "TASK_NOT_FOUND",
		}

	case "file":
		return task.Load(w.cwd.Join(ref.Type.Value))

	case "dep":
		return nil, &WorkflowError{
			Message: "Not implemented yet",
			Code:    "NOT_IMPLEMENTED",
		}

	default:
		return nil, &WorkflowError{
			Message: "Invalid reference type for task",
			Code:    "INVALID_REF_TYPE",
		}
	}
}

// Validate validates the workflow configuration
func (w *WorkflowConfig) Validate() error {
	if w.cwd == nil || w.cwd.Get() == "" {
		return &WorkflowError{
			Message: "Missing file path for workflow",
			Code:    "MISSING_FILE_PATH",
		}
	}

	// Validate tasks
	for _, t := range w.Tasks {
		if !TestMode {
			t.SetCWD(w.cwd.Get())
		}
		if err := t.Validate(); err != nil {
			return err
		}
	}

	// Validate tools
	for _, t := range w.Tools {
		if !TestMode {
			t.SetCWD(w.cwd.Get())
		}
		if err := t.Validate(); err != nil {
			return err
		}
	}

	// Validate agents
	for _, a := range w.Agents {
		if !TestMode {
			a.SetCWD(w.cwd.Get())
		}
		if err := a.Validate(); err != nil {
			return &WorkflowError{
				Message: err.Error(),
				Code:    "AGENT_VALIDATION_ERROR",
			}
		}
	}

	// Validate trigger
	if err := w.Trigger.Validate(); err != nil {
		return err
	}

	return nil
}

// Merge merges another workflow configuration into this one
func (w *WorkflowConfig) Merge(other *WorkflowConfig) error {
	// Use mergo to deep merge the configs
	if err := mergo.Merge(w, other, mergo.WithOverride); err != nil {
		return &WorkflowError{
			Message: "Failed to merge workflow configs: " + err.Error(),
			Code:    "MERGE_ERROR",
		}
	}
	return nil
}
