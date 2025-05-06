package tool

import (
	"errors"
	"os"
	"path/filepath"

	"dario.cat/mergo"
	"gopkg.in/yaml.v3"

	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/package_ref"
	v "github.com/compozy/compozy/internal/parser/validator"
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

	var config ToolConfig
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

// Validate validates the tool configuration
func (t *ToolConfig) Validate() error {
	validator := common.NewCompositeValidator(
		v.NewCWDValidator(t.cwd, string(*t.ID)),
		NewPackageRefValidator(t.Use, t.cwd),
		NewExecuteValidator(t.Execute, t.cwd).WithID(t.ID),
	)
	return validator.Validate()
}

// Merge merges another tool configuration into this one
func (t *ToolConfig) Merge(other interface{}) error {
	otherConfig, ok := other.(*ToolConfig)
	if !ok {
		return NewMergeError(errors.New("invalid type for merge"))
	}
	return mergo.Merge(t, otherConfig, mergo.WithOverride)
}
