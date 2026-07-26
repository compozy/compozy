package daemon

import (
	"context"
	"slices"
	"testing"

	automationpkg "github.com/compozy/agh/internal/automation"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	bundlepkg "github.com/compozy/agh/internal/bundles"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/windowmanager"
)

type fakeBundleActivationProjector struct{}

func (fakeBundleActivationProjector) Build(
	context.Context,
	[]resources.Record[bundlepkg.ActivationResourceSpec],
	[]resources.Record[bundlepkg.BundleResourceSpec],
) (resources.ProjectionPlan, error) {
	return fakeBundleActivationPlan{}, nil
}

func (fakeBundleActivationProjector) Apply(context.Context, resources.ProjectionPlan) error {
	return nil
}

type fakeBundleActivationPlan struct{}

func (fakeBundleActivationPlan) Kind() resources.ResourceKind {
	return bundlepkg.BundleActivationResourceKind
}

func (fakeBundleActivationPlan) Revision() int64 {
	return 0
}

func (fakeBundleActivationPlan) OperationCount() int {
	return 0
}

func TestBundleProjectorRegistrationsKeepActivationCycleFree(t *testing.T) {
	t.Parallel()

	registry := resources.NewCodecRegistry()
	bundleCodec, err := bundlepkg.NewBundleResourceCodec()
	if err != nil {
		t.Fatalf("NewBundleResourceCodec() error = %v", err)
	}
	if err := resources.RegisterCodec(registry, bundleCodec); err != nil {
		t.Fatalf("RegisterCodec(bundle) error = %v", err)
	}
	activationCodec, err := bundlepkg.NewActivationResourceCodec()
	if err != nil {
		t.Fatalf("NewActivationResourceCodec() error = %v", err)
	}
	if err := resources.RegisterCodec(registry, activationCodec); err != nil {
		t.Fatalf("RegisterCodec(activation) error = %v", err)
	}

	registrations, err := appendBundleProjectorRegistrations(nil, &resourceReconcileDriverDeps{
		CodecRegistry: registry,
		Bundles:       fakeBundleActivationProjector{},
	})
	if err != nil {
		t.Fatalf("appendBundleProjectorRegistrations() error = %v", err)
	}
	byKind := make(map[resources.ResourceKind]resources.ProjectorRegistration, len(registrations))
	for _, registration := range registrations {
		byKind[registration.Kind()] = registration
	}

	bundleRegistration, ok := byKind[bundlepkg.BundleResourceKind]
	if !ok {
		t.Fatal("bundle projector registration missing")
	}
	if deps := bundleRegistration.DependsOn(); len(deps) != 0 {
		t.Fatalf("bundle DependsOn() = %#v, want none", deps)
	}

	activationRegistration, ok := byKind[bundlepkg.BundleActivationResourceKind]
	if !ok {
		t.Fatal("bundle activation projector registration missing")
	}
	deps := activationRegistration.DependsOn()
	if !slices.Equal(deps, []resources.ResourceKind{bundlepkg.BundleResourceKind}) {
		t.Fatalf("bundle.activation DependsOn() = %#v, want [bundle]", deps)
	}
	if slices.Contains(deps, automationpkg.JobResourceKind) ||
		slices.Contains(deps, automationpkg.TriggerResourceKind) ||
		slices.Contains(deps, bridgepkg.BridgeInstanceResourceKind) ||
		slices.Contains(deps, windowmanager.WindowLayoutResourceKind) {
		t.Fatalf("bundle.activation DependsOn() includes downstream fan-out kinds: %#v", deps)
	}
}
