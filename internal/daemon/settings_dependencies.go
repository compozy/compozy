package daemon

import (
	settingspkg "github.com/compozy/agh/internal/settings"
	"github.com/compozy/agh/internal/vault"
)

func settingsProviderVaultDependency(service *vault.Service) settingspkg.ProviderSecretStore {
	if service == nil {
		return nil
	}
	return service
}

func settingsMarketplaceCatalogDependency(runtime *marketplaceRuntime) settingspkg.MCPCatalog {
	if runtime == nil {
		return nil
	}
	return runtime
}
