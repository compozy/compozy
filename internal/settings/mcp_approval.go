package settings

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/config"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	"github.com/compozy/compozy/internal/store"
)

type ConsentDecision struct {
	Admitted         bool
	Reason           string
	Waiting          bool
	DefinitionDigest string
}

type MCPApprovalState struct {
	Reason           string `json:"reason,omitempty"`
	Required         bool   `json:"required"`
	State            string `json:"state"`
	DefinitionDigest string `json:"definition_digest"`
}

type MCPConsentRequest struct {
	ProfileName string
	WorkspaceID string
	Server      config.MCPServer
	Scope       string
	Owner       string
}

type MCPConsentProvider interface {
	MCPConsent(context.Context, MCPConsentRequest) (ConsentDecision, error)
	ApproveMCP(context.Context, MCPConsentRequest, string, bool) (store.MCPApproval, error)
}

type WorkspaceMCPWrite struct {
	ProfileName string
	WorkspaceID string
	SourceID    string
	Server      config.MCPServer
	Write       func() error
	// ReadCommitted resolves inheritance after the file is durable.
	ReadCommitted func() (config.MCPServer, error)
}

type WorkspaceMCPApprovalRecorder interface {
	RecordWorkspaceMCPWrite(context.Context, WorkspaceMCPWrite) (warning string, err error)
}

type MCPApprovalError struct{ Code, Message string }

const mcpServerNotFoundCode = "mcp_server_not_found"

func (e *MCPApprovalError) Error() string { return e.Message }
func (e *MCPApprovalError) Unwrap() error {
	if e.Code == mcpServerNotFoundCode {
		return ErrNotFound
	}
	return ErrConflict
}

func WaitingMCPApprovalError(name string, workspace ...string) error {
	label := "CompozyOS"
	if len(workspace) > 0 && workspace[0] != "" {
		label = workspace[0]
	}
	return &MCPApprovalError{
		Code: "mcp_server_waiting_approval",
		Message: fmt.Sprintf(
			"%s is waiting for approval in %s. Allow it first, or sign in from its row to allow and sign in together.",
			name,
			label,
		),
	}
}

func approvalState(decision ConsentDecision, required bool) MCPApprovalState {
	state := "not_required"
	if decision.Waiting {
		state = "waiting"
	} else if required && decision.Admitted {
		state = "allowed"
	}
	reason := ""
	if decision.Waiting {
		reason = decision.Reason
		if reason == "" {
			reason = "new"
		}
	}
	return MCPApprovalState{
		Required:         required,
		State:            state,
		Reason:           reason,
		DefinitionDigest: decision.DefinitionDigest,
	}
}

func consentRequestForTarget(target mcpauth.Target, server config.MCPServer) MCPConsentRequest {
	req := MCPConsentRequest{
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

// SetMCPApproval resolves the current effective definition before binding consent.
func (s *service) SetMCPApproval(
	ctx context.Context,
	req MCPAuthTargetRequest,
	digest string,
	revoke bool,
) (store.MCPApproval, error) {
	if s.mcpConsent == nil {
		return store.MCPApproval{}, unavailableError(fmt.Errorf("settings: MCP consent is unavailable"))
	}
	if req.WorkspaceID != "" && req.ProfileName == "" {
		req.Scope, req.ProfileName = ScopeProfile, mcpAuthDefaultProfileName
	}
	definitions, err := s.mcpApprovalDefinitions(ctx, req)
	if err != nil {
		return store.MCPApproval{}, err
	}
	if len(definitions) == 1 {
		consent := consentRequestForTarget(definitions[0].target, definitions[0].server)
		if consent.Scope == "user" || consent.Scope == "profile" || strings.HasPrefix(consent.Owner, "extension:") {
			return s.mcpConsent.ApproveMCP(ctx, consent, digest, revoke)
		}
	}
	if !revoke {
		resolved, err := selectMCPApprovalDefinition(req.Name, definitions, digest)
		if err != nil {
			return store.MCPApproval{}, err
		}
		consent := consentRequestForTarget(resolved.target, resolved.server)
		consent.ProfileName = req.ProfileName
		return s.mcpConsent.ApproveMCP(ctx, consent, digest, false)
	}
	var approval store.MCPApproval
	for _, resolved := range definitions {
		consent := consentRequestForTarget(resolved.target, resolved.server)
		if consent.WorkspaceID == "" {
			continue
		}
		consent.ProfileName = req.ProfileName
		approval, err = s.mcpConsent.ApproveMCP(ctx, consent, digest, true)
		if err != nil {
			return store.MCPApproval{}, err
		}
	}
	if approval.ServerName == "" {
		consent := consentRequestForTarget(definitions[0].target, definitions[0].server)
		return s.mcpConsent.ApproveMCP(ctx, consent, digest, true)
	}
	return approval, nil
}
