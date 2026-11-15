package daemon

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/resources"
	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type ConsentDecision = settingspkg.ConsentDecision
type MCPApproval = store.MCPApproval

type MCPApprovalStore interface {
	ListMCPServerApprovals(context.Context, string, string) ([]MCPApproval, error)
	PutMCPServerApproval(context.Context, MCPApproval) error
	DeleteMCPServerApproval(context.Context, string, string, string, string) error
}

type mcpApprovalCache struct {
	mu     sync.Mutex
	scopes map[mcpApprovalScope][]MCPApproval
}

type mcpApprovalScope struct{ profile, workspace string }

func (s *bootState) mcpApprovalRecords(ctx context.Context, profileID, workspaceID string) ([]MCPApproval, error) {
	s.mcpApprovals.mu.Lock()
	defer s.mcpApprovals.mu.Unlock()
	key := mcpApprovalScope{profileID, workspaceID}
	if records, ok := s.mcpApprovals.scopes[key]; ok {
		return records, nil
	}
	repository, ok := s.registry.(MCPApprovalStore)
	if !ok {
		return nil, nil
	}
	records, err := repository.ListMCPServerApprovals(ctx, profileID, workspaceID)
	if err != nil {
		return nil, err
	}
	if s.mcpApprovals.scopes == nil {
		s.mcpApprovals.scopes = make(map[mcpApprovalScope][]MCPApproval)
	}
	s.mcpApprovals.scopes[key] = records
	return records, nil
}

func (s *bootState) invalidateMCPApprovals(profileID, workspaceID string) {
	s.mcpApprovals.mu.Lock()
	delete(s.mcpApprovals.scopes, mcpApprovalScope{profileID, workspaceID})
	s.mcpApprovals.mu.Unlock()
	s.toolProjectionEpoch.Advance()
}

func mcpDefinitionConsent(
	ctx context.Context,
	state *bootState,
	scope toolspkg.Scope,
	record resources.Record[config.MCPServer],
	server config.MCPServer,
) (ConsentDecision, error) {
	digest, err := state.mcpDefinitionDigest(server)
	if err != nil {
		return ConsentDecision{}, err
	}
	decision := ConsentDecision{DefinitionDigest: digest}
	if record.Owner.Kind == extensionResourceOwnerKind || record.Scope.Kind == resources.ResourceScopeKindUser ||
		record.Scope.Kind == resources.ResourceScopeKindProfile {
		decision.Admitted = true
		return decision, nil
	}
	profileID := strings.TrimSpace(scope.ProfileID)
	if profileID == "" {
		profileID = store.DefaultProfileID
	}
	workspaceID, _, err := mcpResourceProjectionWorkspace(record.Scope)
	if err != nil {
		return decision, err
	}
	if scope.WorkspaceID != "" && scope.WorkspaceID != workspaceID {
		return decision, nil
	}
	externalDefault := state.cfg.Tools.Policy.ExternalDefault
	if state.workspaceResolver != nil && scope.WorkspaceID != "" {
		profileName := daemonDefaultProfileName
		if state.profiles != nil {
			profileName, err = state.profiles.ProfileName(ctx, profileID)
			if err != nil {
				return decision, err
			}
		}
		resolved, resolveErr := state.workspaceResolver.ResolveForProfile(ctx, scope.WorkspaceID, profileName)
		if resolveErr != nil {
			return decision, resolveErr
		}
		externalDefault = resolved.Config.Tools.Policy.ExternalDefault
	}
	if externalDefault == config.ToolsExternalDefaultEnabled {
		decision.Admitted = true
		return decision, nil
	}
	decision.Waiting = true
	decision.Reason = "new"
	records, err := state.mcpApprovalRecords(ctx, profileID, workspaceID)
	if err != nil {
		return decision, err
	}
	sourceID, err := mcpApprovalSourceID(ctx, state, workspaceID, server.DefinitionSource)
	if err != nil {
		return decision, err
	}
	return matchMCPApproval(decision, records, server.Name, sourceID, digest), nil
}

func matchMCPApproval(decision ConsentDecision, records []MCPApproval, name, sourceID, digest string) ConsentDecision {
	for _, approval := range records {
		if approval.ServerName != name || approval.SourceID != sourceID {
			continue
		}
		if approval.DefinitionDigest == digest {
			decision.Admitted, decision.Waiting, decision.Reason = true, false, ""
			return decision
		}
		decision.Reason = "changed"
	}
	return decision
}

func mcpResourceProjectionWorkspace(scope resources.ResourceScope) (string, string, error) {
	if scope.Kind == resources.ResourceScopeKindWorkspace {
		return scope.ID, "", nil
	}
	if scope.Kind == resources.ResourceScopeKindWorkspaceProfile {
		workspace, profile, ok := strings.Cut(scope.ID, "@pf:")
		if ok {
			return workspace, profile, nil
		}
	}
	return "", "", errors.New("daemon: MCP approval requires a workspace record")
}

func mcpApprovalSourceID(ctx context.Context, state *bootState, workspaceID, path string) (string, error) {
	if path == "" {
		return "", errors.New("daemon: MCP declaring file is unavailable")
	}
	if state.workspaceResolver == nil {
		return "", errors.New("daemon: MCP workspace resolver is unavailable")
	}
	workspace, err := state.workspaceResolver.Resolve(ctx, workspaceID)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(workspace.RootDir, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) ||
		filepath.IsAbs(relative) {
		return "", errors.New("daemon: MCP declaring file is outside the workspace")
	}
	return filepath.ToSlash(relative), nil
}

func mcpSourceConsent(
	ctx context.Context,
	state *bootState,
	scope toolspkg.Scope,
	source toolspkg.SourceRef,
) (ConsentDecision, error) {
	if source.ResourceID == "" {
		resolved, found, err := mcpSourceResource(ctx, state, source, "")
		if err != nil {
			return ConsentDecision{}, err
		}
		if found {
			source = resolved
		} else {
			resolved, err := resolveDaemonMCPServer(ctx, state, source, "")
			if err != nil {
				return ConsentDecision{}, err
			}
			return (settingsMCPConsent{state: state}).MCPConsent(
				ctx,
				settingsConsentRequest(resolved.Target, resolved.Server),
			)
		}
	}
	if state.mcpServerCatalog == nil {
		return ConsentDecision{}, errors.New("daemon: MCP catalog is unavailable")
	}
	for _, record := range state.mcpServerCatalog.Snapshot() {
		if record.ID != source.ResourceID {
			continue
		}
		server, err := projectProfileConfigMCPServer(ctx, state, record, scope.ProfileID, record.Spec)
		if err != nil {
			return ConsentDecision{}, err
		}
		return mcpDefinitionConsent(ctx, state, scope, record, server)
	}
	return ConsentDecision{}, nil
}

func requireMCPSourceConsent(ctx context.Context, state *bootState, source toolspkg.SourceRef) error {
	decision, err := mcpSourceConsent(
		ctx,
		state,
		toolspkg.Scope{ProfileID: source.ProfileID, WorkspaceID: source.WorkspaceID},
		source,
	)
	if err != nil {
		return err
	}
	if !decision.Admitted {
		name := firstNonEmpty(source.RawServerName, source.Owner)
		message := fmt.Sprintf("%s was removed from CompozyOS", name)
		if decision.DefinitionDigest != "" {
			workspace := source.WorkspaceID
			if state.workspaceResolver != nil {
				if resolved, resolveErr := state.workspaceResolver.Resolve(ctx, workspace); resolveErr == nil {
					workspace = resolved.Name
				}
			}
			message = fmt.Sprintf(
				"%s is waiting for approval in %s. Allow it in Settings → MCP servers.",
				name,
				workspace,
			)
		}
		return toolspkg.NewToolError(
			"source_disabled",
			"",
			message,
			toolspkg.ErrToolDenied,
			toolspkg.ReasonSourceDisabled,
		)
	}
	return nil
}

func (s settingsMCPConsent) RecordWorkspaceMCPWrite(
	ctx context.Context,
	w settingspkg.WorkspaceMCPWrite,
) (string, error) {
	if w.Write == nil {
		return "", errors.New("daemon: MCP write is required")
	}
	if err := w.Write(); err != nil {
		return "", err
	}
	server := w.Server
	if w.ReadCommitted != nil {
		committed, err := w.ReadCommitted()
		if err != nil {
			return "MCP server saved but approval failed; waiting for approval", nil
		}
		server = committed
	}
	server.DefinitionSource = w.SourceID
	digest, err := s.state.mcpDefinitionDigest(server)
	if err == nil {
		_, err = s.ApproveMCP(
			ctx,
			settingspkg.MCPConsentRequest{
				ProfileName: w.ProfileName,
				WorkspaceID: w.WorkspaceID,
				Scope:       resourceScopeWorkspaceKind,
				Server:      server,
			},
			digest,
			false,
		)
	}
	if err != nil {
		return fmt.Sprintf("%s saved but approval failed; waiting for approval", server.Name), nil
	}
	return "", nil
}

func approvalTimestamp() time.Time { return time.Now().UTC() }

func (s *bootState) mcpDefinitionDigest(server config.MCPServer) (string, error) {
	if s == nil || s.mcpDigestKeys == nil {
		return "", errors.New("daemon: MCP digest key provider is required")
	}
	key, err := s.mcpDigestKeys.Key()
	if err != nil {
		return "", err
	}
	return config.MCPDefinitionDigest(server, key)
}
