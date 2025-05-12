package workflow

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"dario.cat/mergo"

	"github.com/compozy/compozy/internal/parser/agent"
	"github.com/compozy/compozy/internal/parser/author"
	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/task"
	"github.com/compozy/compozy/internal/parser/tool"
	"github.com/compozy/compozy/internal/parser/trigger"
	"github.com/compozy/compozy/internal/parser/validator"
)

var TestMode bool

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

	cwd *common.CWD
}

func (w *WorkflowConfig) Component() common.ComponentType {
	return common.ComponentWorkflow
}

func (w *WorkflowConfig) SetCWD(path string) error {
	normalizedPath, err := common.CWDFromPath(path)
	if err != nil {
		return fmt.Errorf("failed to normalize path: %w", err)
	}
	w.cwd = normalizedPath
	setComponentsCWD(w, path)
	return nil
}

func (w *WorkflowConfig) GetCWD() string {
	if w.cwd == nil {
		return ""
	}
	return w.cwd.Get()
}

func Load(cwd *common.CWD, path string) (*WorkflowConfig, error) {
	config, err := common.LoadConfig[*WorkflowConfig](cwd, path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to open workflow config file: %w", err)
		}
		return nil, fmt.Errorf("failed to decode workflow config: %w", err)
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

	var toolComponents []common.Config
	for i := range w.Tools {
		w.Tools[i].SetCWD(w.cwd.Get())
		toolComponents = append(toolComponents, &w.Tools[i])
	}
	if err := NewComponentsValidator(toolComponents, w.cwd).Validate(); err != nil {
		return err
	}

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
		return fmt.Errorf("failed to merge workflow configs: %w", errors.New("invalid type for merge"))
	}
	return mergo.Merge(w, otherConfig, mergo.WithOverride)
}

func (w *WorkflowConfig) LoadID() (string, error) {
	return string(w.ID), nil
}

func setComponentsCWD(config *WorkflowConfig, cwd string) {
	workflowCWD := cwd

	for i := range config.Tasks {
		// If the task has a Use reference, check its type
		if config.Tasks[i].Use != nil {
			ref, err := config.Tasks[i].Use.IntoRef()
			if err == nil && ref.Type.Type == "file" {
				// Get the directory containing the referenced file
				taskPath := filepath.Join(workflowCWD, ref.Type.Value)
				taskDir := filepath.Dir(taskPath)
				config.Tasks[i].SetCWD(taskDir)
				continue
			}
		}
		// Otherwise, inherit the workflow's CWD
		config.Tasks[i].SetCWD(workflowCWD)
	}

	for i := range config.Tools {
		// If the tool has a Use reference, check its type
		if config.Tools[i].Use != nil {
			ref, err := config.Tools[i].Use.IntoRef()
			if err == nil && ref.Type.Type == "file" {
				// Get the directory containing the referenced file
				toolPath := filepath.Join(workflowCWD, ref.Type.Value)
				toolDir := filepath.Dir(toolPath)
				config.Tools[i].SetCWD(toolDir)
				continue
			}
		}
		// Otherwise, inherit the workflow's CWD
		config.Tools[i].SetCWD(workflowCWD)
	}

	for i := range config.Agents {
		// If the agent has a Use reference, check its type
		if config.Agents[i].Use != nil {
			ref, err := config.Agents[i].Use.IntoRef()
			if err == nil && ref.Type.Type == "file" {
				// Get the directory containing the referenced file
				agentPath := filepath.Join(workflowCWD, ref.Type.Value)
				agentDir := filepath.Dir(agentPath)
				config.Agents[i].SetCWD(agentDir)
				continue
			}
		}
		// Otherwise, inherit the workflow's CWD
		config.Agents[i].SetCWD(workflowCWD)
	}
}
