package tool

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"dario.cat/mergo"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
)

// Config represents a tool configuration
type Config struct {
	ID           string                 `json:"id,omitempty"          yaml:"id,omitempty"`
	Description  string                 `json:"description,omitempty" yaml:"description,omitempty"`
	Execute      string                 `json:"execute,omitempty"     yaml:"execute,omitempty"`
	Use          *core.PackageRefConfig `json:"use,omitempty"         yaml:"use,omitempty"`
	InputSchema  *schema.InputSchema    `json:"input,omitempty"       yaml:"input,omitempty"`
	OutputSchema *schema.OutputSchema   `json:"output,omitempty"      yaml:"output,omitempty"`
	With         *core.Input            `json:"with,omitempty"        yaml:"with,omitempty"`
	Env          core.EnvMap            `json:"env,omitempty"         yaml:"env,omitempty"`

	cwd *core.CWD // internal field for current working directory
}

func (t *Config) Component() core.ConfigType {
	return core.ConfigTool
}

// SetCWD sets the current working directory for the tool
func (t *Config) SetCWD(path string) error {
	cwd, err := core.CWDFromPath(path)
	if err != nil {
		return err
	}
	t.cwd = cwd
	return nil
}

// GetCWD returns the current working directory
func (t *Config) GetCWD() *core.CWD {
	return t.cwd
}

func (t *Config) GetEnv() *core.EnvMap {
	if t.Env == nil {
		t.Env = make(core.EnvMap)
		return &t.Env
	}
	return &t.Env
}

func (t *Config) GetInput() *core.Input {
	if t.With == nil {
		t.With = &core.Input{}
	}
	return t.With
}

// Load loads a tool configuration from a file
func Load(cwd *core.CWD, path string) (*Config, error) {
	config, err := core.LoadConfig[*Config](cwd, path)
	if err != nil {
		return nil, err
	}
	return config, nil
}

// Validate validates the tool configuration
func (t *Config) Validate() error {
	v := schema.NewCompositeValidator(
		schema.NewCWDValidator(t.cwd, t.ID),
		NewSchemaValidator(t.Use, t.InputSchema, t.OutputSchema),
		NewPackageRefValidator(t.Use, t.cwd.PathStr()),
		NewExecuteValidator(t.Execute, t.cwd).WithID(t.ID),
	)
	return v.Validate()
}

func (t *Config) ValidateParams(input *core.Input) error {
	if t.InputSchema == nil || input == nil {
		return nil
	}
	return schema.NewParamsValidator(*input, t.InputSchema.Schema, t.ID).Validate()
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
	return core.LoadID(t, t.ID, t.Use)
}

func IsTypeScript(path string) bool {
	ext := filepath.Ext(path)
	return strings.EqualFold(ext, ".ts")
}

func (t *Config) LoadFileRef(cwd *core.CWD) (*Config, error) {
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
