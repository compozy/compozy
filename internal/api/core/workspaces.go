package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/session"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

// CreateWorkspace registers a workspace.
func (h *BaseHandlers) CreateWorkspace(c *gin.Context) {
	var req contract.CreateWorkspaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode create workspace request: %w", h.transportName(), err),
		)
		return
	}

	rootDir := strings.TrimSpace(req.RootDir)
	if err := validateAbsolutePathInternal(h.transportName(), "root_dir", rootDir); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}

	addDirs := trimStringSliceInternal(req.AddDirs)
	if err := validateAbsolutePathsInternal(h.transportName(), "add_dirs", addDirs); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}

	workspace, err := h.Workspaces.Register(c.Request.Context(), workspacepkg.RegisterOptions{
		RootDir:        rootDir,
		Name:           strings.TrimSpace(req.Name),
		AdditionalDirs: addDirs,
		DefaultAgent:   strings.TrimSpace(req.DefaultAgent),
		SandboxRef:     strings.TrimSpace(req.SandboxRef),
	})
	if err != nil {
		h.respondError(c, StatusForWorkspaceError(err), err)
		return
	}

	c.JSON(http.StatusCreated, contract.WorkspaceResponse{
		Workspace: WorkspacePayloadFromWorkspace(workspace),
	})
}

// ListWorkspaces returns all registered workspaces.
func (h *BaseHandlers) ListWorkspaces(c *gin.Context) {
	workspaces, err := h.Workspaces.List(c.Request.Context())
	if err != nil {
		h.respondError(c, StatusForWorkspaceError(err), err)
		return
	}

	payload := make([]contract.WorkspacePayload, 0, len(workspaces))
	for _, workspace := range workspaces {
		payload = append(payload, WorkspacePayloadFromWorkspace(workspace))
	}

	c.JSON(http.StatusOK, contract.WorkspacesResponse{Workspaces: payload})
}

// GetWorkspace returns one resolved workspace with related sessions, agents, and skills.
func (h *BaseHandlers) GetWorkspace(c *gin.Context) {
	resolved, err := h.Workspaces.Resolve(c.Request.Context(), workspaceRefFromRoute(c))
	if err != nil {
		h.respondError(c, StatusForWorkspaceError(err), err)
		return
	}

	sessions, err := h.Sessions.ListAll(c.Request.Context())
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}

	agents, err := h.workspaceDetailAgentEntries(c.Request.Context(), &resolved)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, contract.WorkspaceDetailPayload{
		Workspace: WorkspacePayloadFromWorkspace(resolved.Workspace),
		Sessions:  SessionPayloadsForWorkspace(sessions, resolved.WorkspaceID),
		Agents:    AgentPayloadsFromEntries(agents),
		Skills:    WorkspaceSkillPayloads(resolved.Skills),
		Providers: SessionProviderOptionPayloadsFromConfig(&resolved.Config),
	})
}

func (h *BaseHandlers) workspaceDetailAgentEntries(
	ctx context.Context,
	resolved *workspacepkg.ResolvedWorkspace,
) ([]AgentCatalogEntry, error) {
	if resolved == nil {
		return nil, errors.New("api: resolved workspace is required")
	}

	merged := make(map[string]AgentCatalogEntry, len(resolved.Agents))
	for _, agent := range resolved.Agents {
		name := strings.TrimSpace(agent.Name)
		if name == "" {
			continue
		}
		merged[name] = h.agentCatalogEntryFromDef(agent, resolved.ID)
	}

	if h.AgentCatalog != nil {
		catalogAgents, err := h.AgentCatalog.ListAgents(ctx)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return nil, err
			}
		}
		for _, entry := range catalogAgents {
			agent := entry.Def
			name := strings.TrimSpace(agent.Name)
			if name == "" {
				continue
			}
			if _, exists := merged[name]; exists {
				continue
			}
			merged[name] = entry
		}
	}

	names := make([]string, 0, len(merged))
	for name := range merged {
		names = append(names, name)
	}
	sort.Strings(names)

	agents := make([]AgentCatalogEntry, 0, len(names))
	for _, name := range names {
		agents = append(agents, merged[name])
	}
	return agents, nil
}

// UpdateWorkspace updates a registered workspace.
func (h *BaseHandlers) UpdateWorkspace(c *gin.Context) {
	workspace, err := h.Workspaces.Get(c.Request.Context(), workspaceRefFromRoute(c))
	if err != nil {
		h.respondError(c, StatusForWorkspaceError(err), err)
		return
	}

	var req contract.UpdateWorkspaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode update workspace request: %w", h.transportName(), err),
		)
		return
	}

	var opts workspacepkg.UpdateOptions
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			h.respondError(c, http.StatusBadRequest, fmt.Errorf("%s: name is required", h.transportName()))
			return
		}
		opts.Name = &name
	}
	if req.AddDirs != nil {
		addDirs := trimStringSliceInternal(*req.AddDirs)
		if err := validateAbsolutePathsInternal(h.transportName(), "add_dirs", addDirs); err != nil {
			h.respondError(c, http.StatusBadRequest, err)
			return
		}
		opts.AdditionalDirs = &addDirs
	}
	if req.DefaultAgent != nil {
		defaultAgent := strings.TrimSpace(*req.DefaultAgent)
		opts.DefaultAgent = &defaultAgent
	}
	if req.SandboxRef != nil {
		sandboxRef := strings.TrimSpace(*req.SandboxRef)
		opts.SandboxRef = &sandboxRef
	}

	if err := h.Workspaces.Update(c.Request.Context(), workspace.ID, opts); err != nil {
		h.respondError(c, StatusForWorkspaceError(err), err)
		return
	}

	updated, err := h.Workspaces.Get(c.Request.Context(), workspace.ID)
	if err != nil {
		h.respondError(c, StatusForWorkspaceError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.WorkspaceResponse{
		Workspace: WorkspacePayloadFromWorkspace(updated),
	})
}

// DeleteWorkspace unregisters a workspace.
func (h *BaseHandlers) DeleteWorkspace(c *gin.Context) {
	workspace, err := h.Workspaces.Get(c.Request.Context(), workspaceRefFromRoute(c))
	if err != nil {
		h.respondError(c, StatusForWorkspaceError(err), err)
		return
	}

	if err := h.Workspaces.Unregister(c.Request.Context(), workspace.ID); err != nil {
		h.respondError(c, StatusForWorkspaceError(err), err)
		return
	}

	c.Status(http.StatusNoContent)
}

// ResolveWorkspace resolves or registers a workspace from a path.
func (h *BaseHandlers) ResolveWorkspace(c *gin.Context) {
	var req contract.ResolveWorkspaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode resolve workspace request: %w", h.transportName(), err),
		)
		return
	}

	path := strings.TrimSpace(req.Path)
	if err := validateAbsolutePathInternal(h.transportName(), "path", path); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}

	resolved, err := h.Workspaces.ResolveOrRegister(c.Request.Context(), path)
	if err != nil {
		h.respondError(c, StatusForWorkspaceError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.WorkspaceResponse{
		Workspace: WorkspacePayloadFromWorkspace(resolved.Workspace),
	})
}

func (h *BaseHandlers) validateCreateSessionRequest(req contract.CreateSessionRequest) error {
	if err := validateCreateSessionRequest(h.transportName(), req.Workspace, req.WorkspacePath); err != nil {
		return err
	}
	return validateCreateSessionRuntimeOverrides(
		h.transportName(),
		req.Provider,
		req.Model,
		string(req.ReasoningEffort),
	)
}

func (h *BaseHandlers) lookupWorkspaceID(ctx context.Context, ref string) (string, error) {
	return lookupWorkspaceID(ctx, h.transportName(), h.Workspaces, ref)
}

// SessionPayloadsForWorkspace filters and converts sessions for one workspace.
func SessionPayloadsForWorkspace(infos []*session.Info, workspaceID string) []contract.SessionPayload {
	return SessionPayloadsFromInfos(
		visibleSessionInfosInternal(filterSessionInfosByWorkspaceIDInternal(infos, workspaceID)),
	)
}
