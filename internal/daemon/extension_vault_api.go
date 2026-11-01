package daemon

import (
	"context"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/vault"
)

// extensionAwareVaultService preserves generic Vault writes while serializing extension-owned refs
// with package removal, including its snapshots, deletion and compensation.
func extensionAwareVaultService(service core.VaultService, extensions core.ExtensionService) core.VaultService {
	owner, ok := extensions.(*daemonExtensionService)
	if service == nil || !ok || owner == nil {
		return service
	}
	return &extensionVaultAPI{VaultService: service, lifecycle: owner.lifecycle}
}

type extensionVaultAPI struct {
	core.VaultService
	lifecycle *extensionLifecycleCoordinator
}

// PutSecret retains the generic endpoint contract and waits for any package retirement to finish.
func (s *extensionVaultAPI) PutSecret(ctx context.Context, ref, kind, value string) (vault.Metadata, error) {
	var result vault.Metadata
	err := s.withOwner(ctx, ref, func() error {
		var err error
		result, err = s.VaultService.PutSecret(ctx, ref, kind, value)
		return err
	})
	return result, err
}

// DeleteSecret participates in the same lock so removal rollback cannot resurrect a concurrent deletion.
func (s *extensionVaultAPI) DeleteSecret(ctx context.Context, ref string) error {
	return s.withOwner(ctx, ref, func() error { return s.VaultService.DeleteSecret(ctx, ref) })
}

// Generic refs need not name an installed package, so use the package lock without attachment validation.
func (s *extensionVaultAPI) withOwner(ctx context.Context, ref string, fn func() error) error {
	name, err := vault.ExtensionSecretRefOwner(ref)
	if err != nil {
		return fn()
	}
	return s.lifecycle.withPackages(ctx, []string{name}, exclusiveExtensionPackageWeight, fn)
}
