package task

import (
	"os"
	"path/filepath"

	"dario.cat/mergo"
	"gopkg.in/yaml.v3"

	"github.com/compozy/compozy/internal/parser/agent"
	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/package_ref"
	"github.com/compozy/compozy/internal/parser/transition"
)

// TaskError represents errors that can occur during task configuration
type TaskError struct {
	Message string
	Code    string
}

func (e *TaskError) Error() string {
	return e.Message
}

type TaskType string

const (
	TaskTypeBasic    TaskType = "basic"
	TaskTypeDecision TaskType = "decision"
)

// TaskConfig represents a task configuration
type TaskConfig struct {
	ID   *TaskID                       `json:"id,omitempty" yaml:"id,omitempty"`
	Use  *package_ref.PackageRefConfig `json:"use,omitempty" yaml:"use,omitempty"`
	Type TaskType                      `json:"type,omitempty" yaml:"type,omitempty"`

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
		return nil, &TaskError{
			Message: "Failed to open task config file: " + err.Error(),
			Code:    "FILE_OPEN_ERROR",
		}
	}
	defer file.Close()

	var config TaskConfig
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, &TaskError{
			Message: "Failed to decode task config: " + err.Error(),
			Code:    "DECODE_ERROR",
		}
	}

	config.SetCWD(filepath.Dir(path))
	return &config, nil
}

// Validate validates the task configuration
func (t *TaskConfig) Validate() error {
	if err := t.validateCWD(); err != nil {
		return err
	}
	if err := t.validatePackageRef(); err != nil {
		return err
	}
	if err := t.validateInputSchema(); err != nil {
		return err
	}
	if err := t.validateOutputSchema(); err != nil {
		return err
	}
	if err := t.validateTaskType(); err != nil {
		return err
	}
	return nil
}

func (t *TaskConfig) validateCWD() error {
	if t.cwd == nil || t.cwd.Get() == "" {
		return &TaskError{
			Message: "Missing file path for task",
			Code:    "MISSING_FILE_PATH",
		}
	}
	return nil
}

func (t *TaskConfig) validatePackageRef() error {
	if t.Use == nil {
		return nil
	}
	ref, err := package_ref.Parse(string(*t.Use))
	if err != nil {
		return &TaskError{
			Message: "Invalid package reference: " + err.Error(),
			Code:    "INVALID_PACKAGE_REF",
		}
	}
	if !ref.Component.IsTask() && !ref.Component.IsAgent() && !ref.Component.IsTool() {
		return &TaskError{
			Message: "Package reference must be a task, agent, or tool",
			Code:    "INVALID_COMPONENT_TYPE",
		}
	}
	if err := ref.Type.Validate(t.cwd.Get()); err != nil {
		return &TaskError{
			Message: "Invalid package reference: " + err.Error(),
			Code:    "INVALID_PACKAGE_REF",
		}
	}
	return nil
}

func (t *TaskConfig) validateInputSchema() error {
	if t.InputSchema == nil {
		return nil
	}
	if err := t.InputSchema.Validate(); err != nil {
		return &TaskError{
			Message: "Invalid input schema: " + err.Error(),
			Code:    "INVALID_INPUT_SCHEMA",
		}
	}
	return nil
}

func (t *TaskConfig) validateOutputSchema() error {
	if t.OutputSchema == nil {
		return nil
	}
	if err := t.OutputSchema.Validate(); err != nil {
		return &TaskError{
			Message: "Invalid output schema: " + err.Error(),
			Code:    "INVALID_OUTPUT_SCHEMA",
		}
	}
	return nil
}

func (t *TaskConfig) validateTaskType() error {
	if t.Type == "" {
		return nil
	}
	switch t.Type {
	case TaskTypeBasic:
		if t.Action == nil {
			return &TaskError{
				Message: "Basic task configuration is required for basic task type",
				Code:    "INVALID_TASK_TYPE",
			}
		}
	case TaskTypeDecision:
		if t.Condition == "" && len(t.Routes) == 0 {
			return &TaskError{
				Message: "Decision task configuration is required for decision task type",
				Code:    "INVALID_TASK_TYPE",
			}
		}
		if len(t.Routes) == 0 {
			return &TaskError{
				Message: "Decision task must have at least one route",
				Code:    "INVALID_DECISION_TASK",
			}
		}
	default:
		return &TaskError{
			Message: "Invalid task type: " + string(t.Type),
			Code:    "INVALID_TASK_TYPE",
		}
	}
	return nil
}

// Merge merges another task configuration into this one
func (t *TaskConfig) Merge(other *TaskConfig) error {
	// Use mergo to deep merge the configs
	if err := mergo.Merge(t, other, mergo.WithOverride); err != nil {
		return &TaskError{
			Message: "Failed to merge task configs: " + err.Error(),
			Code:    "MERGE_ERROR",
		}
	}
	return nil
}
