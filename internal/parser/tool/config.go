package tool

import (
	"errors"
	"os"

	"dario.cat/mergo"

	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/pkgref"
	v "github.com/compozy/compozy/internal/parser/validator"
)

// TestMode indicates whether we are running in test mode
var TestMode bool

// ToolConfig represents a tool configuration
type ToolConfig struct {
	ID           *ToolID                  `json:"id,omitempty" yaml:"id,omitempty"`
	Description  *ToolDescription         `json:"description,omitempty" yaml:"description,omitempty"`
	Execute      *ToolExecute             `json:"execute,omitempty" yaml:"execute,omitempty"`
	Use          *pkgref.PackageRefConfig `json:"use,omitempty" yaml:"use,omitempty"`
	InputSchema  *common.InputSchema      `json:"input,omitempty" yaml:"input,omitempty"`
	OutputSchema *common.OutputSchema     `json:"output,omitempty" yaml:"output,omitempty"`
	With         *common.WithParams       `json:"with,omitempty" yaml:"with,omitempty"`
	Env          common.EnvMap            `json:"env,omitempty" yaml:"env,omitempty"`

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
	config, err := common.LoadConfig[*ToolConfig](path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, NewFileOpenError(err)
		}
		return nil, NewDecodeError(err)
	}
	return config, nil
}

// Validate validates the tool configuration
func (t *ToolConfig) Validate() error {
	validator := common.NewCompositeValidator(
		v.NewCWDValidator(t.cwd, string(*t.ID)),
		v.NewSchemaValidator(t.Use, t.InputSchema, t.OutputSchema),
		NewPackageRefValidator(t.Use, t.cwd),
		NewExecuteValidator(t.Execute, t.cwd).WithID(t.ID),
	)
	return validator.Validate()
}

// Merge merges another tool configuration into this one
func (t *ToolConfig) Merge(other any) error {
	otherConfig, ok := other.(*ToolConfig)
	if !ok {
		return NewMergeError(errors.New("invalid type for merge"))
	}
	return mergo.Merge(t, otherConfig, mergo.WithOverride)
}
