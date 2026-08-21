package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/gin-gonic/gin"
)

// ListAgents returns all readable agent definitions in home paths.
func (h *BaseHandlers) ListAgents(c *gin.Context) {
	if workspaceRef := strings.TrimSpace(c.Query("workspace")); workspaceRef != "" {
		profileName, err := h.agentResourceProfileName(c)
		if err != nil {
			h.respondProfileReadScopeError(c, err)
			return
		}
		resolved, err := h.workspaceAgentEntriesWithDiagnostics(
			c.Request.Context(),
			workspaceRef,
			profileName,
		)
		if err != nil {
			h.respondError(c, statusForAgentWorkspaceError(err), err)
			return
		}
		h.respondAgentEntries(
			c,
			resolved.Entries,
			&resolved.Config,
			resolved.WorkspaceID,
			resolved.Diagnostics,
		)
		return
	}
	profileName, err := h.agentResourceProfileName(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	agentDefs, err := compozyconfig.LoadWorkspaceAgentDefs("", nil, h.HomePaths, profileName)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	h.respondAgentDefs(c, agentDefs, &h.Config, "")
}

// CreateAgent writes a new global or workspace-local AGENT.md definition.
func (h *BaseHandlers) CreateAgent(c *gin.Context) {
	startedAt := time.Now()
	var req contract.CreateAgentRequest
	if err := decodeStrictCreateAgentRequest(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode create agent request: %w", h.transportName(), err),
		)
		return
	}

	profileName := ""
	var err error
	if req.Scope == contract.AgentCreateScopeWorkspace {
		profileName, err = h.agentResourceProfileName(c)
		if err != nil {
			h.respondProfileReadScopeError(c, err)
			return
		}
	}
	draft, path, workspaceID, runtimeConfig, err := h.createAgentDraftAndPath(c.Request.Context(), req, profileName)
	if err != nil {
		h.respondError(c, statusForCreateAgentError(err), err)
		return
	}
	if h.AgentDefinitionSync == nil {
		h.respondError(c, http.StatusServiceUnavailable, errAgentDefinitionSyncUnavailable)
		return
	}

	agent, err := compozyconfig.CreateAgentDefFile(path, draft, false)
	if err != nil {
		h.respondError(c, statusForCreateAgentError(err), err)
		return
	}
	syncStartedAt := time.Now()
	if err := h.AgentDefinitionSync.Sync(c.Request.Context()); err != nil {
		syncErr := errors.Join(
			fmt.Errorf("api: sync created agent definition: %w", err),
			h.rollbackCreatedAgentDefinition(c.Request.Context(), agent.SourcePath),
		)
		h.logAgentMutationFailure("create", agent.SourcePath, startedAt, time.Since(syncStartedAt), syncErr)
		h.respondError(c, http.StatusInternalServerError, syncErr)
		return
	}
	entry := h.agentCatalogEntryFromDef(agent, workspaceID)
	h.logAgentMutation("create", entry, startedAt, time.Since(syncStartedAt))
	c.JSON(http.StatusCreated, contract.AgentResponse{
		Agent: AgentPayloadFromEntryWithConfig(entry, &runtimeConfig),
	})
}

func (h *BaseHandlers) rollbackCreatedAgentDefinition(ctx context.Context, sourcePath string) error {
	agentsRoot := filepath.Dir(filepath.Dir(sourcePath))
	if err := compozyconfig.DeleteAgentDefinition(agentsRoot, sourcePath); err != nil {
		return fmt.Errorf("api: roll back created agent definition: %w", err)
	}
	if err := h.AgentDefinitionSync.Sync(ctx); err != nil {
		return fmt.Errorf("api: reconcile catalog after create rollback: %w", err)
	}
	return nil
}

// GetAgent returns one agent definition by name.
func (h *BaseHandlers) GetAgent(c *gin.Context) {
	if workspaceRef := strings.TrimSpace(c.Query("workspace")); workspaceRef != "" {
		profileName, err := h.agentResourceProfileName(c)
		if err != nil {
			h.respondProfileReadScopeError(c, err)
			return
		}
		entry, cfg, err := h.workspaceAgentDef(
			c.Request.Context(),
			workspaceRef,
			c.Param("name"),
			profileName,
		)
		if err != nil {
			h.respondError(c, statusForAgentWorkspaceError(err), err)
			return
		}
		c.JSON(http.StatusOK, contract.AgentResponse{Agent: AgentPayloadFromEntryWithConfig(entry, &cfg)})
		return
	}
	profileName, err := h.agentResourceProfileName(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	agentDefs, err := compozyconfig.LoadWorkspaceAgentDefs("", nil, h.HomePaths, profileName)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	for _, agent := range agentDefs {
		if strings.TrimSpace(agent.Name) != strings.TrimSpace(c.Param("name")) {
			continue
		}
		c.JSON(http.StatusOK, contract.AgentResponse{
			Agent: AgentPayloadFromEntryWithConfig(h.agentCatalogEntryFromDef(agent, ""), &h.Config),
		})
		return
	}
	h.respondError(c, http.StatusNotFound, os.ErrNotExist)
}
