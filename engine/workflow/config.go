package workflow

import (
	"errors"
	"fmt"

	"dario.cat/mergo"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
)

type Opts struct {
	OnError     *task.ErrorTransitionConfig `json:"on_error,omitempty" yaml:"on_error,omitempty"`
	InputSchema *schema.InputSchema         `json:"input,omitempty"    yaml:"input,omitempty"`
}

type Config struct {
	ID          string         `json:"id"                    yaml:"id"`
	Tasks       []task.Config  `json:"tasks"                 yaml:"tasks"`
	Opts        Opts           `json:"config"               yaml:"config"`
	Version     string         `json:"version,omitempty"     yaml:"version,omitempty"`
	Description string         `json:"description,omitempty" yaml:"description,omitempty"`
	Author      *core.Author   `json:"author,omitempty"      yaml:"author,omitempty"`
	Tools       []tool.Config  `json:"tools,omitempty"       yaml:"tools,omitempty"`
	Agents      []agent.Config `json:"agents,omitempty"      yaml:"agents,omitempty"`
	Env         core.EnvMap    `json:"env,omitempty"         yaml:"env,omitempty"`

	cwd *core.CWD
}

func (w *Config) Component() core.ConfigType {
	return core.ConfigWorkflow
}

func (w *Config) SetCWD(path string) error {
	cwd, err := core.CWDFromPath(path)
	if err != nil {
		return err
	}
	w.cwd = cwd
	if err := setComponentsCWD(w, w.cwd); err != nil {
		return err
	}
	return nil
}

func (w *Config) GetCWD() *core.CWD {
	return w.cwd
}

func (w *Config) GetEnv() *core.EnvMap {
	if w.Env == nil {
		w.Env = make(core.EnvMap)
		return &w.Env
	}
	return &w.Env
}

func (w *Config) GetInput() *core.Input {
	return &core.Input{}
}

func Load(cwd *core.CWD, path string) (*Config, error) {
	wc, err := core.LoadConfig[*Config](cwd, path)
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

func (w *Config) ValidateParams(input *core.Input) error {
	if input == nil {
		return nil
	}
	inputSchema := w.Opts.InputSchema
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

func WorkflowsFromProject(projectConfig *project.Config) ([]*Config, error) {
	cwd := projectConfig.GetCWD()
	var ws []*Config
	for _, wf := range projectConfig.Workflows {
		config, err := Load(cwd, wf.Source)
		if err != nil {
			return nil, err
		}
		ws = append(ws, config)
	}
	return ws, nil
}

func setComponentsCWD(wc *Config, cwd *core.CWD) error {
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

func setTasksCWD(wc *Config, cwd *core.CWD) error {
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

func setToolsCWD(wc *Config, cwd *core.CWD) error {
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

func setAgentsCWD(wc *Config, cwd *core.CWD) error {
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

// -----------------------------------------------------------------------------
// Loaders
// -----------------------------------------------------------------------------

func LoadAgentsRef(wc *Config) error {
	for i := range wc.Agents {
		cfg, err := wc.Agents[i].LoadFileRef(wc.GetCWD())
		if err != nil {
			return err
		}
		if cfg != nil {
			wc.Agents[i] = *cfg
		}
	}
	return nil
}

func LoadToolsRef(wc *Config) error {
	for i := range wc.Tools {
		cfg, err := wc.Tools[i].LoadFileRef(wc.GetCWD())
		if err != nil {
			return err
		}
		if cfg != nil {
			wc.Tools[i] = *cfg
		}
	}
	return nil
}

func LoadTasksRef(wc *Config) error {
	for i := 0; i < len(wc.Tasks); i++ {
		tc := &wc.Tasks[i]
		cfg, err := tc.LoadFileRef(wc.GetCWD())
		if err != nil {
			return fmt.Errorf("failed to load task reference for task %s: %w", tc.ID, err)
		}
		if cfg != nil {
			wc.Tasks[i] = *cfg
			if err := loadReferencedComponents(wc, cfg); err != nil {
				return err
			}
		}
	}
	return nil
}

func loadReferencedComponents(wc *Config, tc *task.Config) error {
	if err := loadAgentsRefOnTask(wc, tc); err != nil {
		return fmt.Errorf("failed to load agent reference for task %s: %w", tc.ID, err)
	}

	if err := loadToolsRefOnTask(wc, tc); err != nil {
		return fmt.Errorf("failed to load tool reference for task %s: %w", tc.ID, err)
	}

	return nil
}

func loadAgentsRefOnTask(wc *Config, tc *task.Config) error {
	if tc.Use == nil {
		return nil
	}

	ref, err := tc.Use.IntoRef()
	if err != nil {
		return err
	}

	if !ref.Type.IsFile() || !ref.Component.IsAgent() {
		return nil
	}

	cfg, err := agent.Load(tc.GetCWD(), ref.Value())
	if err != nil {
		return err
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid agent configuration: %w", err)
	}

	for i := 0; i < len(wc.Agents); i++ {
		ac := &wc.Agents[i]
		if ac.ID == cfg.ID {
			return nil
		}
	}

	wc.Agents = append(wc.Agents, *cfg)
	return nil
}

func loadToolsRefOnTask(wc *Config, tc *task.Config) error {
	if tc.Use == nil {
		return nil
	}

	ref, err := tc.Use.IntoRef()
	if err != nil {
		return err
	}

	if !ref.Type.IsFile() || !ref.Component.IsTool() {
		return nil
	}

	cfg, err := tool.Load(tc.GetCWD(), ref.Value())
	if err != nil {
		return err
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid tool configuration: %w", err)
	}

	for _, tc := range wc.Tools {
		if tc.ID == cfg.ID {
			return nil
		}
	}

	wc.Tools = append(wc.Tools, *cfg)
	return nil
}
