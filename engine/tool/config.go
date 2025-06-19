package tool

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"dario.cat/mergo"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/ref"
	"github.com/tmc/langchaingo/llms"
)

// Config represents a tool configuration
type Config struct {
	Resource     string         `json:"resource,omitempty"    yaml:"resource,omitempty"    mapstructure:"resource,omitempty"`
	ID           string         `json:"id,omitempty"          yaml:"id,omitempty"          mapstructure:"id,omitempty"`
	Description  string         `json:"description,omitempty" yaml:"description,omitempty" mapstructure:"description,omitempty"`
	Execute      string         `json:"execute,omitempty"     yaml:"execute,omitempty"     mapstructure:"execute,omitempty"`
	Timeout      string         `json:"timeout,omitempty"     yaml:"timeout,omitempty"     mapstructure:"timeout,omitempty"`
	InputSchema  *schema.Schema `json:"input,omitempty"       yaml:"input,omitempty"       mapstructure:"input,omitempty"`
	OutputSchema *schema.Schema `json:"output,omitempty"      yaml:"output,omitempty"      mapstructure:"output,omitempty"`
	With         *core.Input    `json:"with,omitempty"        yaml:"with,omitempty"        mapstructure:"with,omitempty"`
	Env          *core.EnvMap   `json:"env,omitempty"         yaml:"env,omitempty"         mapstructure:"env,omitempty"`

	filePath string
	CWD      *core.PathCWD
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
	CWD, err := core.CWDFromPath(path)
	if err != nil {
		return err
	}
	t.CWD = CWD
	return nil
}

// GetCWD returns the current working directory
func (t *Config) GetCWD() *core.PathCWD {
	return t.CWD
}

func (t *Config) GetEnv() core.EnvMap {
	if t.Env == nil {
		t.Env = &core.EnvMap{}
		return *t.Env
	}
	return *t.Env
}

// GetTimeout returns the tool-specific timeout with fallback to global timeout
func (t *Config) GetTimeout(globalTimeout time.Duration) (time.Duration, error) {
	if t.Timeout == "" {
		return globalTimeout, nil
	}
	timeout, err := time.ParseDuration(t.Timeout)
	if err != nil {
		// Log warning for debugging
		// Note: We can't get activity context here, so using context.Background()
		logger.FromContext(context.Background()).Warn(
			"Invalid tool timeout format",
			"tool_id", t.ID,
			"configured_timeout", t.Timeout,
			"error", err,
		)
		return 0, fmt.Errorf("invalid tool timeout '%s': %w", t.Timeout, err)
	}
	if timeout <= 0 {
		return 0, fmt.Errorf("tool timeout must be positive, got: %v", timeout)
	}
	return timeout, nil
}

func (t *Config) GetInput() *core.Input {
	if t.With == nil {
		return &core.Input{}
	}
	return t.With
}

func (t *Config) HasSchema() bool {
	return t.InputSchema != nil || t.OutputSchema != nil
}

// Validate validates the tool configuration
func (t *Config) Validate() error {
	v := schema.NewCompositeValidator(
		schema.NewCWDValidator(t.CWD, t.ID),
		NewExecuteValidator(t),
	)
	if err := v.Validate(); err != nil {
		return err
	}
	if t.Timeout != "" {
		timeout, err := time.ParseDuration(t.Timeout)
		if err != nil {
			return fmt.Errorf("invalid timeout format '%s': %w", t.Timeout, err)
		}
		if timeout <= 0 {
			return fmt.Errorf("timeout must be positive, got: %v", timeout)
		}
	}
	return nil
}

func (t *Config) ValidateInput(ctx context.Context, input *core.Input) error {
	if t.InputSchema == nil || input == nil {
		return nil
	}
	return schema.NewParamsValidator(input, t.InputSchema, t.ID).Validate(ctx)
}

func (t *Config) ValidateOutput(ctx context.Context, output *core.Output) error {
	if t.OutputSchema == nil || output == nil {
		return nil
	}
	return schema.NewParamsValidator(output, t.OutputSchema, t.ID).Validate(ctx)
}

// Merge merges another tool configuration into this one
func (t *Config) Merge(other any) error {
	otherConfig, ok := other.(*Config)
	if !ok {
		return fmt.Errorf("failed to merge tool configs: %w", errors.New("invalid type for merge"))
	}
	return mergo.Merge(t, otherConfig, mergo.WithOverride)
}

func (t *Config) Clone() (*Config, error) {
	if t == nil {
		return nil, nil
	}
	return core.DeepCopy(t)
}

func (t *Config) AsMap() (map[string]any, error) {
	return core.AsMapDefault(t)
}

func (t *Config) FromMap(data any) error {
	config, err := core.FromMapDefault[*Config](data)
	if err != nil {
		return err
	}
	return t.Merge(config)
}

func (t *Config) GetLLMDefinition() llms.Tool {
	return llms.Tool{
		Type: "function",
		Function: &llms.FunctionDefinition{
			Name:        t.ID,
			Description: t.Description,
			Parameters:  t.InputSchema,
		},
	}
}

func IsTypeScript(path string) bool {
	ext := filepath.Ext(path)
	return strings.EqualFold(ext, ".ts")
}

func Load(cwd *core.PathCWD, path string) (*Config, error) {
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

func LoadAndEval(cwd *core.PathCWD, path string, ev *ref.Evaluator) (*Config, error) {
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
