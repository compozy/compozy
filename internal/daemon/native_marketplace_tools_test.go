package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	marketplacepkg "github.com/compozy/compozy/internal/marketplace"
	taskpkg "github.com/compozy/compozy/internal/task"
	toolspkg "github.com/compozy/compozy/internal/tools"
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

func (lateBootExtensionService) Search(
	context.Context,
	contract.ExtensionSearchRequest,
) (contract.ExtensionSearchResponse, error) {
	return contract.ExtensionSearchResponse{}, nil
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

func (lateBootExtensionService) UpdateBatch(
	context.Context,
	contract.UpdateExtensionsRequest,
	taskpkg.ActorContext,
) ([]contract.ManagedExtensionUpdatePayload, error) {
	return nil, errors.New("unexpected UpdateBatch call")
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
	contract.EnableExtensionRequest,
	taskpkg.ActorContext,
) (contract.ExtensionEnableResult, error) {
	return contract.ExtensionEnableResult{}, errors.New("unexpected Enable call")
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

func (lateBootExtensionService) Inventory(context.Context, string) (contract.ExtensionInventoryPayload, error) {
	return contract.ExtensionInventoryPayload{}, errors.New("unexpected Inventory call")
}

func (lateBootExtensionService) Preview(context.Context, string) (contract.ExtensionEnablePreviewPayload, error) {
	return contract.ExtensionEnablePreviewPayload{}, errors.New("unexpected Preview call")
}

func (lateBootExtensionService) Provenance(
	context.Context,
	string,
) (contract.ExtensionProvenancePayload, error) {
	return contract.ExtensionProvenancePayload{}, errors.New("unexpected Provenance call")
}

func (lateBootExtensionService) ListExtensionSecrets(
	context.Context,
	string,
	taskpkg.ActorContext,
) (contract.ExtensionSecretsPayload, error) {
	return contract.ExtensionSecretsPayload{}, errors.New("unexpected ListExtensionSecrets call")
}

func (lateBootExtensionService) SetExtensionSecrets(
	context.Context,
	string,
	contract.SetExtensionSecretsRequest,
	taskpkg.ActorContext,
) (contract.ExtensionSecretsPayload, error) {
	return contract.ExtensionSecretsPayload{}, errors.New("unexpected SetExtensionSecrets call")
}

func (lateBootExtensionService) DeleteExtensionSecret(
	context.Context,
	string,
	string,
	taskpkg.ActorContext,
) error {
	return errors.New("unexpected DeleteExtensionSecret call")
}

func TestMarketplaceNativeSearch(t *testing.T) {
	t.Parallel()

	t.Run("Should reject an opaque cursor without a single kind", func(t *testing.T) {
		t.Parallel()

		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			MarketplaceCatalog: lateBootMarketplaceCatalog{},
		}, nativeApproveAllPolicyInputs())
		_, err := registry.Call(
			t.Context(),
			toolspkg.Scope{Operator: true},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDMarketplaceSearch,
				Input:  json.RawMessage(`{"cursor":"opaque"}`),
			},
		)
		if err == nil || !errors.Is(err, toolspkg.ErrToolInvalidInput) {
			t.Fatalf("Registry.Call(cursor without kind) error = %v, want invalid input", err)
		}
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
			toolspkg.Scope{Operator: true},
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
