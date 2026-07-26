package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	extensionpkg "github.com/compozy/agh/internal/extension"
	"github.com/compozy/agh/internal/extension/surfaces"
	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	"github.com/compozy/agh/internal/resources"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/google/uuid"
)

const mcpServePrincipalPrefix = "__agh_mcp_serve__"

// HostAPIFacade binds MCP calls to one resolved workspace before dispatching existing Host API logic.
type HostAPIFacade struct {
	handler        *extensionpkg.HostAPIHandler
	checker        *extensionpkg.CapabilityChecker
	workspaces     workspacepkg.RuntimeResolver
	sourceSessions resources.SourceSessionManager
	salt           string

	mu       sync.Mutex
	closed   bool
	sessions map[string]*hostAPIFacadeSession
}

type hostAPIFacadeSession struct {
	mu        sync.RWMutex
	closing   bool
	workspace string
	principal string
	actor     resources.MutationActor
}

// NewHostAPIFacade constructs the daemon-side MCP projection adapter.
func NewHostAPIFacade(
	handler *extensionpkg.HostAPIHandler,
	checker *extensionpkg.CapabilityChecker,
	workspaces workspacepkg.RuntimeResolver,
	sourceSessions resources.SourceSessionManager,
) *HostAPIFacade {
	return &HostAPIFacade{
		handler:        handler,
		checker:        checker,
		workspaces:     workspaces,
		sourceSessions: sourceSessions,
		salt:           uuid.NewString(),
		sessions:       make(map[string]*hostAPIFacadeSession),
	}
}

// Close releases every daemon-local grant and resource source retained by MCP serve clients.
func (f *HostAPIFacade) Close(ctx context.Context) error {
	if f == nil {
		return nil
	}
	f.mu.Lock()
	sessions := f.sessions
	f.closed = true
	f.sessions = make(map[string]*hostAPIFacadeSession)
	f.mu.Unlock()

	var closeErr error
	for _, session := range sessions {
		closeErr = errors.Join(closeErr, f.closeSession(ctx, session))
	}
	return closeErr
}

// CloseHostAPISession releases one MCP serve client's grant and source-owned resources.
func (f *HostAPIFacade) CloseHostAPISession(ctx context.Context, serveSessionID string) error {
	if f == nil {
		return nil
	}
	serveSessionID = strings.TrimSpace(serveSessionID)
	if serveSessionID == "" {
		return errors.New("mcp: serve session id is required")
	}
	f.mu.Lock()
	session := f.sessions[serveSessionID]
	f.mu.Unlock()
	if session == nil {
		return nil
	}
	if err := f.closeSession(ctx, session); err != nil {
		return err
	}
	f.mu.Lock()
	if f.sessions[serveSessionID] == session {
		delete(f.sessions, serveSessionID)
	}
	f.mu.Unlock()
	return nil
}

func (f *HostAPIFacade) closeSession(ctx context.Context, session *hostAPIFacadeSession) error {
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.closing {
		return nil
	}
	session.closing = true
	if f.sourceSessions != nil {
		if err := f.sourceSessions.ResetSource(ctx, mcpServeResourceManagerActor(), session.actor.Source); err != nil {
			session.closing = false
			return fmt.Errorf("mcp: reset resource source: %w", err)
		}
	}
	if f.checker != nil {
		f.checker.Unregister(session.principal)
	}
	return nil
}

// InvokeHostAPI resolves, validates, and binds a projected request before invoking the Host API.
func (f *HostAPIFacade) InvokeHostAPI(
	ctx context.Context,
	serveSessionID string,
	workspaceRef string,
	method string,
	params json.RawMessage,
) (json.RawMessage, error) {
	if f == nil || f.handler == nil || f.checker == nil || f.workspaces == nil {
		return nil, errors.New("mcp: host api facade is not configured")
	}
	decision, ok := hostAPIProjectionDecisions[extensionprotocol.HostAPIMethod(strings.TrimSpace(method))]
	if !ok || !decision.Publish {
		return nil, fmt.Errorf("mcp: host api method %q is not projected", method)
	}
	resolved, err := f.workspaces.Resolve(ctx, strings.TrimSpace(workspaceRef))
	if err != nil {
		return nil, fmt.Errorf("mcp: resolve workspace: %w", err)
	}
	workspaceID := strings.TrimSpace(resolved.ID)
	workspaceRoot := strings.TrimSpace(resolved.RootDir)
	if workspaceID == "" || workspaceRoot == "" {
		return nil, errors.New("mcp: resolved workspace identity is incomplete")
	}

	boundParams, err := bindHostAPIParams(params, decision.Binding, workspaceID, workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("mcp: bind %q to workspace: %w", method, err)
	}
	session, err := f.session(ctx, serveSessionID, workspaceID)
	if err != nil {
		return nil, err
	}
	session.mu.RLock()
	defer session.mu.RUnlock()
	if session.closing {
		return nil, errors.New("mcp: host api session is closing")
	}
	result, err := f.handler.HandleWithResourceActor(
		ctx,
		session.principal,
		strings.TrimSpace(method),
		boundParams,
		session.actor,
	)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("mcp: encode host api result: %w", err)
	}
	return encoded, nil
}

func (f *HostAPIFacade) session(
	ctx context.Context,
	serveSessionID string,
	workspaceID string,
) (*hostAPIFacadeSession, error) {
	serveSessionID = strings.TrimSpace(serveSessionID)
	if serveSessionID == "" {
		return nil, errors.New("mcp: serve session id is required")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return nil, errors.New("mcp: host api facade is closed")
	}
	if session, ok := f.sessions[serveSessionID]; ok {
		if session.workspace != workspaceID {
			return nil, errors.New("mcp: serve session workspace changed")
		}
		return session, nil
	}

	principal := mcpServePrincipalPrefix + f.salt + "__" + serveSessionID + "__" + workspaceID
	manifest := &extensionpkg.Manifest{
		Actions:  extensionpkg.ActionsConfig{Requires: ProjectedHostAPIMethods()},
		Security: extensionpkg.SecurityConfig{Capabilities: []string{"*"}},
		Resources: extensionpkg.ResourcesConfig{Publish: extensionpkg.ResourceGrantRequest{
			Families: resourceManifestFamilies(),
			MaxScope: resources.ResourceScopeKindWorkspace,
		}},
	}
	grant, err := f.checker.RegisterForSession(
		principal,
		extensionpkg.SourceUser,
		manifest,
		resources.ResourceScopeKindWorkspace,
	)
	if err != nil {
		return nil, fmt.Errorf("mcp: register host api principal: %w", err)
	}
	actor := resources.MutationActor{
		Kind:         resources.MutationActorKindExtension,
		ID:           principal,
		SessionNonce: uuid.NewString(),
		Source: resources.ResourceSource{
			Kind: resources.ResourceSourceKind("mcp_serve"),
			ID:   principal,
		},
		MaxScope: resources.ResourceScope{
			Kind: resources.ResourceScopeKindWorkspace,
			ID:   workspaceID,
		},
		GrantedKinds:  grant.ResourceKinds,
		GrantedScopes: grant.ResourceScopes,
	}
	if f.sourceSessions != nil {
		if err := f.sourceSessions.ActivateSourceSession(
			ctx,
			mcpServeResourceManagerActor(),
			actor.Source,
			actor.SessionNonce,
		); err != nil {
			f.checker.Unregister(principal)
			return nil, fmt.Errorf("mcp: activate resource session: %w", err)
		}
	}
	session := &hostAPIFacadeSession{workspace: workspaceID, principal: principal, actor: actor}
	f.sessions[serveSessionID] = session
	return session, nil
}

func resourceManifestFamilies() []string {
	families := make([]string, 0)
	for _, surface := range surfaces.All() {
		if surface.ExtensionPublish {
			families = append(families, string(surface.ManifestFamily))
		}
	}
	return families
}

func mcpServeResourceManagerActor() resources.MutationActor {
	return resources.MutationActor{
		Kind: resources.MutationActorKindDaemon,
		ID:   "mcp-serve",
		Source: resources.ResourceSource{
			Kind: "daemon",
			ID:   "mcp-serve",
		},
		MaxScope: resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
	}
}
