package task

import (
	"errors"
	"fmt"

	"dario.cat/mergo"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
)

type Type string

const (
	TaskTypeBasic    Type = "basic"
	TaskTypeDecision Type = "decision"
)

type Config struct {
	ID           string                   `json:"id,omitempty"         yaml:"id,omitempty"`
	Use          *core.PackageRefConfig   `json:"use,omitempty"        yaml:"use,omitempty"`
	Type         Type                     `json:"type,omitempty"       yaml:"type,omitempty"`
	OnSuccess    *SuccessTransitionConfig `json:"on_success,omitempty" yaml:"on_success,omitempty"`
	OnError      *ErrorTransitionConfig   `json:"on_error,omitempty"   yaml:"on_error,omitempty"`
	Final        bool                     `json:"final,omitempty"      yaml:"final,omitempty"`
	InputSchema  *schema.InputSchema      `json:"input,omitempty"      yaml:"input,omitempty"`
	OutputSchema *schema.OutputSchema     `json:"output,omitempty"     yaml:"output,omitempty"`
	With         *core.Input              `json:"with,omitempty"       yaml:"with,omitempty"`
	Env          core.EnvMap              `json:"env,omitempty"        yaml:"env,omitempty"`
	cwd          *core.CWD

	// Basic task properties
	Action string `json:"action,omitempty" yaml:"action,omitempty"`

	// Decision task properties
	Condition string            `json:"condition,omitempty" yaml:"condition,omitempty"`
	Routes    map[string]string `json:"routes,omitempty"    yaml:"routes,omitempty"`
}

func (t *Config) Component() core.ConfigType {
	return core.ConfigTask
}

func (t *Config) SetCWD(path string) error {
	cwd, err := core.CWDFromPath(path)
	if err != nil {
		return err
	}
	t.cwd = cwd
	return nil
}

func (t *Config) GetCWD() *core.CWD {
	return t.cwd
}

func (t *Config) GetEnv() *core.EnvMap {
	if t.Env == nil {
		t.Env = make(core.EnvMap)
		return &t.Env
	}
	return &t.Env
}

func (t *Config) GetInput() *core.Input {
	if t.With == nil {
		t.With = &core.Input{}
	}
	return t.With
}

func Load(cwd *core.CWD, path string) (*Config, error) {
	config, err := core.LoadConfig[*Config](cwd, path)
	if err != nil {
		return nil, err
	}
	if string(config.Type) == "" {
		config.Type = TaskTypeBasic
	}
	return config, nil
}

func (t *Config) Validate() error {
	v := schema.NewCompositeValidator(
		schema.NewCWDValidator(t.cwd, t.ID),
		NewSchemaValidator(t.Use, t.InputSchema, t.OutputSchema),
		NewPackageRefValidator(t.Use, t.cwd.PathStr()),
		NewTaskTypeValidator(t.Use, t.Type, t.Action, t.Condition, t.Routes),
	)
	return v.Validate()
}

func (t *Config) ValidateParams(input *core.Input) error {
	if t.InputSchema == nil || input == nil {
		return nil
	}
	return schema.NewParamsValidator(*input, t.InputSchema.Schema, t.ID).Validate()
}

func (t *Config) Merge(other any) error {
	otherConfig, ok := other.(*Config)
	if !ok {
		return fmt.Errorf("failed to merge task configs: %w", errors.New("invalid type for merge"))
	}
	return mergo.Merge(t, otherConfig, mergo.WithOverride)
}

func (t *Config) LoadID() (string, error) {
	return core.LoadID(t, t.ID, t.Use)
}

func (t *Config) LoadFileRef(cwd *core.CWD) (*Config, error) {
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

func FindConfig(tasks []Config, taskID string) (*Config, error) {
	for i := range tasks {
		if tasks[i].ID == taskID {
			return &tasks[i], nil
		}
	}
	return nil, fmt.Errorf("task not found")
}
