//go:build integration

package resources

import (
	"testing"

	"github.com/compozy/agh/internal/testutil"
)

func TestTypedStoreIntegrationPersistLoadAndList(t *testing.T) {
	t.Parallel()

	kernel, _ := openTestKernel(t)
	ctx := testutil.Context(t)
	codec := mustJSONCodec(t, testResourceKind, 1024, validateTestTypedSpec)
	store, err := NewStore(kernel, codec)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	record, err := store.Put(ctx, testDaemonActor(), Draft[testTypedSpec]{
		ID:    "integration-tool",
		Scope: ResourceScope{Kind: ResourceScopeKindWorkspace, ID: "ws-integration"},
		Spec:  testTypedSpec{Name: "integration"},
	})
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if got, want := record.Version, int64(1); got != want {
		t.Fatalf("record.Version = %d, want %d", got, want)
	}

	loaded, err := store.Get(ctx, testDaemonActor(), record.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got, want := loaded.Spec.Name, "integration"; got != want {
		t.Fatalf("loaded.Spec.Name = %q, want %q", got, want)
	}

	records, err := store.List(ctx, testDaemonActor(), ResourceFilter{
		Scope: &ResourceScope{Kind: ResourceScopeKindWorkspace, ID: "ws-integration"},
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if got, want := len(records), 1; got != want {
		t.Fatalf("len(List()) = %d, want %d", got, want)
	}

	rawRecords, err := kernel.ListRaw(ctx, testDaemonActor(), ResourceFilter{Kind: testResourceKind})
	if err != nil {
		t.Fatalf("ListRaw() error = %v", err)
	}
	if got, want := len(rawRecords), 1; got != want {
		t.Fatalf("len(ListRaw()) = %d, want %d", got, want)
	}
}

func TestBundleActivationProjectorRegistrationIntegration(t *testing.T) {
	t.Parallel()

	kernel, _ := openTestKernel(t)
	ctx := testutil.Context(t)
	registry := NewCodecRegistry()

	activationCodec := mustJSONCodec(t, bundleActivationKind, 1024, validateTestTypedSpec)
	bundleCodec := mustJSONCodec(t, bundleKind, 1024, validateOtherTypedSpec)
	if err := RegisterCodec(registry, activationCodec); err != nil {
		t.Fatalf("RegisterCodec(activation) error = %v", err)
	}
	if err := RegisterCodec(registry, bundleCodec); err != nil {
		t.Fatalf("RegisterCodec(bundle) error = %v", err)
	}

	activationStore, err := NewStore(kernel, activationCodec)
	if err != nil {
		t.Fatalf("NewStore(activation) error = %v", err)
	}
	bundleStore, err := NewStore(kernel, bundleCodec)
	if err != nil {
		t.Fatalf("NewStore(bundle) error = %v", err)
	}

	if _, err := bundleStore.Put(ctx, testDaemonActor(), Draft[otherTypedSpec]{
		ID:    "bundle-1",
		Scope: ResourceScope{Kind: ResourceScopeKindGlobal},
		Spec:  otherTypedSpec{Value: "bundle-1"},
	}); err != nil {
		t.Fatalf("bundleStore.Put() error = %v", err)
	}
	if _, err := activationStore.Put(ctx, testDaemonActor(), Draft[testTypedSpec]{
		ID:    "activation-1",
		Scope: ResourceScope{Kind: ResourceScopeKindGlobal},
		Spec:  testTypedSpec{Name: "activation-1"},
	}); err != nil {
		t.Fatalf("activationStore.Put() error = %v", err)
	}

	activationRaw, err := kernel.ListRaw(ctx, testDaemonActor(), ResourceFilter{Kind: bundleActivationKind})
	if err != nil {
		t.Fatalf("ListRaw(bundle.activation) error = %v", err)
	}
	bundleRaw, err := kernel.ListRaw(ctx, testDaemonActor(), ResourceFilter{Kind: bundleKind})
	if err != nil {
		t.Fatalf("ListRaw(bundle) error = %v", err)
	}

	domainProjector := &captureBundleActivationProjector{}
	registration, err := NewBundleActivationProjectorRegistration(registry, domainProjector)
	if err != nil {
		t.Fatalf("NewBundleActivationProjectorRegistration() error = %v", err)
	}
	internalProjector, err := unwrapProjectorRegistration(registration)
	if err != nil {
		t.Fatalf("unwrapProjectorRegistration() error = %v", err)
	}

	plan, err := internalProjector.Build(ctx, projectionInput{
		kind:    bundleActivationKind,
		records: activationRaw,
		dependencies: map[ResourceKind][]RawRecord{
			bundleKind: bundleRaw,
		},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if got, want := len(domainProjector.activations), 1; got != want {
		t.Fatalf("len(activations) = %d, want %d", got, want)
	}
	if got, want := len(domainProjector.bundles), 1; got != want {
		t.Fatalf("len(bundles) = %d, want %d", got, want)
	}
	if got, want := plan.OperationCount(), 2; got != want {
		t.Fatalf("plan.OperationCount() = %d, want %d", got, want)
	}
}
