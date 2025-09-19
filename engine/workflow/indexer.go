package workflow

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/pkg/logger"
)

const wfMetaSourceYAML = "yaml"

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
	prev := resources.GetMetaSource(ctx, store, project, resources.ResourceWorkflow, w.ID)
	if _, err := store.Put(ctx, key, w); err != nil {
		return fmt.Errorf("store put workflow '%s': %w", w.ID, err)
	}
	if prev != "" && prev != wfMetaSourceYAML {
		logger.FromContext(ctx).Warn(
			"yaml indexing overwrote existing resource",
			"project", project,
			"type", string(resources.ResourceWorkflow),
			"id", w.ID,
			"old_source", prev,
			"new_source", wfMetaSourceYAML,
		)
	}
	if err := resources.WriteMeta(
		ctx,
		store,
		project,
		resources.ResourceWorkflow,
		w.ID,
		wfMetaSourceYAML,
		"indexer",
	); err != nil {
		return err
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
		prev := resources.GetMetaSource(ctx, store, project, resources.ResourceAgent, ag.ID)
		if _, err := store.Put(ctx, key, ag); err != nil {
			return fmt.Errorf("store put agent '%s': %w", ag.ID, err)
		}
		if prev != "" && prev != wfMetaSourceYAML {
			logger.FromContext(ctx).Warn(
				"yaml indexing overwrote existing resource",
				"project", project,
				"type", string(resources.ResourceAgent),
				"id", ag.ID,
				"old_source", prev,
				"new_source", wfMetaSourceYAML,
			)
		}
		if err := resources.WriteMeta(
			ctx,
			store,
			project,
			resources.ResourceAgent,
			ag.ID,
			wfMetaSourceYAML,
			"indexer",
		); err != nil {
			return err
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
		prev := resources.GetMetaSource(ctx, store, project, resources.ResourceTool, tl.ID)
		if _, err := store.Put(ctx, key, tl); err != nil {
			return fmt.Errorf("store put tool '%s': %w", tl.ID, err)
		}
		if prev != "" && prev != wfMetaSourceYAML {
			logger.FromContext(ctx).Warn(
				"yaml indexing overwrote existing resource",
				"project", project,
				"type", string(resources.ResourceTool),
				"id", tl.ID,
				"old_source", prev,
				"new_source", wfMetaSourceYAML,
			)
		}
		if err := resources.WriteMeta(
			ctx,
			store,
			project,
			resources.ResourceTool,
			tl.ID,
			wfMetaSourceYAML,
			"indexer",
		); err != nil {
			return err
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
		prev := resources.GetMetaSource(ctx, store, project, resources.ResourceSchema, sid)
		if _, err := store.Put(ctx, key, sc); err != nil {
			return fmt.Errorf("store put schema '%s': %w", sid, err)
		}
		if prev != "" && prev != wfMetaSourceYAML {
			logger.FromContext(ctx).Warn(
				"yaml indexing overwrote existing resource",
				"project", project,
				"type", string(resources.ResourceSchema),
				"id", sid,
				"old_source", prev,
				"new_source", wfMetaSourceYAML,
			)
		}
		if err := resources.WriteMeta(
			ctx,
			store,
			project,
			resources.ResourceSchema,
			sid,
			wfMetaSourceYAML,
			"indexer",
		); err != nil {
			return err
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
		prev := resources.GetMetaSource(ctx, store, project, resources.ResourceMCP, mc.ID)
		if _, err := store.Put(ctx, key, mc); err != nil {
			return fmt.Errorf("store put mcp '%s': %w", mc.ID, err)
		}
		if prev != "" && prev != wfMetaSourceYAML {
			logger.FromContext(ctx).Warn(
				"yaml indexing overwrote existing resource",
				"project", project,
				"type", string(resources.ResourceMCP),
				"id", mc.ID,
				"old_source", prev,
				"new_source", wfMetaSourceYAML,
			)
		}
		if err := resources.WriteMeta(
			ctx,
			store,
			project,
			resources.ResourceMCP,
			mc.ID,
			wfMetaSourceYAML,
			"indexer",
		); err != nil {
			return err
		}
	}
	return nil
}

func schemaID(s *schema.Schema) string { return schema.GetID(s) }
