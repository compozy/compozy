package workflow

import (
	"errors"
	"os"

	"dario.cat/mergo"

	"github.com/compozy/compozy/internal/parser/agent"
	"github.com/compozy/compozy/internal/parser/author"
	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/schema"
	"github.com/compozy/compozy/internal/parser/task"
	"github.com/compozy/compozy/internal/parser/tool"
	"github.com/compozy/compozy/internal/parser/trigger"
)

// TestMode is used to skip file existence checks during testing
var TestMode bool

// WorkflowConfig represents a workflow configuration
type WorkflowConfig struct {
	ID          string                `json:"id" yaml:"id"`
	Tasks       []task.TaskConfig     `json:"tasks" yaml:"tasks"`
	Trigger     trigger.TriggerConfig `json:"trigger" yaml:"trigger"`
	Version     string                `json:"version,omitempty" yaml:"version,omitempty"`
	Description string                `json:"description,omitempty" yaml:"description,omitempty"`
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
	config, err := common.LoadConfig[*WorkflowConfig](path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, NewFileOpenError(err)
		}
		return nil, NewDecodeError(err)
	}

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

	return config, nil
}

// Validate validates the workflow configuration
func (w *WorkflowConfig) Validate() error {
	// Validate CWD
	validator := common.NewCompositeValidator(
		schema.NewCWDValidator(w.cwd, string(w.ID)),
	)
	if err := validator.Validate(); err != nil {
		return err
	}

	// Validate tasks
	var taskComponents []common.Config
	for i := range w.Tasks {
		w.Tasks[i].SetCWD(w.cwd.Get())
		taskComponents = append(taskComponents, &w.Tasks[i])
	}
	if err := NewComponentsValidator(taskComponents, w.cwd).Validate(); err != nil {
		return err
	}

	// Validate tools
	var toolComponents []common.Config
	for i := range w.Tools {
		w.Tools[i].SetCWD(w.cwd.Get())
		toolComponents = append(toolComponents, &w.Tools[i])
	}
	if err := NewComponentsValidator(toolComponents, w.cwd).Validate(); err != nil {
		return err
	}

	// Validate agents
	var agentComponents []common.Config
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
func (w *WorkflowConfig) Merge(other any) error {
	otherConfig, ok := other.(*WorkflowConfig)
	if !ok {
		return NewMergeError(errors.New("invalid type for merge"))
	}
	return mergo.Merge(w, otherConfig, mergo.WithOverride)
}

// LoadID loads the ID from either the direct ID field or resolves it from a package reference
func (w *WorkflowConfig) LoadID() (string, error) {
	// Workflow configs don't support package references, so just return the ID
	return string(w.ID), nil
}
