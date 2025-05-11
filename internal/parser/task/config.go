package task

import (
	"errors"
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

// TaskConfig represents a task configuration
type TaskConfig struct {
	ID   string                   `json:"id,omitempty" yaml:"id,omitempty"`
	Use  *pkgref.PackageRefConfig `json:"use,omitempty" yaml:"use,omitempty"`
	Type TaskType                 `json:"type,omitempty" yaml:"type,omitempty"`

	// Common properties
	OnSuccess    *transition.SuccessTransitionConfig `json:"on_success,omitempty" yaml:"on_success,omitempty"`
	OnError      *transition.ErrorTransitionConfig   `json:"on_error,omitempty" yaml:"on_error,omitempty"`
	Final        string                              `json:"final,omitempty" yaml:"final,omitempty"`
	InputSchema  *schema.InputSchema                 `json:"input,omitempty" yaml:"input,omitempty"`
	OutputSchema *schema.OutputSchema                `json:"output,omitempty" yaml:"output,omitempty"`
	With         *common.Input                       `json:"with,omitempty" yaml:"with,omitempty"`
	Env          common.EnvMap                       `json:"env,omitempty" yaml:"env,omitempty"`

	// Basic task properties
	Action string `json:"action,omitempty" yaml:"action,omitempty"`

	// Decision task properties
	Condition string            `json:"condition,omitempty" yaml:"condition,omitempty"`
	Routes    map[string]string `json:"routes,omitempty" yaml:"routes,omitempty"`

	cwd *common.CWD // internal field for current working directory
}

func (t *TaskConfig) Component() common.ComponentType {
	return common.ComponentTask
}

// SetCWD sets the current working directory for the task
func (t *TaskConfig) SetCWD(path string) {
	if t.cwd == nil {
		t.cwd = common.NewCWD(path)
	} else {
		t.cwd.Set(path)
	}
}

// GetCWD returns the current working directory
func (t *TaskConfig) GetCWD() string {
	if t.cwd == nil {
		return ""
	}
	return t.cwd.Get()
}

// Load loads a task configuration from a file
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

// Validate validates the task configuration
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

// Merge merges another task configuration into this one
func (t *TaskConfig) Merge(other any) error {
	otherConfig, ok := other.(*TaskConfig)
	if !ok {
		return NewMergeError(errors.New("invalid type for merge"))
	}
	return mergo.Merge(t, otherConfig, mergo.WithOverride)
}

// LoadID loads the ID from either the direct ID field or resolves it from a package reference
func (t *TaskConfig) LoadID() (string, error) {
	return common.LoadID(t, t.ID, t.Use, func(path string) (common.Config, error) {
		return Load(path)
	})
}
