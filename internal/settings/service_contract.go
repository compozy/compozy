package settings

import (
	"context"

	aghconfig "github.com/compozy/agh/internal/config"
	mcpauth "github.com/compozy/agh/internal/mcp/auth"
)

// Service is the daemon-facing settings orchestration boundary.
type Service interface {
	GetSection(ctx context.Context, req SectionRequest) (SectionEnvelope, error)
	UpdateSection(ctx context.Context, req SectionUpdateRequest) (MutationResult, error)
	ApplySection(ctx context.Context, req SectionUpdateRequest) (ApplyResult, error)
	ListCollection(ctx context.Context, req CollectionRequest) (CollectionEnvelope, error)
	PutCollectionItem(ctx context.Context, req CollectionItemPutRequest) (MutationResult, error)
	ApplyCollectionItem(ctx context.Context, req CollectionItemPutRequest) (ApplyResult, error)
	ApplyProviderModelCuration(
		ctx context.Context,
		req ProviderModelCurationRequest,
	) (ProviderModelCurationResult, error)
	InstallMCPCatalog(ctx context.Context, req MCPCatalogInstallRequest) (MCPCatalogInstallResult, error)
	BeginMCPAuth(ctx context.Context, req MCPAuthBeginRequest) (mcpauth.BeginResult, error)
	ExchangeMCPAuth(ctx context.Context, req MCPAuthExchangeRequest) (mcpauth.Status, error)
	CompleteMCPAuthCallback(ctx context.Context, callbackURL string) (mcpauth.Status, error)
	LogoutMCPAuth(ctx context.Context, req MCPAuthTargetRequest) (mcpauth.Status, error)
	DeleteCollectionItem(ctx context.Context, req CollectionItemDeleteRequest) (MutationResult, error)
	ApplyCollectionDelete(ctx context.Context, req CollectionItemDeleteRequest) (ApplyResult, error)
	Reload(ctx context.Context) (ApplyResult, error)
	ActiveConfig(ctx context.Context) (aghconfig.Config, error)
	ListApplyRecords(ctx context.Context, filter ApplyRecordFilter) ([]ApplyRecord, error)
}
