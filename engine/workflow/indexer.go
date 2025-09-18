package workflow

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/schema"
)

// IndexToResourceStore publishes workflow-scoped resources (workflow, agents,
// tools, schemas, mcps) to the provided ResourceStore.
func (w *Config) IndexToResourceStore(ctx context.Context, project string, store resources.ResourceStore) error {
	if w == nil {
		return fmt.Errorf("nil workflow config")
	}
	if store == nil {
		return fmt.Errorf("resource store is required")
	}
	if project == "" {
		return fmt.Errorf("project name is required for indexing")
	}
	if w.ID == "" {
		return fmt.Errorf("workflow id is required for indexing")
	}
	if err := w.indexWorkflow(ctx, project, store); err != nil {
		return err
	}
	if err := w.indexAgents(ctx, project, store); err != nil {
		return err
	}
	if err := w.indexTools(ctx, project, store); err != nil {
		return err
	}
	if err := w.indexSchemas(ctx, project, store); err != nil {
		return err
	}
	if err := w.indexMCPs(ctx, project, store); err != nil {
		return err
	}
	return nil
}

func (w *Config) indexWorkflow(ctx context.Context, project string, store resources.ResourceStore) error {
	key := resources.ResourceKey{Project: project, Type: resources.ResourceWorkflow, ID: w.ID}
	if _, err := store.Put(ctx, key, w); err != nil {
		return fmt.Errorf("store put workflow '%s': %w", w.ID, err)
	}
	return nil
}

func (w *Config) indexAgents(ctx context.Context, project string, store resources.ResourceStore) error {
	for i := range w.Agents {
		ag := &w.Agents[i]
		if ag.ID == "" {
			continue
		}
		key := resources.ResourceKey{Project: project, Type: resources.ResourceAgent, ID: ag.ID}
		if _, err := store.Put(ctx, key, ag); err != nil {
			return fmt.Errorf("store put agent '%s': %w", ag.ID, err)
		}
	}
	return nil
}

func (w *Config) indexTools(ctx context.Context, project string, store resources.ResourceStore) error {
	for i := range w.Tools {
		tl := &w.Tools[i]
		if tl.ID == "" {
			continue
		}
		key := resources.ResourceKey{Project: project, Type: resources.ResourceTool, ID: tl.ID}
		if _, err := store.Put(ctx, key, tl); err != nil {
			return fmt.Errorf("store put tool '%s': %w", tl.ID, err)
		}
	}
	return nil
}

func (w *Config) indexSchemas(ctx context.Context, project string, store resources.ResourceStore) error {
	for i := range w.Schemas {
		sc := &w.Schemas[i]
		sid := schemaID(sc)
		if sid == "" {
			continue
		}
		key := resources.ResourceKey{Project: project, Type: resources.ResourceSchema, ID: sid}
		if _, err := store.Put(ctx, key, sc); err != nil {
			return fmt.Errorf("store put schema '%s': %w", sid, err)
		}
	}
	return nil
}

func (w *Config) indexMCPs(ctx context.Context, project string, store resources.ResourceStore) error {
	for i := range w.MCPs {
		mc := &w.MCPs[i]
		if mc.ID == "" {
			continue
		}
		key := resources.ResourceKey{Project: project, Type: resources.ResourceMCP, ID: mc.ID}
		if _, err := store.Put(ctx, key, mc); err != nil {
			return fmt.Errorf("store put mcp '%s': %w", mc.ID, err)
		}
	}
	return nil
}

func schemaID(s *schema.Schema) string {
	if s == nil {
		return ""
	}
	if v, ok := (*s)["id"]; ok {
		if str, ok2 := v.(string); ok2 {
			return str
		}
	}
	return ""
}
