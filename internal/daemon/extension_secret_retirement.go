package daemon

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/extensioninput"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/vault"
)

type extensionSecretRetirement struct {
	bindings             []extensionpkg.EnvBinding
	ownedSecretSnapshots []extensionSecretSnapshot
	inputRows            []retiredExtensionInputRows
}

// retiredExtensionInputRows holds the deletions applied to one instance's typed input rows.
type retiredExtensionInputRows struct {
	instance  extensioninput.Instance
	deletions []extensioninput.Mutation
}

// Development and attached-instance removal retire credentials only in the selected workspace.
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
	scopes := make([]extensionpkg.InstallationScope, 0, len(profileIDs))
	for _, profileID := range profileIDs {
		scopes = append(scopes, extensionpkg.InstallationScope{ProfileID: profileID, WorkspaceID: key.WorkspaceID})
	}
	return s.retireExtensionBindingScopes(ctx, key.Name, scopes)
}

// An orphan has no installation rows, so credential ownership must come from persisted binding cells.
func (s *daemonExtensionService) retireOrphanSecretBindings(
	ctx context.Context,
	key extensionpkg.InstanceKey,
) (*extensionSecretRetirement, error) {
	scopes, err := s.registry.EnvBindingScopes(ctx, key.Name)
	if err != nil {
		return nil, err
	}
	retirement, err := s.retireExtensionBindingScopes(ctx, key.Name, scopes)
	if err != nil {
		return nil, err
	}
	if err := s.retireUnboundOrphanSecrets(ctx, key.Name, retirement); err != nil {
		return nil, errors.Join(err, retirement.rollback(ctx, s))
	}
	return retirement, nil
}

// Reference counts are captured across all retiring cells before any owned secret is collected.
func (s *daemonExtensionService) retireExtensionBindingScopes(
	ctx context.Context,
	name string,
	scopes []extensionpkg.InstallationScope,
) (*extensionSecretRetirement, error) {
	if s.envBindings == nil && s.secretVault == nil {
		return &extensionSecretRetirement{}, nil
	}
	if s.envBindings == nil || s.secretVault == nil {
		return nil, errors.New("daemon: extension secret storage is incomplete")
	}
	bindings := make([]extensionpkg.EnvBinding, 0)
	for _, scope := range scopes {
		profileBindings, listErr := s.envBindings.ListEnvBindings(ctx, name, scope.ProfileID, scope.WorkspaceID)
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
	if err := s.retireSubprocessInputRows(ctx, name, retirement); err != nil {
		return nil, errors.Join(err, retirement.rollback(ctx, s))
	}
	for _, scope := range scopes {
		if err := s.envBindings.DeleteEnvBindings(ctx, name, scope.ProfileID, scope.WorkspaceID); err != nil {
			return nil, errors.Join(
				fmt.Errorf("daemon: retire extension bindings for profile %q: %w", scope.ProfileID, err),
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

// rollback restores secret values before their bindings so compensated reloads can resolve credentials.
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
	for _, retired := range r.inputRows {
		restores := make([]extensioninput.Mutation, 0, len(retired.deletions))
		for _, deletion := range retired.deletions {
			restores = append(restores, extensioninput.Mutation{InputID: deletion.InputID, After: deletion.Before})
		}
		if err := s.inputs.Apply(rollbackCtx, retired.instance, restores); err != nil {
			rollbackErr = errors.Join(rollbackErr, fmt.Errorf("daemon: restore retired extension inputs: %w", err))
		}
	}
	r.inputRows = nil
	return rollbackErr
}

// retireSubprocessInputRows deletes the typed rows that mirror retiring subprocess input bindings, so a
// reinstall in the same cell starts from defaults instead of carrying a removed value forward into a
// new binding (rule 3a). Rows of inputs without an active input binding keep their prior retention: an
// inactive binding no longer mirrors its input, whose typed row may hold a later server-only value.
func (s *daemonExtensionService) retireSubprocessInputRows(
	ctx context.Context, name string, retirement *extensionSecretRetirement,
) error {
	if s.inputs == nil {
		return nil
	}
	var instances []extensioninput.Instance
	inputIDs := map[extensioninput.Instance][]string{}
	for _, binding := range retirement.bindings {
		if binding.InputID == "" || binding.MCPServer != "" || binding.Inactive {
			continue
		}
		instance := extensioninput.Instance{
			Extension:   name,
			ProfileID:   binding.ProfileID,
			WorkspaceID: binding.WorkspaceID,
		}
		if _, seen := inputIDs[instance]; !seen {
			instances = append(instances, instance)
		}
		inputIDs[instance] = append(inputIDs[instance], binding.InputID)
	}
	for _, instance := range instances {
		rows, err := s.inputs.List(ctx, instance)
		if err != nil {
			return fmt.Errorf("daemon: snapshot retiring extension inputs: %w", err)
		}
		var deletions []extensioninput.Mutation
		for _, inputID := range slices.Compact(slices.Sorted(slices.Values(inputIDs[instance]))) {
			if row, found := rows[inputID]; found {
				deletions = append(deletions, extensioninput.Mutation{InputID: inputID, Before: &row})
			}
		}
		if len(deletions) == 0 {
			continue
		}
		if err := s.inputs.Apply(ctx, instance, deletions); err != nil {
			return fmt.Errorf("daemon: retire extension inputs: %w", err)
		}
		retirement.inputRows = append(retirement.inputRows, retiredExtensionInputRows{
			instance: instance, deletions: deletions,
		})
	}
	return nil
}

// The caller holds withPackageMutation's exclusive name lock across every workspace.
// The store filters encoded ownership before materialization; validate it again before deleting.
func (s *daemonExtensionService) retireUnboundOrphanSecrets(
	ctx context.Context, name string, retirement *extensionSecretRetirement,
) error {
	if s.secretVault == nil {
		return nil
	}
	entries, err := s.secretVault.ListExtensionMetadata(ctx, name)
	if err != nil {
		return fmt.Errorf("daemon: list orphan extension secrets: %w", err)
	}
	for _, entry := range entries {
		if vault.ValidateExtensionSecretRefOwner(entry.Ref, name) != nil {
			continue
		}
		snapshot, eligible, err := s.extensionSecretGCSnapshot(ctx, entry.Ref)
		if err != nil {
			return err
		}
		if !eligible {
			continue
		}
		retirement.ownedSecretSnapshots = append(retirement.ownedSecretSnapshots, snapshot)
		if err := s.gcOwnedExtensionSecret(ctx, entry.Ref); err != nil {
			return err
		}
	}
	return nil
}
