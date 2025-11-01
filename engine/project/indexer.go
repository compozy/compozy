package project

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/pkg/logger"
)

const metaSourceYAML = "yaml"

func indexWorkerLimit(total int) int {
	cpuCount := runtime.NumCPU()
	if cpuCount < 1 {
		cpuCount = 1
	}
	if total > 0 && total < cpuCount {
		return total
	}
	return cpuCount
}

func (p *Config) countIndexableResources() int {
	if p == nil {
		return 0
	}
	total := len(
		p.Tools,
	) + len(
		p.Memories,
	) + len(
		p.Schemas,
	) + len(
		p.Embedders,
	) + len(
		p.VectorDBs,
	) + len(
		p.KnowledgeBases,
	)
	for i := range p.Models {
		mdl := p.Models[i]
		if mdl == nil {
			continue
		}
		if mdl.Model == "" {
			continue
		}
		total++
	}
	return total
}

type metaCache struct {
	data sync.Map
}

func newMetaCache(source map[string]string) *metaCache {
	cache := &metaCache{}
	for key, value := range source {
		cache.data.Store(key, value)
	}
	return cache
}

func (c *metaCache) Load(key string) string {
	if c == nil {
		return ""
	}
	value, ok := c.data.Load(key)
	if !ok {
		return ""
	}
	str, ok := value.(string)
	if !ok {
		return ""
	}
	return str
}

func (c *metaCache) Store(key, value string) {
	if c == nil {
		return
	}
	c.data.Store(key, value)
}

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
	metaSources, err := p.loadMetaSources(ctx, store)
	if err != nil {
		return err
	}
	totalResources := p.countIndexableResources()
	group, groupCtx := errgroup.WithContext(ctx)
	if limit := indexWorkerLimit(totalResources); limit > 0 {
		group.SetLimit(limit)
	}
	if err := p.indexProjectTools(groupCtx, group, store, metaSources); err != nil {
		return err
	}
	if err := p.indexProjectMemories(groupCtx, group, store, metaSources); err != nil {
		return err
	}
	if err := p.indexProjectSchemas(groupCtx, group, store, metaSources); err != nil {
		return err
	}
	if err := p.indexProjectEmbedders(groupCtx, group, store, metaSources); err != nil {
		return err
	}
	if err := p.indexProjectVectorDBs(groupCtx, group, store, metaSources); err != nil {
		return err
	}
	if err := p.indexProjectKnowledgeBases(groupCtx, group, store, metaSources); err != nil {
		return err
	}
	if err := p.indexProjectModels(groupCtx, group, store, metaSources); err != nil {
		return err
	}
	if err := group.Wait(); err != nil {
		return err
	}
	return nil
}

// loadMetaSources builds a cache of existing meta sources keyed by project/type/id.
func (p *Config) loadMetaSources(ctx context.Context, store resources.ResourceStore) (*metaCache, error) {
	items, err := store.ListWithValues(ctx, p.Name, resources.ResourceMeta)
	if err != nil {
		if errors.Is(err, resources.ErrNotFound) {
			return newMetaCache(nil), nil
		}
		return nil, fmt.Errorf("list meta for project '%s': %w", p.Name, err)
	}
	meta := make(map[string]string, len(items))
	for _, item := range items {
		source := extractMetaSource(item.Value)
		if source == "" {
			continue
		}
		typ, id := extractMetaIdentity(item.Value)
		if typ == "" || id == "" {
			continue
		}
		meta[metaCacheKey(p.Name, resources.ResourceType(typ), id)] = source
	}
	return newMetaCache(meta), nil
}

// extractMetaSource returns the string source from a meta value when present.
func extractMetaSource(value any) string {
	if m, ok := value.(map[string]any); ok {
		if src, ok := m["source"].(string); ok {
			return src
		}
	}
	return ""
}

// extractMetaIdentity returns the stored type and id from a meta value.
func extractMetaIdentity(value any) (string, string) {
	m, ok := value.(map[string]any)
	if !ok {
		return "", ""
	}
	typVal, ok := m["type"].(string)
	if !ok {
		return "", ""
	}
	idVal, ok := m["id"].(string)
	if !ok {
		return "", ""
	}
	return typVal, idVal
}

// metaCacheKey produces the canonical meta cache key for a resource.
func metaCacheKey(project string, typ resources.ResourceType, id string) string {
	return project + ":" + string(typ) + ":" + id
}

// putResourceWithMeta writes the resource, emits overwrite warnings, and updates metadata.
func (p *Config) putResourceWithMeta(
	ctx context.Context,
	store resources.ResourceStore,
	metaSources *metaCache,
	key resources.ResourceKey,
	value any,
) error {
	cacheKey := metaCacheKey(key.Project, key.Type, key.ID)
	prev := metaSources.Load(cacheKey)
	if _, err := store.Put(ctx, key, value); err != nil {
		return fmt.Errorf("store put %s '%s': %w", string(key.Type), key.ID, err)
	}
	if prev != "" && prev != metaSourceYAML {
		logger.FromContext(ctx).
			Warn(
				"yaml indexing overwrote existing resource",
				"project", key.Project,
				"type", string(key.Type),
				"id", key.ID,
				"old_source", prev,
				"new_source", metaSourceYAML,
			)
	}
	if err := resources.WriteMeta(
		ctx,
		store,
		key.Project,
		key.Type,
		key.ID,
		metaSourceYAML,
		"indexer",
	); err != nil {
		return err
	}
	metaSources.Store(cacheKey, metaSourceYAML)
	return nil
}

func schemaID(s *schema.Schema) string { return schema.GetID(s) }

// indexProjectTools publishes project-level tools to the store.
func (p *Config) indexProjectTools(
	ctx context.Context,
	group *errgroup.Group,
	store resources.ResourceStore,
	metaSources *metaCache,
) error {
	for i := range p.Tools {
		tool := &p.Tools[i]
		if tool.ID == "" {
			return fmt.Errorf("project tool at index %d missing id", i)
		}
		key := resources.ResourceKey{Project: p.Name, Type: resources.ResourceTool, ID: tool.ID}
		keyCopy := key
		toolCopy := tool
		group.Go(func() error {
			return p.putResourceWithMeta(ctx, store, metaSources, keyCopy, toolCopy)
		})
	}
	return nil
}

// indexProjectMemories publishes project-level memory resources to the store.
func (p *Config) indexProjectMemories(
	ctx context.Context,
	group *errgroup.Group,
	store resources.ResourceStore,
	metaSources *metaCache,
) error {
	for i, memory := range p.Memories {
		if memory == nil {
			return fmt.Errorf("project memory at index %d cannot be nil", i)
		}
		if memory.ID == "" {
			return fmt.Errorf("project memory at index %d missing id", i)
		}
		if memory.Resource == "" {
			memory.Resource = string(resources.ResourceMemory)
		}
		if err := memory.Validate(ctx); err != nil {
			return fmt.Errorf("memory '%s' validation failed: %w", memory.ID, err)
		}
		key := resources.ResourceKey{Project: p.Name, Type: resources.ResourceMemory, ID: memory.ID}
		keyCopy := key
		mem := memory
		group.Go(func() error {
			return p.putResourceWithMeta(ctx, store, metaSources, keyCopy, mem)
		})
	}
	return nil
}

// indexProjectSchemas publishes project-level schemas to the store.
func (p *Config) indexProjectSchemas(
	ctx context.Context,
	group *errgroup.Group,
	store resources.ResourceStore,
	metaSources *metaCache,
) error {
	for i := range p.Schemas {
		schemaValue := &p.Schemas[i]
		sid := schemaID(schemaValue)
		if sid == "" {
			continue
		}
		key := resources.ResourceKey{Project: p.Name, Type: resources.ResourceSchema, ID: sid}
		keyCopy := key
		schemaCopy := schemaValue
		group.Go(func() error {
			return p.putResourceWithMeta(ctx, store, metaSources, keyCopy, schemaCopy)
		})
	}
	return nil
}

// indexProjectModels publishes project-level models to the store.
func (p *Config) indexProjectModels(
	ctx context.Context,
	group *errgroup.Group,
	store resources.ResourceStore,
	metaSources *metaCache,
) error {
	for i := range p.Models {
		model := p.Models[i]
		if model == nil || model.Model == "" {
			continue
		}
		id := fmt.Sprintf("%s:%s", string(model.Provider), model.Model)
		key := resources.ResourceKey{Project: p.Name, Type: resources.ResourceModel, ID: id}
		keyCopy := key
		modelCopy := model
		group.Go(func() error {
			return p.putResourceWithMeta(ctx, store, metaSources, keyCopy, modelCopy)
		})
	}
	return nil
}

func (p *Config) indexProjectEmbedders(
	ctx context.Context,
	group *errgroup.Group,
	store resources.ResourceStore,
	metaSources *metaCache,
) error {
	for i := range p.Embedders {
		embedder := &p.Embedders[i]
		if embedder.ID == "" {
			return fmt.Errorf("project embedder at index %d missing id", i)
		}
		key := resources.ResourceKey{Project: p.Name, Type: resources.ResourceEmbedder, ID: embedder.ID}
		keyCopy := key
		embedderCopy := embedder
		group.Go(func() error {
			return p.putResourceWithMeta(ctx, store, metaSources, keyCopy, embedderCopy)
		})
	}
	return nil
}

func (p *Config) indexProjectVectorDBs(
	ctx context.Context,
	group *errgroup.Group,
	store resources.ResourceStore,
	metaSources *metaCache,
) error {
	for i := range p.VectorDBs {
		vectorDB := &p.VectorDBs[i]
		if vectorDB.ID == "" {
			return fmt.Errorf("project vector_db at index %d missing id", i)
		}
		key := resources.ResourceKey{Project: p.Name, Type: resources.ResourceVectorDB, ID: vectorDB.ID}
		keyCopy := key
		vectorDBCopy := vectorDB
		group.Go(func() error {
			return p.putResourceWithMeta(ctx, store, metaSources, keyCopy, vectorDBCopy)
		})
	}
	return nil
}

func (p *Config) indexProjectKnowledgeBases(
	ctx context.Context,
	group *errgroup.Group,
	store resources.ResourceStore,
	metaSources *metaCache,
) error {
	for i := range p.KnowledgeBases {
		knowledgeBase := &p.KnowledgeBases[i]
		if knowledgeBase.ID == "" {
			return fmt.Errorf("project knowledge_base at index %d missing id", i)
		}
		if knowledgeBase.Ingest == "" {
			knowledgeBase.Ingest = knowledge.IngestManual
		}
		key := resources.ResourceKey{Project: p.Name, Type: resources.ResourceKnowledgeBase, ID: knowledgeBase.ID}
		keyCopy := key
		knowledgeBaseCopy := knowledgeBase
		group.Go(func() error {
			return p.putResourceWithMeta(ctx, store, metaSources, keyCopy, knowledgeBaseCopy)
		})
	}
	return nil
}
