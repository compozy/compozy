package bundles

import (
	"context"
	"fmt"
	"testing"
	"time"

	extensionpkg "github.com/compozy/agh/internal/extension"
	"github.com/compozy/agh/internal/resources"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

const benchmarkOperationsPerActivation = 6

func BenchmarkServiceListActivationsLargeCatalog(b *testing.B) {
	b.ReportAllocs()

	service, activationCount := newBenchmarkBundleService(256, 128)
	ctx := context.Background()

	for b.Loop() {
		previews, err := service.ListActivations(ctx)
		if err != nil {
			b.Fatalf("ListActivations() error: %v", err)
		}
		if len(previews) != activationCount {
			b.Fatalf("len(ListActivations()) = %d, want %d", len(previews), activationCount)
		}
	}
}

func BenchmarkServiceBuildLargeCatalog(b *testing.B) {
	b.ReportAllocs()

	service, activationRecords, bundleRecords := newBenchmarkBuildInputs(256, 128)
	ctx := context.Background()

	for b.Loop() {
		plan, err := service.Build(ctx, activationRecords, bundleRecords)
		if err != nil {
			b.Fatalf("Build() error: %v", err)
		}
		if got, want := plan.OperationCount(), len(activationRecords)*benchmarkOperationsPerActivation; got != want {
			b.Fatalf("plan.OperationCount() = %d, want %d", got, want)
		}
	}
}

func newBenchmarkBundleService(bundleCount int, activationCount int) (*Service, int) {
	store := newMemoryStore()
	bundleRecords, activations := newBenchmarkCatalogData(bundleCount, activationCount)
	store.bundles = bundleRecords
	for _, activation := range activations {
		store.activations[activation.ID] = activation
	}

	service := NewService(
		store,
		staticExtensionLister{},
		func(context.Context, string) (*extensionpkg.Extension, error) {
			return nil, extensionpkg.ErrExtensionNotFound
		},
		WithLogger(discardBundleTestLogger()),
		WithNow(func() time.Time {
			return time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
		}),
		WithWorkspaceResolver(memoryWorkspaceResolver{
			resolveFn: func(_ context.Context, idOrPath string) (workspacepkg.ResolvedWorkspace, error) {
				return workspacepkg.ResolvedWorkspace{
					Workspace: workspacepkg.Workspace{
						ID:      idOrPath,
						Name:    idOrPath,
						RootDir: fmt.Sprintf("/tmp/agh-benchmark/%s", idOrPath),
					},
				}, nil
			},
		}),
	)
	return service, len(activations)
}

func newBenchmarkBuildInputs(
	bundleCount int,
	activationCount int,
) (*Service, []resources.Record[ActivationResourceSpec], []resources.Record[BundleResourceSpec]) {
	service, _ := newBenchmarkBundleService(bundleCount, activationCount)

	store, ok := service.store.(*memoryStore)
	if !ok {
		panic("benchmark service must use memoryStore")
	}

	activationRecords := make([]resources.Record[ActivationResourceSpec], 0, len(store.activations))
	for _, activation := range store.activations {
		activationRecords = append(activationRecords, resources.Record[ActivationResourceSpec]{
			Kind:    BundleActivationResourceKind,
			ID:      activation.ID,
			Version: 1,
			Scope:   resourceScopeForActivation(activation),
			Spec:    activationResourceSpecFromActivation(activation),
		})
	}
	return service, activationRecords, append([]resources.Record[BundleResourceSpec](nil), store.bundles...)
}

func newBenchmarkCatalogData(
	bundleCount int,
	activationCount int,
) ([]resources.Record[BundleResourceSpec], []Activation) {
	ext := newMarketingExtension()
	bundles := make([]resources.Record[BundleResourceSpec], 0, bundleCount)
	activations := make([]Activation, 0, activationCount)

	for i := range bundleCount {
		extensionName := fmt.Sprintf("marketing-team-%03d", i)
		bundleName := fmt.Sprintf("marketing-%03d", i)
		bundle := cloneBundleSpec(ext.Bundles[0])
		bundle.Name = bundleName

		bundles = append(bundles, resources.Record[BundleResourceSpec]{
			Kind:  BundleResourceKind,
			ID:    BundleResourceID(extensionName, bundleName),
			Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
			Spec: BundleResourceSpec{
				ExtensionName:              extensionName,
				Bundle:                     bundle,
				OwnerBridgePlatform:        ext.Manifest.Bridge.Platform,
				OwnerProvidesBridgeAdapter: true,
			},
		})
	}

	for i := range activationCount {
		bundleIndex := i % bundleCount
		extensionName := fmt.Sprintf("marketing-team-%03d", bundleIndex)
		bundleName := fmt.Sprintf("marketing-%03d", bundleIndex)
		workspaceID := fmt.Sprintf("ws-%03d", i)
		activations = append(activations, Activation{
			ID:            ActivationResourceID(extensionName, bundleName, "default", ScopeWorkspace, workspaceID),
			ExtensionName: extensionName,
			BundleName:    bundleName,
			ProfileName:   "default",
			Scope:         ScopeWorkspace,
			WorkspaceID:   workspaceID,
		})
	}

	return bundles, activations
}
