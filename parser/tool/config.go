package tool

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/compozy/compozy/parser/common"
	"github.com/compozy/compozy/parser/package_ref"
)

// ToolError represents errors that can occur during tool configuration
type ToolError struct {
	Message string
	Code    string
}

func (e *ToolError) Error() string {
	return e.Message
}

// ToolConfig represents a tool configuration
type ToolConfig struct {
	ID           *ToolID                       `json:"id,omitempty" yaml:"id,omitempty"`
	Description  *ToolDescription              `json:"description,omitempty" yaml:"description,omitempty"`
	Execute      *ToolExecute                  `json:"execute,omitempty" yaml:"execute,omitempty"`
	Use          *package_ref.PackageRefConfig `json:"use,omitempty" yaml:"use,omitempty"`
	InputSchema  *common.InputSchema           `json:"input,omitempty" yaml:"input,omitempty"`
	OutputSchema *common.OutputSchema          `json:"output,omitempty" yaml:"output,omitempty"`
	With         *common.WithParams            `json:"with,omitempty" yaml:"with,omitempty"`
	Env          common.EnvMap                 `json:"env,omitempty" yaml:"env,omitempty"`

	cwd *common.CWD // internal field for current working directory
}

// SetCWD sets the current working directory for the tool
func (t *ToolConfig) SetCWD(path string) {
	if t.cwd == nil {
		t.cwd = common.NewCWD(path)
	} else {
		t.cwd.Set(path)
	}
}

// GetCWD returns the current working directory
func (t *ToolConfig) GetCWD() string {
	if t.cwd == nil {
		return ""
	}
	return t.cwd.Get()
}

// Load loads a tool configuration from a file
func Load(path string) (*ToolConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, &ToolError{
			Message: "Failed to open tool config file: " + err.Error(),
			Code:    "FILE_OPEN_ERROR",
		}
	}
	defer file.Close()

	var config ToolConfig
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, &ToolError{
			Message: "Failed to decode tool config: " + err.Error(),
			Code:    "DECODE_ERROR",
		}
	}

	config.SetCWD(filepath.Dir(path))
	return &config, nil
}

// Validate validates the tool configuration
func (t *ToolConfig) Validate() error {
	if t.cwd == nil || t.cwd.Get() == "" {
		return &ToolError{
			Message: "Missing file path for tool",
			Code:    "MISSING_FILE_PATH",
		}
	}

	// Validate package reference if present
	if t.Use != nil {
		ref, err := t.Use.IntoRef()
		if err != nil {
			return &ToolError{
				Message: "Invalid package reference: " + err.Error(),
				Code:    "INVALID_PACKAGE_REF",
			}
		}

		// Validate that it's a tool reference
		if !ref.Component.IsTool() {
			return &ToolError{
				Message: "Package reference must be a tool",
				Code:    "INVALID_COMPONENT_TYPE",
			}
		}

		// Validate the reference against the current working directory
		if err := ref.Type.Validate(t.cwd.Get()); err != nil {
			return &ToolError{
				Message: "Invalid package reference: " + err.Error(),
				Code:    "INVALID_PACKAGE_REF",
			}
		}
	}

	// Validate execute path if present
	if t.Execute != nil {
		executePath := t.cwd.Join(string(*t.Execute))
		executePath, err := filepath.Abs(executePath)
		if err != nil {
			return &ToolError{
				Message: "Invalid execute path: " + err.Error(),
				Code:    "INVALID_EXECUTE_PATH",
			}
		}

		if t.Execute.IsTypeScript() && !fileExists(executePath) {
			if t.ID == nil {
				return &ToolError{
					Message: "Tool ID is required for TypeScript execution",
					Code:    "MISSING_TOOL_ID",
				}
			}
			return &ToolError{
				Message: "Invalid tool execute path: " + executePath,
				Code:    "INVALID_TOOL_EXECUTE",
			}
		}
	}

	// Validate input schema if present
	if t.InputSchema != nil {
		if err := t.InputSchema.Validate(); err != nil {
			return &ToolError{
				Message: "Invalid input schema: " + err.Error(),
				Code:    "INVALID_INPUT_SCHEMA",
			}
		}
	}

	// Validate output schema if present
	if t.OutputSchema != nil {
		if err := t.OutputSchema.Validate(); err != nil {
			return &ToolError{
				Message: "Invalid output schema: " + err.Error(),
				Code:    "INVALID_OUTPUT_SCHEMA",
			}
		}
	}

	return nil
}

// Merge merges another tool configuration into this one
func (t *ToolConfig) Merge(other *ToolConfig) error {
	if t.Env == nil {
		t.Env = other.Env
	} else if other.Env != nil {
		t.Env.Merge(other.Env)
	}
	if t.With == nil {
		t.With = other.With
	}
	return nil
}

// Helper function to check if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
