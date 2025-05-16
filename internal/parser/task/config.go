package task

import (
	"errors"
	"fmt"

	"dario.cat/mergo"

	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/pkgref"
	"github.com/compozy/compozy/internal/parser/schema"
	"github.com/compozy/compozy/internal/parser/transition"
	"github.com/compozy/compozy/internal/parser/validator"
)

type Type string

const (
	TaskTypeBasic    Type = "basic"
	TaskTypeDecision Type = "decision"
)

type Config struct {
	ID           string                              `json:"id,omitempty"         yaml:"id,omitempty"`
	Use          *pkgref.PackageRefConfig            `json:"use,omitempty"        yaml:"use,omitempty"`
	Type         Type                                `json:"type,omitempty"       yaml:"type,omitempty"`
	OnSuccess    *transition.SuccessTransitionConfig `json:"on_success,omitempty" yaml:"on_success,omitempty"`
	OnError      *transition.ErrorTransitionConfig   `json:"on_error,omitempty"   yaml:"on_error,omitempty"`
	Final        bool                                `json:"final,omitempty"      yaml:"final,omitempty"`
	InputSchema  *schema.InputSchema                 `json:"input,omitempty"      yaml:"input,omitempty"`
	OutputSchema *schema.OutputSchema                `json:"output,omitempty"     yaml:"output,omitempty"`
	With         *common.Input                       `json:"with,omitempty"       yaml:"with,omitempty"`
	Env          common.EnvMap                       `json:"env,omitempty"        yaml:"env,omitempty"`
	cwd          *common.CWD

	// Basic task properties
	Action string `json:"action,omitempty" yaml:"action,omitempty"`

	// Decision task properties
	Condition string            `json:"condition,omitempty" yaml:"condition,omitempty"`
	Routes    map[string]string `json:"routes,omitempty"    yaml:"routes,omitempty"`
}

func (t *Config) Component() common.ComponentType {
	return common.ComponentTask
}

func (t *Config) SetCWD(path string) error {
	cwd, err := common.CWDFromPath(path)
	if err != nil {
		return err
	}
	t.cwd = cwd
	return nil
}

func (t *Config) GetCWD() *common.CWD {
	return t.cwd
}

func Load(cwd *common.CWD, path string) (*Config, error) {
	config, err := common.LoadConfig[*Config](cwd, path)
	if err != nil {
		return nil, err
	}
	if string(config.Type) == "" {
		config.Type = TaskTypeBasic
	}
	return config, nil
}

func (t *Config) Validate() error {
	v := validator.NewCompositeValidator(
		validator.NewCWDValidator(t.cwd, t.ID),
		NewSchemaValidator(t.Use, t.InputSchema, t.OutputSchema),
		NewPackageRefValidator(t.Use, t.cwd.PathStr()),
		NewTaskTypeValidator(t.Use, t.Type, t.Action, t.Condition, t.Routes),
	)
	return v.Validate()
}

func (t *Config) ValidateParams(input map[string]any) error {
	return validator.NewParamsValidator(input, t.InputSchema.Schema, t.ID).Validate()
}

func (t *Config) Merge(other any) error {
	otherConfig, ok := other.(*Config)
	if !ok {
		return fmt.Errorf("failed to merge task configs: %w", errors.New("invalid type for merge"))
	}
	return mergo.Merge(t, otherConfig, mergo.WithOverride)
}

func (t *Config) LoadID() (string, error) {
	return common.LoadID(t, t.ID, t.Use)
}

func (t *Config) LoadFileRef(cwd *common.CWD) (*Config, error) {
	if t.Use == nil {
		return nil, nil
	}
	ref, err := t.Use.IntoRef()
	if err != nil {
		return nil, err
	}
	if !ref.Type.IsFile() {
		return t, nil
	}
	if ref.Component.IsTask() {
		tc, err := Load(cwd, ref.Value())
		if err != nil {
			return nil, err
		}
		// TODO: adjust this when we have other task types
		err = t.Merge(tc)
		if err != nil {
			return nil, err
		}
	}
	return t, nil
}
