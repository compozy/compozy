package daemon

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/mcppolicy"
	"github.com/compozy/compozy/internal/vault"
)

const extensionSecretRollbackTimeout = 5 * time.Second

type preparedExtensionSecret struct {
	envName    string
	ref        string
	value      *string
	mcpServer  string
	headerName string
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
	profileID string,
	ext *extensionpkg.Extension,
	req contract.SetExtensionSecretsRequest,
) ([]preparedExtensionSecret, error) {
	declared := normalizedDeclaredEnv(ext)
	byName, err := normalizeExtensionSecretInputs(ext, declared, req.Bindings)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	slices.Sort(names)
	prepared := make([]preparedExtensionSecret, 0, len(names))
	for _, name := range names {
		secret, err := s.prepareExtensionSecret(ctx, key, profileID, byName[name])
		if err != nil {
			return nil, err
		}
		prepared = append(prepared, secret)
	}
	return prepared, nil
}

func normalizeExtensionSecretInputs(
	ext *extensionpkg.Extension,
	declared []string,
	inputs []contract.ExtensionSecretBindingInput,
) (map[string]contract.ExtensionSecretBindingInput, error) {
	declaredSet := make(map[string]struct{}, len(declared))
	for _, name := range declared {
		declaredSet[name] = struct{}{}
	}
	byName := make(map[string]contract.ExtensionSecretBindingInput, len(inputs))
	for _, input := range inputs {
		name := strings.TrimSpace(input.EnvName)
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
		mcpServer := strings.TrimSpace(input.MCPServer)
		headerName := strings.TrimSpace(input.HeaderName)
		if (mcpServer == "") != (headerName == "") {
			return nil, &extensionpkg.EnvBindingValidationError{
				EnvName: name,
				Cause:   extensionpkg.ErrExtensionEnvBindingInvalid,
			}
		}
		if mcpServer == "" {
			if _, ok := declaredSet[name]; !ok {
				return nil, &extensionpkg.EnvBindingValidationError{
					EnvName:  name,
					Declared: slices.Clone(declared),
					Cause:    extensionpkg.ErrExtensionEnvBindingUndeclared,
				}
			}
		} else if err := validateRemoteHeaderTarget(ext, mcpServer, headerName); err != nil {
			return nil, &extensionpkg.EnvBindingValidationError{
				EnvName: name,
				Cause:   errors.Join(extensionpkg.ErrExtensionEnvBindingInvalid, err),
			}
		}
		if (input.Value == nil) == (input.VaultRef == nil) {
			return nil, &extensionpkg.EnvBindingValidationError{
				EnvName: name,
				Cause:   extensionpkg.ErrExtensionEnvBindingInvalid,
			}
		}
		input.EnvName = name
		input.MCPServer = mcpServer
		input.HeaderName = headerName
		byName[name] = input
	}
	return byName, nil
}

func (s *daemonExtensionService) prepareExtensionSecret(
	ctx context.Context,
	key extensionpkg.InstanceKey,
	profileID string,
	input contract.ExtensionSecretBindingInput,
) (preparedExtensionSecret, error) {
	name := input.EnvName
	if input.Value != nil {
		if strings.TrimSpace(*input.Value) == "" {
			return preparedExtensionSecret{}, &extensionpkg.EnvBindingValidationError{
				EnvName: name, Cause: extensionpkg.ErrExtensionEnvBindingInvalid,
			}
		}
		return preparedExtensionSecret{
			envName:    name,
			ref:        vault.ExtensionProfileSecretRef(key.Name, profileID, key.WorkspaceID, name),
			value:      input.Value,
			mcpServer:  input.MCPServer,
			headerName: input.HeaderName,
		}, nil
	}
	ref := vault.NormalizeRef(*input.VaultRef)
	if err := vault.ValidateSecretRefNamespace(ref, "extensions"); err != nil {
		return preparedExtensionSecret{}, &extensionpkg.EnvBindingValidationError{
			EnvName: name, Cause: extensionpkg.ErrExtensionEnvBindingInvalid,
		}
	}
	metadata, err := s.secretVault.GetMetadata(ctx, ref)
	if errors.Is(err, vault.ErrSecretNotFound) || (err == nil && !metadata.Present) {
		return preparedExtensionSecret{}, &extensionpkg.EnvBindingValidationError{
			EnvName: name, Cause: extensionpkg.ErrExtensionEnvBindingDangling,
		}
	}
	if err != nil {
		return preparedExtensionSecret{}, fmt.Errorf("daemon: inspect extension secret binding %q: %w", name, err)
	}
	return preparedExtensionSecret{
		envName: name, ref: ref, mcpServer: input.MCPServer, headerName: input.HeaderName,
	}, nil
}

func validateRemoteHeaderTarget(ext *extensionpkg.Extension, serverName, headerName string) error {
	if ext == nil || ext.Manifest == nil {
		return errors.New("extension manifest is required")
	}
	server, ok := ext.Manifest.Resources.MCPServers[serverName]
	if !ok {
		return fmt.Errorf("mcp server %q is not declared", serverName)
	}
	if strings.TrimSpace(server.Transport) != string(compozyconfig.MCPServerTransportHTTP) {
		return fmt.Errorf("mcp server %q is not remote", serverName)
	}
	if err := mcppolicy.ValidateHeaders(
		server.Headers,
		map[string]string{headerName: "secret-reference"},
		mcppolicy.SourceOperatorSecret,
		false,
	); err != nil {
		return err
	}
	return nil
}

func (s *daemonExtensionService) applyExtensionSecret(
	ctx context.Context,
	key extensionpkg.InstanceKey,
	profileID string,
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
			rollbackErr := s.rollbackExtensionSecretMutations(ctx, key, profileID, []extensionSecretMutation{mutation})
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
		ExtensionName: key.Name, ProfileID: profileID,
		WorkspaceID: key.WorkspaceID, EnvName: write.envName,
		SecretRef: write.ref, MCPServer: write.mcpServer, HeaderName: write.headerName,
		Kind: extensionpkg.ExtensionEnvBindingKind, CreatedAt: createdAt, UpdatedAt: now,
	}
	if err := s.envBindings.PutEnvBinding(ctx, binding); err != nil {
		rollbackErr := s.rollbackExtensionSecretMutations(ctx, key, profileID, []extensionSecretMutation{mutation})
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
	profileID string,
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
				profileID,
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
