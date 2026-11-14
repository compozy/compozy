package settings

import (
	"context"
	"errors"
	"fmt"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func (s *service) deleteMCPServer(
	ctx context.Context,
	scope ScopeKind,
	workspaceID string,
	profileName string,
	name string,
	selector TargetSelector,
) (MutationResult, error) {
	s.mcpMutationMu.Lock()
	defer s.mcpMutationMu.Unlock()
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
	result = mutationResultAtPath(result, target.Path())
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
	if isMCPDefinitionSidecarTarget(target.Kind()) {
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
