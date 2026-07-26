package core

import (
	"context"
	"errors"
	"fmt"
	"io"

	"os"
	"path/filepath"

	"strings"

	memcontract "github.com/compozy/agh/internal/memory/contract"

	"github.com/compozy/agh/internal/api/contract"

	"github.com/compozy/agh/internal/memory"

	aghworkspace "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

// ResolveMemoryLocation resolves the storage location for a memory document.
func (h *BaseHandlers) ResolveMemoryLocation(
	filename string,
	rawScope string,
	rawWorkspace string,
) (MemoryLocation, error) {
	return h.resolveMemoryLocation(context.Background(), filename, memorySelector{
		Scope:       memcontract.Scope(rawScope),
		WorkspaceID: rawWorkspace,
	})
}

func (h *BaseHandlers) resolveMemoryLocation(
	ctx context.Context,
	filename string,
	selector memorySelector,
) (MemoryLocation, error) {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return MemoryLocation{}, NewMemoryValidationError(errors.New("filename is required"))
	}
	if h.MemoryStore == nil {
		return MemoryLocation{}, errors.New("memory store is not configured")
	}

	resolved, err := h.resolveMemorySelector(ctx, selector, false)
	if err != nil {
		return MemoryLocation{}, err
	}
	if resolved.Scope != "" {
		return h.resolveScopedMemoryLocation(ctx, filename, resolved)
	}

	candidates := []MemoryLocation{{Scope: memcontract.ScopeGlobal, Filename: filename}}
	if resolved.Workspace != "" {
		candidates = append(candidates, MemoryLocation{
			Scope:       memcontract.ScopeWorkspace,
			Workspace:   resolved.Workspace,
			WorkspaceID: resolved.WorkspaceID,
			Filename:    filename,
		})
	}
	if resolved.AgentName != "" && resolved.AgentTier != "" {
		candidates = append(candidates, MemoryLocation{
			Scope:       memcontract.ScopeAgent,
			Workspace:   resolved.Workspace,
			WorkspaceID: resolved.WorkspaceID,
			AgentName:   resolved.AgentName,
			AgentTier:   resolved.AgentTier,
			Filename:    filename,
		})
	}

	matches := make([]MemoryLocation, 0, len(candidates))
	for _, candidate := range candidates {
		store, err := h.memoryStoreForLocation(ctx, candidate)
		if err != nil {
			return MemoryLocation{}, err
		}
		exists, err := store.Exists(candidate.Scope, filename)
		if err != nil {
			return MemoryLocation{}, err
		}
		if exists {
			matches = append(matches, candidate)
		}
	}

	switch len(matches) {
	case 0:
		return MemoryLocation{}, fmt.Errorf("%w: memory %q not found", os.ErrNotExist, filename)
	case 1:
		return matches[0], nil
	default:
		return MemoryLocation{}, NewMemoryValidationError(
			fmt.Errorf("memory %q exists in multiple scopes; set scope explicitly", filename),
		)
	}
}

func (h *BaseHandlers) resolveScopedMemoryLocation(
	ctx context.Context,
	filename string,
	resolved memorySelector,
) (MemoryLocation, error) {
	store, err := h.memoryStoreForSelector(ctx, resolved)
	if err != nil {
		return MemoryLocation{}, err
	}
	exists, err := store.Exists(resolved.Scope, filename)
	if err != nil {
		return MemoryLocation{}, err
	}
	if !exists {
		return MemoryLocation{}, fmt.Errorf("%w: memory %q not found", os.ErrNotExist, filename)
	}
	return MemoryLocation{
		Scope:       resolved.Scope,
		Workspace:   resolved.Workspace,
		WorkspaceID: resolved.WorkspaceID,
		AgentName:   resolved.AgentName,
		AgentTier:   resolved.AgentTier,
		Filename:    filename,
	}, nil
}

func (h *BaseHandlers) memoryStoreForSelector(ctx context.Context, selector memorySelector) (*memory.Store, error) {
	if h.MemoryStore == nil {
		return nil, errors.New("memory store is not configured")
	}

	switch selector.Scope.Normalize() {
	case memcontract.ScopeGlobal:
		return h.MemoryStore, nil
	case memcontract.ScopeWorkspace:
		resolved, err := h.resolveMemorySelector(ctx, selector, true)
		if err != nil {
			return nil, err
		}
		return h.MemoryStore.ForWorkspace(resolved.Workspace), nil
	case memcontract.ScopeAgent:
		resolved, err := h.resolveMemorySelector(ctx, selector, true)
		if err != nil {
			return nil, err
		}
		base := h.MemoryStore
		if resolved.AgentTier.Normalize() == memcontract.AgentTierWorkspace {
			base = base.ForWorkspace(resolved.Workspace)
		}
		return base.ForAgent(resolved.WorkspaceID, resolved.AgentName, resolved.AgentTier), nil
	default:
		return nil, NewMemoryValidationError(fmt.Errorf("unsupported scope %q", selector.Scope))
	}
}

func (h *BaseHandlers) memoryRecallStoreForSelector(
	ctx context.Context,
	selector memorySelector,
) (*memory.Store, error) {
	if h.MemoryStore == nil {
		return nil, errors.New("memory store is not configured")
	}
	resolved, err := h.resolveMemorySelector(ctx, selector, false)
	if err != nil {
		return nil, err
	}
	store := h.MemoryStore
	needsWorkspaceStore := strings.TrimSpace(resolved.Workspace) != "" &&
		(resolved.Scope == "" ||
			resolved.Scope == memcontract.ScopeWorkspace ||
			(resolved.Scope == memcontract.ScopeAgent &&
				resolved.AgentTier.Normalize() == memcontract.AgentTierWorkspace))
	if needsWorkspaceStore {
		store = store.ForWorkspace(resolved.Workspace)
	}
	if strings.TrimSpace(resolved.AgentName) != "" && resolved.AgentTier.Normalize() != "" {
		store = store.ForAgent(resolved.WorkspaceID, resolved.AgentName, resolved.AgentTier)
	}
	return store, nil
}

func (h *BaseHandlers) memoryStoreForLocation(ctx context.Context, location MemoryLocation) (*memory.Store, error) {
	return h.memoryStoreForSelector(ctx, memorySelector{
		Scope:       location.Scope,
		Workspace:   location.Workspace,
		WorkspaceID: location.WorkspaceID,
		AgentName:   location.AgentName,
		AgentTier:   location.AgentTier,
	})
}

func (h *BaseHandlers) resolveMemorySelector(
	ctx context.Context,
	selector memorySelector,
	requireScope bool,
) (memorySelector, error) {
	resolved := selector
	scope, err := parseOptionalMemoryScope(string(selector.Scope))
	if err != nil {
		return memorySelector{}, err
	}
	resolved.Scope = scope
	if requireScope && resolved.Scope == "" {
		return memorySelector{}, NewMemoryValidationError(errors.New("scope is required"))
	}
	resolved.AgentName = strings.TrimSpace(resolved.AgentName)
	resolved.AgentTier = resolved.AgentTier.Normalize()
	workspaceRef := firstNonEmptyString(resolved.Workspace, resolved.WorkspaceID)
	needsWorkspace := resolved.Scope == memcontract.ScopeWorkspace || workspaceRef != "" ||
		(resolved.Scope == memcontract.ScopeAgent && resolved.AgentTier == memcontract.AgentTierWorkspace)
	if needsWorkspace {
		workspaceRoot, workspaceID, err := h.resolveMemoryWorkspaceRef(ctx, workspaceRef)
		if err != nil {
			return memorySelector{}, err
		}
		resolved.Workspace = workspaceRoot
		resolved.WorkspaceID = workspaceID
	}
	if resolved.Scope == memcontract.ScopeAgent {
		if resolved.AgentName == "" {
			return memorySelector{}, NewMemoryValidationError(errors.New("agent_name is required for agent scope"))
		}
		if resolved.AgentTier == "" {
			return memorySelector{}, NewMemoryValidationError(errors.New("agent_tier is required for agent scope"))
		}
	}
	if resolved.AgentTier != "" {
		if err := resolved.AgentTier.Validate(); err != nil {
			return memorySelector{}, NewMemoryValidationError(err)
		}
	}
	return resolved, nil
}

func (h *BaseHandlers) resolveMemoryWorkspaceRef(ctx context.Context, raw string) (string, string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", "", NewMemoryValidationError(errors.New("workspace_id is required for workspace scope"))
	}
	if h.Workspaces != nil {
		resolved, err := h.Workspaces.Resolve(ctx, trimmed)
		if err == nil {
			workspaceID := strings.TrimSpace(resolved.WorkspaceID)
			if workspaceID == "" {
				return "", "", errors.New("memory: resolved workspace_id is empty")
			}
			return strings.TrimSpace(resolved.RootDir), workspaceID, nil
		}
		if !filepath.IsAbs(trimmed) {
			return "", "", err
		}
	}
	workspace, err := resolveMemoryWorkspace(trimmed)
	if err != nil {
		return "", "", err
	}
	identity, err := aghworkspace.EnsureIdentity(ctx, workspace)
	if err != nil {
		return "", "", fmt.Errorf("memory: resolve workspace identity: %w", err)
	}
	return workspace, identity.WorkspaceID, nil
}

func memorySelectorFromQuery(c *gin.Context) memorySelector {
	workspaceID := firstNonEmptyString(c.Query("workspace_id"), c.Query("workspace"))
	return memorySelector{
		Scope:         memcontract.Scope(c.Query("scope")),
		WorkspaceID:   workspaceID,
		AgentName:     c.Query("agent_name"),
		AgentTier:     memcontract.AgentTier(c.Query("agent_tier")),
		IncludeSystem: c.Query("include_system") == "true",
	}
}

func memorySelectorFromScopePayload(payload contract.MemoryScopeSelectorPayload) memorySelector {
	return memorySelector{
		Scope:       payload.Scope,
		WorkspaceID: payload.WorkspaceID,
		AgentName:   payload.AgentName,
		AgentTier:   payload.AgentTier,
	}
}

func (h *BaseHandlers) decodeMemoryReloadSelector(c *gin.Context) (memorySelector, error) {
	selector := memorySelectorFromQuery(c)
	var body struct {
		Scope                 memcontract.Scope     `json:"scope"`
		WorkspaceID           string                `json:"workspace_id"`
		AgentName             string                `json:"agent_name"`
		AgentTier             memcontract.AgentTier `json:"agent_tier"`
		LegacyScope           memcontract.Scope     `json:"Scope"`
		LegacyWorkspaceID     string                `json:"WorkspaceID"`
		LegacyAgentName       string                `json:"AgentName"`
		LegacyAgentTier       memcontract.AgentTier `json:"AgentTier"`
		LegacyIncludeSystem   bool                  `json:"IncludeSystem"`
		DiscardedIncludeState bool                  `json:"include_system"`
	}
	if err := c.ShouldBindJSON(&body); err != nil && !errors.Is(err, io.EOF) {
		return memorySelector{}, fmt.Errorf("%s: decode memory reload request: %w", h.transportName(), err)
	}
	selector.Scope = firstNonEmptyScope(selector.Scope, body.Scope, body.LegacyScope)
	selector.WorkspaceID = firstNonEmptyString(selector.WorkspaceID, body.WorkspaceID, body.LegacyWorkspaceID)
	selector.AgentName = firstNonEmptyString(selector.AgentName, body.AgentName, body.LegacyAgentName)
	selector.AgentTier = firstNonEmptyAgentTier(selector.AgentTier, body.AgentTier, body.LegacyAgentTier)
	_ = body.LegacyIncludeSystem
	_ = body.DiscardedIncludeState
	return selector, nil
}

func (h *BaseHandlers) resolveMemoryCreateSelector(
	ctx context.Context,
	req contract.MemoryCreateRequest,
) (memorySelector, error) {
	scope := req.Scope.Normalize()
	if scope == "" {
		defaultScope, err := memcontract.DefaultScopeForType(req.Type)
		if err != nil {
			return memorySelector{}, NewMemoryValidationError(err)
		}
		scope = defaultScope
	}
	return h.resolveMemorySelector(ctx, memorySelector{
		Scope:       scope,
		WorkspaceID: req.WorkspaceID,
		AgentName:   req.AgentName,
		AgentTier:   req.AgentTier,
	}, true)
}
