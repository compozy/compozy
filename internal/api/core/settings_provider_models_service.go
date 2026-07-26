package core

import (
	"context"

	settingspkg "github.com/compozy/agh/internal/settings"
)

// ProviderModelSettingsCurator exposes provider model curation to API transports.
type ProviderModelSettingsCurator interface {
	ApplyProviderModelCuration(
		ctx context.Context,
		req settingspkg.ProviderModelCurationRequest,
	) (settingspkg.ProviderModelCurationResult, error)
}
