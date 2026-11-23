package daemon

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/extensioninput"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/vault"
)

type extensionSecretVault interface {
	ListExtensionMetadata(context.Context, string) ([]vault.Metadata, error)
	PutSecret(context.Context, string, string, string) (vault.Metadata, error)
	ResolveRef(context.Context, string) (string, error)
	GetMetadata(context.Context, string) (vault.Metadata, error)
	DeleteSecret(context.Context, string) error
}

func (s *daemonExtensionService) ListExtensionSecrets(
	ctx context.Context,
	name string,
	actor taskpkg.ActorContext,
) (contract.ExtensionSecretsPayload, error) {
	if err := actor.Validate(); err != nil {
		return contract.ExtensionSecretsPayload{}, err
	}
	if !actor.Authority.Read {
		return contract.ExtensionSecretsPayload{}, taskpkg.ErrPermissionDenied
	}
	key, err := s.extensionSecretInstanceKey(ctx, name, actor)
	if err != nil {
		return contract.ExtensionSecretsPayload{}, err
	}
	profileID, err := extensionSecretProfileID(actor)
	if err != nil {
		return contract.ExtensionSecretsPayload{}, err
	}
	return s.extensionSecretsForProfileInstance(ctx, key, profileID)
}

func (s *daemonExtensionService) SetExtensionSecrets(
	ctx context.Context,
	name string,
	req contract.SetExtensionSecretsRequest,
	actor taskpkg.ActorContext,
) (contract.ExtensionSecretsPayload, error) {
	if err := validateExtensionWriteActor(actor); err != nil {
		return contract.ExtensionSecretsPayload{}, err
	}
	key, err := s.extensionSecretInstanceKey(ctx, name, actor)
	if err != nil {
		return contract.ExtensionSecretsPayload{}, err
	}
	profileID, err := extensionSecretProfileID(actor)
	if err != nil {
		return contract.ExtensionSecretsPayload{}, err
	}
	var payload contract.ExtensionSecretsPayload
	err = s.lifecycle.withInstance(ctx, key, func() error {
		var setErr error
		payload, setErr = s.setExtensionSecretsForProfileInstance(ctx, key, profileID, req)
		if setErr != nil {
			if !isExtensionSecretValidationRefusal(setErr) {
				setErr = errors.Join(setErr, s.recordExtensionSecretsFailedEvent(ctx, actor, key))
			}
			return setErr
		}
		if invalidateErr := s.invalidateExtensionSecretProfileRuntime(ctx, key, profileID); invalidateErr != nil {
			return errors.Join(invalidateErr, s.recordExtensionSecretsFailedEvent(ctx, actor, key))
		}
		return s.recordExtensionSecretsUpdatedEvent(ctx, actor, key, payload)
	})
	return payload, err
}

func (s *daemonExtensionService) DeleteExtensionSecret(
	ctx context.Context,
	name string,
	envName string,
	actor taskpkg.ActorContext,
) error {
	if err := validateExtensionWriteActor(actor); err != nil {
		return err
	}
	key, err := s.extensionSecretInstanceKey(ctx, name, actor)
	if err != nil {
		return err
	}
	profileID, err := extensionSecretProfileID(actor)
	if err != nil {
		return err
	}
	return s.lifecycle.withInstance(ctx, key, func() error {
		if deleteErr := s.deleteExtensionSecretForProfileInstance(ctx, key, profileID, envName); deleteErr != nil {
			if !isExtensionSecretValidationRefusal(deleteErr) {
				deleteErr = errors.Join(deleteErr, s.recordExtensionSecretsFailedEvent(ctx, actor, key))
			}
			return deleteErr
		}
		if invalidateErr := s.invalidateExtensionSecretProfileRuntime(ctx, key, profileID); invalidateErr != nil {
			return errors.Join(invalidateErr, s.recordExtensionSecretsFailedEvent(ctx, actor, key))
		}
		payload, payloadErr := s.extensionSecretsForProfileInstance(ctx, key, profileID)
		if payloadErr != nil {
			return errors.Join(payloadErr, s.recordExtensionSecretsFailedEvent(ctx, actor, key))
		}
		return s.recordExtensionSecretsUpdatedEvent(ctx, actor, key, payload)
	})
}

type extensionProfileRuntimeInvalidator interface {
	InvalidateProfileRuntime(context.Context, extensionpkg.InstanceKey) error
}

func (s *daemonExtensionService) invalidateExtensionSecretProfileRuntime(
	ctx context.Context,
	key extensionpkg.InstanceKey,
	profileID string,
) error {
	if strings.TrimSpace(profileID) == "" {
		return nil
	}
	runtime, err := s.devRuntime()
	if err != nil {
		return err
	}
	invalidator, ok := runtime.(extensionProfileRuntimeInvalidator)
	if !ok {
		return nil
	}
	return invalidator.InvalidateProfileRuntime(
		ctx,
		extensionpkg.ProfileInstanceKey(key.Name, profileID, key.WorkspaceID),
	)
}

func extensionSecretProfileID(actor taskpkg.ActorContext) (string, error) {
	if err := actor.ReadScope.Validate(); err != nil {
		return "", fmt.Errorf("daemon: extension secret profile scope: %w", err)
	}
	if actor.ReadScope.AllProfiles {
		return "", errors.New("daemon: extension secrets require one profile")
	}
	return strings.TrimSpace(actor.ReadScope.ProfileID), nil
}

func (s *daemonExtensionService) extensionSecretInstanceKey(
	ctx context.Context,
	name string,
	actor taskpkg.ActorContext,
) (extensionpkg.InstanceKey, error) {
	globalKey := extensionpkg.GlobalInstanceKey(name)
	if err := globalKey.Validate(); err != nil {
		return extensionpkg.InstanceKey{}, err
	}
	workspaceID, err := s.scopedDevelopmentWorkspaceID(ctx, actor)
	if err != nil {
		return extensionpkg.InstanceKey{}, err
	}
	if workspaceID == "" {
		return globalKey, nil
	}
	runtime, err := s.devRuntime()
	if err != nil {
		return extensionpkg.InstanceKey{}, err
	}
	workspaceKey := extensionpkg.InstanceKey{Name: globalKey.Name, WorkspaceID: workspaceID}
	ext, err := runtime.GetForInstance(workspaceKey)
	if err != nil {
		return extensionpkg.InstanceKey{}, err
	}
	if strings.TrimSpace(ext.Status.WorkspaceID) == workspaceID {
		return workspaceKey, nil
	}
	return globalKey, nil
}

func (s *daemonExtensionService) extensionSecretsForProfileInstance(
	ctx context.Context,
	key extensionpkg.InstanceKey,
	profileID string,
) (contract.ExtensionSecretsPayload, error) {
	ext, err := s.extensionSecretTarget(ctx, key)
	if err != nil {
		return contract.ExtensionSecretsPayload{}, err
	}
	bindings, err := s.envBindings.ResolveEnvBindings(ctx, key.Name, profileID, key.WorkspaceID)
	if err != nil {
		return contract.ExtensionSecretsPayload{}, fmt.Errorf("daemon: list extension secret bindings: %w", err)
	}
	bound := make([]string, 0, len(bindings))
	headerInputs := ext.Manifest.AgentPluginHeaderSecretInputs()
	declared := normalizedDeclaredEnv(ext, headerInputs)
	declaredSet := make(map[string]struct{}, len(declared))
	for _, envName := range declared {
		declaredSet[envName] = struct{}{}
	}
	items := make([]contract.ExtensionSecretBindingPayload, 0, len(bindings))
	for _, binding := range bindings {
		envName := strings.TrimSpace(binding.EnvName)
		if _, template := headerInputs[envName]; template && binding.MCPServer == "" &&
			(binding.ProfileID != profileID || binding.WorkspaceID != key.WorkspaceID) {
			continue
		}
		bound = append(bound, envName)
		_, current := declaredSet[envName]
		mcpServer := strings.TrimSpace(binding.MCPServer)
		headerName := strings.TrimSpace(binding.HeaderName)
		if mcpServer != "" {
			_, current = ext.Manifest.Resources.MCPServers[mcpServer]
		} else if inputID, template := headerInputs[envName]; template {
			current = binding.InputID == inputID
		}
		items = append(items, contract.ExtensionSecretBindingPayload{
			EnvName: envName, Stale: !current, MCPServer: mcpServer, HeaderName: headerName,
		})
	}
	slices.Sort(bound)
	slices.SortFunc(items, func(left, right contract.ExtensionSecretBindingPayload) int {
		return strings.Compare(left.EnvName, right.EnvName)
	})
	return contract.ExtensionSecretsPayload{
		DeclaredEnv:  declared,
		BoundEnvKeys: bound,
		Bindings:     items,
	}, nil
}

func (s *daemonExtensionService) setExtensionSecretsForProfileInstance(
	ctx context.Context,
	key extensionpkg.InstanceKey,
	profileID string,
	req contract.SetExtensionSecretsRequest,
) (contract.ExtensionSecretsPayload, error) {
	ext, err := s.extensionSecretTarget(ctx, key)
	if err != nil {
		return contract.ExtensionSecretsPayload{}, err
	}
	prepared, err := s.prepareExtensionSecrets(ctx, key, profileID, ext, req)
	if err != nil {
		return contract.ExtensionSecretsPayload{}, err
	}
	previous, err := s.envBindings.ListEnvBindings(ctx, key.Name, profileID, key.WorkspaceID)
	if err != nil {
		return contract.ExtensionSecretsPayload{}, fmt.Errorf("daemon: snapshot extension secret bindings: %w", err)
	}
	previousByName := extensionBindingsByName(previous)
	mutations := make([]extensionSecretMutation, 0, len(prepared))
	for _, write := range prepared {
		mutation, mutationErr := s.applyExtensionSecret(ctx, key, profileID, write, previousByName[write.envName])
		if mutationErr != nil {
			rollbackErr := s.rollbackExtensionSecretMutations(ctx, key, profileID, mutations)
			return contract.ExtensionSecretsPayload{}, errors.Join(mutationErr, rollbackErr)
		}
		mutations = append(mutations, mutation)
	}
	if err := s.gcSupersededExtensionSecrets(ctx, previous, prepared); err != nil {
		rollbackErr := s.rollbackExtensionSecretMutations(ctx, key, profileID, mutations)
		return contract.ExtensionSecretsPayload{}, errors.Join(err, rollbackErr)
	}
	return s.extensionSecretsForProfileInstance(ctx, key, profileID)
}

func (s *daemonExtensionService) deleteExtensionSecretForProfileInstance(
	ctx context.Context,
	key extensionpkg.InstanceKey,
	profileID string,
	envName string,
) error {
	ext, err := s.extensionSecretTarget(ctx, key)
	if err != nil {
		return err
	}
	envName = strings.TrimSpace(envName)
	if !vault.EnvNamePattern.MatchString(envName) {
		return &extensionpkg.EnvBindingValidationError{
			EnvName: envName,
			Cause:   extensionpkg.ErrExtensionEnvBindingInvalid,
		}
	}
	bindings, err := s.envBindings.ListEnvBindings(ctx, key.Name, profileID, key.WorkspaceID)
	if err != nil {
		return fmt.Errorf("daemon: list extension secret bindings: %w", err)
	}
	previous, ok := extensionBindingsByName(bindings)[envName]
	if !ok {
		return nil
	}
	// The binding's stored input link names the row it mirrors; only an unlinked legacy binding falls
	// back to the current manifest. An inactive binding mirrors no row.
	inputID := ""
	if previous.MCPServer == "" && !previous.Inactive {
		inputID = cmp.Or(previous.InputID, envBindingInputID(ext.Manifest, envName))
	}
	instance := extensioninput.Instance{Extension: key.Name, ProfileID: profileID, WorkspaceID: key.WorkspaceID}
	restoreRow, err := s.unsetInputRow(ctx, instance, inputID)
	if err != nil {
		return err
	}
	rollbackCtx, cancel := extensionSecretRollbackContext(ctx)
	defer cancel()
	if err := s.envBindings.DeleteEnvBinding(
		ctx,
		key.Name,
		profileID,
		key.WorkspaceID,
		envName,
	); err != nil {
		return errors.Join(
			fmt.Errorf("daemon: delete extension secret binding %q: %w", envName, err), restoreRow(rollbackCtx),
		)
	}
	if err := s.gcOwnedExtensionSecret(ctx, previous.SecretRef); err != nil {
		return errors.Join(err, s.envBindings.PutEnvBinding(rollbackCtx, previous), restoreRow(rollbackCtx))
	}
	return nil
}

// unsetInputRow deletes the typed row that mirrors an unbound input for MCP publication, so an explicit
// unset is neither read back as set nor carried forward into a new binding by a later write (rule 3a).
// It returns the restore for the deleted row.
func (s *daemonExtensionService) unsetInputRow(
	ctx context.Context, instance extensioninput.Instance, inputID string,
) (func(context.Context) error, error) {
	noop := func(context.Context) error { return nil }
	if s.inputs == nil || inputID == "" {
		return noop, nil
	}
	rows, err := s.inputs.List(ctx, instance)
	if err != nil {
		return nil, fmt.Errorf("daemon: list extension input rows: %w", err)
	}
	row, found := rows[inputID]
	if !found {
		return noop, nil
	}
	if err := s.inputs.Apply(ctx, instance, []extensioninput.Mutation{{InputID: inputID, Before: &row}}); err != nil {
		return nil, fmt.Errorf("daemon: unset extension input %q: %w", inputID, err)
	}
	return func(ctx context.Context) error {
		return s.inputs.Apply(ctx, instance, []extensioninput.Mutation{{InputID: inputID, After: &row}})
	}, nil
}

func (s *daemonExtensionService) extensionSecretTarget(
	ctx context.Context,
	key extensionpkg.InstanceKey,
) (*extensionpkg.Extension, error) {
	if err := s.checkReady(); err != nil {
		return nil, err
	}
	if ctx == nil {
		return nil, errors.New("daemon: extension secrets context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	key = key.Normalize()
	if err := key.Validate(); err != nil {
		return nil, err
	}
	if s.envBindings == nil || s.secretVault == nil {
		return nil, errors.New("daemon: extension secret storage is unavailable")
	}
	if key.IsGlobal() {
		return s.lookup(key.Name)
	}
	runtime, err := s.devRuntime()
	if err != nil {
		return nil, err
	}
	ext, err := runtime.GetForInstance(key)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(ext.Status.WorkspaceID) != key.WorkspaceID {
		return nil, extensionpkg.ErrExtensionNotDevLinked
	}
	populateExtensionManifest(s.logger, ext)
	return ext, nil
}

func normalizedDeclaredEnv(ext *extensionpkg.Extension, headerInputs map[string]string) []string {
	if ext == nil || ext.Manifest == nil {
		return nil
	}
	seen := make(map[string]struct{}, len(ext.Manifest.RequiresEnv))
	declared := make([]string, 0, len(ext.Manifest.RequiresEnv))
	for _, raw := range ext.Manifest.RequiresEnv {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		declared = append(declared, name)
	}
	for name := range headerInputs {
		if _, exists := seen[name]; !exists {
			declared = append(declared, name)
		}
	}
	slices.Sort(declared)
	return declared
}
