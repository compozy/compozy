package task

import (
	"context"
	"fmt"

	"dario.cat/mergo"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
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
	Ref  ref.Node     `json:"$ref" yaml:"$ref"`
}

type Config struct {
	ref.WithRef
	Ref       *ref.Node                `json:"$ref,omitempty"       yaml:"$ref,omitempty"`
	ID        string                   `json:"id,omitempty"         yaml:"id,omitempty"`
	Executor  Executor                 `json:"executor" yaml:"executor"`
	Type      Type                     `json:"type,omitempty"       yaml:"type,omitempty"`
	OnSuccess *SuccessTransitionConfig `json:"on_success,omitempty" yaml:"on_success,omitempty"`
	OnError   *ErrorTransitionConfig   `json:"on_error,omitempty"   yaml:"on_error,omitempty"`
	Final     bool                     `json:"final,omitempty"      yaml:"final,omitempty"`
	With      *core.Input              `json:"with,omitempty"       yaml:"with,omitempty"`
	Env       core.EnvMap              `json:"env,omitempty"        yaml:"env,omitempty"`

	// Basic task properties
	Action string `json:"action,omitempty" yaml:"action,omitempty"`

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

func Load(ctx context.Context, cwd *core.CWD, projectRoot string, filePath string) (*Config, error) {
	config, err := core.LoadConfig[*Config](ctx, cwd, projectRoot, filePath)
	if err != nil {
		return nil, err
	}
	if string(config.Type) == "" {
		config.Type = TaskTypeBasic
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

	// Resolve executor $ref
	if !config.Executor.Ref.IsEmpty() {
		config.Executor.SetRefMetadata(filePath, projectRoot)
		if err := config.Executor.WithRef.ResolveAndMergeNode(
			ctx,
			&config.Executor.Ref,
			&config.Executor,
			currentDoc,
			ref.ModeMerge,
		); err != nil {
			return nil, errors.Wrap(err, "failed to resolve executor $ref")
		}
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
