package settings

import (
	"context"
	"errors"
	"strings"

	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	"github.com/compozy/compozy/internal/runtimemcp"
)

type RuntimeMCPResolver interface {
	Resolve(context.Context, runtimemcp.Request, string) (runtimemcp.ResolvedServer, error)
}

type RuntimeMCPImportRequest struct {
	Scope       ScopeKind
	WorkspaceID string
	ProfileName string
	Runtime     string
	ServerName  string
	Name        string
}

type RuntimeMCPImportResult struct {
	Mutation MutationResult
	Source   runtimemcp.Server
	Notes    []string
	Partial  bool
}

func (s *service) ImportRuntimeMCP(ctx context.Context, req RuntimeMCPImportRequest) (RuntimeMCPImportResult, error) {
	targetWorkspaceID := req.WorkspaceID
	if req.Scope != ScopeWorkspace {
		targetWorkspaceID = ""
	}
	scope, workspaceID, err := s.normalizeReadScope(req.Scope, targetWorkspaceID)
	if err != nil {
		return RuntimeMCPImportResult{}, err
	}
	if _, err := validateCollectionScope(CollectionMCPServers, scope); err != nil {
		return RuntimeMCPImportResult{}, err
	}
	profile, err := normalizeSettingsProfileName(scope, req.ProfileName)
	if err != nil {
		return RuntimeMCPImportResult{}, err
	}
	resolver, ok := s.runtimeMCP.(RuntimeMCPResolver)
	if !ok {
		return RuntimeMCPImportResult{}, unavailableError(errors.New("runtime MCP copy unavailable"))
	}
	readScope := scope
	if req.WorkspaceID != "" && scope == ScopeUser {
		readScope = ScopeWorkspace
	}
	readContext, err := s.runtimeMCPReadContext(ctx, readScope, req.WorkspaceID, profile)
	if err != nil {
		return RuntimeMCPImportResult{}, err
	}
	resolved, err := resolver.Resolve(
		ctx,
		runtimemcp.Request{Context: readContext, Runtime: req.Runtime},
		req.ServerName,
	)
	if err != nil {
		return RuntimeMCPImportResult{}, importSettingsError(err)
	}
	server, err := resolved.CopyDefinition()
	if err != nil {
		return RuntimeMCPImportResult{}, importSettingsError(err)
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = req.ServerName
	}
	server.Name = name
	secrets := MCPSecretValues{SecretEnv: server.Env, SecretHeaders: server.Headers, allowEmpty: true}
	server.Env, server.Headers = nil, nil
	mutation, err := s.putMCPServer(
		ctx,
		scope,
		workspaceID,
		profile,
		name,
		TargetAuto,
		server,
		secrets,
		MCPSecretPreservation{},
		nil,
		true,
	)
	mutation, err = s.finalizeCommittedCollectionMutation(ctx, mutation, err, "replace", true)
	if err != nil {
		return RuntimeMCPImportResult{}, err
	}
	if mcpauth.OAuthCapable(server) && (server.Auth.Enabled() || len(secrets.SecretHeaders) == 0) &&
		mutation.MCPServer != nil {
		mutation.MCPServer.AuthStatus = &mcpauth.Status{
			Status:      mcpauth.StatusNeedsLogin,
			ServerName:  name,
			Owner:       mcpManualOwner,
			Scope:       mcpauth.Scope(scope),
			WorkspaceID: workspaceID,
		}
	}
	source := resolved.Metadata()
	return RuntimeMCPImportResult{Mutation: mutation, Source: source, Partial: len(mutation.Warnings) > 0,
		Notes: []string{runtimeDisplayName(source.Runtime) + " still loads its own copy"}}, nil
}

func importSettingsError(err error) error {
	if failure, ok := errors.AsType[*runtimemcp.ImportError](err); ok {
		if failure.Code == mcpServerNotFoundCode {
			return notFoundError(err)
		}
		return unprocessableError(err)
	}
	return err
}

func importScopeLabel(scope ScopeKind) string {
	switch scope {
	case ScopeWorkspace:
		return "Project"
	case ScopeProfile:
		return "Profile"
	default:
		return "Personal"
	}
}

func runtimeDisplayName(runtime string) string {
	for _, descriptor := range runtimemcp.Descriptors() {
		if descriptor.Runtime == runtime {
			return descriptor.DisplayName
		}
	}
	return runtime
}

func (s *service) runtimeMCPReadContext(
	ctx context.Context,
	scope ScopeKind,
	workspaceID, profile string,
) (runtimemcp.ReadContext, error) {
	_, resolved, err := s.loadConfig(ctx, scope, workspaceID, profile)
	if err != nil {
		return runtimemcp.ReadContext{}, err
	}
	readContext, err := s.operatorRuntimeMCPContext()
	if err != nil {
		return runtimemcp.ReadContext{}, err
	}
	readContext.Profile = profile
	if resolved != nil {
		readContext.CWD, readContext.WorkspaceRoot = resolved.RootDir, resolved.RootDir
	}
	return readContext, nil
}
