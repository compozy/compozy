package task

import (
	"errors"
	"fmt"
	"os"

	"dario.cat/mergo"

	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/pkgref"
	"github.com/compozy/compozy/internal/parser/schema"
	"github.com/compozy/compozy/internal/parser/transition"
	"github.com/compozy/compozy/internal/parser/validator"
)

type TaskType string

const (
	TaskTypeBasic    TaskType = "basic"
	TaskTypeDecision TaskType = "decision"
)

type TaskConfig struct {
	ID   string                   `json:"id,omitempty" yaml:"id,omitempty"`
	Use  *pkgref.PackageRefConfig `json:"use,omitempty" yaml:"use,omitempty"`
	Type TaskType                 `json:"type,omitempty" yaml:"type,omitempty"`

	OnSuccess    *transition.SuccessTransitionConfig `json:"on_success,omitempty" yaml:"on_success,omitempty"`
	OnError      *transition.ErrorTransitionConfig   `json:"on_error,omitempty" yaml:"on_error,omitempty"`
	Final        string                              `json:"final,omitempty" yaml:"final,omitempty"`
	InputSchema  *schema.InputSchema                 `json:"input,omitempty" yaml:"input,omitempty"`
	OutputSchema *schema.OutputSchema                `json:"output,omitempty" yaml:"output,omitempty"`
	With         *common.Input                       `json:"with,omitempty" yaml:"with,omitempty"`
	Env          common.EnvMap                       `json:"env,omitempty" yaml:"env,omitempty"`

	Action string `json:"action,omitempty" yaml:"action,omitempty"`

	Condition string            `json:"condition,omitempty" yaml:"condition,omitempty"`
	Routes    map[string]string `json:"routes,omitempty" yaml:"routes,omitempty"`

	cwd *common.CWD
}

func (t *TaskConfig) Component() common.ComponentType {
	return common.ComponentTask
}

func (t *TaskConfig) SetCWD(path string) error {
	normalizedPath, err := common.CWDFromPath(path)
	if err != nil {
		return fmt.Errorf("failed to normalize path: %w", err)
	}
	t.cwd = normalizedPath
	return nil
}

func (t *TaskConfig) GetCWD() string {
	if t.cwd == nil {
		return ""
	}
	return t.cwd.Get()
}

func Load(path string) (*TaskConfig, error) {
	config, err := common.LoadConfig[*TaskConfig](path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, NewFileOpenError(err)
		}
		return nil, NewDecodeError(err)
	}
	return config, nil
}

func (t *TaskConfig) Validate() error {
	v := validator.NewCompositeValidator(
		validator.NewCWDValidator(t.cwd, t.ID),
		schema.NewSchemaValidator(t.Use, t.InputSchema, t.OutputSchema),
		NewPackageRefValidator(t.Use, t.cwd),
		NewTaskTypeValidator(t.Type, t.Action, t.Condition, t.Routes),
	)
	return v.Validate()
}

func (t *TaskConfig) ValidateParams(input map[string]any) error {
	return validator.NewParamsValidator(input, t.InputSchema.Schema, t.ID).Validate()
}

func (t *TaskConfig) Merge(other any) error {
	otherConfig, ok := other.(*TaskConfig)
	if !ok {
		return NewMergeError(errors.New("invalid type for merge"))
	}
	return mergo.Merge(t, otherConfig, mergo.WithOverride)
}

func (t *TaskConfig) LoadID() (string, error) {
	return common.LoadID(t, t.ID, t.Use)
}
