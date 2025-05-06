package workflow

import (
	"os"
	"path/filepath"

	"dario.cat/mergo"
	"gopkg.in/yaml.v3"

	"github.com/compozy/compozy/internal/parser/agent"
	"github.com/compozy/compozy/internal/parser/author"
	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/package_ref"
	"github.com/compozy/compozy/internal/parser/task"
	"github.com/compozy/compozy/internal/parser/tool"
	"github.com/compozy/compozy/internal/parser/trigger"
)

// TestMode is used to skip file existence checks during testing
var TestMode bool

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
		return nil, NewFileOpenError(err)
	}
	defer file.Close()

	var config WorkflowConfig
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, NewDecodeError(err)
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
		return nil, NewInvalidComponentError("agent")
	}

	switch ref.Type.Type {
	case "id":
		if w.Agents == nil {
			return nil, NewNoAgentsDefinedError()
		}

		agentID := agent.AgentID(ref.Type.Value)
		for i := range w.Agents {
			if w.Agents[i].ID != nil && *w.Agents[i].ID == agentID {
				agent := w.Agents[i]
				agent.SetCWD(w.cwd.Get())
				return &agent, nil
			}
		}

		return nil, NewAgentNotFoundError(ref.Type.Value)

	case "file":
		return agent.Load(w.cwd.Join(ref.Type.Value))

	case "dep":
		return nil, NewNotImplementedError()

	default:
		return nil, NewInvalidRefTypeError("agent")
	}
}

// ToolByRef finds a tool configuration by its package reference
func (w *WorkflowConfig) ToolByRef(ref *package_ref.PackageRef) (*tool.ToolConfig, error) {
	if !ref.Component.IsTool() {
		return nil, NewInvalidComponentError("tool")
	}

	switch ref.Type.Type {
	case "id":
		if w.Tools == nil {
			return nil, NewNoToolsDefinedError()
		}

		toolID := tool.ToolID(ref.Type.Value)
		for i := range w.Tools {
			if w.Tools[i].ID != nil && *w.Tools[i].ID == toolID {
				tool := w.Tools[i]
				tool.SetCWD(w.cwd.Get())
				return &tool, nil
			}
		}

		return nil, NewToolNotFoundError(ref.Type.Value)

	case "file":
		return tool.Load(w.cwd.Join(ref.Type.Value))

	case "dep":
		return nil, NewNotImplementedError()

	default:
		return nil, NewInvalidRefTypeError("tool")
	}
}

// TaskByRef finds a task configuration by its package reference
func (w *WorkflowConfig) TaskByRef(ref *package_ref.PackageRef) (*task.TaskConfig, error) {
	if !ref.Component.IsTask() {
		return nil, NewInvalidComponentError("task")
	}

	switch ref.Type.Type {
	case "id":
		if w.Tasks == nil {
			return nil, NewNoTasksDefinedError()
		}

		taskID := task.TaskID(ref.Type.Value)
		for i := range w.Tasks {
			if w.Tasks[i].ID != nil && *w.Tasks[i].ID == taskID {
				task := w.Tasks[i]
				task.SetCWD(w.cwd.Get())
				return &task, nil
			}
		}

		return nil, NewTaskNotFoundError(ref.Type.Value)

	case "file":
		return task.Load(w.cwd.Join(ref.Type.Value))

	case "dep":
		return nil, NewNotImplementedError()

	default:
		return nil, NewInvalidRefTypeError("task")
	}
}

func validateComponents(w *WorkflowConfig, components []common.ComponentConfig) error {
	for _, c := range components {
		if !TestMode {
			c.SetCWD(w.cwd.Get())
		}
		if err := c.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func validateTrigger(trigger trigger.TriggerConfig) error {
	if err := trigger.Validate(); err != nil {
		return NewTriggerValidationError(err)
	}
	return nil
}

// Validate validates the workflow configuration
func (w *WorkflowConfig) Validate() error {
	if w.cwd == nil || w.cwd.Get() == "" {
		return NewMissingPathError()
	}

	// Use the helper functions for validation
	var taskComponents []common.ComponentConfig
	for _, t := range w.Tasks {
		taskComponents = append(taskComponents, &t)
	}
	if err := validateComponents(w, taskComponents); err != nil {
		return err
	}
	var toolComponents []common.ComponentConfig
	for _, t := range w.Tools {
		toolComponents = append(toolComponents, &t)
	}
	if err := validateComponents(w, toolComponents); err != nil {
		return err
	}
	var agentComponents []common.ComponentConfig
	for _, a := range w.Agents {
		agentComponents = append(agentComponents, &a)
	}
	if err := validateComponents(w, agentComponents); err != nil {
		return err
	}
	if err := validateTrigger(w.Trigger); err != nil {
		return err
	}

	return nil
}

// Merge merges another workflow configuration into this one
func (w *WorkflowConfig) Merge(other *WorkflowConfig) error {
	// Use mergo to deep merge the configs
	if err := mergo.Merge(w, other, mergo.WithOverride); err != nil {
		return NewMergeError(err)
	}
	return nil
}
