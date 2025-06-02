package tool

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"dario.cat/mergo"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/pkg/ref"
)

// Config represents a tool configuration
type Config struct {
	ID           string         `json:"id,omitempty"          yaml:"id,omitempty"`
	Description  string         `json:"description,omitempty" yaml:"description,omitempty"`
	Execute      string         `json:"execute,omitempty"     yaml:"execute,omitempty"`
	InputSchema  *schema.Schema `json:"input,omitempty"       yaml:"input,omitempty"`
	OutputSchema *schema.Schema `json:"output,omitempty"      yaml:"output,omitempty"`
	With         *core.Input    `json:"with,omitempty"        yaml:"with,omitempty"`
	Env          core.EnvMap    `json:"env,omitempty"         yaml:"env,omitempty"`

	filePath string
	cwd      *core.CWD
}

func (t *Config) Component() core.ConfigType {
	return core.ConfigTool
}

func (t *Config) GetFilePath() string {
	return t.filePath
}

func (t *Config) SetFilePath(path string) {
	t.filePath = path
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

// Validate validates the tool configuration
func (t *Config) Validate() error {
	v := schema.NewCompositeValidator(
		schema.NewCWDValidator(t.cwd, t.ID),
		NewExecuteValidator(t),
	)
	return v.Validate()
}

func (t *Config) ValidateParams(ctx context.Context, input *core.Input) error {
	if t.InputSchema == nil || input == nil {
		return nil
	}
	return schema.NewParamsValidator(input, t.InputSchema, t.ID).Validate(ctx)
}

// Merge merges another tool configuration into this one
func (t *Config) Merge(other any) error {
	otherConfig, ok := other.(*Config)
	if !ok {
		return fmt.Errorf("failed to merge tool configs: %w", errors.New("invalid type for merge"))
	}
	return mergo.Merge(t, otherConfig, mergo.WithOverride)
}

func IsTypeScript(path string) bool {
	ext := filepath.Ext(path)
	return strings.EqualFold(ext, ".ts")
}

func Load(cwd *core.CWD, path string) (*Config, error) {
	filePath, err := core.ResolvePath(cwd, path)
	if err != nil {
		return nil, err
	}
	config, _, err := core.LoadConfig[*Config](filePath)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func LoadAndEval(cwd *core.CWD, path string, ev *ref.Evaluator) (*Config, error) {
	filePath, err := core.ResolvePath(cwd, path)
	if err != nil {
		return nil, err
	}
	config, _, err := core.LoadConfigWithEvaluator[*Config](filePath, ev)
	if err != nil {
		return nil, err
	}
	return config, nil
}
