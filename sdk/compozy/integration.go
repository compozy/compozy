package compozy

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
)

const sdkMetaSource = "sdk"

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
	log := logger.FromContext(ctx)
	log.Info("loading project into engine", "project", proj.Name)
	if err := c.RegisterProject(ctx, proj); err != nil {
		return err
	}
	c.mu.RLock()
	order := append([]string(nil), c.workflowOrder...)
	c.mu.RUnlock()
	for _, id := range order {
		c.mu.RLock()
		wf := c.workflowByID[id]
		c.mu.RUnlock()
		if wf == nil {
			continue
		}
		if err := c.RegisterWorkflow(ctx, wf); err != nil {
			return err
		}
	}
	log.Info("project registered in engine", "project", proj.Name, "workflows", len(order))
	return nil
}

// RegisterProject validates and registers the provided project configuration in the resource store.
func (c *Compozy) RegisterProject(ctx context.Context, proj *project.Config) error {
	if c == nil {
		return fmt.Errorf("compozy instance is required")
	}
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	if proj == nil {
		return fmt.Errorf("project config is required")
	}
	name := strings.TrimSpace(proj.Name)
	if name == "" {
		return fmt.Errorf("project name is required for registration")
	}
	store := c.ResourceStore()
	if store == nil {
		return fmt.Errorf("resource store is not configured")
	}
	if err := proj.Validate(ctx); err != nil {
		return fmt.Errorf("project %s validation failed: %w", name, err)
	}
	key := resources.ResourceKey{Project: name, Type: resources.ResourceProject, ID: name}
	if _, _, err := store.Get(ctx, key); err == nil {
		return fmt.Errorf("project %s already registered", name)
	} else if err != nil && !errors.Is(err, resources.ErrNotFound) {
		return fmt.Errorf("inspect project %s registration state: %w", name, err)
	}
	if _, err := store.Put(ctx, key, proj); err != nil {
		return fmt.Errorf("store project %s: %w", name, err)
	}
	if err := resources.WriteMeta(ctx, store, name, resources.ResourceProject, name, sdkMetaSource, "sdk"); err != nil {
		return fmt.Errorf("write project %s metadata: %w", name, err)
	}
	c.mu.Lock()
	c.project = proj
	c.mu.Unlock()
	logger.FromContext(ctx).Info("project registered", "project", name)
	return nil
}

// RegisterWorkflow validates and registers a workflow configuration in the resource store.
func (c *Compozy) RegisterWorkflow(ctx context.Context, wf *workflow.Config) error {
	if c == nil {
		return fmt.Errorf("compozy instance is required")
	}
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	if wf == nil {
		return fmt.Errorf("workflow config is required")
	}
	store := c.ResourceStore()
	if store == nil {
		return fmt.Errorf("resource store is not configured")
	}
	c.mu.RLock()
	projectName := ""
	if c.project != nil {
		projectName = strings.TrimSpace(c.project.Name)
	}
	c.mu.RUnlock()
	if projectName == "" {
		return fmt.Errorf("project name is required for workflow registration")
	}
	id := strings.TrimSpace(wf.ID)
	if id == "" {
		return fmt.Errorf("workflow id is required for registration")
	}
	if err := wf.Validate(ctx); err != nil {
		return fmt.Errorf("workflow %s validation failed: %w", id, err)
	}
	key := resources.ResourceKey{Project: projectName, Type: resources.ResourceWorkflow, ID: id}
	if _, _, err := store.Get(ctx, key); err == nil {
		return fmt.Errorf("workflow %s already registered", id)
	} else if err != nil && !errors.Is(err, resources.ErrNotFound) {
		return fmt.Errorf("inspect workflow %s registration state: %w", id, err)
	}
	if _, err := store.Put(ctx, key, wf); err != nil {
		return fmt.Errorf("store workflow %s: %w", id, err)
	}
	if err := resources.WriteMeta(ctx, store, projectName, resources.ResourceWorkflow, id, sdkMetaSource, "sdk"); err != nil {
		return fmt.Errorf("write workflow %s metadata: %w", id, err)
	}
	logger.FromContext(ctx).Info("workflow registered", "project", projectName, "workflow", id)
	return nil
}
