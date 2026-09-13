package daemon

import (
	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/compozy/compozy/internal/vault"
)

func settingsProviderVaultDependency(service *vault.Service) settingspkg.ProviderSecretStore {
	if service == nil {
		return nil
	}
	return service
}
