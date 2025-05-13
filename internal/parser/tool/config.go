package tool

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"dario.cat/mergo"

	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/pkgref"
	"github.com/compozy/compozy/internal/parser/schema"
	"github.com/compozy/compozy/internal/parser/validator"
)

// Config represents a tool configuration
type Config struct {
	ID           string                   `json:"id,omitempty"          yaml:"id,omitempty"`
	Description  string                   `json:"description,omitempty" yaml:"description,omitempty"`
	Execute      string                   `json:"execute,omitempty"     yaml:"execute,omitempty"`
	Use          *pkgref.PackageRefConfig `json:"use,omitempty"         yaml:"use,omitempty"`
	InputSchema  *schema.InputSchema      `json:"input,omitempty"       yaml:"input,omitempty"`
	OutputSchema *schema.OutputSchema     `json:"output,omitempty"      yaml:"output,omitempty"`
	With         *common.Input            `json:"with,omitempty"        yaml:"with,omitempty"`
	Env          common.EnvMap            `json:"env,omitempty"         yaml:"env,omitempty"`

	cwd *common.CWD // internal field for current working directory
}

func (t *Config) Component() common.ComponentType {
	return common.ComponentTool
}

// SetCWD sets the current working directory for the tool
func (t *Config) SetCWD(path string) error {
	cwd, err := common.CWDFromPath(path)
	if err != nil {
		return err
	}
	t.cwd = cwd
	return nil
}

// GetCWD returns the current working directory
func (t *Config) GetCWD() *common.CWD {
	return t.cwd
}

// Load loads a tool configuration from a file
func Load(cwd *common.CWD, path string) (*Config, error) {
	config, err := common.LoadConfig[*Config](cwd, path)
	if err != nil {
		return nil, err
	}
	return config, nil
}

// Validate validates the tool configuration
func (t *Config) Validate() error {
	v := validator.NewCompositeValidator(
		validator.NewCWDValidator(t.cwd, t.ID),
		NewSchemaValidator(t.Use, t.InputSchema, t.OutputSchema),
		NewPackageRefValidator(t.Use, t.cwd.PathStr()),
		NewExecuteValidator(t.Execute, t.cwd).WithID(t.ID),
	)
	return v.Validate()
}

func (t *Config) ValidateParams(input map[string]any) error {
	return validator.NewParamsValidator(input, t.InputSchema.Schema, t.ID).Validate()
}

// Merge merges another tool configuration into this one
func (t *Config) Merge(other any) error {
	otherConfig, ok := other.(*Config)
	if !ok {
		return fmt.Errorf("failed to merge tool configs: %w", errors.New("invalid type for merge"))
	}
	return mergo.Merge(t, otherConfig, mergo.WithOverride)
}

// LoadID loads the ID from either the direct ID field or resolves it from a package reference
func (t *Config) LoadID() (string, error) {
	return common.LoadID(t, t.ID, t.Use)
}

func IsTypeScript(path string) bool {
	ext := filepath.Ext(path)
	return strings.EqualFold(ext, ".ts")
}

func (t *Config) LoadFileRef(cwd *common.CWD) (*Config, error) {
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
	if ref.Component.IsTool() {
		cfg, err := Load(cwd, ref.Value())
		if err != nil {
			return nil, err
		}
		err = t.Merge(cfg)
		if err != nil {
			return nil, err
		}
	}
	return t, nil
}
