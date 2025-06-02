package agent

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/pkg/ref"
)

type ActionConfig struct {
	ref.WithRef
	ID           string         `json:"id"               yaml:"id"`
	Prompt       string         `json:"prompt"           yaml:"prompt"           validate:"required"`
	InputSchema  *schema.Schema `json:"input,omitempty"  yaml:"input,omitempty"`
	OutputSchema *schema.Schema `json:"output,omitempty" yaml:"output,omitempty"`
	With         *core.Input    `json:"with,omitempty"   yaml:"with,omitempty"`

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

func (a *ActionConfig) ValidateParams(ctx context.Context, input *core.Input) error {
	return schema.NewParamsValidator(input, a.InputSchema, a.ID).Validate(ctx)
}
