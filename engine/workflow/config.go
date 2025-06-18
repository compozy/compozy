package workflow

import (
	"context"
	"errors"
	"fmt"

	"dario.cat/mergo"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/ref"
)

type TriggerType string

const (
	TriggerTypeSignal TriggerType = "signal"
)

type Trigger struct {
	Type   TriggerType    `json:"type"             yaml:"type"             mapstructure:"type"`
	Name   string         `json:"name"             yaml:"name"             mapstructure:"name"`
	Schema *schema.Schema `json:"schema,omitempty" yaml:"schema,omitempty" mapstructure:"schema,omitempty"`
}

type Opts struct {
	core.GlobalOpts `json:",inline" yaml:",inline" mapstructure:",squash"`
	InputSchema     *schema.Schema `json:"input,omitempty" yaml:"input,omitempty" mapstructure:"input,omitempty"`
	Env             *core.EnvMap   `json:"env,omitempty"   yaml:"env,omitempty"   mapstructure:"env,omitempty"`
}

type Config struct {
	Resource    string          `json:"resource,omitempty"    yaml:"resource,omitempty"    mapstructure:"resource,omitempty"`
	ID          string          `json:"id"                    yaml:"id"                    mapstructure:"id"`
	Version     string          `json:"version,omitempty"     yaml:"version,omitempty"     mapstructure:"version,omitempty"`
	Description string          `json:"description,omitempty" yaml:"description,omitempty" mapstructure:"description,omitempty"`
	Schemas     []schema.Schema `json:"schemas,omitempty"     yaml:"schemas,omitempty"     mapstructure:"schemas,omitempty"`
	Opts        Opts            `json:"config"                yaml:"config"                mapstructure:"config"`
	Author      *core.Author    `json:"author,omitempty"      yaml:"author,omitempty"      mapstructure:"author,omitempty"`
	Tools       []tool.Config   `json:"tools,omitempty"       yaml:"tools,omitempty"       mapstructure:"tools,omitempty"`
	Agents      []agent.Config  `json:"agents,omitempty"      yaml:"agents,omitempty"      mapstructure:"agents,omitempty"`
	MCPs        []mcp.Config    `json:"mcps,omitempty"        yaml:"mcps,omitempty"        mapstructure:"mcps,omitempty"`
	Triggers    []Trigger       `json:"triggers,omitempty"    yaml:"triggers,omitempty"    mapstructure:"triggers,omitempty"`
	Tasks       []task.Config   `json:"tasks"                 yaml:"tasks"                 mapstructure:"tasks"`
	Outputs     *core.Input     `json:"outputs,omitempty"     yaml:"outputs,omitempty"     mapstructure:"outputs,omitempty"`

	filePath string
	CWD      *core.PathCWD
}

func (w *Config) Component() core.ConfigType {
	return core.ConfigWorkflow
}

func (w *Config) SetCWD(path string) error {
	CWD, err := core.CWDFromPath(path)
	if err != nil {
		return err
	}
	w.CWD = CWD
	if err := setComponentsCWD(w, w.CWD); err != nil {
		return err
	}
	return nil
}

func (w *Config) GetCWD() *core.PathCWD {
	return w.CWD
}

func (w *Config) GetEnv() core.EnvMap {
	if w.Opts.Env == nil {
		w.Opts.Env = &core.EnvMap{}
		return *w.Opts.Env
	}
	return *w.Opts.Env
}

func (w *Config) GetInput() *core.Input {
	return &core.Input{}
}

func (w *Config) GetOutputs() *core.Input {
	return w.Outputs
}

func (w *Config) GetFilePath() string {
	return w.filePath
}

func (w *Config) SetFilePath(path string) {
	w.filePath = path
}

func (w *Config) HasSchema() bool {
	return w.Opts.InputSchema != nil
}

func (w *Config) Validate() error {
	validator := NewWorkflowValidator(w)
	return validator.Validate()
}

func (w *Config) ValidateInput(ctx context.Context, input *core.Input) error {
	validator := NewInputValidator(w, input)
	return validator.Validate(ctx)
}

// ApplyInputDefaults merges default values from the input schema with the provided input
func (w *Config) ApplyInputDefaults(input *core.Input) (*core.Input, error) {
	if w.Opts.InputSchema == nil {
		// No schema, return input as-is
		if input == nil {
			input = &core.Input{}
		}
		return input, nil
	}
	var inputMap map[string]any
	if input == nil {
		inputMap = make(map[string]any)
	} else {
		inputMap = *input
	}
	// Apply defaults from schema
	mergedInput, err := w.Opts.InputSchema.ApplyDefaults(inputMap)
	if err != nil {
		return nil, fmt.Errorf("failed to apply input defaults for workflow %s: %w", w.ID, err)
	}
	result := core.Input(mergedInput)
	return &result, nil
}

func (w *Config) ValidateOutput(_ context.Context, _ *core.Output) error {
	// Does not make sense the workflow having a schema
	return nil
}

func (w *Config) Merge(other any) error {
	otherConfig, ok := other.(*Config)
	if !ok {
		return fmt.Errorf("failed to merge workflow configs: %w", errors.New("invalid type for merge"))
	}
	return mergo.Merge(w, otherConfig, mergo.WithOverride)
}

func (w *Config) AsMap() (map[string]any, error) {
	return core.AsMapDefault(w)
}

func (w *Config) FromMap(data any) error {
	config, err := core.FromMapDefault[*Config](data)
	if err != nil {
		return err
	}
	return w.Merge(config)
}

func (w *Config) GetID() string {
	return w.ID
}

func (w *Config) SetDefaults() {
	for i := range w.MCPs {
		w.MCPs[i].SetDefaults()
	}
}

// GetTasks returns the workflow tasks
func (w *Config) GetTasks() []task.Config {
	return w.Tasks
}

// GetMCPs returns the workflow MCPs
func (w *Config) GetMCPs() []mcp.Config {
	mcps := make([]mcp.Config, len(w.MCPs))
	copy(mcps, w.MCPs)
	return mcps
}

func (w *Config) DetermineNextTask(
	taskConfig *task.Config,
	success bool,
) *task.Config {
	var nextTaskID string
	if success && taskConfig.OnSuccess != nil && taskConfig.OnSuccess.Next != nil {
		nextTaskID = *taskConfig.OnSuccess.Next
	} else if !success && taskConfig.OnError != nil && taskConfig.OnError.Next != nil {
		nextTaskID = *taskConfig.OnError.Next
	}
	if nextTaskID == "" {
		return nil
	}
	// Find the next task config
	nextTask, err := task.FindConfig(w.Tasks, nextTaskID)
	if err != nil {
		return nil
	}
	return nextTask
}

func (w *Config) Clone() (*Config, error) {
	if w == nil {
		return nil, nil
	}
	return core.DeepCopy(w)
}

func WorkflowsFromProject(projectConfig *project.Config, ev *ref.Evaluator) ([]*Config, error) {
	cwd := projectConfig.GetCWD()
	projectEnv := projectConfig.GetEnv()
	var ws []*Config
	for _, wf := range projectConfig.Workflows {
		config, err := LoadAndEval(cwd, wf.Source, ev)
		if err != nil {
			return nil, err
		}
		if config != nil {
			config.Opts.Env = &projectEnv
		}
		ws = append(ws, config)
	}
	return ws, nil
}

func setComponentsCWD(wc *Config, cwd *core.PathCWD) error {
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

func setTasksCWD(wc *Config, cwd *core.PathCWD) error {
	for i := range wc.Tasks {
		if err := wc.Tasks[i].SetCWD(cwd.PathStr()); err != nil {
			return err
		}
	}
	return nil
}

func setToolsCWD(wc *Config, cwd *core.PathCWD) error {
	for i := range wc.Tools {
		if err := wc.Tools[i].SetCWD(cwd.PathStr()); err != nil {
			return err
		}
	}
	return nil
}

func setAgentsCWD(wc *Config, cwd *core.PathCWD) error {
	for i := range wc.Agents {
		if err := wc.Agents[i].SetCWD(cwd.PathStr()); err != nil {
			return err
		}
	}
	return nil
}

func Load(cwd *core.PathCWD, path string) (*Config, error) {
	filePath, err := core.ResolvePath(cwd, path)
	if err != nil {
		return nil, err
	}
	config, _, err := core.LoadConfig[*Config](filePath)
	if err != nil {
		return nil, err
	}
	config.SetDefaults()
	return config, nil
}

func LoadAndEval(cwd *core.PathCWD, path string, ev *ref.Evaluator) (*Config, error) {
	filePath, err := core.ResolvePath(cwd, path)
	if err != nil {
		return nil, err
	}
	scope, err := core.MapFromFilePath(filePath)
	if err != nil {
		return nil, err
	}
	ev.WithLocalScope(scope)
	config, _, err := core.LoadConfigWithEvaluator[*Config](filePath, ev)
	if err != nil {
		return nil, err
	}
	config.SetDefaults()
	return config, nil
}

func FindConfig(workflows []*Config, workflowID string) (*Config, error) {
	for _, wf := range workflows {
		if wf.ID == workflowID {
			return wf, nil
		}
	}
	return nil, fmt.Errorf("workflow not found")
}

func FindAgentConfig[C core.Config](workflows []*Config, agentID string) (C, error) {
	var cfg C
	for _, wf := range workflows {
		for i := range wf.Agents {
			if wf.Agents[i].ID == agentID {
				cfg, ok := any(&wf.Agents[i]).(C)
				if !ok {
					return cfg, fmt.Errorf("agent config is not of type %T", cfg)
				}
				return cfg, nil
			}
		}
	}
	return cfg, fmt.Errorf("agent not found: %s", agentID)
}
