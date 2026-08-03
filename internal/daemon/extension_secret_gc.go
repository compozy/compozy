package daemon

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/vault"
)

func (s *daemonExtensionService) gcSupersededExtensionSecrets(
	ctx context.Context,
	previous []extensionpkg.EnvBinding,
	prepared []preparedExtensionSecret,
) error {
	nextByName := make(map[string]string, len(prepared))
	for _, write := range prepared {
		nextByName[write.envName] = vault.NormalizeRef(write.ref)
	}
	candidates := make(map[string]struct{}, len(previous))
	for _, binding := range previous {
		nextRef, updated := nextByName[strings.TrimSpace(binding.EnvName)]
		if !updated {
			continue
		}
		ref := vault.NormalizeRef(binding.SecretRef)
		if ref != nextRef {
			candidates[ref] = struct{}{}
		}
	}
	refs := make([]string, 0, len(candidates))
	for ref := range candidates {
		refs = append(refs, ref)
	}
	slices.Sort(refs)
	deleted := make([]extensionSecretSnapshot, 0, len(refs))
	for _, ref := range refs {
		snapshot, eligible, err := s.extensionSecretGCSnapshot(ctx, ref)
		if err != nil {
			return errors.Join(err, s.restoreExtensionSecretSnapshots(ctx, deleted))
		}
		if !eligible {
			continue
		}
		if err := s.secretVault.DeleteSecret(ctx, ref); err != nil && !errors.Is(err, vault.ErrSecretNotFound) {
			return errors.Join(
				fmt.Errorf("daemon: garbage-collect superseded extension secret: %w", err),
				s.restoreExtensionSecretSnapshots(ctx, deleted),
			)
		}
		deleted = append(deleted, snapshot)
	}
	return nil
}

func (s *daemonExtensionService) gcOwnedExtensionSecret(ctx context.Context, ref string) error {
	metadata, eligible, err := s.extensionSecretGCMetadata(ctx, ref)
	if err != nil || !eligible {
		return err
	}
	if err := s.secretVault.DeleteSecret(ctx, metadata.Ref); err != nil && !errors.Is(err, vault.ErrSecretNotFound) {
		return fmt.Errorf("daemon: garbage-collect extension secret: %w", err)
	}
	return nil
}

func (s *daemonExtensionService) extensionSecretGCSnapshot(
	ctx context.Context,
	ref string,
) (extensionSecretSnapshot, bool, error) {
	metadata, eligible, err := s.extensionSecretGCMetadata(ctx, ref)
	if err != nil || !eligible {
		return extensionSecretSnapshot{}, eligible, err
	}
	return s.snapshotExtensionOwnedSecret(ctx, metadata)
}

func (s *daemonExtensionService) extensionSecretGCMetadata(
	ctx context.Context,
	ref string,
) (vault.Metadata, bool, error) {
	ref = vault.NormalizeRef(ref)
	if strings.TrimSpace(ref) == "" {
		return vault.Metadata{}, false, nil
	}
	count, err := s.envBindings.CountEnvBindingsBySecretRef(ctx, ref)
	if err != nil {
		return vault.Metadata{}, false, fmt.Errorf("daemon: count extension bindings for secret ref: %w", err)
	}
	if count != 0 {
		return vault.Metadata{}, false, nil
	}
	return s.inspectExtensionOwnedSecretMetadata(ctx, ref)
}

func (s *daemonExtensionService) extensionOwnedSecretSnapshot(
	ctx context.Context,
	ref string,
) (extensionSecretSnapshot, bool, error) {
	ref = vault.NormalizeRef(ref)
	if strings.TrimSpace(ref) == "" {
		return extensionSecretSnapshot{}, false, nil
	}
	metadata, owned, err := s.inspectExtensionOwnedSecretMetadata(ctx, ref)
	if err != nil || !owned {
		return extensionSecretSnapshot{}, owned, err
	}
	return s.snapshotExtensionOwnedSecret(ctx, metadata)
}

func (s *daemonExtensionService) inspectExtensionOwnedSecretMetadata(
	ctx context.Context,
	ref string,
) (vault.Metadata, bool, error) {
	metadata, err := s.secretVault.GetMetadata(ctx, ref)
	if errors.Is(err, vault.ErrSecretNotFound) || (err == nil && !metadata.Present) {
		return vault.Metadata{}, false, nil
	}
	if err != nil {
		return vault.Metadata{}, false, fmt.Errorf("daemon: inspect extension-owned secret: %w", err)
	}
	if strings.TrimSpace(metadata.Kind) != extensionpkg.ExtensionEnvBindingKind {
		return vault.Metadata{}, false, nil
	}
	metadata.Ref = ref
	return metadata, true, nil
}

func (s *daemonExtensionService) snapshotExtensionOwnedSecret(
	ctx context.Context,
	metadata vault.Metadata,
) (extensionSecretSnapshot, bool, error) {
	value, err := s.secretVault.ResolveRef(ctx, metadata.Ref)
	if err != nil {
		return extensionSecretSnapshot{}, false, fmt.Errorf("daemon: snapshot extension-owned secret: %w", err)
	}
	return extensionSecretSnapshot{
		ref: metadata.Ref, kind: metadata.Kind, value: value, existed: true,
	}, true, nil
}

func (s *daemonExtensionService) restoreExtensionSecretSnapshots(
	ctx context.Context,
	snapshots []extensionSecretSnapshot,
) error {
	rollbackCtx, cancel := extensionSecretRollbackContext(ctx)
	defer cancel()
	errList := make([]error, 0, len(snapshots))
	for _, snapshot := range slices.Backward(snapshots) {
		if _, err := s.secretVault.PutSecret(rollbackCtx, snapshot.ref, snapshot.kind, snapshot.value); err != nil {
			errList = append(errList, fmt.Errorf("daemon: restore extension secret snapshot: %w", err))
		}
	}
	return errors.Join(errList...)
}
