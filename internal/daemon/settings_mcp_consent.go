package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/config"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	"github.com/compozy/compozy/internal/resources"
	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type settingsMCPConsent struct{ state *bootState }

var _ settingspkg.MCPConsentProvider = settingsMCPConsent{}
var _ settingspkg.WorkspaceMCPApprovalRecorder = settingsMCPConsent{}

func (s settingsMCPConsent) profileID(ctx context.Context, name string) (string, error) {
	if name == "" || name == daemonDefaultProfileName {
		return store.DefaultProfileID, nil
	}
	if s.state.profiles == nil {
		return "", errors.New("daemon: MCP profile catalog is unavailable")
	}
	return s.state.profiles.AvailableProfileID(ctx, name)
}

func (s settingsMCPConsent) MCPConsent(
	ctx context.Context,
	req settingspkg.MCPConsentRequest,
) (ConsentDecision, error) {
	if s.state == nil {
		return ConsentDecision{}, errors.New("daemon: MCP consent is unavailable")
	}
	profileID, err := s.profileID(ctx, req.ProfileName)
	if err != nil {
		return ConsentDecision{}, err
	}
	scope := resources.ResourceScope{Kind: resources.ResourceScopeKind(req.Scope), ID: req.WorkspaceID}
	if req.Scope == "profile" {
		scope.ID = profileID
	}
	if req.Scope == "workspace_profile" {
		scope.Kind = resources.ResourceScopeKindWorkspaceProfile
		scope.ID = req.WorkspaceID + "@pf:" + req.ProfileName
	}
	owner := resources.ResourceOwner{}
	if strings.HasPrefix(req.Owner, "extension:") {
		owner = *extensionOwner(strings.TrimPrefix(req.Owner, "extension:"))
	}
	return mcpDefinitionConsent(
		ctx,
		s.state,
		toolspkg.Scope{ProfileID: profileID, WorkspaceID: req.WorkspaceID},
		resources.Record[config.MCPServer]{Scope: scope, Owner: owner, Spec: req.Server},
		req.Server,
	)
}

func (s settingsMCPConsent) ApproveMCP(
	ctx context.Context,
	req settingspkg.MCPConsentRequest,
	digest string,
	revoke bool,
) (MCPApproval, error) {
	if req.WorkspaceID == "" || (req.Scope == "user" || req.Scope == "profile") ||
		strings.HasPrefix(req.Owner, "extension:") {
		return MCPApproval{}, &settingspkg.MCPApprovalError{
			Code:    "mcp_approval_not_required",
			Message: fmt.Sprintf("%s does not need approval: it was added through CompozyOS", req.Server.Name),
		}
	}
	current, err := config.MCPDefinitionDigest(req.Server)
	if err != nil {
		return MCPApproval{}, err
	}
	if !revoke && (digest == "" || current != digest) {
		return MCPApproval{}, &settingspkg.MCPApprovalError{
			Code: "mcp_approval_definition_changed",
			Message: fmt.Sprintf(
				"%s changed since you looked at it. List it again and approve the new definition",
				req.Server.Name,
			),
		}
	}
	profileID, err := s.profileID(ctx, req.ProfileName)
	if err != nil {
		return MCPApproval{}, err
	}
	sourceID, err := mcpApprovalSourceID(ctx, s.state, req.WorkspaceID, req.Server.DefinitionSource)
	if err != nil {
		return MCPApproval{}, err
	}
	repository, ok := s.state.registry.(MCPApprovalStore)
	if !ok {
		return MCPApproval{}, errors.New("daemon: MCP approval store is unavailable")
	}
	approval := MCPApproval{
		ProfileID:        profileID,
		WorkspaceID:      req.WorkspaceID,
		ServerName:       req.Server.Name,
		SourceID:         sourceID,
		DefinitionDigest: current,
		ApprovedAt:       approvalTimestamp(),
	}
	approval.ID, err = store.NewID("mcpa")
	if err != nil {
		return MCPApproval{}, err
	}
	if revoke {
		err = repository.DeleteMCPServerApproval(ctx, profileID, req.WorkspaceID, req.Server.Name, sourceID)
	} else {
		err = repository.PutMCPServerApproval(ctx, approval)
	}
	if err != nil {
		return MCPApproval{}, err
	}
	s.state.invalidateMCPApprovals(profileID, req.WorkspaceID)
	return approval, nil
}

func (s *settingsRuntimeSurface) requireMCPConsent(
	ctx context.Context,
	target mcpauth.Target,
	server config.MCPServer,
) error {
	if s.state == nil {
		return nil
	}
	req := settingsConsentRequest(target, server)
	lens := mcpauth.ConsentScopeFromContext(ctx)
	if lens.ProfileName != "" {
		req.ProfileName = lens.ProfileName
	}
	decision, err := (settingsMCPConsent{state: s.state}).MCPConsent(ctx, req)
	if err != nil {
		return err
	}
	if !decision.Admitted {
		return waitingDaemonMCPApprovalError(ctx, s.state, server.Name, req.WorkspaceID)
	}
	return nil
}

func settingsConsentRequest(target mcpauth.Target, server config.MCPServer) settingspkg.MCPConsentRequest {
	req := settingspkg.MCPConsentRequest{
		Server:      server,
		Scope:       string(target.Scope),
		WorkspaceID: target.WorkspaceID,
		Owner:       target.Owner,
	}
	if target.Scope == mcpauth.ScopeProfile {
		req.ProfileName = target.WorkspaceID
		req.WorkspaceID = ""
	}
	if target.Scope == mcpauth.ScopeWorkspaceProfile {
		req.WorkspaceID, req.ProfileName, _ = strings.Cut(target.WorkspaceID, "@pf:")
	}
	return req
}

func waitingDaemonMCPApprovalError(ctx context.Context, state *bootState, name, workspaceID string) error {
	label := workspaceID
	if state.workspaceResolver != nil && label != "" {
		if workspace, err := state.workspaceResolver.Resolve(ctx, label); err == nil {
			label = workspace.Name
		}
	}
	return settingspkg.WaitingMCPApprovalError(name, label)
}
