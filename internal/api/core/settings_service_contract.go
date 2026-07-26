package core

import (
	"context"

	settingspkg "github.com/compozy/agh/internal/settings"
)

// SettingsService exposes the daemon-owned settings read and mutation surface to API transports.
type SettingsService interface {
	MCPSettingsInstaller
	MCPSettingsAuth
	ProviderModelSettingsCurator
	GetSection(ctx context.Context, req settingspkg.SectionRequest) (settingspkg.SectionEnvelope, error)
	UpdateSection(ctx context.Context, req settingspkg.SectionUpdateRequest) (settingspkg.MutationResult, error)
	ApplySection(ctx context.Context, req settingspkg.SectionUpdateRequest) (settingspkg.ApplyResult, error)
	ListCollection(ctx context.Context, req settingspkg.CollectionRequest) (settingspkg.CollectionEnvelope, error)
	PutCollectionItem(ctx context.Context, req settingspkg.CollectionItemPutRequest) (settingspkg.MutationResult, error)
	ApplyCollectionItem(
		ctx context.Context,
		req settingspkg.CollectionItemPutRequest,
	) (settingspkg.ApplyResult, error)
	DeleteCollectionItem(
		ctx context.Context,
		req settingspkg.CollectionItemDeleteRequest,
	) (
		settingspkg.MutationResult,
		error,
	)
	ApplyCollectionDelete(
		ctx context.Context,
		req settingspkg.CollectionItemDeleteRequest,
	) (settingspkg.ApplyResult, error)
	Reload(ctx context.Context) (settingspkg.ApplyResult, error)
	ListApplyRecords(ctx context.Context, filter settingspkg.ApplyRecordFilter) ([]settingspkg.ApplyRecord, error)
}
