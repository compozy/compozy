package task

import (
	"context"
	"fmt"
	"path/filepath"

	"dario.cat/mergo"
	"gopkg.in/yaml.v3"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/ref"
	"github.com/pkg/errors"
)

type Type string

const (
	TaskTypeBasic    Type = "basic"
	TaskTypeDecision Type = "decision"
)

type ExecutorType string

const (
	ExecutorAgent ExecutorType = "agent"
	ExecutorTool  ExecutorType = "tool"
)

type Executor struct {
	ref.WithRef
	Type ExecutorType `json:"type" yaml:"type"`
	Ref  any          `json:"$ref" yaml:"$ref" is_ref:"true"`

	// Basic task properties
	Action string `json:"action,omitempty" yaml:"action,omitempty"`

	agent    *agent.Config
	tool     *tool.Config
	metadata *core.ConfigMetadata
}

func (e *Executor) SetMetadata(metadata *core.ConfigMetadata) {
	e.metadata = metadata
}

func (e *Executor) GetMetadata() *core.ConfigMetadata {
	return e.metadata
}

// GetAgent returns the resolved agent configuration if the executor type is agent
func (e *Executor) GetAgent() (*agent.Config, error) {
	if e.Type == ExecutorAgent && e.agent != nil {
		return e.agent, nil
	}
	return nil, fmt.Errorf("executor type is not agent")
}

// GetTool returns the resolved tool configuration if the executor type is tool
func (e *Executor) GetTool() (*tool.Config, error) {
	if e.Type == ExecutorTool && e.tool != nil {
		return e.tool, nil
	}
	return nil, fmt.Errorf("executor type is not tool")
}

// ResolveRef resolves all references within the executor configuration
func (e *Executor) ResolveRef(ctx context.Context, currentDoc map[string]any, projectRoot, filePath string) error {
	if e == nil {
		return nil
	}
	if e.Ref == nil {
		return nil // Don't error for empty ref, just skip resolution
	}
	// Ensure we have metadata to work with
	if e.metadata == nil {
		return errors.New("executor metadata is not set")
	}
	e.SetRefMetadata(filePath, projectRoot)
	resolvedValue, err := e.ResolveRefWithInlineData(ctx, e.Ref, map[string]any{}, currentDoc)
	if err != nil {
		return errors.Wrap(err, "failed to resolve executor reference")
	}
	// Convert the resolved value to YAML and unmarshal into the appropriate config type
	yamlData, err := yaml.Marshal(resolvedValue)
	if err != nil {
		return errors.Wrap(err, "failed to marshal resolved value to YAML")
	}
	switch e.Type {
	case ExecutorAgent:
		var agentConfig agent.Config
		if err := yaml.Unmarshal(yamlData, &agentConfig); err != nil {
			return errors.Wrap(err, "failed to unmarshal resolved value to agent config")
		}
		// Set metadata for the resolved agent config
		// Use the resolved reference path to determine the correct CWD
		agentFilePath := filePath
		executorRefMetadata := e.GetRefMetadata()
		if executorRefMetadata != nil && executorRefMetadata.RefPath != "" {
			agentFilePath = executorRefMetadata.RefPath
		}
		agentDir := filepath.Dir(agentFilePath)
		agentCWD, err := core.CWDFromPath(agentDir)
		if err != nil {
			return errors.Wrap(err, "failed to create CWD for agent config")
		}
		metadata := &core.ConfigMetadata{
			CWD:         agentCWD,
			FilePath:    agentFilePath,
			ProjectRoot: projectRoot,
		}
		agentConfig.SetMetadata(metadata)
		// Resolve any references within the agent config
		if err := agentConfig.ResolveRef(ctx, currentDoc, projectRoot, agentFilePath); err != nil {
			return errors.Wrap(err, "failed to resolve agent config references")
		}
		e.agent = &agentConfig
	case ExecutorTool:
		var toolConfig tool.Config
		if err := yaml.Unmarshal(yamlData, &toolConfig); err != nil {
			return errors.Wrap(err, "failed to unmarshal resolved value as tool config")
		}
		// Set metadata for the resolved tool config
		// Use the resolved reference path to determine the correct CWD
		toolFilePath := filePath
		executorRefMetadata := e.GetRefMetadata()
		if executorRefMetadata != nil && executorRefMetadata.RefPath != "" {
			toolFilePath = executorRefMetadata.RefPath
		}
		toolDir := filepath.Dir(toolFilePath)
		toolCWD, err := core.CWDFromPath(toolDir)
		if err != nil {
			return errors.Wrap(err, "failed to create CWD for tool config")
		}
		metadata := &core.ConfigMetadata{
			CWD:         toolCWD,
			FilePath:    toolFilePath,
			ProjectRoot: projectRoot,
		}
		toolConfig.SetMetadata(metadata)
		// Resolve any references within the tool config
		if err := toolConfig.ResolveRef(ctx, currentDoc, projectRoot, toolFilePath); err != nil {
			return errors.Wrap(err, "failed to resolve tool config references")
		}
		e.tool = &toolConfig
	default:
		return fmt.Errorf("unknown executor type: %s", e.Type)
	}
	return nil
}

// -----------------------------------------------------------------------------
// TaskConfig
// -----------------------------------------------------------------------------

type Config struct {
	ref.WithRef
	Ref       any                      `json:"$ref,omitempty"       yaml:"$ref,omitempty"       is_ref:"true"`
	ID        string                   `json:"id,omitempty"         yaml:"id,omitempty"`
	Executor  Executor                 `json:"executor" yaml:"executor"`
	Type      Type                     `json:"type,omitempty"       yaml:"type,omitempty"`
	OnSuccess *SuccessTransitionConfig `json:"on_success,omitempty" yaml:"on_success,omitempty"`
	OnError   *ErrorTransitionConfig   `json:"on_error,omitempty"   yaml:"on_error,omitempty"`
	Final     bool                     `json:"final,omitempty"      yaml:"final,omitempty"`
	With      *core.Input              `json:"with,omitempty"       yaml:"with,omitempty"`
	Env       core.EnvMap              `json:"env,omitempty"        yaml:"env,omitempty"`

	// Decision task properties
	Condition string            `json:"condition,omitempty" yaml:"condition,omitempty"`
	Routes    map[string]string `json:"routes,omitempty"    yaml:"routes,omitempty"`

	metadata *core.ConfigMetadata
}

func (t *Config) Component() core.ConfigType {
	return core.ConfigTask
}

func (t *Config) GetCWD() *core.CWD {
	return t.metadata.CWD
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

// ResolveRef resolves all references within the task configuration, including top-level $ref
func (t *Config) ResolveRef(ctx context.Context, currentDoc map[string]any, projectRoot, filePath string) error {
	if t == nil {
		return nil
	}
	// Resolve top-level $ref if present
	if t.Ref != nil {
		t.SetRefMetadata(filePath, projectRoot)
		if err := t.ResolveAndMergeReferences(ctx, t, currentDoc, ref.ModeMerge); err != nil {
			return errors.Wrap(err, "failed to resolve top-level $ref")
		}
		// After resolving the top-level $ref, update the task's metadata to reflect the new file path
		refMetadata := t.GetRefMetadata()
		if refMetadata != nil && refMetadata.RefPath != "" {
			// Update the task's CWD to be based on the referenced file's directory
			refDir := filepath.Dir(refMetadata.RefPath)
			refCWD, err := core.CWDFromPath(refDir)
			if err != nil {
				return errors.Wrap(err, "failed to create CWD from referenced file path")
			}
			// Update the task's metadata with the new CWD and file path
			t.metadata.CWD = refCWD
			t.metadata.FilePath = refMetadata.RefPath
		}
	}
	// Set metadata for executor and resolve its references
	// Use the task's current metadata which should now be updated after reference resolution
	t.Executor.SetMetadata(t.metadata)

	// Determine the correct file path for executor reference resolution
	executorFilePath := filePath
	refMetadata := t.GetRefMetadata()
	if refMetadata != nil && refMetadata.RefPath != "" {
		// If the task was loaded via $ref, use the task file's path for executor resolution
		executorFilePath = refMetadata.RefPath
	}

	if err := t.Executor.ResolveRef(ctx, currentDoc, projectRoot, executorFilePath); err != nil {
		return errors.Wrap(err, "failed to resolve executor $ref")
	}
	// Resolve task input (With) $ref
	if t.With != nil {
		if err := t.With.ResolveRef(ctx, currentDoc, projectRoot, executorFilePath); err != nil {
			return errors.Wrap(err, "failed to resolve task input (with) $ref")
		}
	}
	return nil
}

func Load(ctx context.Context, cwd *core.CWD, projectRoot string, filePath string) (*Config, error) {
	config, err := core.LoadConfig[*Config](ctx, cwd, projectRoot, filePath)
	if err != nil {
		return nil, err
	}
	if string(config.Type) == "" {
		config.Type = TaskTypeBasic
	}
	// Get the resolved absolute path for loading the current document
	resolvedFilePath, err := core.ResolvedPath(cwd, filePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve file path")
	}
	currentDoc, err := core.LoadYAMLMap(resolvedFilePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load current document")
	}
	// Resolve all references using the resolved absolute file path
	if err := config.ResolveRef(ctx, currentDoc, projectRoot, resolvedFilePath); err != nil {
		return nil, err
	}
	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, "failed to validate task config")
	}
	return config, nil
}

func (t *Config) Validate() error {
	v := schema.NewCompositeValidator(
		schema.NewCWDValidator(t.metadata.CWD, t.ID),
		NewTaskTypeValidator(t),
		NewExecutorValidator(t),
	)
	return v.Validate()
}

func (t *Config) ValidateParams(input *core.Input) error {
	// Note: Parameter validation should be done against the schema from the referenced
	// agent or tool, not at the task level. This method is kept for interface compatibility
	// but actual validation should happen when resolving the executor reference.
	return nil
}

func (t *Config) Merge(other any) error {
	otherConfig, ok := other.(*Config)
	if !ok {
		return fmt.Errorf("failed to merge task configs: %w", errors.New("invalid type for merge"))
	}
	err := mergo.Merge(t, otherConfig, mergo.WithOverride)
	if err != nil {
		return err
	}
	return nil
}

func FindConfig(tasks []Config, taskID string) (*Config, error) {
	for i := range tasks {
		if tasks[i].ID == taskID {
			return &tasks[i], nil
		}
	}
	return nil, fmt.Errorf("task not found")
}
