package agent

import (
	"context"

	"dario.cat/mergo"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
)

type ActionConfig struct {
	ID           string         `json:"id"               yaml:"id"               mapstructure:"id"`
	Prompt       string         `json:"prompt"           yaml:"prompt"           mapstructure:"prompt"           validate:"required"`
	InputSchema  *schema.Schema `json:"input,omitempty"  yaml:"input,omitempty"  mapstructure:"input,omitempty"`
	OutputSchema *schema.Schema `json:"output,omitempty" yaml:"output,omitempty" mapstructure:"output,omitempty"`
	With         *core.Input    `json:"with,omitempty"   yaml:"with,omitempty"   mapstructure:"with,omitempty"`

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

func (a *ActionConfig) GetInput() *core.Input {
	if a.With == nil {
		return &core.Input{}
	}
	return a.With
}

func (a *ActionConfig) Validate() error {
	v := schema.NewCompositeValidator(
		schema.NewCWDValidator(a.cwd, a.ID),
		schema.NewStructValidator(a),
	)
	return v.Validate()
}

func (a *ActionConfig) ValidateParams(ctx context.Context, input *core.Input) error {
	return schema.NewParamsValidator(input, a.InputSchema, a.ID).Validate(ctx)
}

// AsMap converts the action configuration to a map for template normalization
func (a *ActionConfig) AsMap() (map[string]any, error) {
	return core.AsMapDefault(a)
}

// FromMap updates the action configuration from a normalized map
func (a *ActionConfig) FromMap(data any) error {
	config, err := core.FromMapDefault[ActionConfig](data)
	if err != nil {
		return err
	}
	return mergo.Merge(a, config, mergo.WithOverride)
}

func FindActionConfig(actions []*ActionConfig, id string) *ActionConfig {
	for _, action := range actions {
		if action.ID == id {
			return action
		}
	}
	return nil
}
