package daemon

import (
	"context"
	"errors"
	"fmt"

	automationpkg "github.com/compozy/agh/internal/automation"
	"github.com/compozy/agh/internal/resources"
)

type automationResourceProjectorTarget interface {
	BuildJobResourceState(context.Context, []resources.Record[automationpkg.Job]) (resources.ProjectionPlan, error)
	ApplyJobResourceState(context.Context, resources.ProjectionPlan) error
	BuildTriggerResourceState(
		context.Context,
		[]resources.Record[automationpkg.Trigger],
	) (resources.ProjectionPlan, error)
	ApplyTriggerResourceState(context.Context, resources.ProjectionPlan) error
}

func automationResourceTarget(runtime automationRuntime) automationResourceProjectorTarget {
	if runtime == nil {
		return nil
	}
	target, ok := runtime.(automationResourceProjectorTarget)
	if !ok {
		return nil
	}
	return target
}

type automationJobProjector struct {
	target automationResourceProjectorTarget
}

var _ resources.TypedProjector[automationpkg.Job] = (*automationJobProjector)(nil)

func newAutomationJobProjector(target automationResourceProjectorTarget) resources.TypedProjector[automationpkg.Job] {
	if target == nil {
		return nil
	}
	return &automationJobProjector{target: target}
}

func (p *automationJobProjector) Kind() resources.ResourceKind {
	return automationpkg.JobResourceKind
}

func (p *automationJobProjector) DependsOn() []resources.ResourceKind {
	return nil
}

func (p *automationJobProjector) Build(
	ctx context.Context,
	records []resources.Record[automationpkg.Job],
) (resources.ProjectionPlan, error) {
	if p == nil || p.target == nil {
		return nil, errors.New("daemon: automation job projector target is required")
	}
	return p.target.BuildJobResourceState(ctx, records)
}

func (p *automationJobProjector) Apply(ctx context.Context, plan resources.ProjectionPlan) error {
	if p == nil || p.target == nil {
		return errors.New("daemon: automation job projector target is required")
	}
	return p.target.ApplyJobResourceState(ctx, plan)
}

type automationTriggerProjector struct {
	target automationResourceProjectorTarget
}

var _ resources.TypedProjector[automationpkg.Trigger] = (*automationTriggerProjector)(nil)

func newAutomationTriggerProjector(
	target automationResourceProjectorTarget,
) resources.TypedProjector[automationpkg.Trigger] {
	if target == nil {
		return nil
	}
	return &automationTriggerProjector{target: target}
}

func (p *automationTriggerProjector) Kind() resources.ResourceKind {
	return automationpkg.TriggerResourceKind
}

func (p *automationTriggerProjector) DependsOn() []resources.ResourceKind {
	return nil
}

func (p *automationTriggerProjector) Build(
	ctx context.Context,
	records []resources.Record[automationpkg.Trigger],
) (resources.ProjectionPlan, error) {
	if p == nil || p.target == nil {
		return nil, errors.New("daemon: automation trigger projector target is required")
	}
	return p.target.BuildTriggerResourceState(ctx, records)
}

func (p *automationTriggerProjector) Apply(ctx context.Context, plan resources.ProjectionPlan) error {
	if p == nil || p.target == nil {
		return errors.New("daemon: automation trigger projector target is required")
	}
	return p.target.ApplyTriggerResourceState(ctx, plan)
}

func automationResourceStores(
	raw resources.RawStore,
	codecs *resources.CodecRegistry,
) (
	resources.Store[automationpkg.Job],
	resources.Store[automationpkg.Trigger],
	error,
) {
	if raw == nil && codecs == nil {
		return nil, nil, nil
	}
	if raw == nil {
		return nil, nil, errors.New(
			"daemon: automation resource raw store is required when codec registry is configured",
		)
	}
	if codecs == nil {
		return nil, nil, errors.New(
			"daemon: automation resource codec registry is required when raw store is configured",
		)
	}

	jobCodec, err := resources.ResolveCodec[automationpkg.Job](codecs, automationpkg.JobResourceKind)
	if err != nil {
		return nil, nil, fmt.Errorf("daemon: resolve automation job codec: %w", err)
	}
	jobStore, err := resources.NewStore(raw, jobCodec)
	if err != nil {
		return nil, nil, fmt.Errorf("daemon: create automation job store: %w", err)
	}

	triggerCodec, err := resources.ResolveCodec[automationpkg.Trigger](codecs, automationpkg.TriggerResourceKind)
	if err != nil {
		return nil, nil, fmt.Errorf("daemon: resolve automation trigger codec: %w", err)
	}
	triggerStore, err := resources.NewStore(raw, triggerCodec)
	if err != nil {
		return nil, nil, fmt.Errorf("daemon: create automation trigger store: %w", err)
	}

	return jobStore, triggerStore, nil
}
