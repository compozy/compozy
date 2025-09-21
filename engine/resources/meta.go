package resources

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/pkg/logger"
)

// newMetaID builds the canonical meta ID: project:type:id
func newMetaID(project string, typ ResourceType, id string) string {
	return project + ":" + string(typ) + ":" + id
}

// writeMeta stores provenance metadata for a resource.
// Schema: {source, updated_at, updated_by}
func writeMeta(
	ctx context.Context,
	store ResourceStore,
	project string,
	typ ResourceType,
	id, source, updatedBy string,
) error {
	log := logger.FromContext(ctx)
	if store == nil {
		return fmt.Errorf("resource store is nil")
	}
	metaID := newMetaID(project, typ, id)
	meta := map[string]any{
		"project":    project,
		"type":       string(typ),
		"id":         id,
		"source":     source,
		"updated_at": time.Now().UTC().Format(time.RFC3339),
		"updated_by": updatedBy,
	}
	if _, err := store.Put(ctx, ResourceKey{Project: project, Type: ResourceMeta, ID: metaID}, meta); err != nil {
		log.Warn("meta write failed", "error", err, "id", metaID)
		return err
	}
	return nil
}

// getMetaSource returns prior meta source if present; empty when not found
func getMetaSource(ctx context.Context, store ResourceStore, project string, typ ResourceType, id string) string {
	if store == nil {
		return ""
	}
	v, _, err := store.Get(ctx, ResourceKey{Project: project, Type: ResourceMeta, ID: newMetaID(project, typ, id)})
	if err != nil {
		return ""
	}
	if m, ok := v.(map[string]any); ok {
		if s, ok2 := m["source"].(string); ok2 {
			return s
		}
	}
	return ""
}

// WriteMetaForAutoload helper with fixed source="autoload"
func WriteMetaForAutoload(ctx context.Context, store ResourceStore, project string, typ ResourceType, id string) error {
	return writeMeta(ctx, store, project, typ, id, "autoload", "autoload")
}

// WriteMeta is the public helper to store provenance metadata
func WriteMeta(
	ctx context.Context,
	store ResourceStore,
	project string,
	typ ResourceType,
	id, source, updatedBy string,
) error {
	return writeMeta(ctx, store, project, typ, id, source, updatedBy)
}

// GetMetaSource returns the stored provenance source for a resource, if any
func GetMetaSource(ctx context.Context, store ResourceStore, project string, typ ResourceType, id string) string {
	return getMetaSource(ctx, store, project, typ, id)
}

// IndexPutWithMeta centralizes indexing logic with conflict logging and meta write.
func IndexPutWithMeta(
	ctx context.Context,
	store ResourceStore,
	project string,
	typ ResourceType,
	id string,
	value any,
	source string,
	updatedBy string,
) error {
	log := logger.FromContext(ctx)
	prev := GetMetaSource(ctx, store, project, typ, id)
	key := ResourceKey{Project: project, Type: typ, ID: id}
	if _, err := store.Put(ctx, key, value); err != nil {
		return fmt.Errorf("store put %s '%s': %w", string(typ), id, err)
	}
	if prev != "" && prev != source {
		log.Warn(
			"yaml indexing overwrote existing resource",
			"project", project,
			"type", string(typ),
			"id", id,
			"old_source", prev,
			"new_source", source,
		)
	}
	if err := WriteMeta(ctx, store, project, typ, id, source, updatedBy); err != nil {
		return fmt.Errorf("write meta for %s '%s': %w", string(typ), id, err)
	}
	return nil
}
