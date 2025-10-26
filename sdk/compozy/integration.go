package compozy

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/pkg/logger"
)

// loadProjectIntoEngine registers the project and associated workflows within
// the engine resource store so they are available for execution.
func (c *Compozy) loadProjectIntoEngine(ctx context.Context, proj *project.Config) error {
	if c == nil {
		return fmt.Errorf("compozy instance is required")
	}
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	if proj == nil {
		return fmt.Errorf("project config is required")
	}
	store := c.ResourceStore()
	if store == nil {
		return fmt.Errorf("resource store is not configured")
	}
	log := logger.FromContext(ctx)
	log.Info("loading project into engine", "project", proj.Name)
	if err := proj.Validate(ctx); err != nil {
		return fmt.Errorf("project validation failed: %w", err)
	}
	if err := proj.IndexToResourceStore(ctx, store); err != nil {
		return fmt.Errorf("failed to index project resources: %w", err)
	}
	c.mu.RLock()
	projectName := proj.Name
	order := append([]string(nil), c.workflowOrder...)
	c.mu.RUnlock()
	for _, id := range order {
		c.mu.RLock()
		wf := c.workflowByID[id]
		c.mu.RUnlock()
		if wf == nil {
			continue
		}
		if err := wf.Validate(ctx); err != nil {
			return fmt.Errorf("workflow %s validation failed: %w", id, err)
		}
		if err := wf.IndexToResourceStore(ctx, projectName, store); err != nil {
			return fmt.Errorf("failed to index workflow %s: %w", id, err)
		}
	}
	log.Info("project registered in engine", "project", proj.Name, "workflows", len(order))
	return nil
}
