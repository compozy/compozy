package workflow

import (
	"context"
	"fmt"

	"dario.cat/mergo"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/pkg/errors"
)

type Opts struct {
	OnError     *task.ErrorTransitionConfig `json:"on_error,omitempty" yaml:"on_error,omitempty"`
	InputSchema *schema.InputSchema         `json:"input,omitempty"    yaml:"input,omitempty"`
}

type Config struct {
	ID          string         `json:"id"                    yaml:"id"`
	Tasks       []task.Config  `json:"tasks"                 yaml:"tasks"`
	Opts        Opts           `json:"config"               yaml:"config"`
	Version     string         `json:"version,omitempty"     yaml:"version,omitempty"`
	Description string         `json:"description,omitempty" yaml:"description,omitempty"`
	Author      *core.Author   `json:"author,omitempty"      yaml:"author,omitempty"`
	Tools       []tool.Config  `json:"tools,omitempty"       yaml:"tools,omitempty"`
	Agents      []agent.Config `json:"agents,omitempty"      yaml:"agents,omitempty"`
	Env         core.EnvMap    `json:"env,omitempty"         yaml:"env,omitempty"`

	metadata *core.ConfigMetadata
}

func (w *Config) Component() core.ConfigType {
	return core.ConfigWorkflow
}

func (w *Config) GetCWD() *core.CWD {
	if w.metadata != nil {
		return w.metadata.CWD
	}
	return nil
}

func (w *Config) GetEnv() *core.EnvMap {
	if w.Env == nil {
		w.Env = make(core.EnvMap)
		return &w.Env
	}
	return &w.Env
}

func (w *Config) GetInput() *core.Input {
	return &core.Input{}
}

func (w *Config) GetMetadata() *core.ConfigMetadata {
	return w.metadata
}

func (w *Config) SetMetadata(metadata *core.ConfigMetadata) {
	w.metadata = metadata
	// Set metadata for all child components - create copies to avoid shared state
	for i := range w.Tasks {
		taskMetadata := &core.ConfigMetadata{
			CWD:         metadata.CWD,
			FilePath:    metadata.FilePath,
			ProjectRoot: metadata.ProjectRoot,
		}
		w.Tasks[i].SetMetadata(taskMetadata)
	}
	for i := range w.Agents {
		agentMetadata := &core.ConfigMetadata{
			CWD:         metadata.CWD,
			FilePath:    metadata.FilePath,
			ProjectRoot: metadata.ProjectRoot,
		}
		w.Agents[i].SetMetadata(agentMetadata)
	}
	for i := range w.Tools {
		toolMetadata := &core.ConfigMetadata{
			CWD:         metadata.CWD,
			FilePath:    metadata.FilePath,
			ProjectRoot: metadata.ProjectRoot,
		}
		w.Tools[i].SetMetadata(toolMetadata)
	}
}

func (w *Config) ResolveRef(ctx context.Context, currentDoc map[string]any, projectRoot, filePath string) error {
	if w == nil {
		return nil
	}
	// Resolve task references
	for i := range w.Tasks {
		if err := w.Tasks[i].ResolveRef(ctx, currentDoc, projectRoot, filePath); err != nil {
			return errors.Wrapf(err, "failed to resolve task reference for task %s", w.Tasks[i].ID)
		}
	}
	// Resolve agent references
	for i := range w.Agents {
		if err := w.Agents[i].ResolveRef(ctx, currentDoc, projectRoot, filePath); err != nil {
			return errors.Wrapf(err, "failed to resolve agent reference for agent %s", w.Agents[i].ID)
		}
	}
	// Resolve tool references
	for i := range w.Tools {
		if err := w.Tools[i].ResolveRef(ctx, currentDoc, projectRoot, filePath); err != nil {
			return errors.Wrapf(err, "failed to resolve tool reference for tool %s", w.Tools[i].ID)
		}
	}
	// Resolve input schema reference
	if w.Opts.InputSchema != nil {
		if err := w.Opts.InputSchema.ResolveRef(ctx, currentDoc, projectRoot, filePath); err != nil {
			return errors.Wrap(err, "failed to resolve workflow input schema reference")
		}
	}
	// Collect referenced tools and agents from tasks
	if err := w.collectReferencedComponents(); err != nil {
		return errors.Wrap(err, "failed to collect referenced components from tasks")
	}
	return nil
}

// collectReferencedComponents collects tools and agents referenced by tasks
// and adds them to the workflow's Tools and Agents arrays if not already present
func (w *Config) collectReferencedComponents() error {
	// Track existing IDs to avoid duplicates
	existingToolIDs := make(map[string]bool)
	for _, tool := range w.Tools {
		existingToolIDs[tool.ID] = true
	}
	existingAgentIDs := make(map[string]bool)
	for _, agent := range w.Agents {
		existingAgentIDs[agent.ID] = true
	}
	// Collect tools and agents from task executors
	for i := range w.Tasks {
		taskConfig := &w.Tasks[i]
		if taskConfig.Executor.Type == task.ExecutorTool {
			if tool, err := taskConfig.Executor.GetTool(); err == nil && tool != nil {
				if !existingToolIDs[tool.ID] {
					w.Tools = append(w.Tools, *tool)
					existingToolIDs[tool.ID] = true
				}
			}
		}
		if taskConfig.Executor.Type == task.ExecutorAgent {
			if agent, err := taskConfig.Executor.GetAgent(); err == nil && agent != nil {
				if !existingAgentIDs[agent.ID] {
					w.Agents = append(w.Agents, *agent)
					existingAgentIDs[agent.ID] = true
				}
			}
		}
	}
	return nil
}

func Load(ctx context.Context, cwd *core.CWD, projectRoot string, filePath string) (*Config, error) {
	config, err := core.LoadConfig[*Config](ctx, cwd, projectRoot, filePath)
	if err != nil {
		return nil, err
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
	// Resolve all references - use resolvedFilePath so ref resolution can find relative files
	if err := config.ResolveRef(ctx, currentDoc, projectRoot, resolvedFilePath); err != nil {
		return nil, err
	}
	err = config.Validate()
	if err != nil {
		return nil, err
	}
	return config, nil
}

func (w *Config) Validate() error {
	var cwd *core.CWD
	if w.metadata != nil {
		cwd = w.metadata.CWD
	}
	v := schema.NewCompositeValidator(
		schema.NewCWDValidator(cwd, w.ID),
	)
	return v.Validate()
}

func (w *Config) ValidateParams(input *core.Input) error {
	if input == nil {
		return nil
	}
	inputSchema := w.Opts.InputSchema
	if inputSchema == nil {
		return nil
	}
	return schema.NewParamsValidator(*input, inputSchema.Schema, w.ID).Validate()
}

func (w *Config) Merge(other any) error {
	otherConfig, ok := other.(*Config)
	if !ok {
		return fmt.Errorf("failed to merge workflow configs: %w", fmt.Errorf("invalid type for merge"))
	}
	return mergo.Merge(w, otherConfig, mergo.WithOverride)
}

func WorkflowsFromProject(ctx context.Context, projectConfig *project.Config) ([]*Config, error) {
	cwd := projectConfig.GetCWD()
	projectRoot := projectConfig.GetMetadata().ProjectRoot
	var ws []*Config
	for _, wf := range projectConfig.Workflows {
		config, err := Load(ctx, cwd, projectRoot, wf.Source)
		if err != nil {
			return nil, err
		}
		ws = append(ws, config)
	}
	return ws, nil
}

func FindConfig(workflows []*Config, workflowID string) (*Config, error) {
	for _, wf := range workflows {
		if wf.ID == workflowID {
			return wf, nil
		}
	}
	return nil, fmt.Errorf("workflow not found")
}
