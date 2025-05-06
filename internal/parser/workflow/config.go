package workflow

import (
	"os"
	"path/filepath"

	"dario.cat/mergo"
	"gopkg.in/yaml.v3"

	"github.com/compozy/compozy/internal/parser/agent"
	"github.com/compozy/compozy/internal/parser/author"
	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/pkgref"
	"github.com/compozy/compozy/internal/parser/task"
	"github.com/compozy/compozy/internal/parser/tool"
	"github.com/compozy/compozy/internal/parser/trigger"
	v "github.com/compozy/compozy/internal/parser/validator"
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

	var config WorkflowConfig
	decoder := yaml.NewDecoder(file)
	decodeErr := decoder.Decode(&config)
	closeErr := file.Close()

	if decodeErr != nil {
		return nil, NewDecodeError(decodeErr)
	}
	if closeErr != nil {
		return nil, NewFileCloseError(closeErr)
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
func (w *WorkflowConfig) AgentByRef(ref *pkgref.PackageRef) (*agent.AgentConfig, error) {
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
func (w *WorkflowConfig) ToolByRef(ref *pkgref.PackageRef) (*tool.ToolConfig, error) {
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
func (w *WorkflowConfig) TaskByRef(ref *pkgref.PackageRef) (*task.TaskConfig, error) {
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

// Validate validates the workflow configuration
func (w *WorkflowConfig) Validate() error {
	// Validate CWD
	validator := common.NewCompositeValidator(
		v.NewCWDValidator(w.cwd, string(w.ID)),
	)
	if err := validator.Validate(); err != nil {
		return err
	}

	// Validate tasks
	var taskComponents []common.ComponentConfig
	for i := range w.Tasks {
		w.Tasks[i].SetCWD(w.cwd.Get())
		taskComponents = append(taskComponents, &w.Tasks[i])
	}
	if err := NewComponentsValidator(taskComponents, w.cwd).Validate(); err != nil {
		return err
	}

	// Validate tools
	var toolComponents []common.ComponentConfig
	for i := range w.Tools {
		w.Tools[i].SetCWD(w.cwd.Get())
		toolComponents = append(toolComponents, &w.Tools[i])
	}
	if err := NewComponentsValidator(toolComponents, w.cwd).Validate(); err != nil {
		return err
	}

	// Validate agents
	var agentComponents []common.ComponentConfig
	for i := range w.Agents {
		w.Agents[i].SetCWD(w.cwd.Get())
		agentComponents = append(agentComponents, &w.Agents[i])
	}
	if err := NewComponentsValidator(agentComponents, w.cwd).Validate(); err != nil {
		return err
	}

	// Validate trigger
	return NewTriggerValidator(w.Trigger).Validate()
}

// Merge merges another workflow configuration into this one
func (w *WorkflowConfig) Merge(other *WorkflowConfig) error {
	// Use mergo to deep merge the configs
	if err := mergo.Merge(w, other, mergo.WithOverride); err != nil {
		return NewMergeError(err)
	}
	return nil
}
