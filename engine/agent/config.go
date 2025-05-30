package agent

import (
	"context"
	"fmt"

	"dario.cat/mergo"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/ref"
	"github.com/pkg/errors"
)

type ActionConfig struct {
	ref.WithRef
	Ref          any                  `json:"$ref,omitempty"   yaml:"$ref,omitempty"   is_ref:"true"`
	ID           string               `json:"id"               yaml:"id"`
	Prompt       string               `json:"prompt"           yaml:"prompt"           validate:"required"`
	InputSchema  *schema.InputSchema  `json:"input,omitempty"  yaml:"input,omitempty"`
	OutputSchema *schema.OutputSchema `json:"output,omitempty" yaml:"output,omitempty"`
	With         *core.Input          `json:"with,omitempty"   yaml:"with,omitempty"`

	metadata *core.ConfigMetadata
}

func (a *ActionConfig) SetMetadata(metadata *core.ConfigMetadata) {
	a.metadata = metadata
}

func (a *ActionConfig) GetMetadata() *core.ConfigMetadata {
	return a.metadata
}

// GetInputSchema implements schema.SchemaContainer interface
func (a *ActionConfig) GetInputSchema() *schema.InputSchema {
	return a.InputSchema
}

// SetInputSchema implements schema.SchemaContainer interface
func (a *ActionConfig) SetInputSchema(inputSchema *schema.InputSchema) {
	a.InputSchema = inputSchema
}

// GetOutputSchema implements schema.SchemaContainer interface
func (a *ActionConfig) GetOutputSchema() *schema.OutputSchema {
	return a.OutputSchema
}

// SetOutputSchema implements schema.SchemaContainer interface
func (a *ActionConfig) SetOutputSchema(outputSchema *schema.OutputSchema) {
	a.OutputSchema = outputSchema
}

func (a *ActionConfig) Validate() error {
	var cwd *core.CWD
	if a.metadata != nil {
		cwd = a.metadata.CWD
	}
	v := schema.NewCompositeValidator(
		schema.NewCWDValidator(cwd, a.ID),
		schema.NewStructValidator(a),
	)
	return v.Validate()
}

func (a *ActionConfig) ValidateParams(input map[string]any) error {
	return schema.NewParamsValidator(input, a.InputSchema.Schema, a.ID).Validate()
}

// ResolveRef resolves all references within the action configuration, including top-level $ref
func (a *ActionConfig) ResolveRef(ctx context.Context, currentDoc map[string]any, projectRoot, filePath string) error {
	if a == nil {
		return nil
	}
	// Resolve all references in a single call
	if err := schema.ResolveConfigSchemas(
		ctx,
		&a.WithRef,
		a.Ref,
		a,
		currentDoc,
		projectRoot,
		filePath,
		a,
	); err != nil {
		return errors.Wrapf(err, "failed to resolve action reference for action %s", a.ID)
	}
	// Resolve action input (With) $ref
	if a.With != nil {
		if err := a.With.ResolveRef(ctx, currentDoc, projectRoot, filePath); err != nil {
			return errors.Wrapf(err, "failed to resolve action input (with) $ref for action %s", a.ID)
		}
	}
	return nil
}

type Config struct {
	ref.WithRef
	Ref          any                  `json:"$ref,omitempty"         yaml:"$ref,omitempty"         is_ref:"true"`
	ID           string               `json:"id"                     yaml:"id"                validate:"required"`
	Instructions string               `json:"instructions"           yaml:"instructions"      validate:"required"`
	Config       ProviderConfig       `json:"config"                 yaml:"config"            validate:"required"`
	Tools        []tool.Config        `json:"tools,omitempty"        yaml:"tools,omitempty"`
	Actions      []*ActionConfig      `json:"actions,omitempty"      yaml:"actions,omitempty"`
	InputSchema  *schema.InputSchema  `json:"input,omitempty"        yaml:"input,omitempty"`
	OutputSchema *schema.OutputSchema `json:"output,omitempty"       yaml:"output,omitempty"`
	With         *core.Input          `json:"with,omitempty"         yaml:"with,omitempty"`
	Env          core.EnvMap          `json:"env,omitempty"          yaml:"env,omitempty"`

	metadata *core.ConfigMetadata
}

func (a *Config) Component() core.ConfigType {
	return core.ConfigAgent
}

func (a *Config) GetCWD() *core.CWD {
	return a.metadata.CWD
}

func (a *Config) GetEnv() *core.EnvMap {
	if a.Env == nil {
		a.Env = make(core.EnvMap)
		return &a.Env
	}
	return &a.Env
}

func (a *Config) GetInput() *core.Input {
	if a.With == nil {
		a.With = &core.Input{}
	}
	return a.With
}

func (a *Config) GetMetadata() *core.ConfigMetadata {
	return a.metadata
}

func (a *Config) SetMetadata(metadata *core.ConfigMetadata) {
	a.metadata = metadata
	for i := range a.Actions {
		if a.Actions[i] != nil {
			a.Actions[i].SetMetadata(metadata)
		}
	}
}

// GetInputSchema implements schema.SchemaContainer interface
func (a *Config) GetInputSchema() *schema.InputSchema {
	return a.InputSchema
}

// SetInputSchema implements schema.SchemaContainer interface
func (a *Config) SetInputSchema(inputSchema *schema.InputSchema) {
	a.InputSchema = inputSchema
}

// GetOutputSchema implements schema.SchemaContainer interface
func (a *Config) GetOutputSchema() *schema.OutputSchema {
	return a.OutputSchema
}

// SetOutputSchema implements schema.SchemaContainer interface
func (a *Config) SetOutputSchema(outputSchema *schema.OutputSchema) {
	a.OutputSchema = outputSchema
}

// ResolveRef resolves all references within the agent configuration, including top-level $ref
func (a *Config) ResolveRef(ctx context.Context, currentDoc map[string]any, projectRoot, filePath string) error {
	if a == nil {
		return nil
	}
	// Resolve all references in a single call
	if err := schema.ResolveConfigSchemas(
		ctx,
		&a.WithRef,
		a.Ref,
		a,
		currentDoc,
		projectRoot,
		filePath,
		a,
	); err != nil {
		return errors.Wrap(err, "failed to resolve references")
	}
	// Resolve provider config reference
	if err := loadProvider(ctx, &a.Config, currentDoc, projectRoot, filePath); err != nil {
		return err
	}
	// Resolve tool references
	for i := range a.Tools {
		if err := loadTool(ctx, &a.Tools[i], currentDoc, projectRoot, filePath); err != nil {
			return errors.Wrapf(err, "failed to resolve tool reference for tool %d", i)
		}
	}
	// Resolve action references
	for i := range a.Actions {
		if err := loadAction(ctx, a.Actions[i], currentDoc, projectRoot, filePath); err != nil {
			return err
		}
	}
	// Resolve input (With) $ref
	if a.With != nil {
		if err := a.With.ResolveRef(ctx, currentDoc, projectRoot, filePath); err != nil {
			return errors.Wrap(err, "failed to resolve agent input (with) $ref")
		}
	}
	return nil
}

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
	if err := config.ResolveRef(ctx, currentDoc, projectRoot, filePath); err != nil {
		return nil, err
	}
	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, "failed to validate agent config")
	}
	return config, nil
}

func (a *Config) Validate() error {
	var cwd *core.CWD
	if a.metadata != nil {
		cwd = a.metadata.CWD
	}
	v := schema.NewCompositeValidator(
		schema.NewCWDValidator(cwd, a.ID),
		NewActionsValidator(a),
		schema.NewStructValidator(a),
	)
	return v.Validate()
}

func (a *Config) ValidateParams(input *core.Input) error {
	if a.InputSchema == nil || input == nil {
		return nil
	}
	return schema.NewParamsValidator(*input, a.InputSchema.Schema, a.ID).Validate()
}

func (a *Config) Merge(other any) error {
	otherConfig, ok := other.(*Config)
	if !ok {
		return fmt.Errorf("failed to merge agent configs: %s", "invalid type for merge")
	}
	return mergo.Merge(a, otherConfig, mergo.WithOverride)
}

func FindConfig(agents []Config, agentID string) (*Config, error) {
	for i := range agents {
		if agents[i].ID == agentID {
			return &agents[i], nil
		}
	}
	return nil, fmt.Errorf("agent not found")
}

func loadTool(
	ctx context.Context,
	tool *tool.Config,
	currentDoc map[string]any,
	projectRoot, filePath string,
) error {
	if tool == nil {
		return nil
	}
	if err := tool.ResolveRef(ctx, currentDoc, projectRoot, filePath); err != nil {
		return errors.Wrapf(err, "failed to resolve tool references for tool %s", tool.ID)
	}
	return nil
}

func loadAction(
	ctx context.Context,
	action *ActionConfig,
	currentDoc map[string]any,
	projectRoot, filePath string,
) error {
	if action == nil {
		return nil
	}
	metadata := &core.ConfigMetadata{
		CWD:         action.metadata.CWD,
		FilePath:    filePath,
		ProjectRoot: projectRoot,
	}
	action.SetMetadata(metadata)
	return action.ResolveRef(ctx, currentDoc, projectRoot, filePath)
}

func loadProvider(
	ctx context.Context,
	provider *ProviderConfig,
	currentDoc map[string]any,
	projectRoot, filePath string,
) error {
	if provider == nil {
		return nil
	}
	// Resolve all references using the standardized method
	return provider.ResolveRef(ctx, currentDoc, projectRoot, filePath)
}
