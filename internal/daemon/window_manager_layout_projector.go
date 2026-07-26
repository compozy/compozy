package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/windowmanager"
)

type windowLayoutProjector struct {
	delegate *resourceCatalogProjector[windowmanager.LayoutResource]
}

func newWindowLayoutProjector(
	catalog *resourceCatalog[windowmanager.LayoutResource],
) resources.TypedProjector[windowmanager.LayoutResource] {
	if catalog == nil {
		return nil
	}
	return &windowLayoutProjector{delegate: &resourceCatalogProjector[windowmanager.LayoutResource]{
		kind: windowmanager.WindowLayoutResourceKind, catalog: catalog,
		cloneSpec: windowmanager.CloneLayoutResource,
	}}
}

func (p *windowLayoutProjector) Kind() resources.ResourceKind {
	return windowmanager.WindowLayoutResourceKind
}

func (p *windowLayoutProjector) DependsOn() []resources.ResourceKind {
	return nil
}

func (p *windowLayoutProjector) Build(
	ctx context.Context,
	records []resources.Record[windowmanager.LayoutResource],
) (resources.ProjectionPlan, error) {
	if p == nil || p.delegate == nil {
		return nil, errors.New("daemon: window layout projector is required")
	}
	for _, record := range records {
		if record.ID != record.Spec.ID {
			return nil, fmt.Errorf(
				"%w: window layout record ID %q does not match spec ID %q",
				resources.ErrValidation,
				record.ID,
				record.Spec.ID,
			)
		}
	}
	return p.delegate.Build(ctx, records)
}

func (p *windowLayoutProjector) Apply(ctx context.Context, plan resources.ProjectionPlan) error {
	if p == nil || p.delegate == nil {
		return errors.New("daemon: window layout projector is required")
	}
	return p.delegate.Apply(ctx, plan)
}
