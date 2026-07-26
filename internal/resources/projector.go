package resources

import (
	"context"
	"errors"
	"fmt"
)

const (
	bundleKind           = ResourceKind("bundle")
	bundleActivationKind = ResourceKind("bundle.activation")
)

// ProjectionPlan is the generic metadata surface returned by domain projectors.
type ProjectionPlan interface {
	Kind() ResourceKind
	Revision() int64
	OperationCount() int
}

// TypedProjector is the standard single-kind typed projector contract.
type TypedProjector[T any] interface {
	Kind() ResourceKind
	DependsOn() []ResourceKind
	Build(ctx context.Context, records []Record[T]) (ProjectionPlan, error)
	Apply(ctx context.Context, plan ProjectionPlan) error
}

// BundleActivationProjector is the explicit mixed-kind projector escape hatch for bundle activations.
type BundleActivationProjector[A any, B any] interface {
	Build(ctx context.Context, activations []Record[A], bundles []Record[B]) (ProjectionPlan, error)
	Apply(ctx context.Context, plan ProjectionPlan) error
}

// ProjectorRegistration is an opaque registration token consumed by internal projector wiring.
type ProjectorRegistration interface {
	Kind() ResourceKind
	DependsOn() []ResourceKind
	projectorRegistration()
}

type projectionInput struct {
	kind         ResourceKind
	revision     int64
	records      []RawRecord
	dependencies map[ResourceKind][]RawRecord
}

type projector interface {
	Kind() ResourceKind
	DependsOn() []ResourceKind
	Build(ctx context.Context, input projectionInput) (ProjectionPlan, error)
	Apply(ctx context.Context, plan ProjectionPlan) error
}

type projectorRegistration struct {
	kind      ResourceKind
	dependsOn []ResourceKind
	build     func(ctx context.Context, input projectionInput) (ProjectionPlan, error)
	apply     func(ctx context.Context, plan ProjectionPlan) error
}

func (r *projectorRegistration) Kind() ResourceKind {
	return r.kind
}

func (r *projectorRegistration) DependsOn() []ResourceKind {
	return append([]ResourceKind(nil), r.dependsOn...)
}

func (r *projectorRegistration) projectorRegistration() {}

func (r *projectorRegistration) Build(ctx context.Context, input projectionInput) (ProjectionPlan, error) {
	return r.build(ctx, input)
}

func (r *projectorRegistration) Apply(ctx context.Context, plan ProjectionPlan) error {
	return r.apply(ctx, plan)
}

// NewTypedProjectorRegistration adapts a single-kind typed projector to the internal raw reconcile seam.
func NewTypedProjectorRegistration[T any](
	codec KindCodec[T],
	projector TypedProjector[T],
) (ProjectorRegistration, error) {
	normalizedKind, err := validateCodec(codec)
	if err != nil {
		return nil, err
	}
	if projector == nil {
		return nil, errors.New("resources: typed projector is required")
	}
	if projector.Kind().Normalize() != normalizedKind {
		return nil, fmt.Errorf(
			"%w: typed projector kind %q does not match codec kind %q",
			ErrValidation,
			projector.Kind(),
			normalizedKind,
		)
	}

	dependsOn := normalizeKinds(projector.DependsOn())
	return &projectorRegistration{
		kind:      normalizedKind,
		dependsOn: dependsOn,
		build: func(ctx context.Context, input projectionInput) (ProjectionPlan, error) {
			if input.kind.Normalize() != normalizedKind {
				return nil, fmt.Errorf(
					"%w: typed projector build expected kind %q, got %q",
					ErrValidation,
					normalizedKind,
					input.kind,
				)
			}
			if input.revision < 0 {
				return nil, fmt.Errorf("%w: revision cannot be negative: %d", ErrValidation, input.revision)
			}

			records, err := decodeTypedRecords(ctx, codec, input.records)
			if err != nil {
				return nil, err
			}
			return projector.Build(ctx, records)
		},
		apply: projector.Apply,
	}, nil
}

// NewBundleActivationProjectorRegistration adapts the explicit mixed-kind bundle activation projector seam.
func NewBundleActivationProjectorRegistration[A any, B any](
	registry *CodecRegistry,
	projector BundleActivationProjector[A, B],
) (ProjectorRegistration, error) {
	if projector == nil {
		return nil, errors.New("resources: bundle activation projector is required")
	}

	activationCodec, err := ResolveCodec[A](registry, bundleActivationKind)
	if err != nil {
		return nil, err
	}
	bundleCodec, err := ResolveCodec[B](registry, bundleKind)
	if err != nil {
		return nil, err
	}

	return &projectorRegistration{
		kind:      bundleActivationKind,
		dependsOn: []ResourceKind{bundleKind},
		build: func(ctx context.Context, input projectionInput) (ProjectionPlan, error) {
			if input.kind.Normalize() != bundleActivationKind {
				return nil, fmt.Errorf(
					"%w: bundle activation projector build expected kind %q, got %q",
					ErrValidation,
					bundleActivationKind,
					input.kind,
				)
			}
			if input.revision < 0 {
				return nil, fmt.Errorf("%w: revision cannot be negative: %d", ErrValidation, input.revision)
			}
			if err := validateBundleActivationDependencies(input.dependencies); err != nil {
				return nil, err
			}

			activations, err := decodeTypedRecords(ctx, activationCodec, input.records)
			if err != nil {
				return nil, err
			}
			bundles, err := decodeTypedRecords(ctx, bundleCodec, input.dependencies[bundleKind])
			if err != nil {
				return nil, err
			}
			return projector.Build(ctx, activations, bundles)
		},
		apply: projector.Apply,
	}, nil
}

func validateBundleActivationDependencies(dependencies map[ResourceKind][]RawRecord) error {
	for kind, records := range dependencies {
		if kind.Normalize() == bundleKind {
			continue
		}
		if len(records) == 0 {
			continue
		}
		return fmt.Errorf(
			"%w: bundle activation projector does not accept dependency kind %q",
			ErrValidation,
			kind,
		)
	}
	return nil
}

func unwrapProjectorRegistration(registration ProjectorRegistration) (projector, error) {
	if registration == nil {
		return nil, errors.New("resources: projector registration is required")
	}

	projector, ok := registration.(projector)
	if !ok {
		return nil, errors.New("resources: projector registration does not implement internal projector")
	}
	return projector, nil
}
