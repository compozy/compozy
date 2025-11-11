package workflow

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/v2/internal/errors"
	"github.com/compozy/compozy/sdk/v2/internal/validate"
)

// New creates a workflow configuration using functional options
func New(ctx context.Context, id string, opts ...Option) (*engineworkflow.Config, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	log.Debug("creating workflow configuration", "workflow", id)
	cfg := &engineworkflow.Config{
		ID:     strings.TrimSpace(id),
		Agents: make([]agent.Config, 0),
		Tasks:  make([]task.Config, 0),
	}
	for _, opt := range opts {
		opt(cfg)
	}
	collected := make([]error, 0)
	if err := validateID(ctx, cfg); err != nil {
		collected = append(collected, err)
	}
	if err := validateTasks(ctx, cfg); err != nil {
		collected = append(collected, err)
	}
	if err := validateTaskDuplicates(cfg); err != nil {
		collected = append(collected, err)
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
	cloned, err := core.DeepCopy(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to clone workflow config: %w", err)
	}
	return cloned, nil
}

func validateID(ctx context.Context, cfg *engineworkflow.Config) error {
	cfg.ID = strings.TrimSpace(cfg.ID)
	if err := validate.ID(ctx, cfg.ID); err != nil {
		return fmt.Errorf("workflow id is invalid: %w", err)
	}
	return nil
}

func validateTasks(_ context.Context, cfg *engineworkflow.Config) error {
	if len(cfg.Tasks) == 0 {
		return fmt.Errorf("at least one task must be registered")
	}
	return nil
}

func validateTaskDuplicates(cfg *engineworkflow.Config) error {
	if len(cfg.Tasks) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(cfg.Tasks))
	dupes := make([]string, 0)
	for i := range cfg.Tasks {
		taskCfg := &cfg.Tasks[i]
		id := strings.TrimSpace(taskCfg.ID)
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
	if len(dupes) > 0 {
		return fmt.Errorf("duplicate task ids found: %s", strings.Join(dupes, ", "))
	}
	return nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
