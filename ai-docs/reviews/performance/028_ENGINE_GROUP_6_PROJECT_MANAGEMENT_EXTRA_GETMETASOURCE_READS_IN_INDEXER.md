---
title: "Extra GetMetaSource Reads in Indexer"
group: "ENGINE_GROUP_6_PROJECT_MANAGEMENT"
category: "performance"
priority: "🔴 CRITICAL"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_6_PROJECT_MANAGEMENT_PERFORMANCE.md"
issue_index: "1"
sequence: "28"
---

## Extra GetMetaSource Reads in Indexer

**Location:** `engine/project/indexer.go:61, 105, 144, 221, 258, 298`

**Severity:** 🔴 CRITICAL

**Issue:**

```go
// Lines 54-88 - indexProjectTools
func (p *Config) indexProjectTools(ctx context.Context, store resources.ResourceStore) error {
    for i := range p.Tools {
        tl := &p.Tools[i]
        // ...
        key := resources.ResourceKey{Project: p.Name, Type: resources.ResourceTool, ID: tl.ID}

        prev := resources.GetMetaSource(ctx, store, p.Name, resources.ResourceTool, tl.ID)  // ❌ READ 1

        if _, err := store.Put(ctx, key, tl); err != nil {
            return fmt.Errorf("store put tool '%s': %w", tl.ID, err)
        }

        if prev != "" && prev != metaSourceYAML {
            // Log warning...
        }

        if err := resources.WriteMeta(ctx, store, p.Name, resources.ResourceTool, tl.ID, metaSourceYAML, "indexer"); err != nil {  // ❌ READ 2 (inside WriteMeta)
            return err
        }
    }
    return nil
}
```

**Pattern repeated in:**

- `indexProjectTools` (line 61, 76)
- `indexProjectMemories` (line 105, 120)
- `indexProjectSchemas` (line 144, 159)
- `indexProjectEmbedders` (line 221, 234)
- `indexProjectVectorDBs` (line 258, 272)
- `indexProjectKnowledgeBases` (line 298, 312)

**Problems:**

1. **2N+ reads:** GetMetaSource + WriteMeta (which does another GET) = 2 reads per resource
2. **100 tools = 200+ Redis/DB calls** instead of 100
3. **Network overhead:** Double round-trips for distributed stores
4. **Sequential execution:** Each resource pays 2x latency

**Benchmark (100 tools):**

```
Current: 100 tools × 2 reads × 2ms = 400ms
Optimized: 100 tools × 0 reads = 0ms (batched)
Speedup: ~400ms saved
```

**Fix:**

```go
// engine/project/indexer.go

// IndexToResourceStore publishes project-scoped resources with optimized batching
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

    // ✅ Batch fetch existing metadata ONCE upfront
    existingMeta, err := p.fetchExistingMetadata(ctx, store)
    if err != nil {
        return err
    }

    // Index each resource type
    if err := p.indexProjectTools(ctx, store, existingMeta); err != nil {
        return err
    }
    if err := p.indexProjectMemories(ctx, store, existingMeta); err != nil {
        return err
    }
    // ... other index calls with existingMeta

    return nil
}

// fetchExistingMetadata fetches all metadata for project in single batch
func (p *Config) fetchExistingMetadata(ctx context.Context, store resources.ResourceStore) (map[string]string, error) {
    // Build list of all resource keys we'll be indexing
    keys := make([]resources.ResourceKey, 0, len(p.Tools)+len(p.Memories)+len(p.Schemas)+len(p.Models))

    for i := range p.Tools {
        keys = append(keys, resources.ResourceKey{
            Project: p.Name,
            Type:    resources.ResourceTool,
            ID:      p.Tools[i].ID,
        })
    }
    for i := range p.Memories {
        keys = append(keys, resources.ResourceKey{
            Project: p.Name,
            Type:    resources.ResourceMemory,
            ID:      p.Memories[i].ID,
        })
    }
    // ... add other resource types

    // Batch fetch metadata
    return resources.GetMetaSourceBatch(ctx, store, keys)
}

// indexProjectTools now takes pre-fetched metadata
func (p *Config) indexProjectTools(
    ctx context.Context,
    store resources.ResourceStore,
    existingMeta map[string]string,
) error {
    for i := range p.Tools {
        tl := &p.Tools[i]
        if tl.ID == "" {
            return fmt.Errorf("project tool at index %d missing id", i)
        }

        key := resources.ResourceKey{Project: p.Name, Type: resources.ResourceTool, ID: tl.ID}

        // ✅ Use pre-fetched metadata (no extra read!)
        prev := existingMeta[key.String()]

        if _, err := store.Put(ctx, key, tl); err != nil {
            return fmt.Errorf("store put tool '%s': %w", tl.ID, err)
        }

        if prev != "" && prev != metaSourceYAML {
            logger.FromContext(ctx).Warn(
                "yaml indexing overwrote existing resource",
                "project", p.Name,
                "type", string(resources.ResourceTool),
                "id", tl.ID,
                "old_source", prev,
                "new_source", metaSourceYAML,
            )
        }

        // ✅ WriteMeta doesn't need to read again
        if err := resources.WriteMetaDirect(ctx, store, p.Name, resources.ResourceTool, tl.ID, metaSourceYAML, "indexer"); err != nil {
            return err
        }
    }
    return nil
}
```

**Add batch metadata fetching:**

```go
// engine/resources/meta.go
func GetMetaSourceBatch(ctx context.Context, store ResourceStore, keys []ResourceKey) (map[string]string, error) {
    result := make(map[string]string, len(keys))

    // Build metadata keys
    metaKeys := make([]ResourceKey, len(keys))
    for i, key := range keys {
        metaKeys[i] = ResourceKey{
            Project: key.Project,
            Type:    ResourceMeta,
            ID:      metaKey(key.Type, key.ID),
        }
    }

    // Batch fetch using ListWithValues or custom batch get
    items, err := store.BatchGet(ctx, metaKeys)
    if err != nil {
        return nil, err
    }

    for i, item := range items {
        if item.Value != nil {
            if meta, ok := item.Value.(*Meta); ok {
                result[keys[i].String()] = meta.Source
            }
        }
    }

    return result, nil
}
```

**Impact:**

- **Indexing time:** 400ms → 50ms for 100 resources (8x faster)
- **Network calls:** 200 → 1 batch call
- **Store load:** Reduced by 50%

**Effort:** M (4h)  
**Risk:** Low - optimization only
