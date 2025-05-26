package agent

import (
	"fmt"

	"dario.cat/mergo"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool"
)

type ActionConfig struct {
	ID           string               `json:"id"               yaml:"id"`
	Prompt       string               `json:"prompt"           yaml:"prompt"           validate:"required"`
	InputSchema  *schema.InputSchema  `json:"input,omitempty"  yaml:"input,omitempty"`
	OutputSchema *schema.OutputSchema `json:"output,omitempty" yaml:"output,omitempty"`
	With         *core.Input          `json:"with,omitempty"   yaml:"with,omitempty"`

	cwd *core.CWD
}

func (a *ActionConfig) SetCWD(path string) error {
	cwd, err := core.CWDFromPath(path)
	if err != nil {
		return err
	}
	a.cwd = cwd
	return nil
}

func (a *ActionConfig) GetCWD() *core.CWD {
	return a.cwd
}

func (a *ActionConfig) Validate() error {
	v := schema.NewCompositeValidator(
		schema.NewCWDValidator(a.cwd, a.ID),
		schema.NewStructValidator(a),
	)
	return v.Validate()
}

func (a *ActionConfig) ValidateParams(input map[string]any) error {
	return schema.NewParamsValidator(input, a.InputSchema.Schema, a.ID).Validate()
}

type Config struct {
	ID           string                 `json:"id"                yaml:"id"                validate:"required"`
	Use          *core.PackageRefConfig `json:"use,omitempty"     yaml:"use,omitempty"`
	Config       ProviderConfig         `json:"config"            yaml:"config"            validate:"required"`
	Instructions string                 `json:"instructions"      yaml:"instructions"      validate:"required"`
	Tools        []tool.Config          `json:"tools,omitempty"   yaml:"tools,omitempty"`
	Actions      []*ActionConfig        `json:"actions,omitempty" yaml:"actions,omitempty"`
	InputSchema  *schema.InputSchema    `json:"input,omitempty"   yaml:"input,omitempty"`
	OutputSchema *schema.OutputSchema   `json:"output,omitempty"  yaml:"output,omitempty"`
	With         *core.Input            `json:"with,omitempty"    yaml:"with,omitempty"`
	Env          core.EnvMap            `json:"env,omitempty"     yaml:"env,omitempty"`

	cwd *core.CWD
}

func (a *Config) Component() core.ConfigType {
	return core.ConfigAgent
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

func Load(cwd *core.CWD, path string) (*Config, error) {
	config, err := core.LoadConfig[*Config](cwd, path)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func (a *Config) Validate() error {
	v := schema.NewCompositeValidator(
		schema.NewCWDValidator(a.cwd, a.ID),
		NewSchemaValidator(a.Use, a.InputSchema, a.OutputSchema),
		NewPackageRefValidator(a.Use, a.cwd.PathStr()),
		NewActionsValidator(a.Actions),
		schema.NewStructValidator(a),
	)
	return v.Validate()
}

func (a *Config) ValidateParams(input *core.Input) error {
	if a.InputSchema == nil || input == nil {
		return nil
	}
	return schema.NewParamsValidator(*input, a.InputSchema.Schema, a.ID).Validate()
}

func (a *Config) Merge(other any) error {
	otherConfig, ok := other.(*Config)
	if !ok {
		return fmt.Errorf("failed to merge agent configs: %s", "invalid type for merge")
	}
	return mergo.Merge(a, otherConfig, mergo.WithOverride)
}

func (a *Config) LoadID() (string, error) {
	return core.LoadID(a, a.ID, a.Use)
}

func (a *Config) LoadFileRef(cwd *core.CWD) (*Config, error) {
	if a.Use == nil {
		return a, nil
	}
	ref, err := a.Use.IntoRef()
	if err != nil {
		return nil, err
	}
	if !ref.Type.IsFile() {
		return a, nil
	}
	if ref.Component.IsAgent() {
		cfg, err := Load(cwd, ref.Value())
		if err != nil {
			return nil, err
		}
		for i := range cfg.Tools {
			tc, err := cfg.Tools[i].LoadFileRef(a.cwd)
			if err != nil {
				return nil, err
			}
			if tc != nil {
				cfg.Tools[i] = *tc
			}
		}
		err = a.Merge(cfg)
		if err != nil {
			return nil, err
		}
	}
	return a, nil
}
