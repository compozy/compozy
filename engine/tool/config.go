package tool

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"dario.cat/mergo"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/pkg/ref"
	"github.com/pkg/errors"
)

// Config represents a tool configuration
type Config struct {
	ref.WithRef
	Ref          *ref.Node            `json:"$ref,omitempty"         yaml:"$ref,omitempty"`
	ID           string               `json:"id,omitempty"           yaml:"id,omitempty"`
	Description  string               `json:"description,omitempty"  yaml:"description,omitempty"`
	Execute      string               `json:"execute,omitempty"      yaml:"execute,omitempty"`
	InputSchema  *schema.InputSchema  `json:"input,omitempty"        yaml:"input,omitempty"`
	OutputSchema *schema.OutputSchema `json:"output,omitempty"       yaml:"output,omitempty"`
	With         *core.Input          `json:"with,omitempty"         yaml:"with,omitempty"`
	Env          core.EnvMap          `json:"env,omitempty"          yaml:"env,omitempty"`

	metadata *core.ConfigMetadata
}

func (t *Config) Component() core.ConfigType {
	return core.ConfigTool
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

func (t *Config) GetMetadata() *core.ConfigMetadata {
	return t.metadata
}

func (t *Config) SetMetadata(metadata *core.ConfigMetadata) {
	t.metadata = metadata
}

// Load loads a tool configuration from a file
func Load(ctx context.Context, cwd *core.CWD, projectRoot string, filePath string) (*Config, error) {
	config, err := core.LoadConfig[*Config](ctx, cwd, projectRoot, filePath)
	if err != nil {
		return nil, err
	}

	filePath = config.metadata.FilePath
	currentDoc, err := core.LoadYAMLMap(filePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load current document")
	}

	// Resolve top-level $ref
	if config.Ref != nil && !config.Ref.IsEmpty() {
		config.SetRefMetadata(filePath, projectRoot)
		if err := config.WithRef.ResolveAndMergeNode(
			ctx,
			config.Ref,
			config,
			currentDoc,
			ref.ModeMerge,
		); err != nil {
			return nil, errors.Wrap(err, "failed to resolve top-level $ref")
		}
	}

	// Resolve input schema $ref
	inSchema := config.InputSchema
	if inSchema != nil && !inSchema.Ref.IsEmpty() {
		if err := inSchema.ResolveRef(ctx, currentDoc, projectRoot, filePath); err != nil {
			return nil, errors.Wrap(err, "failed to resolve input schema $ref")
		}
	}

	// Resolve output schema $ref
	outSchema := config.OutputSchema
	if outSchema != nil && !outSchema.Ref.IsEmpty() {
		if err := outSchema.ResolveRef(ctx, currentDoc, projectRoot, filePath); err != nil {
			return nil, errors.Wrap(err, "failed to resolve output schema $ref")
		}
	}

	return config, nil
}

// Validate validates the tool configuration
func (t *Config) Validate() error {
	v := schema.NewCompositeValidator(
		schema.NewCWDValidator(t.metadata.CWD, t.ID),
		NewExecuteValidator(t),
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

func IsTypeScript(path string) bool {
	ext := filepath.Ext(path)
	return strings.EqualFold(ext, ".ts")
}

func FindConfig(tools []Config, toolID string) (*Config, error) {
	for i := range tools {
		if tools[i].ID == toolID {
			return &tools[i], nil
		}
	}
	return nil, fmt.Errorf("tool not found")
}
