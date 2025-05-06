package tool

import (
	"os"
	"path/filepath"

	"dario.cat/mergo"
	"gopkg.in/yaml.v3"

	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/package_ref"
)

// TestMode indicates whether we are running in test mode
var TestMode bool

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
		return nil, NewFileOpenError(err)
	}
	defer file.Close()

	var config ToolConfig
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, NewDecodeError(err)
	}

	config.SetCWD(filepath.Dir(path))
	return &config, nil
}

// Validate validates the tool configuration
func (t *ToolConfig) Validate() error {
	if err := t.validateCWD(); err != nil {
		return err
	}
	if err := t.validatePackageRef(); err != nil {
		return err
	}
	if err := t.validateExecute(); err != nil {
		return err
	}
	if err := t.validateInputSchema(); err != nil {
		return err
	}
	if err := t.validateOutputSchema(); err != nil {
		return err
	}
	return nil
}

func (t *ToolConfig) validateCWD() error {
	if t.cwd == nil || t.cwd.Get() == "" {
		return NewMissingPathError()
	}
	return nil
}

func (t *ToolConfig) validatePackageRef() error {
	if t.Use == nil {
		return nil
	}
	ref, err := t.Use.IntoRef()
	if err != nil {
		return NewInvalidPackageRefError(err)
	}
	if !ref.Component.IsTool() {
		return NewInvalidTypeError()
	}
	if err := ref.Type.Validate(t.cwd.Get()); err != nil {
		return NewInvalidPackageRefError(err)
	}
	return nil
}

func (t *ToolConfig) validateExecute() error {
	if t.Execute == nil {
		return nil
	}
	executePath := t.cwd.Join(string(*t.Execute))
	executePath, err := filepath.Abs(executePath)
	if err != nil {
		return NewInvalidExecutePathError(err)
	}
	if !TestMode && t.Execute.IsTypeScript() && !fileExists(executePath) {
		if t.ID == nil {
			return NewMissingToolIDError()
		}
		return NewInvalidToolExecuteError(executePath)
	}
	return nil
}

func (t *ToolConfig) validateInputSchema() error {
	if t.InputSchema == nil {
		return nil
	}
	if err := t.InputSchema.Validate(); err != nil {
		return NewInvalidInputSchemaError(err)
	}
	return nil
}

func (t *ToolConfig) validateOutputSchema() error {
	if t.OutputSchema == nil {
		return nil
	}
	if err := t.OutputSchema.Validate(); err != nil {
		return NewInvalidOutputSchemaError(err)
	}
	return nil
}

// Merge merges another tool configuration into this one
func (t *ToolConfig) Merge(other *ToolConfig) error {
	if err := mergo.Merge(t, other, mergo.WithOverride); err != nil {
		return NewMergeError(err)
	}
	return nil
}

// Helper function to check if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
