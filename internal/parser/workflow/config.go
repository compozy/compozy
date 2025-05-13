package workflow

import (
	"errors"
	"fmt"

	"dario.cat/mergo"

	"github.com/compozy/compozy/internal/parser/agent"
	"github.com/compozy/compozy/internal/parser/author"
	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/task"
	"github.com/compozy/compozy/internal/parser/tool"
	"github.com/compozy/compozy/internal/parser/trigger"
	"github.com/compozy/compozy/internal/parser/validator"
)

type Config struct {
	ID          string         `json:"id" yaml:"id"`
	Tasks       []task.Config  `json:"tasks" yaml:"tasks"`
	Trigger     trigger.Config `json:"trigger" yaml:"trigger"`
	Version     string         `json:"version,omitempty" yaml:"version,omitempty"`
	Description string         `json:"description,omitempty" yaml:"description,omitempty"`
	Author      *author.Author `json:"author,omitempty" yaml:"author,omitempty"`
	Tools       []tool.Config  `json:"tools,omitempty" yaml:"tools,omitempty"`
	Agents      []agent.Config `json:"agents,omitempty" yaml:"agents,omitempty"`
	Env         common.EnvMap  `json:"env,omitempty" yaml:"env,omitempty"`

	cwd *common.CWD
}

func (w *Config) Component() common.ComponentType {
	return common.ComponentWorkflow
}

func (w *Config) SetCWD(path string) error {
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

func (w *Config) GetCWD() *common.CWD {
	return w.cwd
}

func Load(cwd *common.CWD, path string) (*Config, error) {
	wc, err := common.LoadConfig[*Config](cwd, path)
	if err != nil {
		return nil, err
	}
	if err := loadFileRefs(wc); err != nil {
		return nil, err
	}
	err = wc.Validate()
	if err != nil {
		return nil, err
	}
	return wc, nil
}

func (w *Config) Validate() error {
	v := validator.NewCompositeValidator(
		validator.NewCWDValidator(w.cwd, string(w.ID)),
	)
	if err := v.Validate(); err != nil {
		return err
	}

	trigger := w.Trigger
	if err := trigger.Validate(); err != nil {
		return fmt.Errorf("trigger validation error: %w", err)
	}

	for _, tc := range w.Tasks {
		err := tc.Validate()
		if err != nil {
			return fmt.Errorf("task validation error: %s", err)
		}
	}

	for _, ac := range w.Agents {
		err := ac.Validate()
		if err != nil {
			return fmt.Errorf("agent validation error: %s", err)
		}
	}

	for _, tc := range w.Tools {
		err := tc.Validate()
		if err != nil {
			return fmt.Errorf("tool validation error: %s", err)
		}
	}

	return nil
}

func (w *Config) ValidateParams(input map[string]any) error {
	inputSchema := w.Trigger.InputSchema
	return validator.NewParamsValidator(input, inputSchema.Schema, w.ID).Validate()
}

func (w *Config) Merge(other any) error {
	otherConfig, ok := other.(*Config)
	if !ok {
		return fmt.Errorf("failed to merge workflow configs: %w", errors.New("invalid type for merge"))
	}
	return mergo.Merge(w, otherConfig, mergo.WithOverride)
}

func (w *Config) LoadID() (string, error) {
	return string(w.ID), nil
}

func setComponentsCWD(wc *Config, cwd *common.CWD) error {
	if err := setTasksCWD(wc, cwd); err != nil {
		return err
	}
	if err := setToolsCWD(wc, cwd); err != nil {
		return err
	}
	if err := setAgentsCWD(wc, cwd); err != nil {
		return err
	}
	return nil
}

func setTasksCWD(wc *Config, cwd *common.CWD) error {
	for i := range wc.Tasks {
		if wc.Tasks[i].Use != nil {
			ref, err := wc.Tasks[i].Use.IntoRef()
			if err != nil {
				return err
			}
			if ref.Type.IsFile() && ref.Component.IsTask() {
				taskPath, err := cwd.JoinAndCheck(ref.Type.Value)
				if err != nil {
					return err
				}
				wc.Tasks[i].SetCWD(taskPath)
				continue
			}
		}
		wc.Tasks[i].SetCWD(cwd.PathStr())
	}
	return nil
}

func setToolsCWD(wc *Config, cwd *common.CWD) error {
	for i := range wc.Tools {
		if wc.Tools[i].Use != nil {
			ref, err := wc.Tools[i].Use.IntoRef()
			if err != nil {
				return err
			}
			if ref.Type.IsFile() && ref.Component.IsTool() {
				toolPath, err := cwd.JoinAndCheck(ref.Type.Value)
				if err != nil {
					return err
				}
				wc.Tools[i].SetCWD(toolPath)
				continue
			}
		}
		wc.Tools[i].SetCWD(cwd.PathStr())
	}
	return nil
}

func setAgentsCWD(wc *Config, cwd *common.CWD) error {
	for i := range wc.Agents {
		if wc.Agents[i].Use != nil {
			ref, err := wc.Agents[i].Use.IntoRef()
			if err != nil {
				return err
			}
			if ref.Type.IsFile() && ref.Component.IsAgent() {
				agentPath, err := cwd.JoinAndCheck(ref.Type.Value)
				if err != nil {
					return err
				}
				wc.Agents[i].SetCWD(agentPath)
				continue
			}
		}
		wc.Agents[i].SetCWD(cwd.PathStr())
	}
	return nil
}

func loadFileRefs(wc *Config) error {
	if err := LoadTasksRef(wc); err != nil {
		return err
	}
	if err := LoadAgentsRef(wc); err != nil {
		return err
	}
	if err := LoadToolsRef(wc); err != nil {
		return err
	}
	return nil
}
