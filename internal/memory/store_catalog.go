package memory

import (
	"context"
	"errors"
	"fmt"

	"strings"

	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"

	aghworkspace "github.com/compozy/agh/internal/workspace"
)

type scanCandidate struct {
	name    string
	path    string
	modTime time.Time
}

func (s *Store) normalizeScopeAndWorkspace(
	ctx context.Context,
	scope memcontract.Scope,
	workspace string,
) (memcontract.Scope, string, string, error) {
	normalizedScope := scope.Normalize()
	if normalizedScope != "" {
		if err := normalizedScope.Validate(); err != nil {
			return "", "", "", wrapValidationError("resolve scope", string(scope), err)
		}
	}

	workspaceRoot := canonicalWorkspaceRoot(workspace)
	if workspaceRoot == "" {
		workspaceRoot = strings.TrimSpace(s.workspaceRoot)
	}
	if normalizedScope == memcontract.ScopeWorkspace && workspaceRoot == "" {
		return "", "", "", wrapValidationError(
			"resolve scope",
			string(scope),
			errors.New("workspace directory is required"),
		)
	}
	if normalizedScope == memcontract.ScopeAgent && s.agentTier.Normalize() == memcontract.AgentTierWorkspace &&
		workspaceRoot == "" {
		return "", "", "", wrapValidationError(
			"resolve scope",
			string(scope),
			errors.New("workspace directory is required"),
		)
	}
	workspaceID := ""
	if workspaceRoot != "" {
		identity, err := aghworkspace.EnsureIdentity(ctx, workspaceRoot)
		if err != nil {
			return "", "", "", fmt.Errorf("memory: resolve workspace identity for %q: %w", workspaceRoot, err)
		}
		workspaceID = identity.WorkspaceID
	}
	return normalizedScope, workspaceRoot, workspaceID, nil
}

func (s *Store) workspaceIDForRoot(ctx context.Context, workspaceRoot string) (string, error) {
	root := strings.TrimSpace(workspaceRoot)
	if root == "" {
		return "", wrapValidationError(
			"resolve workspace identity",
			"workspace",
			errors.New("workspace directory is required"),
		)
	}
	identity, err := aghworkspace.EnsureIdentity(ctx, root)
	if err != nil {
		return "", fmt.Errorf("memory: resolve workspace identity for %q: %w", root, err)
	}
	return identity.WorkspaceID, nil
}

func (s *Store) catalogWorkspaceForScope(
	ctx context.Context,
	scope memcontract.Scope,
) (string, string, error) {
	switch scope.Normalize() {
	case memcontract.ScopeWorkspace:
		workspaceRoot := strings.TrimSpace(s.workspaceRoot)
		workspaceID, err := s.workspaceIDForRoot(ctx, workspaceRoot)
		return workspaceRoot, workspaceID, err
	case memcontract.ScopeAgent:
		if s.agentTier.Normalize() != memcontract.AgentTierWorkspace {
			return "", "", nil
		}
		workspaceRoot := strings.TrimSpace(s.workspaceRoot)
		workspaceID := strings.TrimSpace(s.agentWorkspaceID)
		if workspaceID != "" {
			return workspaceRoot, workspaceID, nil
		}
		resolvedID, err := s.workspaceIDForRoot(ctx, workspaceRoot)
		return workspaceRoot, resolvedID, err
	default:
		return "", "", nil
	}
}

func (s *Store) catalogAgentName(scope memcontract.Scope) string {
	if scope.Normalize() != memcontract.ScopeAgent {
		return ""
	}
	return strings.TrimSpace(s.agentName)
}

func (s *Store) catalogAgentTier(scope memcontract.Scope) memcontract.AgentTier {
	if scope.Normalize() != memcontract.ScopeAgent {
		return ""
	}
	return s.agentTier.Normalize()
}

func (s *Store) collectSearchDocuments(
	scope memcontract.Scope,
	workspaceRoot string,
	workspaceID string,
) ([]catalogDocument, error) {
	scopes := []struct {
		scope       memcontract.Scope
		workspace   string
		workspaceID string
	}{{scope: memcontract.ScopeGlobal}}
	switch scope.Normalize() {
	case memcontract.ScopeWorkspace:
		scopes = []struct {
			scope       memcontract.Scope
			workspace   string
			workspaceID string
		}{{scope: memcontract.ScopeWorkspace, workspace: workspaceRoot, workspaceID: workspaceID}}
	case memcontract.ScopeAgent:
		scopes = []struct {
			scope       memcontract.Scope
			workspace   string
			workspaceID string
		}{{scope: memcontract.ScopeAgent, workspace: workspaceRoot, workspaceID: workspaceID}}
	default:
		if strings.TrimSpace(workspaceRoot) != "" {
			scopes = append(scopes, struct {
				scope       memcontract.Scope
				workspace   string
				workspaceID string
			}{scope: memcontract.ScopeWorkspace, workspace: workspaceRoot, workspaceID: workspaceID})
		}
	}

	docs := make([]catalogDocument, 0)
	for _, item := range scopes {
		headers, err := s.headersForCatalogScope(item.scope, item.workspace)
		if err != nil {
			return nil, err
		}
		items, err := s.documentsForHeaders(item.scope, item.workspace, item.workspaceID, headers)
		if err != nil {
			return nil, err
		}
		docs = append(docs, items...)
	}
	return docs, nil
}

func (s *Store) collectActualCatalogIDs(filters []catalogFilter) (map[string]struct{}, error) {
	actual := make(map[string]struct{})
	for _, filter := range filters {
		headers, err := s.headersForCatalogScope(filter.scope, filter.workspaceRoot)
		if err != nil {
			return nil, err
		}
		for _, header := range headers {
			actual[catalogDocIDForHeader(filter.scope, filter.workspaceID, header)] = struct{}{}
		}
	}
	return actual, nil
}

func (s *Store) logCatalogEvent(ctx context.Context, record memcontract.OperationRecord) error {
	if s.catalog == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return s.catalog.logEvent(ctx, record)
}

func (s *Store) logMutationEvent(action string, scope memcontract.Scope, filename string) {
	_, workspaceID, err := s.catalogWorkspaceForScope(context.Background(), scope)
	if err != nil {
		s.warn("memory: resolve workspace identity for mutation event failed", "error", err)
		return
	}

	if err := s.logCatalogEvent(
		context.Background(),
		memcontract.OperationRecord{
			Operation: memcontract.Operation("memory." + strings.TrimSpace(action)),
			Scope:     scope.Normalize(),
			Workspace: workspaceID,
			Filename:  strings.TrimSpace(filename),
			AgentName: s.agentNameForEvent(scope),
			Summary:   fmt.Sprintf("scope=%s filename=%s", scope.Normalize(), strings.TrimSpace(filename)),
		},
	); err != nil {
		s.warn(
			"memory: record mutation event failed",
			"action", strings.TrimSpace(action),
			"scope", scope.Normalize(),
			"filename", strings.TrimSpace(filename),
			"error", err,
		)
	}
}

func (s *Store) agentNameForEvent(scope memcontract.Scope) string {
	if scope.Normalize() != memcontract.ScopeAgent {
		return ""
	}
	return strings.TrimSpace(s.agentName)
}

func (s *Store) lockMutations() func() {
	if s == nil || s.mu == nil {
		return func() {}
	}
	s.mu.Lock()
	return s.mu.Unlock
}
