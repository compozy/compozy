package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	bundlepkg "github.com/compozy/agh/internal/bundles"
	extensionpkg "github.com/compozy/agh/internal/extension"
	marketplacepkg "github.com/compozy/agh/internal/marketplace"
	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
)

type lateBootMarketplaceCatalog struct {
	entry marketplacepkg.Entry
}

func (c lateBootMarketplaceCatalog) Browse(
	context.Context,
	marketplacepkg.Kind,
	string,
	int,
	int,
) (marketplacepkg.BrowseResult, error) {
	return marketplacepkg.BrowseResult{
		Entries: []marketplacepkg.Entry{c.entry},
		State:   marketplacepkg.KindState{Kind: marketplacepkg.KindExtension},
	}, nil
}

func (c lateBootMarketplaceCatalog) Detail(
	context.Context,
	marketplacepkg.Kind,
	string,
) (*marketplacepkg.Entry, error) {
	return &c.entry, nil
}

func (lateBootMarketplaceCatalog) ResolveExtensionInstall(
	context.Context,
	string,
	string,
) (*marketplacepkg.Entry, error) {
	return nil, errors.New("unexpected ResolveExtensionInstall call")
}

func (lateBootMarketplaceCatalog) Refresh(
	context.Context,
	...marketplacepkg.Kind,
) (marketplacepkg.RefreshReport, error) {
	return marketplacepkg.RefreshReport{}, errors.New("unexpected Refresh call")
}

func (lateBootMarketplaceCatalog) Status(context.Context) ([]marketplacepkg.KindState, error) {
	return nil, errors.New("unexpected Status call")
}

func (lateBootMarketplaceCatalog) ResolveSkillInstalls(
	context.Context,
	[]string,
) ([]marketplacepkg.Entry, error) {
	return nil, errors.New("unexpected ResolveSkillInstalls call")
}

type lateBootExtensionService struct{}

func (lateBootExtensionService) List(context.Context) ([]contract.ExtensionPayload, error) {
	return []contract.ExtensionPayload{}, nil
}

func (lateBootExtensionService) MarketplaceTrust(
	context.Context,
	extensionpkg.MarketplaceTrustEvidence,
) (contract.ExtensionTrustReportPayload, error) {
	return contract.ExtensionTrustReportPayload{
		Decision:         extensionpkg.ExtensionTrustDecisionVerified,
		RegistryTier:     extensionpkg.ExtensionRegistryTierOfficial,
		ChecksumVerified: true,
	}, nil
}

func (lateBootExtensionService) Install(
	context.Context,
	contract.InstallExtensionRequest,
	taskpkg.ActorContext,
) (contract.ExtensionPayload, error) {
	return contract.ExtensionPayload{}, errors.New("unexpected Install call")
}

func (lateBootExtensionService) Update(
	context.Context,
	string,
	contract.UpdateExtensionRequest,
	taskpkg.ActorContext,
) (contract.ManagedExtensionUpdatePayload, error) {
	return contract.ManagedExtensionUpdatePayload{}, errors.New("unexpected Update call")
}

func (lateBootExtensionService) Remove(
	context.Context,
	string,
	taskpkg.ActorContext,
) (contract.ManagedExtensionRemovePayload, error) {
	return contract.ManagedExtensionRemovePayload{}, errors.New("unexpected Remove call")
}

func (lateBootExtensionService) Enable(
	context.Context,
	string,
	taskpkg.ActorContext,
) (contract.ExtensionPayload, error) {
	return contract.ExtensionPayload{}, errors.New("unexpected Enable call")
}

func (lateBootExtensionService) Disable(
	context.Context,
	string,
	taskpkg.ActorContext,
) (contract.ExtensionPayload, error) {
	return contract.ExtensionPayload{}, errors.New("unexpected Disable call")
}

func (lateBootExtensionService) Status(context.Context, string) (contract.ExtensionPayload, error) {
	return contract.ExtensionPayload{}, errors.New("unexpected Status call")
}

func (lateBootExtensionService) Provenance(
	context.Context,
	string,
) (contract.ExtensionProvenancePayload, error) {
	return contract.ExtensionProvenancePayload{}, errors.New("unexpected Provenance call")
}

func TestMarketplaceNativeSearch(t *testing.T) {
	t.Parallel()

	t.Run("Should use the shared core projection", func(t *testing.T) {
		t.Parallel()

		bundles := &nativeBundleServiceStub{catalog: []bundlepkg.CatalogEntry{{
			ExtensionName: "compozy/productivity",
			Bundle: extensionpkg.BundleSpec{
				Name:        "starter",
				Description: "Starter team",
			},
		}}}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			BundleService: func() core.BundleService { return bundles },
		}, nativeApproveAllPolicyInputs())

		result, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDMarketplaceSearch,
				Input:  json.RawMessage(`{"kind":"bundle","query":"starter","limit":5}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(marketplace_search) error = %v", err)
		}
		requireNativeStructuredContains(t, result, []byte(`"kind":"bundle"`))
		requireNativeStructuredContains(t, result, []byte(`"name":"starter"`))
		requireNativeStructuredContains(t, result, []byte(`"stale":false`))
	})

	t.Run("Should continue a single-kind search with the opaque core cursor", func(t *testing.T) {
		t.Parallel()

		bundles := &nativeBundleServiceStub{catalog: []bundlepkg.CatalogEntry{
			{
				ExtensionName: "compozy/productivity",
				Bundle:        extensionpkg.BundleSpec{Name: "alpha", Description: "Alpha team"},
			},
			{
				ExtensionName: "compozy/productivity",
				Bundle:        extensionpkg.BundleSpec{Name: "beta", Description: "Beta team"},
			},
		}}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			BundleService: func() core.BundleService { return bundles },
		}, nativeApproveAllPolicyInputs())

		first, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDMarketplaceSearch,
				Input:  json.RawMessage(`{"kind":"bundle","limit":1}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(first marketplace page) error = %v", err)
		}
		var firstPage contract.MarketplaceKindResponse
		if err := json.Unmarshal(first.Structured, &firstPage); err != nil {
			t.Fatalf("json.Unmarshal(first marketplace page) error = %v", err)
		}
		if len(firstPage.Items) != 1 || firstPage.NextCursor == "" {
			t.Fatalf("first marketplace page = %#v, want one item and continuation", firstPage)
		}

		secondInput, err := json.Marshal(map[string]any{
			"kind": "bundle", "limit": 1, "cursor": firstPage.NextCursor,
		})
		if err != nil {
			t.Fatalf("json.Marshal(second marketplace request) error = %v", err)
		}
		second, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{ToolID: toolspkg.ToolIDMarketplaceSearch, Input: secondInput},
		)
		if err != nil {
			t.Fatalf("Registry.Call(second marketplace page) error = %v", err)
		}
		var secondPage contract.MarketplaceKindResponse
		if err := json.Unmarshal(second.Structured, &secondPage); err != nil {
			t.Fatalf("json.Unmarshal(second marketplace page) error = %v", err)
		}
		if len(secondPage.Items) != 1 ||
			secondPage.Items[0].EntryID == firstPage.Items[0].EntryID {
			t.Fatalf("second marketplace page = %#v, want one distinct item", secondPage)
		}
	})

	t.Run("Should reject an opaque cursor without a single kind", func(t *testing.T) {
		t.Parallel()

		bundles := &nativeBundleServiceStub{catalog: []bundlepkg.CatalogEntry{{
			ExtensionName: "compozy/productivity",
			Bundle:        extensionpkg.BundleSpec{Name: "starter", Description: "Starter team"},
		}}}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			BundleService: func() core.BundleService { return bundles },
		}, nativeApproveAllPolicyInputs())
		_, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDMarketplaceSearch,
				Input:  json.RawMessage(`{"cursor":"opaque"}`),
			},
		)
		if err == nil || !errors.Is(err, toolspkg.ErrToolInvalidInput) {
			t.Fatalf("Registry.Call(cursor without kind) error = %v, want invalid input", err)
		}
	})

	t.Run("Should project installed state from the caller workspace", func(t *testing.T) {
		t.Parallel()

		bundles := &nativeBundleServiceStub{
			catalog: []bundlepkg.CatalogEntry{{
				ExtensionName: "compozy/productivity",
				Bundle: extensionpkg.BundleSpec{
					Name:        "starter",
					Description: "Starter team",
				},
			}},
			activations: []bundlepkg.ActivationPreview{
				{
					Activation: bundlepkg.Activation{
						ID: "act-workspace-a", ExtensionName: "compozy/productivity", BundleName: "starter",
						Scope: bundlepkg.ScopeWorkspace, WorkspaceID: "workspace-a",
					},
					SpecDrift: true,
				},
				{
					Activation: bundlepkg.Activation{
						ID: "act-workspace-b", ExtensionName: "compozy/productivity", BundleName: "starter",
						Scope: bundlepkg.ScopeWorkspace, WorkspaceID: "workspace-b",
					},
				},
			},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			BundleService: func() core.BundleService { return bundles },
		}, nativeApproveAllPolicyInputs())

		result, err := registry.Call(
			t.Context(),
			toolspkg.Scope{WorkspaceID: "workspace-a"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDMarketplaceSearch,
				Input:  json.RawMessage(`{"kind":"bundle","query":"starter","limit":5}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(marketplace_search) error = %v", err)
		}
		requireNativeStructuredContains(t, result, []byte(`"installed":true`))
		requireNativeStructuredContains(t, result, []byte(`"update_available":true`))
		requireNativeStructuredContains(
			t,
			result,
			[]byte(`/marketplace/bundles/activations/act-workspace-a`),
		)
		requireNativeStructuredExcludes(t, result, []byte(`act-workspace-b`))
	})

	t.Run("Should resolve extensions attached after the native registry boots", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		cfg := testConfig(t, homePaths)
		entry := marketplacepkg.Entry{
			Kind:         marketplacepkg.KindExtension,
			EntryID:      "late-boot-extension",
			Name:         "Late boot extension",
			Description:  "Attached after the native registry",
			Version:      "1.0.0",
			InstallSlug:  "acme/late-boot-extension",
			DigestSHA256: strings.Repeat("a", 64),
			Tier:         extensionpkg.ExtensionRegistryTierOfficial,
			Payload: json.RawMessage(
				`{"entry_id":"late-boot-extension","name":"Late boot extension","description":"Attached after the native registry","version":"1.0.0","install_slug":"acme/late-boot-extension","artifact_url":"https://downloads.example.test/late-boot-extension-v1.0.0.tar.gz","digest_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`,
			),
		}
		state := &bootState{
			cfg: cfg,
			deps: RuntimeDeps{
				MarketplaceCatalog: lateBootMarketplaceCatalog{entry: entry},
			},
		}
		daemon := &Daemon{homePaths: homePaths}
		deps := daemon.nativeToolsDeps(state, func() toolspkg.Registry { return nil })
		nativeTools := &daemonNativeTools{deps: &deps}

		state.deps.Extensions = lateBootExtensionService{}
		result, err := nativeTools.marketplaceSearch(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDMarketplaceSearch,
				Input:  json.RawMessage(`{"kind":"extension"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(marketplace_search) error = %v", err)
		}
		requireNativeStructuredContains(t, result, []byte(`"entry_id":"late-boot-extension"`))
	})
}
