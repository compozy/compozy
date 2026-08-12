package testutil

import (
	"context"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	taskpkg "github.com/compozy/compozy/internal/task"
)

// StubExtensionService is the shared transport fake for extension route suites.
type StubExtensionService struct {
	ListFn   func(context.Context) ([]contract.ExtensionPayload, error)
	SearchFn func(
		context.Context,
		contract.ExtensionSearchRequest,
	) (contract.ExtensionSearchResponse, error)
	MarketplaceTrustFn func(
		context.Context,
		extensionpkg.MarketplaceTrustEvidence,
	) (contract.ExtensionTrustReportPayload, error)
	InstallFn func(
		context.Context,
		contract.InstallExtensionRequest,
		taskpkg.ActorContext,
	) (contract.ExtensionPayload, error)
	UpdateFn func(
		context.Context,
		string,
		contract.UpdateExtensionRequest,
		taskpkg.ActorContext,
	) (contract.ManagedExtensionUpdatePayload, error)
	UpdateBatchFn func(
		context.Context,
		contract.UpdateExtensionsRequest,
		taskpkg.ActorContext,
	) ([]contract.ManagedExtensionUpdatePayload, error)
	RemoveFn func(
		context.Context,
		string,
		taskpkg.ActorContext,
	) (contract.ManagedExtensionRemovePayload, error)
	EnableFn func(
		context.Context,
		string,
		contract.EnableExtensionRequest,
		taskpkg.ActorContext,
	) (contract.ExtensionEnableResult, error)
	DisableFn     func(context.Context, string, taskpkg.ActorContext) (contract.ExtensionPayload, error)
	StatusFn      func(context.Context, string) (contract.ExtensionPayload, error)
	ProvenanceFn  func(context.Context, string) (contract.ExtensionProvenancePayload, error)
	InventoryFn   func(context.Context, string) (contract.ExtensionInventoryPayload, error)
	PreviewFn     func(context.Context, string) (contract.ExtensionEnablePreviewPayload, error)
	ListSecretsFn func(context.Context, string, taskpkg.ActorContext) (contract.ExtensionSecretsPayload, error)
	SetSecretsFn  func(
		context.Context,
		string,
		contract.SetExtensionSecretsRequest,
		taskpkg.ActorContext,
	) (contract.ExtensionSecretsPayload, error)
	DeleteSecretFn func(context.Context, string, string, taskpkg.ActorContext) error
	DevFn          func(
		context.Context,
		contract.DevLinkExtensionRequest,
		taskpkg.ActorContext,
	) (contract.ExtensionPayload, error)
	ReloadDevFn func(
		context.Context,
		string,
		contract.ReloadExtensionRequest,
		taskpkg.ActorContext,
	) (contract.ExtensionPayload, error)
	ExtensionLogsFn func(
		context.Context,
		string,
		int64,
		string,
		taskpkg.ActorContext,
	) (contract.ExtensionLogsResponse, error)
}

func (s StubExtensionService) Search(
	ctx context.Context,
	req contract.ExtensionSearchRequest,
) (contract.ExtensionSearchResponse, error) {
	if s.SearchFn == nil {
		return contract.ExtensionSearchResponse{}, nil
	}
	return s.SearchFn(ctx, req)
}

func (s StubExtensionService) List(ctx context.Context) ([]contract.ExtensionPayload, error) {
	if s.ListFn == nil {
		return nil, nil
	}
	return s.ListFn(ctx)
}

func (s StubExtensionService) MarketplaceTrust(
	ctx context.Context,
	evidence extensionpkg.MarketplaceTrustEvidence,
) (contract.ExtensionTrustReportPayload, error) {
	if s.MarketplaceTrustFn == nil {
		return extensionpkg.MarketplaceEntryTrustReport(evidence, false)
	}
	return s.MarketplaceTrustFn(ctx, evidence)
}

func (s StubExtensionService) Install(
	ctx context.Context,
	req contract.InstallExtensionRequest,
	actor taskpkg.ActorContext,
) (contract.ExtensionPayload, error) {
	if s.InstallFn == nil {
		return contract.ExtensionPayload{}, nil
	}
	return s.InstallFn(ctx, req, actor)
}

func (s StubExtensionService) Update(
	ctx context.Context,
	name string,
	req contract.UpdateExtensionRequest,
	actor taskpkg.ActorContext,
) (contract.ManagedExtensionUpdatePayload, error) {
	if s.UpdateFn == nil {
		return contract.ManagedExtensionUpdatePayload{}, nil
	}
	return s.UpdateFn(ctx, name, req, actor)
}

func (s StubExtensionService) UpdateBatch(
	ctx context.Context,
	req contract.UpdateExtensionsRequest,
	actor taskpkg.ActorContext,
) ([]contract.ManagedExtensionUpdatePayload, error) {
	if s.UpdateBatchFn == nil {
		return nil, nil
	}
	return s.UpdateBatchFn(ctx, req, actor)
}

func (s StubExtensionService) Remove(
	ctx context.Context,
	name string,
	actor taskpkg.ActorContext,
) (contract.ManagedExtensionRemovePayload, error) {
	if s.RemoveFn == nil {
		return contract.ManagedExtensionRemovePayload{}, nil
	}
	return s.RemoveFn(ctx, name, actor)
}

func (s StubExtensionService) Enable(
	ctx context.Context,
	name string,
	req contract.EnableExtensionRequest,
	actor taskpkg.ActorContext,
) (contract.ExtensionEnableResult, error) {
	if s.EnableFn == nil {
		return contract.ExtensionEnableResult{}, nil
	}
	return s.EnableFn(ctx, name, req, actor)
}

func (s StubExtensionService) Disable(
	ctx context.Context,
	name string,
	actor taskpkg.ActorContext,
) (contract.ExtensionPayload, error) {
	if s.DisableFn == nil {
		return contract.ExtensionPayload{}, nil
	}
	return s.DisableFn(ctx, name, actor)
}

func (s StubExtensionService) Status(ctx context.Context, name string) (contract.ExtensionPayload, error) {
	if s.StatusFn == nil {
		return contract.ExtensionPayload{}, nil
	}
	return s.StatusFn(ctx, name)
}

func (s StubExtensionService) Provenance(
	ctx context.Context,
	name string,
) (contract.ExtensionProvenancePayload, error) {
	if s.ProvenanceFn == nil {
		return contract.ExtensionProvenancePayload{}, nil
	}
	return s.ProvenanceFn(ctx, name)
}

func (s StubExtensionService) Inventory(
	ctx context.Context,
	name string,
) (contract.ExtensionInventoryPayload, error) {
	if s.InventoryFn == nil {
		return contract.ExtensionInventoryPayload{}, nil
	}
	return s.InventoryFn(ctx, name)
}

func (s StubExtensionService) Preview(
	ctx context.Context,
	name string,
) (contract.ExtensionEnablePreviewPayload, error) {
	if s.PreviewFn == nil {
		return contract.ExtensionEnablePreviewPayload{}, nil
	}
	return s.PreviewFn(ctx, name)
}

func (s StubExtensionService) ListExtensionSecrets(
	ctx context.Context,
	name string,
	actor taskpkg.ActorContext,
) (contract.ExtensionSecretsPayload, error) {
	if s.ListSecretsFn == nil {
		return contract.ExtensionSecretsPayload{}, nil
	}
	return s.ListSecretsFn(ctx, name, actor)
}

func (s StubExtensionService) SetExtensionSecrets(
	ctx context.Context,
	name string,
	req contract.SetExtensionSecretsRequest,
	actor taskpkg.ActorContext,
) (contract.ExtensionSecretsPayload, error) {
	if s.SetSecretsFn == nil {
		return contract.ExtensionSecretsPayload{}, nil
	}
	return s.SetSecretsFn(ctx, name, req, actor)
}

func (s StubExtensionService) DeleteExtensionSecret(
	ctx context.Context,
	name string,
	envName string,
	actor taskpkg.ActorContext,
) error {
	if s.DeleteSecretFn == nil {
		return nil
	}
	return s.DeleteSecretFn(ctx, name, envName, actor)
}

func (s StubExtensionService) Dev(
	ctx context.Context,
	req contract.DevLinkExtensionRequest,
	actor taskpkg.ActorContext,
) (contract.ExtensionPayload, error) {
	if s.DevFn == nil {
		return contract.ExtensionPayload{}, nil
	}
	return s.DevFn(ctx, req, actor)
}

func (s StubExtensionService) ReloadDev(
	ctx context.Context,
	name string,
	req contract.ReloadExtensionRequest,
	actor taskpkg.ActorContext,
) (contract.ExtensionPayload, error) {
	if s.ReloadDevFn == nil {
		return contract.ExtensionPayload{}, nil
	}
	return s.ReloadDevFn(ctx, name, req, actor)
}

func (s StubExtensionService) ExtensionLogs(
	ctx context.Context,
	name string,
	after int64,
	streamEpoch string,
	actor taskpkg.ActorContext,
) (contract.ExtensionLogsResponse, error) {
	if s.ExtensionLogsFn == nil {
		return contract.ExtensionLogsResponse{}, nil
	}
	return s.ExtensionLogsFn(ctx, name, after, streamEpoch, actor)
}
