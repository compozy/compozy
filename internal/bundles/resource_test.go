package bundles

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	automationpkg "github.com/compozy/agh/internal/automation"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	aghconfig "github.com/compozy/agh/internal/config"
	extensionpkg "github.com/compozy/agh/internal/extension"
	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/soul"
	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/windowmanager"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestBundleActivationOwnedKindAllowlist(t *testing.T) {
	t.Parallel()

	for _, kind := range []resources.ResourceKind{
		windowmanager.WindowLayoutResourceKind,
		aghconfig.AgentResourceKind,
		soul.ResourceKind,
		heartbeat.ResourceKind,
		automationpkg.JobResourceKind,
		automationpkg.TriggerResourceKind,
		bridgepkg.BridgeInstanceResourceKind,
	} {
		if !bundleActivationOwnedKindAllowed(kind) {
			t.Fatalf("bundleActivationOwnedKindAllowed(%q) = false, want true", kind)
		}
	}
	if bundleActivationOwnedKindAllowed(resources.ResourceKind("tool")) {
		t.Fatal("bundleActivationOwnedKindAllowed(tool) = true, want false")
	}
}

func TestBundleActivationBuildComposesTypedBundleDependency(t *testing.T) {
	t.Parallel()

	store := newMemoryStore()
	service := newMarketingService(store)
	activation := Activation{
		ID:            ActivationResourceID("marketing-team", "marketing", "default", ScopeGlobal, ""),
		ExtensionName: "marketing-team",
		BundleName:    "marketing",
		ProfileName:   "default",
		Scope:         ScopeGlobal,
	}

	bundleRecords := append([]resources.Record[BundleResourceSpec](nil), store.bundles...)
	bundleRecords[0].Version = 9
	plan, err := service.Build(
		context.Background(),
		[]resources.Record[ActivationResourceSpec]{{
			Kind:    BundleActivationResourceKind,
			ID:      activation.ID,
			Version: 3,
			Scope:   resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
			Spec:    activationResourceSpecFromActivation(activation),
		}},
		bundleRecords,
	)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if got, want := plan.Kind(), BundleActivationResourceKind; got != want {
		t.Fatalf("plan.Kind() = %q, want %q", got, want)
	}
	if got, want := plan.Revision(), int64(9); got != want {
		t.Fatalf("plan.Revision() = %d, want %d", got, want)
	}
	if got, want := plan.OperationCount(), 7; got != want {
		t.Fatalf("plan.OperationCount() = %d, want %d", got, want)
	}
	if err := service.Apply(context.Background(), plan); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got, want := len(store.applied), 1; got != want {
		t.Fatalf("len(applied plans) = %d, want %d", got, want)
	}
	if err := service.Apply(context.Background(), nonBundleActivationPlan{}); err == nil {
		t.Fatal("Apply(wrong plan type) error = nil, want failure")
	} else if !strings.Contains(err.Error(), "activation resource plan has type") {
		t.Fatalf("Apply(wrong plan type) error = %v, want wrong-plan-type context", err)
	}
}

func TestBundleServiceReconcileLoadsBundleResourcesOncePerRun(t *testing.T) {
	t.Parallel()

	ext := newMarketingExtension()
	store := &countingBundleStore{memoryStore: newMemoryStore()}
	store.bundles = []resources.Record[BundleResourceSpec]{{
		Kind:    BundleResourceKind,
		ID:      BundleResourceID(ext.Info.Name, ext.Bundles[0].Name),
		Version: 11,
		Scope:   resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
		Spec: BundleResourceSpec{
			ExtensionName:              ext.Info.Name,
			Bundle:                     ext.Bundles[0],
			OwnerBridgePlatform:        ext.Manifest.Bridge.Platform,
			OwnerProvidesBridgeAdapter: true,
		},
	}}
	store.activations[ActivationResourceID("marketing-team", "marketing", "default", ScopeWorkspace, "ws-2")] = Activation{
		ID:            ActivationResourceID("marketing-team", "marketing", "default", ScopeWorkspace, "ws-2"),
		ExtensionName: "marketing-team",
		BundleName:    "marketing",
		ProfileName:   "default",
		Scope:         ScopeWorkspace,
		WorkspaceID:   "ws-2",
	}
	store.activations[ActivationResourceID("marketing-team", "marketing", "default", ScopeWorkspace, "ws-1")] = Activation{
		ID:            ActivationResourceID("marketing-team", "marketing", "default", ScopeWorkspace, "ws-1"),
		ExtensionName: "marketing-team",
		BundleName:    "marketing",
		ProfileName:   "default",
		Scope:         ScopeWorkspace,
		WorkspaceID:   "ws-1",
	}

	service := NewService(
		store,
		staticExtensionLister{items: []extensionpkg.ExtensionInfo{{Name: "marketing-team"}}},
		func(_ context.Context, name string) (*extensionpkg.Extension, error) {
			if name != "marketing-team" {
				return nil, extensionpkg.ErrExtensionNotFound
			}
			return ext, nil
		},
		WithWorkspaceResolver(memoryWorkspaceResolver{
			resolveFn: func(_ context.Context, idOrPath string) (workspacepkg.ResolvedWorkspace, error) {
				return workspacepkg.ResolvedWorkspace{
					Workspace: workspacepkg.Workspace{
						ID:      strings.TrimSpace(idOrPath),
						RootDir: filepath.Join(t.TempDir(), idOrPath),
						Name:    strings.TrimSpace(idOrPath),
					},
				}, nil
			},
		}),
		WithNow(func() time.Time {
			return time.Date(2026, 4, 14, 22, 0, 0, 0, time.UTC)
		}),
	)
	if err := service.Reconcile(testutil.Context(t)); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if got, want := store.listBundleResourcesCalls, 1; got != want {
		t.Fatalf("ListBundleResources() calls = %d, want %d", got, want)
	}
}

func TestBundleServiceListActivationsLoadsBundleResourcesOncePerRun(t *testing.T) {
	t.Parallel()

	ext := newMarketingExtension()
	store := &countingBundleStore{memoryStore: newMemoryStore()}
	store.bundles = []resources.Record[BundleResourceSpec]{{
		Kind:    BundleResourceKind,
		ID:      BundleResourceID(ext.Info.Name, ext.Bundles[0].Name),
		Version: 11,
		Scope:   resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
		Spec: BundleResourceSpec{
			ExtensionName:              ext.Info.Name,
			Bundle:                     ext.Bundles[0],
			OwnerBridgePlatform:        ext.Manifest.Bridge.Platform,
			OwnerProvidesBridgeAdapter: true,
		},
	}}
	firstID := ActivationResourceID("marketing-team", "marketing", "default", ScopeWorkspace, "ws-1")
	secondID := ActivationResourceID("marketing-team", "marketing", "default", ScopeWorkspace, "ws-2")
	store.activations[firstID] = Activation{
		ID:            firstID,
		ExtensionName: "marketing-team",
		BundleName:    "marketing",
		ProfileName:   "default",
		Scope:         ScopeWorkspace,
		WorkspaceID:   "ws-1",
	}
	store.activations[secondID] = Activation{
		ID:            secondID,
		ExtensionName: "marketing-team",
		BundleName:    "marketing",
		ProfileName:   "default",
		Scope:         ScopeWorkspace,
		WorkspaceID:   "ws-2",
	}

	service := NewService(
		store,
		staticExtensionLister{items: []extensionpkg.ExtensionInfo{{Name: "marketing-team"}}},
		func(_ context.Context, name string) (*extensionpkg.Extension, error) {
			if name != "marketing-team" {
				return nil, extensionpkg.ErrExtensionNotFound
			}
			return ext, nil
		},
		WithWorkspaceResolver(memoryWorkspaceResolver{
			resolveFn: func(_ context.Context, idOrPath string) (workspacepkg.ResolvedWorkspace, error) {
				return workspacepkg.ResolvedWorkspace{
					Workspace: workspacepkg.Workspace{
						ID:      strings.TrimSpace(idOrPath),
						RootDir: filepath.Join(t.TempDir(), idOrPath),
						Name:    strings.TrimSpace(idOrPath),
					},
				}, nil
			},
		}),
		WithLogger(discardBundleTestLogger()),
		WithNow(func() time.Time {
			return time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
		}),
	)

	listed, err := service.ListActivations(testutil.Context(t))
	if err != nil {
		t.Fatalf("ListActivations() error = %v", err)
	}
	if got, want := len(listed), 2; got != want {
		t.Fatalf("len(ListActivations()) = %d, want %d", got, want)
	}
	if got, want := store.listBundleResourcesCalls, 1; got != want {
		t.Fatalf("ListBundleResources() calls = %d, want %d", got, want)
	}
}

type nonBundleActivationPlan struct{}

func (nonBundleActivationPlan) Kind() resources.ResourceKind {
	return BundleActivationResourceKind
}

func (nonBundleActivationPlan) Revision() int64 {
	return 0
}

func (nonBundleActivationPlan) OperationCount() int {
	return 0
}

type countingBundleStore struct {
	*memoryStore
	listBundleResourcesCalls int
}

func (s *countingBundleStore) ListBundleResources(
	ctx context.Context,
) ([]resources.Record[BundleResourceSpec], error) {
	s.listBundleResourcesCalls++
	return s.memoryStore.ListBundleResources(ctx)
}
