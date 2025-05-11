package workflow

import (
	"errors"
	"fmt"
	"os"

	"dario.cat/mergo"

	"github.com/compozy/compozy/internal/parser/agent"
	"github.com/compozy/compozy/internal/parser/author"
	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/task"
	"github.com/compozy/compozy/internal/parser/tool"
	"github.com/compozy/compozy/internal/parser/trigger"
	"github.com/compozy/compozy/internal/parser/validator"
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

func (w *WorkflowConfig) Component() common.ComponentType {
	return common.ComponentWorkflow
}

// SetCWD sets the current working directory for the workflow
func (w *WorkflowConfig) SetCWD(path string) error {
	normalizedPath, err := common.CWDFromPath(path)
	if err != nil {
		return fmt.Errorf("failed to normalize path: %w", err)
	}
	w.cwd = normalizedPath
	setComponentsCWD(w, path)
	return nil
}

// GetCWD returns the current working directory
func (w *WorkflowConfig) GetCWD() string {
	if w.cwd == nil {
		return ""
	}
	return w.cwd.Get()
}

func Load(path string) (*WorkflowConfig, error) {
	config, err := common.LoadConfig[*WorkflowConfig](path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, NewFileOpenError(err)
		}
		return nil, NewDecodeError(err)
	}

	setComponentsCWD(config, config.GetCWD())
	return config, nil
}

func (w *WorkflowConfig) Validate() error {
	v := validator.NewCompositeValidator(
		validator.NewCWDValidator(w.cwd, string(w.ID)),
		NewTriggerValidator(*w),
	)
	if err := v.Validate(); err != nil {
		return err
	}

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

	return nil
}

func (w *WorkflowConfig) ValidateParams(input map[string]any) error {
	inputSchema := w.Trigger.InputSchema
	return validator.NewParamsValidator(input, inputSchema.Schema, w.ID).Validate()
}

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

func setComponentsCWD(config *WorkflowConfig, cwd string) {
	for i := range config.Tasks {
		config.Tasks[i].SetCWD(cwd)
	}
	for i := range config.Tools {
		config.Tools[i].SetCWD(cwd)
	}
	for i := range config.Agents {
		config.Agents[i].SetCWD(cwd)
	}
}
