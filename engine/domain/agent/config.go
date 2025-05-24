package agent

import (
	"fmt"

	"dario.cat/mergo"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/domain/tool"
	"github.com/compozy/compozy/engine/schema"
)

type ActionConfig struct {
	ID           string               `json:"id"               yaml:"id"`
	Prompt       string               `json:"prompt"           yaml:"prompt"           validate:"required"`
	InputSchema  *schema.InputSchema  `json:"input,omitempty"  yaml:"input,omitempty"`
	OutputSchema *schema.OutputSchema `json:"output,omitempty" yaml:"output,omitempty"`
	With         *common.Input        `json:"with,omitempty"   yaml:"with,omitempty"`

	cwd *common.CWD
}

func (a *ActionConfig) SetCWD(path string) error {
	cwd, err := common.CWDFromPath(path)
	if err != nil {
		return err
	}
	a.cwd = cwd
	return nil
}

func (a *ActionConfig) GetCWD() *common.CWD {
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
	ID           string                   `json:"id"                yaml:"id"                validate:"required"`
	Use          *common.PackageRefConfig `json:"use,omitempty"     yaml:"use,omitempty"`
	Config       ProviderConfig           `json:"config"            yaml:"config"            validate:"required"`
	Instructions string                   `json:"instructions"      yaml:"instructions"      validate:"required"`
	Tools        []tool.Config            `json:"tools,omitempty"   yaml:"tools,omitempty"`
	Actions      []*ActionConfig          `json:"actions,omitempty" yaml:"actions,omitempty"`
	InputSchema  *schema.InputSchema      `json:"input,omitempty"   yaml:"input,omitempty"`
	OutputSchema *schema.OutputSchema     `json:"output,omitempty"  yaml:"output,omitempty"`
	With         *common.Input            `json:"with,omitempty"    yaml:"with,omitempty"`
	Env          common.EnvMap            `json:"env,omitempty"     yaml:"env,omitempty"`

	cwd *common.CWD
}

func (a *Config) Component() common.ConfigType {
	return common.ConfigTypeAgent
}

func (a *Config) SetCWD(path string) error {
	cwd, err := common.CWDFromPath(path)
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

func (a *Config) GetCWD() *common.CWD {
	return a.cwd
}

func (a *Config) GetEnv() common.EnvMap {
	if a.Env == nil {
		return make(common.EnvMap)
	}
	return a.Env
}

func Load(cwd *common.CWD, path string) (*Config, error) {
	config, err := common.LoadConfig[*Config](cwd, path)
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

func (a *Config) ValidateParams(input *common.Input) error {
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
	return common.LoadID(a, a.ID, a.Use)
}

func (a *Config) LoadFileRef(cwd *common.CWD) (*Config, error) {
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
