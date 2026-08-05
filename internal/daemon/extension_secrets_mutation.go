package daemon

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/vault"
)

const extensionSecretRollbackTimeout = 5 * time.Second

type preparedExtensionSecret struct {
	envName string
	ref     string
	value   *string
}

type extensionSecretSnapshot struct {
	ref     string
	kind    string
	value   string
	existed bool
}

type extensionSecretMutation struct {
	envName         string
	previousBinding *extensionpkg.EnvBinding
	secret          *extensionSecretSnapshot
}

func (s *daemonExtensionService) prepareExtensionSecrets(
	ctx context.Context,
	key extensionpkg.InstanceKey,
	declared []string,
	req contract.SetExtensionSecretsRequest,
) ([]preparedExtensionSecret, error) {
	declaredSet := make(map[string]struct{}, len(declared))
	for _, name := range declared {
		declaredSet[name] = struct{}{}
	}
	byName := make(map[string]contract.ExtensionSecretInput, len(req.Secrets))
	for rawName, input := range req.Secrets {
		name := strings.TrimSpace(rawName)
		if !vault.EnvNamePattern.MatchString(name) {
			return nil, &extensionpkg.EnvBindingValidationError{
				EnvName: name,
				Cause:   extensionpkg.ErrExtensionEnvBindingInvalid,
			}
		}
		if _, exists := byName[name]; exists {
			return nil, &extensionpkg.EnvBindingValidationError{
				EnvName: name,
				Cause:   extensionpkg.ErrExtensionEnvBindingInvalid,
			}
		}
		if _, ok := declaredSet[name]; !ok {
			return nil, &extensionpkg.EnvBindingValidationError{
				EnvName: name, Declared: slices.Clone(declared), Cause: extensionpkg.ErrExtensionEnvBindingUndeclared,
			}
		}
		if (input.Value == nil) == (input.VaultRef == nil) {
			return nil, &extensionpkg.EnvBindingValidationError{
				EnvName: name,
				Cause:   extensionpkg.ErrExtensionEnvBindingInvalid,
			}
		}
		byName[name] = input
	}
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	slices.Sort(names)
	prepared := make([]preparedExtensionSecret, 0, len(names))
	for _, name := range names {
		input := byName[name]
		if input.Value != nil {
			if strings.TrimSpace(*input.Value) == "" {
				return nil, &extensionpkg.EnvBindingValidationError{
					EnvName: name,
					Cause:   extensionpkg.ErrExtensionEnvBindingInvalid,
				}
			}
			prepared = append(prepared, preparedExtensionSecret{
				envName: name, ref: vault.ExtensionSecretRef(key.Name, key.WorkspaceID, name), value: input.Value,
			})
			continue
		}
		ref := vault.NormalizeRef(*input.VaultRef)
		if err := vault.ValidateSecretRefNamespace(ref, "extensions"); err != nil {
			return nil, &extensionpkg.EnvBindingValidationError{
				EnvName: name,
				Cause:   extensionpkg.ErrExtensionEnvBindingInvalid,
			}
		}
		metadata, err := s.secretVault.GetMetadata(ctx, ref)
		if errors.Is(err, vault.ErrSecretNotFound) || (err == nil && !metadata.Present) {
			return nil, &extensionpkg.EnvBindingValidationError{
				EnvName: name,
				Cause:   extensionpkg.ErrExtensionEnvBindingDangling,
			}
		}
		if err != nil {
			return nil, fmt.Errorf("daemon: inspect extension secret binding %q: %w", name, err)
		}
		prepared = append(prepared, preparedExtensionSecret{envName: name, ref: ref})
	}
	return prepared, nil
}

func (s *daemonExtensionService) applyExtensionSecret(
	ctx context.Context,
	key extensionpkg.InstanceKey,
	write preparedExtensionSecret,
	previous extensionpkg.EnvBinding,
) (extensionSecretMutation, error) {
	mutation := extensionSecretMutation{envName: write.envName}
	if previous.EnvName != "" {
		previousCopy := previous
		mutation.previousBinding = &previousCopy
	}
	if write.value != nil {
		snapshot, err := s.snapshotExtensionSecret(ctx, write.ref)
		if err != nil {
			return extensionSecretMutation{}, fmt.Errorf("daemon: snapshot extension secret %q: %w", write.envName, err)
		}
		mutation.secret = &snapshot
		if _, err := s.secretVault.PutSecret(
			ctx,
			write.ref,
			extensionpkg.ExtensionEnvBindingKind,
			*write.value,
		); err != nil {
			rollbackErr := s.rollbackExtensionSecretMutations(ctx, key, []extensionSecretMutation{mutation})
			return extensionSecretMutation{}, errors.Join(
				fmt.Errorf("daemon: store extension secret %q: %w", write.envName, err),
				rollbackErr,
			)
		}
	}
	now := s.now().UTC()
	createdAt := now
	if mutation.previousBinding != nil && !mutation.previousBinding.CreatedAt.IsZero() {
		createdAt = mutation.previousBinding.CreatedAt
	}
	binding := extensionpkg.EnvBinding{
		ExtensionName: key.Name, WorkspaceID: key.WorkspaceID, EnvName: write.envName,
		SecretRef: write.ref, Kind: extensionpkg.ExtensionEnvBindingKind, CreatedAt: createdAt, UpdatedAt: now,
	}
	if err := s.envBindings.PutEnvBinding(ctx, binding); err != nil {
		rollbackErr := s.rollbackExtensionSecretMutations(ctx, key, []extensionSecretMutation{mutation})
		return extensionSecretMutation{}, errors.Join(
			fmt.Errorf("daemon: store extension secret binding %q: %w", write.envName, err), rollbackErr,
		)
	}
	return mutation, nil
}

func (s *daemonExtensionService) snapshotExtensionSecret(
	ctx context.Context,
	ref string,
) (extensionSecretSnapshot, error) {
	metadata, err := s.secretVault.GetMetadata(ctx, ref)
	if errors.Is(err, vault.ErrSecretNotFound) || (err == nil && !metadata.Present) {
		return extensionSecretSnapshot{ref: ref}, nil
	}
	if err != nil {
		return extensionSecretSnapshot{}, err
	}
	value, err := s.secretVault.ResolveRef(ctx, ref)
	if err != nil {
		return extensionSecretSnapshot{}, err
	}
	return extensionSecretSnapshot{ref: ref, kind: metadata.Kind, value: value, existed: true}, nil
}

func (s *daemonExtensionService) rollbackExtensionSecretMutations(
	ctx context.Context,
	key extensionpkg.InstanceKey,
	mutations []extensionSecretMutation,
) error {
	rollbackCtx, cancel := extensionSecretRollbackContext(ctx)
	defer cancel()
	errList := make([]error, 0)
	for _, mutation := range slices.Backward(mutations) {
		if mutation.previousBinding == nil {
			if err := s.envBindings.DeleteEnvBinding(
				rollbackCtx,
				key.Name,
				key.WorkspaceID,
				mutation.envName,
			); err != nil {
				errList = append(
					errList,
					fmt.Errorf("daemon: rollback extension binding %q: %w", mutation.envName, err),
				)
			}
		} else if err := s.envBindings.PutEnvBinding(rollbackCtx, *mutation.previousBinding); err != nil {
			errList = append(errList, fmt.Errorf("daemon: restore extension binding %q: %w", mutation.envName, err))
		}
		if mutation.secret == nil {
			continue
		}
		if !mutation.secret.existed {
			if err := s.secretVault.DeleteSecret(
				rollbackCtx,
				mutation.secret.ref,
			); err != nil &&
				!errors.Is(err, vault.ErrSecretNotFound) {
				errList = append(
					errList,
					fmt.Errorf("daemon: rollback new extension secret %q: %w", mutation.envName, err),
				)
			}
			continue
		}
		if _, err := s.secretVault.PutSecret(
			rollbackCtx, mutation.secret.ref, mutation.secret.kind, mutation.secret.value,
		); err != nil {
			errList = append(errList, fmt.Errorf("daemon: restore extension secret %q: %w", mutation.envName, err))
		}
	}
	return errors.Join(errList...)
}

func extensionSecretRollbackContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), extensionSecretRollbackTimeout)
}

func extensionBindingsByName(bindings []extensionpkg.EnvBinding) map[string]extensionpkg.EnvBinding {
	result := make(map[string]extensionpkg.EnvBinding, len(bindings))
	for _, binding := range bindings {
		result[strings.TrimSpace(binding.EnvName)] = binding
	}
	return result
}
