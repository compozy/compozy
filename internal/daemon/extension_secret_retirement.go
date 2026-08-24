package daemon

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/vault"
)

type extensionSecretRetirement struct {
	bindings             []extensionpkg.EnvBinding
	ownedSecretSnapshots []extensionSecretSnapshot
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
	profileIDs, err := s.extensionSecretProfileIDs(ctx)
	if err != nil {
		return nil, err
	}
	bindings := make([]extensionpkg.EnvBinding, 0)
	for _, profileID := range profileIDs {
		profileBindings, listErr := s.envBindings.ListEnvBindings(ctx, key.Name, profileID, key.WorkspaceID)
		if listErr != nil {
			return nil, fmt.Errorf("daemon: snapshot retiring extension bindings: %w", listErr)
		}
		bindings = append(bindings, profileBindings...)
	}
	retirement := &extensionSecretRetirement{bindings: slices.Clone(bindings)}
	instanceBindingCountByRef := make(map[string]int, len(bindings))
	for _, binding := range bindings {
		instanceBindingCountByRef[vault.NormalizeRef(binding.SecretRef)]++
	}
	refs := make([]string, 0, len(instanceBindingCountByRef))
	for ref := range instanceBindingCountByRef {
		refs = append(refs, ref)
	}
	slices.Sort(refs)
	for _, ref := range refs {
		count, countErr := s.envBindings.CountEnvBindingsBySecretRef(ctx, ref)
		if countErr != nil {
			return nil, fmt.Errorf("daemon: count retiring extension secret refs: %w", countErr)
		}
		if count != int64(instanceBindingCountByRef[ref]) {
			continue
		}
		snapshot, owned, snapshotErr := s.extensionOwnedSecretSnapshot(ctx, ref)
		if snapshotErr != nil {
			return nil, fmt.Errorf("daemon: snapshot retiring extension secret: %w", snapshotErr)
		}
		if !owned {
			continue
		}
		retirement.ownedSecretSnapshots = append(retirement.ownedSecretSnapshots, snapshot)
	}
	for _, profileID := range profileIDs {
		if err := s.envBindings.DeleteEnvBindings(ctx, key.Name, profileID, key.WorkspaceID); err != nil {
			return nil, errors.Join(
				fmt.Errorf("daemon: retire extension bindings for profile %q: %w", profileID, err),
				retirement.rollback(ctx, s),
			)
		}
	}
	for _, ref := range refs {
		if err := s.gcOwnedExtensionSecret(ctx, ref); err != nil {
			return nil, errors.Join(err, retirement.rollback(ctx, s))
		}
	}
	return retirement, nil
}

func (s *daemonExtensionService) extensionSecretProfileIDs(
	ctx context.Context,
) ([]string, error) {
	profileIDs := []string{store.DefaultProfileID}
	if s.profiles == nil {
		return profileIDs, nil
	}
	profiles, err := s.profiles.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("daemon: list profiles for extension secret retirement: %w", err)
	}
	seen := map[string]struct{}{store.DefaultProfileID: {}}
	for _, profile := range profiles {
		profileID := strings.TrimSpace(profile.ID)
		if profileID == "" {
			continue
		}
		if _, exists := seen[profileID]; exists {
			continue
		}
		seen[profileID] = struct{}{}
		profileIDs = append(profileIDs, profileID)
	}
	slices.Sort(profileIDs)
	return profileIDs, nil
}

func (r *extensionSecretRetirement) rollback(ctx context.Context, s *daemonExtensionService) error {
	if r == nil || s == nil || s.envBindings == nil || s.secretVault == nil {
		return nil
	}
	rollbackErr := s.restoreExtensionSecretSnapshots(ctx, r.ownedSecretSnapshots)
	rollbackCtx, cancel := extensionSecretRollbackContext(ctx)
	defer cancel()
	for _, binding := range r.bindings {
		if err := s.envBindings.PutEnvBinding(rollbackCtx, binding); err != nil {
			rollbackErr = errors.Join(rollbackErr, fmt.Errorf("daemon: restore retired extension binding: %w", err))
		}
	}
	return rollbackErr
}
