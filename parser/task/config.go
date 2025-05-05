package task

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/compozy/compozy/parser/agent"
	"github.com/compozy/compozy/parser/common"
	"github.com/compozy/compozy/parser/package_ref"
	"github.com/compozy/compozy/parser/transition"
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

// BasicTaskConfig represents a basic task configuration
type BasicTaskConfig struct {
	Action *agent.ActionID `json:"action,omitempty" yaml:"action,omitempty"`
}

// DecisionTaskConfig represents a decision task configuration
type DecisionTaskConfig struct {
	Condition TaskCondition           `json:"condition" yaml:"condition"`
	Routes    map[TaskRoute]TaskRoute `json:"routes" yaml:"routes"`
}

// TaskConfig represents a task configuration
type TaskConfig struct {
	ID           *TaskID                             `json:"id,omitempty" yaml:"id,omitempty"`
	PackageRef   *package_ref.PackageRefConfig       `json:"package_ref,omitempty" yaml:"package_ref,omitempty"`
	Type         TaskType                            `json:"type,omitempty" yaml:"type,omitempty"`
	Basic        *BasicTaskConfig                    `json:"basic,omitempty" yaml:"basic,omitempty"`
	Decision     *DecisionTaskConfig                 `json:"decision,omitempty" yaml:"decision,omitempty"`
	OnSuccess    *transition.SuccessTransitionConfig `json:"on_success,omitempty" yaml:"on_success,omitempty"`
	OnError      *transition.ErrorTransitionConfig   `json:"on_error,omitempty" yaml:"on_error,omitempty"`
	Final        *TaskFinal                          `json:"final,omitempty" yaml:"final,omitempty"`
	InputSchema  *common.InputSchema                 `json:"input,omitempty" yaml:"input,omitempty"`
	OutputSchema *common.OutputSchema                `json:"output,omitempty" yaml:"output,omitempty"`
	With         *common.WithParams                  `json:"with,omitempty" yaml:"with,omitempty"`
	Env          common.EnvMap                       `json:"env,omitempty" yaml:"env,omitempty"`

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
	if t.cwd == nil || t.cwd.Get() == "" {
		return &TaskError{
			Message: "Missing file path for task",
			Code:    "MISSING_FILE_PATH",
		}
	}

	// Validate package reference if present
	if t.PackageRef != nil {
		ref, err := t.PackageRef.IntoRef()
		if err != nil {
			return &TaskError{
				Message: "Invalid package reference: " + err.Error(),
				Code:    "INVALID_PACKAGE_REF",
			}
		}

		// Validate that it's a task reference
		if !ref.Component.IsTask() {
			return &TaskError{
				Message: "Package reference must be a task",
				Code:    "INVALID_COMPONENT_TYPE",
			}
		}

		// Validate the reference against the current working directory
		if err := ref.Type.Validate(t.cwd.Get()); err != nil {
			return &TaskError{
				Message: "Invalid package reference: " + err.Error(),
				Code:    "INVALID_PACKAGE_REF",
			}
		}
	}

	// Validate input schema if present
	if t.InputSchema != nil {
		if err := t.InputSchema.Validate(); err != nil {
			return &TaskError{
				Message: "Invalid input schema: " + err.Error(),
				Code:    "INVALID_INPUT_SCHEMA",
			}
		}
	}

	// Validate output schema if present
	if t.OutputSchema != nil {
		if err := t.OutputSchema.Validate(); err != nil {
			return &TaskError{
				Message: "Invalid output schema: " + err.Error(),
				Code:    "INVALID_OUTPUT_SCHEMA",
			}
		}
	}

	// Validate task type configuration
	if t.Type != "" {
		switch t.Type {
		case TaskTypeBasic:
			if t.Basic == nil {
				return &TaskError{
					Message: "Basic task configuration is required for basic task type",
					Code:    "INVALID_TASK_TYPE",
				}
			}
		case TaskTypeDecision:
			if t.Decision == nil {
				return &TaskError{
					Message: "Decision task configuration is required for decision task type",
					Code:    "INVALID_TASK_TYPE",
				}
			}
			if len(t.Decision.Routes) == 0 {
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
	}

	return nil
}

// Merge merges another task configuration into this one
func (t *TaskConfig) Merge(other *TaskConfig) error {
	if t.Env == nil {
		t.Env = other.Env
	} else if other.Env != nil {
		t.Env.Merge(other.Env)
	}
	if t.With == nil {
		t.With = other.With
	}
	if t.OnSuccess == nil {
		t.OnSuccess = other.OnSuccess
	}
	if t.OnError == nil {
		t.OnError = other.OnError
	}
	return nil
}
