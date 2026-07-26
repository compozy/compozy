package daemon

import (
	"context"
	"errors"
	"fmt"
	"sync"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/resources"
	toolspkg "github.com/compozy/agh/internal/tools"
)

type resourceCatalog[T any] struct {
	mu             sync.RWMutex
	revision       int64
	records        []resources.Record[T]
	transientSpecs map[string]T
	cloneSpec      func(T) T
}

func newResourceCatalog[T any](cloneSpec func(T) T) *resourceCatalog[T] {
	return &resourceCatalog[T]{cloneSpec: cloneSpec}
}

func (c *resourceCatalog[T]) Replace(revision int64, records []resources.Record[T]) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.revision = revision
	c.records = cloneResourceRecords(records, c.cloneSpec)
	retainedTransientSpecs := make(map[string]T, len(c.records))
	for index := range c.records {
		transientSpec, ok := c.transientSpecs[c.records[index].ID]
		if !ok {
			continue
		}
		c.records[index].Spec = c.cloneSpec(transientSpec)
		retainedTransientSpecs[c.records[index].ID] = c.cloneSpec(transientSpec)
	}
	c.transientSpecs = retainedTransientSpecs
}

func (c *resourceCatalog[T]) MergeTransientSpecs(specs map[string]T) {
	if c == nil || len(specs) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.transientSpecs == nil {
		c.transientSpecs = make(map[string]T, len(specs))
	}
	for id, spec := range specs {
		c.transientSpecs[id] = c.cloneSpec(spec)
	}
}

func (c *resourceCatalog[T]) Update(update func([]resources.Record[T]) []resources.Record[T]) {
	if c == nil || update == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	records := cloneResourceRecords(c.records, c.cloneSpec)
	c.records = cloneResourceRecords(update(records), c.cloneSpec)
	c.revision++
}

func (c *resourceCatalog[T]) Snapshot() []resources.Record[T] {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return cloneResourceRecords(c.records, c.cloneSpec)
}

func (c *resourceCatalog[T]) cloneRecord(record resources.Record[T]) resources.Record[T] {
	return cloneResourceRecord(record, c.cloneSpec)
}

func (c *resourceCatalog[T]) Revision() int64 {
	if c == nil {
		return 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.revision
}

type resourceCatalogProjectionPlan[T any] struct {
	kind       resources.ResourceKind
	revision   int64
	operations int
	records    []resources.Record[T]
}

func (p *resourceCatalogProjectionPlan[T]) Kind() resources.ResourceKind {
	if p == nil {
		return ""
	}
	return p.kind
}

func (p *resourceCatalogProjectionPlan[T]) Revision() int64 {
	if p == nil {
		return 0
	}
	return p.revision
}

func (p *resourceCatalogProjectionPlan[T]) OperationCount() int {
	if p == nil {
		return 0
	}
	return p.operations
}

type resourceCatalogProjector[T any] struct {
	kind      resources.ResourceKind
	catalog   *resourceCatalog[T]
	cloneSpec func(T) T
}

func newToolProjector(catalog *resourceCatalog[toolspkg.Tool]) resources.TypedProjector[toolspkg.Tool] {
	if catalog == nil {
		return nil
	}
	return &resourceCatalogProjector[toolspkg.Tool]{
		kind:      toolspkg.ToolResourceKind,
		catalog:   catalog,
		cloneSpec: cloneToolSpec,
	}
}

func newMCPServerProjector(
	catalog *resourceCatalog[aghconfig.MCPServer],
) resources.TypedProjector[aghconfig.MCPServer] {
	if catalog == nil {
		return nil
	}
	return &resourceCatalogProjector[aghconfig.MCPServer]{
		kind:      aghconfig.MCPServerResourceKind,
		catalog:   catalog,
		cloneSpec: cloneDaemonMCPServer,
	}
}

func (p *resourceCatalogProjector[T]) Kind() resources.ResourceKind {
	if p == nil {
		return ""
	}
	return p.kind
}

func (p *resourceCatalogProjector[T]) DependsOn() []resources.ResourceKind {
	return nil
}

func (p *resourceCatalogProjector[T]) Build(
	_ context.Context,
	records []resources.Record[T],
) (resources.ProjectionPlan, error) {
	if p == nil || p.catalog == nil {
		return nil, errors.New("daemon: resource catalog projector is required")
	}

	var revision int64
	for _, record := range records {
		if record.Version > revision {
			revision = record.Version
		}
	}

	return &resourceCatalogProjectionPlan[T]{
		kind:       p.kind,
		revision:   revision,
		operations: len(records),
		records:    cloneResourceRecords(records, p.cloneSpec),
	}, nil
}

func (p *resourceCatalogProjector[T]) Apply(ctx context.Context, plan resources.ProjectionPlan) error {
	if p == nil || p.catalog == nil {
		return errors.New("daemon: resource catalog projector is required")
	}
	if ctx == nil {
		return errors.New("daemon: resource catalog projector apply context is required")
	}

	typed, ok := plan.(*resourceCatalogProjectionPlan[T])
	if !ok {
		return fmt.Errorf("daemon: resource catalog projector plan has type %T", plan)
	}
	p.catalog.Replace(typed.revision, typed.records)
	return nil
}
