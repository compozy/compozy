package task

import (
	"errors"
	"os"
	"path/filepath"

	"dario.cat/mergo"
	"gopkg.in/yaml.v3"

	"github.com/compozy/compozy/internal/parser/agent"
	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/pkgref"
	"github.com/compozy/compozy/internal/parser/transition"
	v "github.com/compozy/compozy/internal/parser/validator"
)

type TaskType string

const (
	TaskTypeBasic    TaskType = "basic"
	TaskTypeDecision TaskType = "decision"
)

// TaskConfig represents a task configuration
type TaskConfig struct {
	ID   *TaskID                  `json:"id,omitempty" yaml:"id,omitempty"`
	Use  *pkgref.PackageRefConfig `json:"use,omitempty" yaml:"use,omitempty"`
	Type TaskType                 `json:"type,omitempty" yaml:"type,omitempty"`

	// Common properties
	OnSuccess    *transition.SuccessTransitionConfig `json:"on_success,omitempty" yaml:"on_success,omitempty"`
	OnError      *transition.ErrorTransitionConfig   `json:"on_error,omitempty" yaml:"on_error,omitempty"`
	Final        *TaskFinal                          `json:"final,omitempty" yaml:"final,omitempty"`
	InputSchema  *common.InputSchema                 `json:"input,omitempty" yaml:"input,omitempty"`
	OutputSchema *common.OutputSchema                `json:"output,omitempty" yaml:"output,omitempty"`
	With         *common.WithParams                  `json:"with,omitempty" yaml:"with,omitempty"`
	Env          common.EnvMap                       `json:"env,omitempty" yaml:"env,omitempty"`

	// Basic task properties
	Action *agent.ActionID `json:"action,omitempty" yaml:"action,omitempty"`

	// Decision task properties
	Condition TaskCondition           `json:"condition,omitempty" yaml:"condition,omitempty"`
	Routes    map[TaskRoute]TaskRoute `json:"routes,omitempty" yaml:"routes,omitempty"`

	cwd *common.CWD // internal field for current working directory
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
	file, err := os.Open(path)
	if err != nil {
		return nil, NewFileOpenError(err)
	}

	var config TaskConfig
	decoder := yaml.NewDecoder(file)
	decodeErr := decoder.Decode(&config)
	closeErr := file.Close()

	if decodeErr != nil {
		return nil, NewDecodeError(decodeErr)
	}
	if closeErr != nil {
		return nil, NewFileCloseError(closeErr)
	}

	config.SetCWD(filepath.Dir(path))
	return &config, nil
}

// Validate validates the task configuration
func (t *TaskConfig) Validate() error {
	validator := common.NewCompositeValidator(
		v.NewCWDValidator(t.cwd, string(*t.ID)),
		NewPackageRefValidator(t.Use, t.cwd),
		NewTaskTypeValidator(t.Type, t.Action, t.Condition, t.Routes),
	)
	return validator.Validate()
}

// Merge merges another task configuration into this one
func (t *TaskConfig) Merge(other any) error {
	otherConfig, ok := other.(*TaskConfig)
	if !ok {
		return NewMergeError(errors.New("invalid type for merge"))
	}
	return mergo.Merge(t, otherConfig, mergo.WithOverride)
}
