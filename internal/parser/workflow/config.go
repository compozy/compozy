package workflow

import (
	"errors"
	"fmt"
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
	cwd, err := common.CWDFromPath(path)
	if err != nil {
		return err
	}
	w.cwd = cwd
	if err := setComponentsCWD(w, w.cwd); err != nil {
		return err
	}
	return nil
}

func (w *WorkflowConfig) GetCWD() *common.CWD {
	return w.cwd
}

func Load(cwd *common.CWD, path string) (*WorkflowConfig, error) {
	config, err := common.LoadConfig[*WorkflowConfig](cwd, path)
	if err != nil {
		return nil, err
	}
	if config.Tasks == nil {
		config.Tasks = []task.TaskConfig{}
	}
	if config.Tools == nil {
		config.Tools = []tool.ToolConfig{}
	}
	if config.Agents == nil {
		config.Agents = []agent.AgentConfig{}
	}
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
		taskComponents = append(taskComponents, &w.Tasks[i])
	}
	if err := NewComponentsValidator(taskComponents, w.cwd).Validate(); err != nil {
		return err
	}

	var toolComponents []common.Config
	for i := range w.Tools {
		toolComponents = append(toolComponents, &w.Tools[i])
	}
	if err := NewComponentsValidator(toolComponents, w.cwd).Validate(); err != nil {
		return err
	}

	var agentComponents []common.Config
	for i := range w.Agents {
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

func setComponentsCWD(config *WorkflowConfig, cwd *common.CWD) error {
	if err := setTasksCWD(config, cwd); err != nil {
		return err
	}
	if err := setToolsCWD(config, cwd); err != nil {
		return err
	}
	if err := setAgentsCWD(config, cwd); err != nil {
		return err
	}
	return nil
}

func setTasksCWD(config *WorkflowConfig, cwd *common.CWD) error {
	for i := range config.Tasks {
		if config.Tasks[i].Use != nil {
			ref, err := config.Tasks[i].Use.IntoRef()
			if err == nil && ref.Type.Type == "file" {
				taskPath, err := cwd.JoinAndCheck(ref.Type.Value)
				if err != nil {
					return err
				}
				config.Tasks[i].SetCWD(taskPath)
				continue
			}
		}
		config.Tasks[i].SetCWD(cwd.PathStr())
	}
	return nil
}

func setToolsCWD(config *WorkflowConfig, cwd *common.CWD) error {
	for i := range config.Tools {
		if config.Tools[i].Use != nil {
			ref, err := config.Tools[i].Use.IntoRef()
			if err == nil && ref.Type.Type == "file" {
				toolPath, err := cwd.JoinAndCheck(ref.Type.Value)
				if err != nil {
					return err
				}
				toolDir := filepath.Dir(toolPath)
				config.Tools[i].SetCWD(toolDir)
				continue
			}
		}
		config.Tools[i].SetCWD(cwd.PathStr())
	}
	return nil
}

func setAgentsCWD(config *WorkflowConfig, cwd *common.CWD) error {
	for i := range config.Agents {
		if config.Agents[i].Use != nil {
			ref, err := config.Agents[i].Use.IntoRef()
			if err == nil && ref.Type.Type == "file" {
				agentPath, err := cwd.JoinAndCheck(ref.Type.Value)
				if err != nil {
					return err
				}
				agentDir := filepath.Dir(agentPath)
				config.Agents[i].SetCWD(agentDir)
				continue
			}
		}
		config.Agents[i].SetCWD(cwd.PathStr())
	}
	return nil
}
