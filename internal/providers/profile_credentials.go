package providers

import (
	"context"
	"errors"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/vault"
)

// ProfileCredentialSlots applies present profile-owned provider overrides and preserves user fallbacks.
func ProfileCredentialSlots(
	ctx context.Context,
	providerName string,
	profileName string,
	slots []compozyconfig.ProviderCredentialSlot,
	metadata VaultRefResolver,
) ([]compozyconfig.ProviderCredentialSlot, error) {
	resolved := append([]compozyconfig.ProviderCredentialSlot(nil), slots...)
	profileName = strings.TrimSpace(profileName)
	if profileName == "" || profileName == "default" || len(resolved) == 0 || metadata == nil {
		return resolved, nil
	}
	prefix, err := vault.ProfileSecretOwnerPrefix(profileName, "providers", strings.TrimSpace(providerName))
	if err != nil {
		return nil, fmt.Errorf("providers: build profile credential prefix: %w", err)
	}
	for index := range resolved {
		slotName := strings.TrimSpace(resolved[index].Name)
		profileRef := prefix + slotName
		if err := vault.ValidateProfileSecretRefAccess(profileRef, profileName); err != nil {
			return nil, fmt.Errorf("providers: validate profile credential slot %q: %w", slotName, err)
		}
		stored, err := metadata.GetMetadata(ctx, profileRef)
		switch {
		case err == nil && stored.Present:
			resolved[index].SecretRef = profileRef
		case err == nil:
		case errors.Is(err, vault.ErrSecretNotFound), errors.Is(err, vault.ErrMissingSecret):
		default:
			return nil, fmt.Errorf("providers: inspect profile credential slot %q: %w", slotName, err)
		}
	}
	return resolved, nil
}
