package project

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/schema"
)

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
	// Tools (project-level shared tools)
	for i := range p.Tools {
		tl := &p.Tools[i]
		if tl.ID == "" {
			return fmt.Errorf("project tool at index %d missing id", i)
		}
		key := resources.ResourceKey{Project: p.Name, Type: resources.ResourceTool, ID: tl.ID}
		if _, err := store.Put(ctx, key, tl); err != nil {
			return fmt.Errorf("store put tool '%s': %w", tl.ID, err)
		}
	}
	// Schemas (optional list; expect an 'id' field in the schema map)
	for i := range p.Schemas {
		sc := &p.Schemas[i]
		sid := schemaID(sc)
		if sid == "" {
			continue // skip unnamed schemas
		}
		key := resources.ResourceKey{Project: p.Name, Type: resources.ResourceSchema, ID: sid}
		if _, err := store.Put(ctx, key, sc); err != nil {
			return fmt.Errorf("store put schema '%s': %w", sid, err)
		}
	}
	// Models: derive a stable id as "<provider>:<model>"
	for i := range p.Models {
		m := p.Models[i]
		if m == nil || m.Model == "" {
			continue
		}
		id := fmt.Sprintf("%s:%s", string(m.Provider), m.Model)
		key := resources.ResourceKey{Project: p.Name, Type: resources.ResourceModel, ID: id}
		if _, err := store.Put(ctx, key, m); err != nil {
			return fmt.Errorf("store put model '%s': %w", id, err)
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
