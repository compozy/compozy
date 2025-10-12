package project

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/pkg/logger"
)

const metaSourceYAML = "yaml"

// IndexToResourceStore publishes project-scoped resources (tools, schemas, models)
// to the provided ResourceStore using stable (project,type,id) keys.
func (p *Config) IndexToResourceStore(ctx context.Context, store resources.ResourceStore) error {
	if p == nil {
		return fmt.Errorf("nil project config")
	}
	if store == nil {
		return fmt.Errorf("resource store is required")
	}
	if p.Name == "" {
		return fmt.Errorf("project name is required for indexing")
	}
	if err := p.indexProjectTools(ctx, store); err != nil {
		return err
	}
	if err := p.indexProjectMemories(ctx, store); err != nil {
		return err
	}
	if err := p.indexProjectSchemas(ctx, store); err != nil {
		return err
	}
	if err := p.indexProjectEmbedders(ctx, store); err != nil {
		return err
	}
	if err := p.indexProjectVectorDBs(ctx, store); err != nil {
		return err
	}
	if err := p.indexProjectKnowledgeBases(ctx, store); err != nil {
		return err
	}
	if err := p.indexProjectModels(ctx, store); err != nil {
		return err
	}
	return nil
}

func schemaID(s *schema.Schema) string { return schema.GetID(s) }

// indexProjectTools publishes project-level tools to the store.
func (p *Config) indexProjectTools(ctx context.Context, store resources.ResourceStore) error {
	for i := range p.Tools {
		tl := &p.Tools[i]
		if tl.ID == "" {
			return fmt.Errorf("project tool at index %d missing id", i)
		}
		key := resources.ResourceKey{Project: p.Name, Type: resources.ResourceTool, ID: tl.ID}
		prev := resources.GetMetaSource(ctx, store, p.Name, resources.ResourceTool, tl.ID)
		if _, err := store.Put(ctx, key, tl); err != nil {
			return fmt.Errorf("store put tool '%s': %w", tl.ID, err)
		}
		if prev != "" && prev != metaSourceYAML {
			logger.FromContext(ctx).
				Warn(
					"yaml indexing overwrote existing resource",
					"project", p.Name,
					"type", string(resources.ResourceTool),
					"id", tl.ID,
					"old_source", prev,
					"new_source", metaSourceYAML,
				)
		}
		if err := resources.WriteMeta(
			ctx,
			store,
			p.Name,
			resources.ResourceTool,
			tl.ID,
			metaSourceYAML,
			"indexer",
		); err != nil {
			return err
		}
	}
	return nil
}

// indexProjectMemories publishes project-level memory resources to the store.
func (p *Config) indexProjectMemories(ctx context.Context, store resources.ResourceStore) error {
	for i := range p.Memories {
		m := &p.Memories[i]
		if m.ID == "" {
			return fmt.Errorf("project memory at index %d missing id", i)
		}
		if m.Resource == "" {
			m.Resource = string(resources.ResourceMemory)
		}
		if err := m.Validate(); err != nil {
			return fmt.Errorf("memory '%s' validation failed: %w", m.ID, err)
		}
		key := resources.ResourceKey{Project: p.Name, Type: resources.ResourceMemory, ID: m.ID}
		prev := resources.GetMetaSource(ctx, store, p.Name, resources.ResourceMemory, m.ID)
		if _, err := store.Put(ctx, key, m); err != nil {
			return fmt.Errorf("store put memory '%s': %w", m.ID, err)
		}
		if prev != "" && prev != metaSourceYAML {
			logger.FromContext(ctx).
				Warn(
					"yaml indexing overwrote existing resource",
					"project", p.Name,
					"type", string(resources.ResourceMemory),
					"id", m.ID,
					"old_source", prev,
					"new_source", metaSourceYAML,
				)
		}
		if err := resources.WriteMeta(
			ctx,
			store,
			p.Name,
			resources.ResourceMemory,
			m.ID,
			metaSourceYAML,
			"indexer",
		); err != nil {
			return err
		}
	}
	return nil
}

// indexProjectSchemas publishes project-level schemas to the store.
func (p *Config) indexProjectSchemas(ctx context.Context, store resources.ResourceStore) error {
	for i := range p.Schemas {
		sc := &p.Schemas[i]
		sid := schemaID(sc)
		if sid == "" {
			continue // skip unnamed schemas
		}
		key := resources.ResourceKey{Project: p.Name, Type: resources.ResourceSchema, ID: sid}
		prev := resources.GetMetaSource(ctx, store, p.Name, resources.ResourceSchema, sid)
		if _, err := store.Put(ctx, key, sc); err != nil {
			return fmt.Errorf("store put schema '%s': %w", sid, err)
		}
		if prev != "" && prev != metaSourceYAML {
			logger.FromContext(ctx).
				Warn(
					"yaml indexing overwrote existing resource",
					"project", p.Name,
					"type", string(resources.ResourceSchema),
					"id", sid,
					"old_source", prev,
					"new_source", metaSourceYAML,
				)
		}
		if err := resources.WriteMeta(
			ctx,
			store,
			p.Name,
			resources.ResourceSchema,
			sid,
			metaSourceYAML,
			"indexer",
		); err != nil {
			return err
		}
	}
	return nil
}

// indexProjectModels publishes project-level models to the store.
func (p *Config) indexProjectModels(ctx context.Context, store resources.ResourceStore) error {
	for i := range p.Models {
		m := p.Models[i]
		if m == nil || m.Model == "" {
			continue
		}
		id := fmt.Sprintf("%s:%s", string(m.Provider), m.Model)
		key := resources.ResourceKey{Project: p.Name, Type: resources.ResourceModel, ID: id}
		prev := resources.GetMetaSource(ctx, store, p.Name, resources.ResourceModel, id)
		if _, err := store.Put(ctx, key, m); err != nil {
			return fmt.Errorf("store put model '%s': %w", id, err)
		}
		if prev != "" && prev != metaSourceYAML {
			logger.FromContext(ctx).
				Warn(
					"yaml indexing overwrote existing resource",
					"project", p.Name,
					"type", string(resources.ResourceModel),
					"id", id,
					"old_source", prev,
					"new_source", metaSourceYAML,
				)
		}
		if err := resources.WriteMeta(
			ctx,
			store,
			p.Name,
			resources.ResourceModel,
			id,
			metaSourceYAML,
			"indexer",
		); err != nil {
			return err
		}
	}
	return nil
}

func (p *Config) indexProjectEmbedders(ctx context.Context, store resources.ResourceStore) error {
	log := logger.FromContext(ctx)
	for i := range p.Embedders {
		emb := &p.Embedders[i]
		if emb.ID == "" {
			return fmt.Errorf("project embedder at index %d missing id", i)
		}
		key := resources.ResourceKey{Project: p.Name, Type: resources.ResourceEmbedder, ID: emb.ID}
		prev := resources.GetMetaSource(ctx, store, p.Name, resources.ResourceEmbedder, emb.ID)
		if _, err := store.Put(ctx, key, emb); err != nil {
			return fmt.Errorf("store put embedder '%s': %w", emb.ID, err)
		}
		if prev != "" && prev != metaSourceYAML {
			log.Warn(
				"yaml indexing overwrote existing resource",
				"project", p.Name,
				"type", string(resources.ResourceEmbedder),
				"id", emb.ID,
				"old_source", prev,
				"new_source", metaSourceYAML,
			)
		}
		if err := resources.WriteMeta(
			ctx,
			store,
			p.Name,
			resources.ResourceEmbedder,
			emb.ID,
			metaSourceYAML,
			"indexer",
		); err != nil {
			return err
		}
	}
	return nil
}

func (p *Config) indexProjectVectorDBs(ctx context.Context, store resources.ResourceStore) error {
	log := logger.FromContext(ctx)
	for i := range p.VectorDBs {
		vdb := &p.VectorDBs[i]
		if vdb.ID == "" {
			return fmt.Errorf("project vector_db at index %d missing id", i)
		}
		key := resources.ResourceKey{Project: p.Name, Type: resources.ResourceVectorDB, ID: vdb.ID}
		prev := resources.GetMetaSource(ctx, store, p.Name, resources.ResourceVectorDB, vdb.ID)
		if _, err := store.Put(ctx, key, vdb); err != nil {
			return fmt.Errorf("store put vector_db '%s': %w", vdb.ID, err)
		}
		if prev != "" && prev != metaSourceYAML {
			log.Warn(
				"yaml indexing overwrote existing resource",
				"project", p.Name,
				"type", string(resources.ResourceVectorDB),
				"id", vdb.ID,
				"old_source", prev,
				"new_source", metaSourceYAML,
			)
		}
		if err := resources.WriteMeta(
			ctx,
			store,
			p.Name,
			resources.ResourceVectorDB,
			vdb.ID,
			metaSourceYAML,
			"indexer",
		); err != nil {
			return err
		}
	}
	return nil
}

func (p *Config) indexProjectKnowledgeBases(ctx context.Context, store resources.ResourceStore) error {
	log := logger.FromContext(ctx)
	for i := range p.KnowledgeBases {
		kb := &p.KnowledgeBases[i]
		if kb.ID == "" {
			return fmt.Errorf("project knowledge_base at index %d missing id", i)
		}
		if kb.Ingest == "" {
			kb.Ingest = knowledge.IngestManual
		}
		key := resources.ResourceKey{Project: p.Name, Type: resources.ResourceKnowledgeBase, ID: kb.ID}
		prev := resources.GetMetaSource(ctx, store, p.Name, resources.ResourceKnowledgeBase, kb.ID)
		if _, err := store.Put(ctx, key, kb); err != nil {
			return fmt.Errorf("store put knowledge_base '%s': %w", kb.ID, err)
		}
		if prev != "" && prev != metaSourceYAML {
			log.Warn(
				"yaml indexing overwrote existing resource",
				"project", p.Name,
				"type", string(resources.ResourceKnowledgeBase),
				"id", kb.ID,
				"old_source", prev,
				"new_source", metaSourceYAML,
			)
		}
		if err := resources.WriteMeta(
			ctx,
			store,
			p.Name,
			resources.ResourceKnowledgeBase,
			kb.ID,
			metaSourceYAML,
			"indexer",
		); err != nil {
			return err
		}
	}
	return nil
}
