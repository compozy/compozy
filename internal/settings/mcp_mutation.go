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
	profileName string,
	name string,
	selector TargetSelector,
	server compozyconfig.MCPServer,
	secrets MCPSecretValues,
	preservation MCPSecretPreservation,
	envPreservation []string,
) (MutationResult, error) {
	root, sources, err := s.resolveMCPTargetContext(ctx, scope, workspaceID, profileName)
	if err != nil {
		return MutationResult{}, err
	}
	target, err := s.resolveMCPPutTarget(scope, root, profileName, name, selector, sources)
	if err != nil {
		return MutationResult{}, err
	}

	normalized, err := s.normalizeAndValidateMCPServerWrite(
		ctx, scope, workspaceID, profileName, name, target.Kind(), sources, server, preservation, envPreservation,
	)
	if err != nil {
		return MutationResult{}, err
	}
	secretWrites, err := s.prepareMCPSecretWrites(
		scope, mcpScopeIdentifier(scope, workspaceID, profileName), name, &normalized, secrets,
	)
	if err != nil {
		return MutationResult{}, err
	}
	secretCleanup, err := s.prepareMCPSecretCleanupPlan(
		ctx, scope, mcpScopeIdentifier(scope, workspaceID, profileName), name, target.Kind(), sources, normalized,
	)
	if err != nil {
		return MutationResult{}, err
	}
	secretMutations, err := s.storeMCPSecrets(ctx, secretWrites)
	if err != nil {
		return MutationResult{}, err
	}
	mutationOutcome, err := s.commitMCPServerDefinition(
		ctx, scope, workspaceID, profileName, root, name, target, sources, normalized,
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
		profileName,
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
	result.ProfileName = profileName
	result.writePath = target.Path()
	result.Warnings = append(result.Warnings, cleanupWarnings...)
	item := committedMCPServerItem(normalized, scope, workspaceID, profileName, target.Kind(), sources)
	result.MCPServer = &item
	return result, nil
}

func (s *service) commitMCPServerDefinition(
	ctx context.Context,
	scope ScopeKind,
	workspaceID string,
	profileName string,
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
		profileName,
		name,
		func() error { return s.writeMCPServer(root, name, target, server) },
		rollback,
	)
}

func (s *service) normalizeAndValidateMCPServerWrite(
	ctx context.Context,
	scope ScopeKind,
	workspaceID string,
	profileName string,
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
	if err := s.validateMCPServerWrite(ctx, scope, workspaceID, profileName, name, target, sources, server); err != nil {
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
	if target.Kind() == WriteTargetGlobalMCPSidecar || target.Kind() == WriteTargetProfileMCPSidecar ||
		target.Kind() == WriteTargetWorkspaceMCPSidecar {
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
	profileName string,
	name string,
	target WriteTargetKind,
	sources map[string][]mcpSourceEntry,
	server compozyconfig.MCPServer,
) error {
	cfg, _, err := s.loadConfig(ctx, scope, workspaceID, profileName)
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
		(target == WriteTargetProfileConfig && effective.Target == WriteTargetProfileMCPSidecar) ||
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
	profileName string,
	name string,
	selector TargetSelector,
) (MutationResult, error) {
	root, sources, err := s.resolveMCPTargetContext(ctx, scope, workspaceID, profileName)
	if err != nil {
		return MutationResult{}, err
	}
	target, err := s.resolveMCPDeleteTarget(scope, root, profileName, name, selector, sources)
	if err != nil {
		return MutationResult{}, err
	}
	deletedSource, sourceFound := mcpSourceForTarget(name, target.Kind(), sources)
	var ownedSecrets []ownedMCPSecretSnapshot
	if sourceFound {
		ownedSecrets, err = s.prepareOwnedMCPSecretDeletes(
			ctx, scope, mcpScopeIdentifier(scope, workspaceID, profileName), deletedSource.Server,
		)
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
	mutationOutcome, err := s.withMCPAuthDefinitionMutation(
		ctx,
		scope,
		workspaceID,
		profileName,
		name,
		func() error { return s.deleteMCPServerDefinition(root, name, target) },
		rollbackDefinition,
	)
	if mutationOutcome == mcpDefinitionMutationCommitted &&
		scope == ScopeWorkspace && s.mcpDefinitionRetirer != nil {
		s.mcpDefinitionRetirer.ForgetMCPServer(workspaceID, name)
	}
	if err != nil {
		return MutationResult{}, err
	}

	return s.finishMCPServerDeletion(
		ctx,
		scope,
		workspaceID,
		profileName,
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
	profileName string,
	name string,
	root string,
	target compozyconfig.WriteTarget,
	deletedServer compozyconfig.MCPServer,
	ownedSecrets []ownedMCPSecretSnapshot,
) (MutationResult, error) {
	result := mutationResultForCollection(CollectionMCPServers, scope, workspaceID, target.Kind())
	result.ProfileName = profileName
	result.writePath = target.Path()
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
	invalidateErr := s.invalidateMCPAuthAfterDefinitionRestore(scope, workspaceID, profileName, name)
	return MutationResult{}, errors.Join(cleanupFailure, invalidateErr)
}

func (s *service) deleteMCPServerDefinition(root string, name string, target compozyconfig.WriteTarget) error {
	if target.Kind() == WriteTargetGlobalMCPSidecar || target.Kind() == WriteTargetProfileMCPSidecar ||
		target.Kind() == WriteTargetWorkspaceMCPSidecar {
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
