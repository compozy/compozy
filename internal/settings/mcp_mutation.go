package settings

import (
	"context"
	"errors"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func (s *service) putMCPServer(
	ctx context.Context,
	scope ScopeKind,
	workspaceID string,
	name string,
	selector TargetSelector,
	server compozyconfig.MCPServer,
	secrets MCPSecretValues,
	preservation MCPSecretPreservation,
	envPreservation []string,
) (MutationResult, error) {
	root, sources, err := s.resolveMCPTargetContext(ctx, scope, workspaceID)
	if err != nil {
		return MutationResult{}, err
	}
	target, err := s.resolveMCPPutTarget(scope, root, name, selector, sources)
	if err != nil {
		return MutationResult{}, err
	}

	normalized, err := s.normalizeAndValidateMCPServerWrite(
		ctx, scope, workspaceID, name, target.Kind(), sources, server, preservation, envPreservation,
	)
	if err != nil {
		return MutationResult{}, err
	}
	secretWrites, err := s.prepareMCPSecretWrites(scope, workspaceID, name, &normalized, secrets)
	if err != nil {
		return MutationResult{}, err
	}
	secretCleanup, err := s.prepareMCPSecretCleanupPlan(
		ctx, scope, workspaceID, name, target.Kind(), sources, normalized,
	)
	if err != nil {
		return MutationResult{}, err
	}
	secretMutations, err := s.storeMCPSecrets(ctx, secretWrites)
	if err != nil {
		return MutationResult{}, err
	}
	mutationOutcome, err := s.commitMCPServerDefinition(
		ctx, scope, workspaceID, root, name, target, sources, normalized,
	)
	if err != nil {
		var rollbackErr error
		if mutationOutcome != mcpDefinitionMutationCommitted {
			rollbackErr = s.rollbackMCPSecretMutations(ctx, secretMutations)
		}
		return MutationResult{}, errors.Join(err, rollbackErr)
	}
	cleanupWarnings, err := s.executeMCPSecretCleanupPlan(
		ctx,
		scope,
		workspaceID,
		root,
		name,
		target,
		secretCleanup,
		secretMutations,
	)
	if err != nil {
		return MutationResult{}, err
	}

	result := mutationResultForCollection(CollectionMCPServers, scope, workspaceID, target.Kind())
	result.Warnings = append(result.Warnings, cleanupWarnings...)
	item := committedMCPServerItem(normalized, scope, workspaceID, target.Kind(), sources)
	result.MCPServer = &item
	return result, nil
}

func (s *service) commitMCPServerDefinition(
	ctx context.Context,
	scope ScopeKind,
	workspaceID string,
	root string,
	name string,
	target compozyconfig.WriteTarget,
	sources map[string][]mcpSourceEntry,
	server compozyconfig.MCPServer,
) (mcpDefinitionMutationOutcome, error) {
	previousSource, previousFound := mcpSourceForTarget(name, target.Kind(), sources)
	rollback := func() error {
		if previousFound {
			return s.writeMCPServer(root, name, target, previousSource.Server)
		}
		return s.deleteMCPServerDefinition(root, name, target)
	}
	return s.withMCPAuthDefinitionMutation(
		ctx,
		scope,
		workspaceID,
		name,
		func() error { return s.writeMCPServer(root, name, target, server) },
		rollback,
	)
}

func (s *service) normalizeAndValidateMCPServerWrite(
	ctx context.Context,
	scope ScopeKind,
	workspaceID string,
	name string,
	target WriteTargetKind,
	sources map[string][]mcpSourceEntry,
	server compozyconfig.MCPServer,
	preservation MCPSecretPreservation,
	envPreservation []string,
) (compozyconfig.MCPServer, error) {
	server.Name = strings.TrimSpace(server.Name)
	if server.Name == "" {
		server.Name = name
	}
	if server.Name != name {
		return compozyconfig.MCPServer{}, validationError(fmt.Errorf(
			"settings: MCP server payload name %q does not match request name %q",
			server.Name,
			name,
		))
	}
	if err := preserveMCPEnvValues(name, target, sources, &server, envPreservation); err != nil {
		return compozyconfig.MCPServer{}, err
	}
	if err := preserveMCPSecretBindings(name, target, sources, &server, preservation); err != nil {
		return compozyconfig.MCPServer{}, err
	}
	if err := s.validateMCPServerWrite(ctx, scope, workspaceID, name, target, sources, server); err != nil {
		return compozyconfig.MCPServer{}, fmt.Errorf("settings: write MCP server %q: %w", name, err)
	}
	return server, nil
}

func (s *service) writeMCPServer(
	root string,
	name string,
	target compozyconfig.WriteTarget,
	server compozyconfig.MCPServer,
) error {
	if err := s.mcpDefinitionWriter(s.homePaths, root, name, target, server); err != nil {
		return fmt.Errorf("settings: write MCP server %q: %w", name, err)
	}
	return nil
}

func writeMCPDefinition(
	homePaths compozyconfig.HomePaths,
	root string,
	name string,
	target compozyconfig.WriteTarget,
	server compozyconfig.MCPServer,
) error {
	if target.Kind() == WriteTargetGlobalMCPSidecar || target.Kind() == WriteTargetWorkspaceMCPSidecar {
		if _, err := compozyconfig.PutMCPSidecarServer(homePaths, root, target, server); err != nil {
			return err
		}
		return nil
	}
	if _, err := compozyconfig.EditConfigOverlay(
		homePaths,
		root,
		target,
		func(editor *compozyconfig.OverlayEditor) error {
			return editor.UpsertArrayTableItem([]string{"mcp_servers"}, "name", name, mcpServerMap(server))
		},
	); err != nil {
		return err
	}
	return nil
}

func (s *service) validateMCPServerWrite(
	ctx context.Context,
	scope ScopeKind,
	workspaceID string,
	name string,
	target WriteTargetKind,
	sources map[string][]mcpSourceEntry,
	server compozyconfig.MCPServer,
) error {
	cfg, _, err := s.loadConfig(ctx, scope, workspaceID)
	if err != nil {
		return err
	}
	if projected, ok := projectedMCPServerForValidation(name, target, sources, server); ok {
		cfg.MCPServers = upsertMCPServer(cfg.MCPServers, projected)
	}
	return cfg.Validate()
}

func projectedMCPServerForValidation(
	name string,
	target WriteTargetKind,
	sources map[string][]mcpSourceEntry,
	server compozyconfig.MCPServer,
) (compozyconfig.MCPServer, bool) {
	entries := sources[strings.TrimSpace(name)]
	if len(entries) == 0 {
		return server, true
	}
	effective := entries[len(entries)-1]
	if (target == WriteTargetGlobalConfig && effective.Target == WriteTargetGlobalMCPSidecar) ||
		(target == WriteTargetWorkspaceConfig && effective.Target == WriteTargetWorkspaceMCPSidecar) {
		return compozyconfig.MCPServer{}, false
	}
	return server, true
}

func upsertMCPServer(servers []compozyconfig.MCPServer, server compozyconfig.MCPServer) []compozyconfig.MCPServer {
	name := strings.TrimSpace(server.Name)
	for idx := range servers {
		if strings.TrimSpace(servers[idx].Name) != name {
			continue
		}
		servers[idx] = server
		return servers
	}
	return append(servers, server)
}

func (s *service) deleteMCPServer(
	ctx context.Context,
	scope ScopeKind,
	workspaceID string,
	name string,
	selector TargetSelector,
) (MutationResult, error) {
	root, sources, err := s.resolveMCPTargetContext(ctx, scope, workspaceID)
	if err != nil {
		return MutationResult{}, err
	}
	target, err := s.resolveMCPDeleteTarget(scope, root, name, selector, sources)
	if err != nil {
		return MutationResult{}, err
	}
	deletedSource, sourceFound := mcpSourceForTarget(name, target.Kind(), sources)
	var ownedSecrets []ownedMCPSecretSnapshot
	if sourceFound {
		ownedSecrets, err = s.prepareOwnedMCPSecretDeletes(ctx, scope, workspaceID, deletedSource.Server)
		if err != nil {
			return MutationResult{}, fmt.Errorf("settings: prepare MCP server %q secret cleanup: %w", name, err)
		}
		ownedSecrets = excludeMCPSecretsReferencedByOtherSources(
			ownedSecrets,
			name,
			target.Kind(),
			sources,
		)
	}

	rollbackDefinition := func() error {
		if !sourceFound {
			return fmt.Errorf("settings: MCP server %q rollback source is unavailable", name)
		}
		return s.writeMCPServer(root, name, target, deletedSource.Server)
	}
	_, err = s.withMCPAuthDefinitionMutation(
		ctx,
		scope,
		workspaceID,
		name,
		func() error { return s.deleteMCPServerDefinition(root, name, target) },
		rollbackDefinition,
	)
	if err != nil {
		return MutationResult{}, err
	}

	return s.finishMCPServerDeletion(
		ctx,
		scope,
		workspaceID,
		name,
		root,
		target,
		deletedSource.Server,
		ownedSecrets,
	)
}

func (s *service) finishMCPServerDeletion(
	ctx context.Context,
	scope ScopeKind,
	workspaceID string,
	name string,
	root string,
	target compozyconfig.WriteTarget,
	deletedServer compozyconfig.MCPServer,
	ownedSecrets []ownedMCPSecretSnapshot,
) (MutationResult, error) {
	result := mutationResultForCollection(CollectionMCPServers, scope, workspaceID, target.Kind())
	if len(ownedSecrets) == 0 {
		return result, nil
	}
	cleanupCtx, cancel := mcpSecretRollbackContext(ctx)
	cleanupOutcome, cleanupErr := s.deleteOwnedMCPSecrets(cleanupCtx, ownedSecrets)
	cancel()
	if cleanupErr == nil {
		return result, nil
	}

	cleanupFailure := fmt.Errorf("settings: garbage-collect MCP server %q secrets: %w", name, cleanupErr)
	if !cleanupOutcome.restorationComplete {
		result.Warnings = []string{
			"committed MCP deletion retained after partial secret cleanup rollback: " + cleanupFailure.Error(),
		}
		return result, nil
	}
	restoreErr := s.writeMCPServer(root, name, target, deletedServer)
	if restoreErr != nil {
		restoreErr = fmt.Errorf(
			"settings: restore MCP server %q after secret cleanup failure: %w",
			name,
			restoreErr,
		)
		result.Warnings = []string{
			"committed MCP deletion retained because its prior definition could not be restored: " +
				errors.Join(cleanupFailure, restoreErr).Error(),
		}
		return result, nil
	}
	invalidateErr := s.invalidateMCPAuthAfterDefinitionRestore(scope, workspaceID, name)
	return MutationResult{}, errors.Join(cleanupFailure, invalidateErr)
}

func (s *service) deleteMCPServerDefinition(root string, name string, target compozyconfig.WriteTarget) error {
	if target.Kind() == WriteTargetGlobalMCPSidecar || target.Kind() == WriteTargetWorkspaceMCPSidecar {
		_, deleted, err := compozyconfig.DeleteMCPSidecarServer(s.homePaths, root, target, name)
		if err != nil {
			return fmt.Errorf("settings: delete MCP server %q: %w", name, err)
		}
		if !deleted {
			return notFoundError(fmt.Errorf("settings: MCP server %q not found in %q", name, target.Kind()))
		}
		return nil
	}
	if _, err := compozyconfig.EditConfigOverlay(
		s.homePaths,
		root,
		target,
		func(editor *compozyconfig.OverlayEditor) error {
			deleted, deleteErr := editor.DeleteArrayTableItem([]string{"mcp_servers"}, "name", name)
			if deleteErr != nil {
				return deleteErr
			}
			if !deleted {
				return notFoundError(
					fmt.Errorf("settings: MCP server %q not found in %q", name, target.Kind()),
				)
			}
			return nil
		},
	); err != nil {
		return fmt.Errorf("settings: delete MCP server %q: %w", name, err)
	}
	return nil
}

type mcpDefinitionMutationOutcome uint8

const (
	mcpDefinitionMutationUnchanged mcpDefinitionMutationOutcome = iota
	mcpDefinitionMutationRestored
	mcpDefinitionMutationCommitted
)

func (s *service) withMCPAuthDefinitionMutation(
	ctx context.Context,
	scope ScopeKind,
	workspaceID string,
	name string,
	mutate func() error,
	rollback func() error,
) (mcpDefinitionMutationOutcome, error) {
	if mutate == nil {
		return mcpDefinitionMutationUnchanged, errors.New("settings: MCP definition mutation is required")
	}
	if rollback == nil {
		return mcpDefinitionMutationUnchanged, errors.New("settings: MCP definition rollback is required")
	}
	if s.mcpAuth == nil {
		if err := mutate(); err != nil {
			return mcpDefinitionMutationUnchanged, err
		}
		return mcpDefinitionMutationCommitted, nil
	}
	target, err := normalizeMCPAuthTarget(MCPAuthTargetRequest{
		Scope: scope, WorkspaceID: workspaceID, Name: name,
	})
	if err != nil {
		return mcpDefinitionMutationUnchanged, err
	}
	if err := s.mcpAuth.MCPAuthInvalidate(target); err != nil {
		return mcpDefinitionMutationUnchanged, fmt.Errorf(
			"settings: invalidate pending MCP auth before definition mutation: %w",
			err,
		)
	}
	mutationErr := mutate()
	if mutationErr != nil {
		return mcpDefinitionMutationUnchanged, mutationErr
	}
	deleteStateErr := s.mcpAuth.MCPAuthDeleteState(ctx, target)
	if deleteStateErr != nil {
		deleteStateErr = fmt.Errorf("settings: delete MCP auth state after definition mutation: %w", deleteStateErr)
	}
	if deleteStateErr == nil {
		return mcpDefinitionMutationCommitted, nil
	}

	rollbackErr := rollback()
	if rollbackErr != nil {
		rollbackErr = fmt.Errorf("settings: roll back MCP definition mutation: %w", rollbackErr)
		return mcpDefinitionMutationCommitted, errors.Join(deleteStateErr, rollbackErr)
	}
	rollbackInvalidateErr := s.mcpAuth.MCPAuthInvalidate(target)
	if rollbackInvalidateErr != nil {
		rollbackInvalidateErr = fmt.Errorf(
			"settings: invalidate pending MCP auth after definition rollback: %w",
			rollbackInvalidateErr,
		)
	}
	return mcpDefinitionMutationRestored, errors.Join(deleteStateErr, rollbackInvalidateErr)
}

func (s *service) invalidateMCPAuthAfterDefinitionRestore(
	scope ScopeKind,
	workspaceID string,
	name string,
) error {
	if s.mcpAuth == nil {
		return nil
	}
	target, err := normalizeMCPAuthTarget(MCPAuthTargetRequest{
		Scope: scope, WorkspaceID: workspaceID, Name: name,
	})
	if err != nil {
		return err
	}
	if err := s.mcpAuth.MCPAuthInvalidate(target); err != nil {
		return fmt.Errorf("settings: invalidate pending MCP auth after definition restore: %w", err)
	}
	return nil
}
