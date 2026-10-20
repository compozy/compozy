package settings

import (
	"context"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/providers"
)

func (s *service) sessionEnvironmentStatuses(
	ctx context.Context,
	declarations config.SessionEnv,
) ([]ProviderCredentialStatus, error) {
	env := &providers.ProbeEnv{
		Vault: s.providerSecrets,
		LookupEnv: func(name string) (string, bool) {
			if s.envPresent(name) {
				return "present", true
			}
			return "", false
		},
	}
	statuses, err := providers.SessionEnvStatuses(ctx, declarations, env)
	if err != nil {
		return nil, err
	}
	items := make([]ProviderCredentialStatus, 0, len(statuses))
	for _, status := range statuses {
		items = append(items, ProviderCredentialStatus{
			Name: status.Name, TargetEnv: status.TargetEnv, SecretRef: status.SecretRef,
			Required: status.Required, Present: status.Present, Source: status.Source,
		})
	}
	return items, nil
}
