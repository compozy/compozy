package workflow

import (
	"errors"
	"fmt"

	"dario.cat/mergo"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/domain/agent"
	"github.com/compozy/compozy/engine/domain/task"
	"github.com/compozy/compozy/engine/domain/tool"
	"github.com/compozy/compozy/engine/domain/trigger"
	"github.com/compozy/compozy/engine/schema"
)

type Config struct {
	ID          string         `json:"id"                    yaml:"id"`
	Tasks       []task.Config  `json:"tasks"                 yaml:"tasks"`
	Trigger     trigger.Config `json:"trigger"               yaml:"trigger"`
	Version     string         `json:"version,omitempty"     yaml:"version,omitempty"`
	Description string         `json:"description,omitempty" yaml:"description,omitempty"`
	Author      *common.Author `json:"author,omitempty"      yaml:"author,omitempty"`
	Tools       []tool.Config  `json:"tools,omitempty"       yaml:"tools,omitempty"`
	Agents      []agent.Config `json:"agents,omitempty"      yaml:"agents,omitempty"`
	Env         common.EnvMap  `json:"env,omitempty"         yaml:"env,omitempty"`

	cwd *common.CWD
}

func (w *Config) Component() common.ConfigType {
	return common.ConfigTypeWorkflow
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

func (w *Config) GetEnv() common.EnvMap {
	if w.Env == nil {
		return make(common.EnvMap)
	}
	return w.Env
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
	v := schema.NewCompositeValidator(
		schema.NewCWDValidator(w.cwd, w.ID),
	)
	if err := v.Validate(); err != nil {
		return err
	}

	trigger := w.Trigger
	if err := trigger.Validate(); err != nil {
		return fmt.Errorf("trigger validation error: %w", err)
	}

	for i := 0; i < len(w.Tasks); i++ {
		tc := &w.Tasks[i]
		err := tc.Validate()
		if err != nil {
			return fmt.Errorf("task validation error: %s", err)
		}
	}

	for i := 0; i < len(w.Agents); i++ {
		ac := &w.Agents[i]
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

func (w *Config) ValidateParams(input *common.Input) error {
	if input == nil {
		return nil
	}
	inputSchema := w.Trigger.InputSchema
	return schema.NewParamsValidator(*input, inputSchema.Schema, w.ID).Validate()
}

func (w *Config) Merge(other any) error {
	otherConfig, ok := other.(*Config)
	if !ok {
		return fmt.Errorf("failed to merge workflow configs: %w", errors.New("invalid type for merge"))
	}
	return mergo.Merge(w, otherConfig, mergo.WithOverride)
}

func (w *Config) LoadID() (string, error) {
	return w.ID, nil
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
				if err := wc.Tasks[i].SetCWD(taskPath); err != nil {
					return err
				}
				continue
			}
		}
		if err := wc.Tasks[i].SetCWD(cwd.PathStr()); err != nil {
			return err
		}
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
				if err := wc.Tools[i].SetCWD(toolPath); err != nil {
					return err
				}
				continue
			}
		}
		if err := wc.Tools[i].SetCWD(cwd.PathStr()); err != nil {
			return err
		}
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
				if err := wc.Agents[i].SetCWD(agentPath); err != nil {
					return err
				}
				continue
			}
		}
		if err := wc.Agents[i].SetCWD(cwd.PathStr()); err != nil {
			return err
		}
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

// GetID returns the workflow ID
func (w *Config) GetID() string {
	return w.ID
}

// GetTasks returns the workflow tasks
func (w *Config) GetTasks() []task.Config {
	return w.Tasks
}

func FindConfig(workflows []*Config, workflowID string) (*Config, error) {
	for _, wf := range workflows {
		if wf.ID == workflowID {
			return wf, nil
		}
	}
	return nil, fmt.Errorf("workflow not found")
}
