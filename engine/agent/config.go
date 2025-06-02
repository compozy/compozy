package agent

import (
	"context"
	"fmt"

	"dario.cat/mergo"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/ref"
)

type Config struct {
	ID           string          `json:"id"                yaml:"id"                validate:"required"`
	Config       ProviderConfig  `json:"config"            yaml:"config"            validate:"required"`
	Instructions string          `json:"instructions"      yaml:"instructions"      validate:"required"`
	Tools        []tool.Config   `json:"tools,omitempty"   yaml:"tools,omitempty"`
	Actions      []*ActionConfig `json:"actions,omitempty" yaml:"actions,omitempty"`
	InputSchema  *schema.Schema  `json:"input,omitempty"   yaml:"input,omitempty"`
	OutputSchema *schema.Schema  `json:"output,omitempty"  yaml:"output,omitempty"`
	With         *core.Input     `json:"with,omitempty"    yaml:"with,omitempty"`
	Env          core.EnvMap     `json:"env,omitempty"     yaml:"env,omitempty"`

	filePath string
	cwd      *core.CWD
}

func (a *Config) Component() core.ConfigType {
	return core.ConfigAgent
}

func (a *Config) GetFilePath() string {
	return a.filePath
}

func (a *Config) SetFilePath(path string) {
	a.filePath = path
}

func (a *Config) SetCWD(path string) error {
	cwd, err := core.CWDFromPath(path)
	if err != nil {
		return err
	}
	a.cwd = cwd
	for i := range a.Actions {
		if err := a.Actions[i].SetCWD(path); err != nil {
			return err
		}
	}
	return nil
}

func (a *Config) GetCWD() *core.CWD {
	return a.cwd
}

func (a *Config) GetInput() *core.Input {
	if a.With == nil {
		a.With = &core.Input{}
	}
	return a.With
}

func (a *Config) GetEnv() *core.EnvMap {
	if a.Env == nil {
		a.Env = make(core.EnvMap)
		return &a.Env
	}
	return &a.Env
}

func (a *Config) Validate() error {
	v := schema.NewCompositeValidator(
		schema.NewCWDValidator(a.cwd, a.ID),
		NewActionsValidator(a.Actions),
		schema.NewStructValidator(a),
	)
	return v.Validate()
}

func (a *Config) ValidateParams(ctx context.Context, input *core.Input) error {
	if a.InputSchema == nil || input == nil {
		return nil
	}
	return schema.NewParamsValidator(input, a.InputSchema, a.ID).Validate(ctx)
}

func (a *Config) Merge(other any) error {
	otherConfig, ok := other.(*Config)
	if !ok {
		return fmt.Errorf("failed to merge agent configs: %s", "invalid type for merge")
	}
	return mergo.Merge(a, otherConfig, mergo.WithOverride)
}

func Load(cwd *core.CWD, path string) (*Config, error) {
	filePath, err := core.ResolvePath(cwd, path)
	if err != nil {
		return nil, err
	}
	config, _, err := core.LoadConfig[*Config](filePath)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func LoadAndEval(cwd *core.CWD, path string, ev *ref.Evaluator) (*Config, error) {
	filePath, err := core.ResolvePath(cwd, path)
	if err != nil {
		return nil, err
	}
	config, _, err := core.LoadConfigWithEvaluator[*Config](filePath, ev)
	if err != nil {
		return nil, err
	}
	return config, nil
}
