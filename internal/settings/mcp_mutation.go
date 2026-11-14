package settings

import (
	"context"
	"errors"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/vault"
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
	importing bool,
) (MutationResult, error) {
	s.mcpMutationMu.Lock()
	defer s.mcpMutationMu.Unlock()
	root, sources, err := s.resolveMCPTargetContext(ctx, scope, workspaceID, profileName)
	if err != nil {
		return MutationResult{}, err
	}
	if importing {
		if err := s.validateMCPImportName(ctx, scope, workspaceID, profileName, name, sources); err != nil {
			return MutationResult{}, err
		}
	}
	target, err := s.resolveMCPPutTarget(scope, root, profileName, name, selector, sources)
	if err != nil {
		return MutationResult{}, err
	}

	if importing && len(secrets.SecretHeaders) > 0 && isMCPDefinitionSidecarTarget(target.Kind()) {
		target, err = compozyconfig.ResolveConfigWriteTarget(s.homePaths, root, scope.configWriteScope(), profileName)
		if err != nil {
			return MutationResult{}, err
		}
	}
	normalized, err := s.normalizeAndValidateMCPServerWrite(
		ctx, scope, workspaceID, profileName, name, target.Kind(), sources, server, preservation, envPreservation,
	)
	if err != nil {
		return MutationResult{}, err
	}
	scopeID := mcpScopeIdentifier(scope, workspaceID, profileName)
	secretWrites, err := s.prepareMCPSecretWrites(
		scope, scopeID, name, &normalized, secrets,
	)
	if err != nil {
		return MutationResult{}, err
	}
	if err := normalized.Validate("mcp_servers"); err != nil {
		return MutationResult{}, validationError(err)
	}
	if err := normalizeMCPClientSecretOwner(&normalized, scope, scopeID, name); err != nil {
		return MutationResult{}, err
	}
	secretCleanup, err := s.prepareMCPSecretCleanupPlan(
		ctx, scope, scopeID, name, target.Kind(), sources, normalized,
	)
	if err != nil {
		return MutationResult{}, err
	}
	secretMutations, err := s.storeMCPSecrets(ctx, secretWrites)
	if err != nil {
		rollbackErr := s.rollbackUnreferencedMCPSecrets(ctx, scope, workspaceID, profileName, secretMutations)
		return MutationResult{}, errors.Join(err, rollbackErr)
	}
	mutationOutcome := mcpDefinitionMutationUnchanged
	var approvalWarning string
	commit := func() error {
		var commitErr error
		mutationOutcome, commitErr = s.commitMCPServerDefinition(
			ctx,
			scope,
			workspaceID,
			profileName,
			root,
			name,
			target,
			sources,
			normalized,
		)
		return commitErr
	}
	if workspaceID != "" && s.workspaceMCPApprovals != nil {
		approvalWarning, err = s.workspaceMCPApprovals.RecordWorkspaceMCPWrite(ctx, WorkspaceMCPWrite{
			ProfileName: profileName,
			WorkspaceID: workspaceID,
			SourceID:    target.Path(),
			Server:      normalized,
			Write:       commit,
			ReadCommitted: func() (compozyconfig.MCPServer, error) {
				_, current, readErr := s.resolveMCPTargetContext(ctx, scope, workspaceID, profileName)
				if readErr != nil {
					return compozyconfig.MCPServer{}, readErr
				}
				entry, ok := mcpSourceForTarget(name, target.Kind(), current)
				if !ok {
					return compozyconfig.MCPServer{}, fmt.Errorf("settings: committed MCP definition is unavailable")
				}
				return entry.Server, nil
			},
		})
	} else {
		err = commit()
		approvalWarning = unavailableMCPWriteApprovalWarning(workspaceID, err)
	}
	return s.finishMCPServerWrite(
		ctx, MCPAuthTargetRequest{Scope: scope, WorkspaceID: workspaceID, ProfileName: profileName, Name: name},
		root, target, sources, normalized, &secretCleanup, secretMutations,
		importing, mutationOutcome, approvalWarning, err,
	)
}

func normalizeMCPClientSecretOwner(server *compozyconfig.MCPServer, scope ScopeKind, scopeID, name string) error {
	if strings.TrimSpace(server.Auth.ClientSecretRef) == "" {
		return nil
	}
	ref, err := vault.NormalizeMCPClientSecretRef(server.Auth.ClientSecretRef, vault.MCPSecretTarget{
		Scope: string(scope), WorkspaceID: scopeID, ServerName: name,
	})
	if err != nil {
		return validationError(fmt.Errorf("settings: OAuth client secret reference: %w", err))
	}
	server.Auth.ClientSecretRef = ref
	return nil
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
	if err := s.validateManualMCPName(
		ctx,
		MCPAuthTargetRequest{Scope: scope, WorkspaceID: workspaceID, ProfileName: profileName, Name: name},
	); err != nil {
		return compozyconfig.MCPServer{}, err
	}
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
	if isMCPDefinitionSidecarTarget(target) && (len(server.Headers) > 0 || len(server.SecretHeaders) > 0) {
		return compozyconfig.MCPServer{}, validationError(errors.New("settings: MCP headers require a config target"))
	}
	if err := s.validateMCPServerWrite(
		ctx,
		scope,
		workspaceID,
		profileName,
		name,
		target,
		sources,
		server,
	); err != nil {
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
	if isMCPDefinitionSidecarTarget(target.Kind()) {
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

func (s *service) finishMCPServerWrite(
	ctx context.Context,
	req MCPAuthTargetRequest,
	root string,
	target compozyconfig.WriteTarget,
	sources map[string][]mcpSourceEntry,
	normalized compozyconfig.MCPServer,
	secretCleanup *mcpSecretCleanupPlan,
	secretMutations []mcpSecretMutation,
	importing bool,
	mutationOutcome mcpDefinitionMutationOutcome,
	approvalWarning string,
	err error,
) (MutationResult, error) {
	scope, workspaceID, profileName, name := req.Scope, req.WorkspaceID, req.ProfileName, req.Name
	var importWarning string
	if err != nil {
		if !importing || mutationOutcome != mcpDefinitionMutationCommitted {
			var rollbackErr error
			if mutationOutcome != mcpDefinitionMutationCommitted {
				rollbackErr = s.rollbackUnreferencedMCPSecrets(ctx, scope, workspaceID, profileName, secretMutations)
			}
			return MutationResult{}, errors.Join(err, rollbackErr)
		}
		importWarning = mcpImportCleanupWarning(name)
	}
	var cleanupWarnings []string
	if importWarning == "" {
		cleanupWarnings, err = s.executeMCPSecretCleanupPlan(
			ctx,
			scope,
			workspaceID,
			profileName,
			root,
			name,
			target,
			*secretCleanup,
			secretMutations,
			importing,
		)
		if err != nil {
			return MutationResult{}, err
		}
	}

	result := mutationResultForCollection(CollectionMCPServers, scope, workspaceID, target.Kind())
	result.ProfileName = profileName
	result = mutationResultAtPath(result, target.Path())
	result.Warnings = append(result.Warnings, cleanupWarnings...)
	if importWarning != "" {
		result.Warnings = append(result.Warnings, importWarning)
	}
	if approvalWarning != "" {
		result.Warnings = append(result.Warnings, approvalWarning)
	}
	normalized.DefinitionSource = target.Path()
	result.MCPServer = new(committedMCPServerItem(normalized, scope, workspaceID, profileName, target.Kind(), sources))
	if s.mcpConsent != nil {
		_, committed, readErr := s.resolveMCPTargetContext(ctx, scope, workspaceID, profileName)
		if readErr == nil {
			if entry, ok := mcpSourceForTarget(name, target.Kind(), committed); ok {
				readErr = s.populateMCPServerStatuses(ctx, result.MCPServer, entry, true)
			}
		}
		if readErr != nil {
			result.Warnings = append(result.Warnings, "MCP server saved but consent status could not be loaded")
		}
	}
	return result, nil
}

func unavailableMCPWriteApprovalWarning(workspaceID string, err error) string {
	if err == nil && workspaceID != "" {
		return "MCP server saved but approval is unavailable; waiting for approval"
	}
	return ""
}
