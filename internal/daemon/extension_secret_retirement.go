package daemon

import (
	"context"
	"errors"
	"fmt"
	"slices"

	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/vault"
)

type extensionSecretRetirement struct {
	bindings []extensionpkg.EnvBinding
	secrets  []extensionSecretSnapshot
}

func (s *daemonExtensionService) retireExtensionSecretBindings(
	ctx context.Context,
	key extensionpkg.InstanceKey,
) (*extensionSecretRetirement, error) {
	if s.envBindings == nil && s.secretVault == nil {
		return &extensionSecretRetirement{}, nil
	}
	if s.envBindings == nil || s.secretVault == nil {
		return nil, errors.New("daemon: extension secret storage is incomplete")
	}
	bindings, err := s.envBindings.ListEnvBindings(ctx, key.Name, key.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("daemon: snapshot retiring extension bindings: %w", err)
	}
	retirement := &extensionSecretRetirement{bindings: slices.Clone(bindings)}
	refsForInstance := make(map[string]int, len(bindings))
	for _, binding := range bindings {
		refsForInstance[vault.NormalizeRef(binding.SecretRef)]++
	}
	refs := make([]string, 0, len(refsForInstance))
	for ref := range refsForInstance {
		refs = append(refs, ref)
	}
	slices.Sort(refs)
	for _, ref := range refs {
		count, countErr := s.envBindings.CountEnvBindingsBySecretRef(ctx, ref)
		if countErr != nil {
			return nil, fmt.Errorf("daemon: count retiring extension secret refs: %w", countErr)
		}
		if count != int64(refsForInstance[ref]) {
			continue
		}
		metadata, metadataErr := s.secretVault.GetMetadata(ctx, ref)
		if errors.Is(metadataErr, vault.ErrSecretNotFound) || (metadataErr == nil && !metadata.Present) {
			continue
		}
		if metadataErr != nil {
			return nil, fmt.Errorf("daemon: inspect retiring extension secret: %w", metadataErr)
		}
		if metadata.Kind != extensionpkg.ExtensionEnvBindingKind {
			continue
		}
		value, resolveErr := s.secretVault.ResolveRef(ctx, ref)
		if resolveErr != nil {
			return nil, fmt.Errorf("daemon: snapshot retiring extension secret: %w", resolveErr)
		}
		retirement.secrets = append(retirement.secrets, extensionSecretSnapshot{
			ref: ref, kind: metadata.Kind, value: value, existed: true,
		})
	}
	if err := s.envBindings.DeleteEnvBindings(ctx, key.Name, key.WorkspaceID); err != nil {
		return nil, fmt.Errorf("daemon: retire extension bindings: %w", err)
	}
	for _, ref := range refs {
		if err := s.gcOwnedExtensionSecret(ctx, ref); err != nil {
			return nil, errors.Join(err, retirement.rollback(ctx, s))
		}
	}
	return retirement, nil
}

func (r *extensionSecretRetirement) rollback(ctx context.Context, s *daemonExtensionService) error {
	if r == nil || s == nil || s.envBindings == nil || s.secretVault == nil {
		return nil
	}
	rollbackCtx, cancel := extensionSecretRollbackContext(ctx)
	defer cancel()
	var rollbackErr error
	for _, snapshot := range r.secrets {
		if _, err := s.secretVault.PutSecret(rollbackCtx, snapshot.ref, snapshot.kind, snapshot.value); err != nil {
			rollbackErr = errors.Join(rollbackErr, fmt.Errorf("daemon: restore retired extension secret: %w", err))
		}
	}
	for _, binding := range r.bindings {
		if err := s.envBindings.PutEnvBinding(rollbackCtx, binding); err != nil {
			rollbackErr = errors.Join(rollbackErr, fmt.Errorf("daemon: restore retired extension binding: %w", err))
		}
	}
	return rollbackErr
}
