package workflow

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/task"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/internal/validate"
)

// Builder constructs workflow configurations through a fluent API while capturing validation issues.
type Builder struct {
	config *engineworkflow.Config
	errors []error
}

// New creates a workflow builder initialized with the provided identifier.
func New(id string) *Builder {
	trimmed := strings.TrimSpace(id)
	return &Builder{
		config: &engineworkflow.Config{
			ID:     trimmed,
			Agents: make([]agent.Config, 0),
			Tasks:  make([]task.Config, 0),
		},
		errors: make([]error, 0),
	}
}

// WithDescription registers a human-readable description for the workflow.
func (b *Builder) WithDescription(desc string) *Builder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(desc)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("description cannot be empty"))
		return b
	}
	b.config.Description = trimmed
	return b
}

// AddAgent appends the provided agent configuration to the workflow definition.
func (b *Builder) AddAgent(agentCfg *agent.Config) *Builder {
	if b == nil {
		return nil
	}
	if agentCfg == nil {
		b.errors = append(b.errors, fmt.Errorf("agent cannot be nil"))
		return b
	}
	id := strings.TrimSpace(agentCfg.ID)
	if id == "" {
		b.errors = append(b.errors, fmt.Errorf("agent id cannot be empty"))
		return b
	}
	cloned, err := core.DeepCopy(*agentCfg)
	if err != nil {
		b.errors = append(b.errors, fmt.Errorf("failed to copy agent config: %w", err))
		return b
	}
	cloned.ID = id
	b.config.Agents = append(b.config.Agents, cloned)
	return b
}

// AddTask registers a task configuration with the workflow builder.
func (b *Builder) AddTask(taskCfg *task.Config) *Builder {
	if b == nil {
		return nil
	}
	if taskCfg == nil {
		b.errors = append(b.errors, fmt.Errorf("task cannot be nil"))
		return b
	}
	id := strings.TrimSpace(taskCfg.ID)
	if id == "" {
		b.errors = append(b.errors, fmt.Errorf("task id cannot be empty"))
		return b
	}
	cloned, err := core.DeepCopy(*taskCfg)
	if err != nil {
		b.errors = append(b.errors, fmt.Errorf("failed to copy task config: %w", err))
		return b
	}
	cloned.ID = id
	b.config.Tasks = append(b.config.Tasks, cloned)
	return b
}

// WithInput sets the workflow input schema used for validation and defaulting.
func (b *Builder) WithInput(schema *schema.Schema) *Builder {
	if b == nil {
		return nil
	}
	b.config.Opts.InputSchema = schema
	return b
}

// WithOutputs configures named workflow outputs using template expressions.
func (b *Builder) WithOutputs(outputs map[string]string) *Builder {
	if b == nil {
		return nil
	}
	if outputs == nil {
		b.config.Outputs = nil
		return b
	}
	result := make(core.Output, len(outputs))
	for rawKey, value := range outputs {
		key := strings.TrimSpace(rawKey)
		if key == "" {
			b.errors = append(b.errors, fmt.Errorf("output key cannot be empty"))
			continue
		}
		result[key] = value
	}
	out := result
	b.config.Outputs = &out
	return b
}

// Build validates the accumulated configuration and returns a workflow config when successful.
func (b *Builder) Build(ctx context.Context) (*engineworkflow.Config, error) {
	if b == nil {
		return nil, fmt.Errorf("workflow builder is required")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	log.Debug("building workflow configuration", "workflow", b.config.ID, "tasks", len(b.config.Tasks))

	collected := make([]error, 0, len(b.errors)+4)
	collected = append(collected, b.errors...)

	if err := validate.ID(ctx, b.config.ID); err != nil {
		collected = append(collected, fmt.Errorf("workflow id is invalid: %w", err))
	}
	if len(b.config.Tasks) == 0 {
		collected = append(collected, fmt.Errorf("at least one task must be registered"))
	}
	duplicates := findDuplicateTaskIDs(b.config.Tasks)
	if len(duplicates) > 0 {
		collected = append(collected, fmt.Errorf("duplicate task ids found: %s", strings.Join(duplicates, ", ")))
	}

	filtered := make([]error, 0, len(collected))
	for _, err := range collected {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	if len(filtered) > 0 {
		return nil, &sdkerrors.BuildError{Errors: filtered}
	}

	cloned, err := core.DeepCopy(b.config)
	if err != nil {
		return nil, fmt.Errorf("failed to clone workflow config: %w", err)
	}
	return cloned, nil
}

func findDuplicateTaskIDs(tasks []task.Config) []string {
	if len(tasks) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(tasks))
	dupes := make([]string, 0)
	for i := range tasks {
		cfg := &tasks[i]
		id := strings.TrimSpace(cfg.ID)
		if id == "" {
			continue
		}
		if seen[id] {
			if !containsString(dupes, id) {
				dupes = append(dupes, id)
			}
			continue
		}
		seen[id] = true
	}
	return dupes
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
