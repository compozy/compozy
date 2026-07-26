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

	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/gin-gonic/gin"
)

// ListAgents returns all readable agent definitions in home paths.
func (h *BaseHandlers) ListAgents(c *gin.Context) {
	if workspaceRef := strings.TrimSpace(c.Query("workspace")); workspaceRef != "" {
		entries, workspaceID, cfg, diagnostics, err := h.workspaceAgentEntriesWithDiagnostics(
			c.Request.Context(),
			workspaceRef,
		)
		if err != nil {
			h.respondError(c, statusForAgentWorkspaceError(err), err)
			return
		}
		for index := range entries {
			if entries[index].Origin == contract.AgentOriginWorkspace {
				entries[index].WorkspaceID = workspaceID
			}
		}
		h.respondAgentEntries(c, entries, &cfg, workspaceID, diagnostics)
		return
	}

	if h.AgentCatalog != nil {
		entries, err := h.AgentCatalog.ListAgents(c.Request.Context())
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				c.JSON(http.StatusOK, contract.AgentsResponse{Agents: []contract.AgentPayload{}})
				return
			}
			h.respondError(c, http.StatusInternalServerError, err)
			return
		}
		h.respondAgentEntries(c, entries, &h.Config, "")
		return
	}

	entries, err := os.ReadDir(h.HomePaths.AgentsDir)
	switch {
	case err == nil:
	case errors.Is(err, os.ErrNotExist):
		c.JSON(http.StatusOK, contract.AgentsResponse{Agents: []contract.AgentPayload{}})
		return
	default:
		h.respondError(
			c,
			http.StatusInternalServerError,
			fmt.Errorf("%s: read agents directory %q: %w", h.transportName(), h.HomePaths.AgentsDir, err),
		)
		return
	}

	agentDefs := make([]aghconfig.AgentDef, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := strings.TrimSpace(entry.Name())
		if name == "" {
			continue
		}

		agent, loadErr := h.AgentLoader(name, h.HomePaths)
		if loadErr != nil {
			h.Logger.Warn(
				h.transportName()+": skip unreadable agent definition",
				"agent_name",
				name,
				handlersErrorKey,
				loadErr,
			)
			continue
		}
		agentDefs = append(agentDefs, agent)
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

	draft, path, workspaceID, runtimeConfig, err := h.createAgentDraftAndPath(c.Request.Context(), req)
	if err != nil {
		h.respondError(c, statusForCreateAgentError(err), err)
		return
	}
	if h.AgentDefinitionSync == nil {
		h.respondError(c, http.StatusServiceUnavailable, errAgentDefinitionSyncUnavailable)
		return
	}

	agent, err := aghconfig.CreateAgentDefFile(path, draft, false)
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
	if err := aghconfig.DeleteAgentDefinition(agentsRoot, sourcePath); err != nil {
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
		entry, cfg, err := h.workspaceAgentDef(c.Request.Context(), workspaceRef, c.Param("name"))
		if err != nil {
			h.respondError(c, statusForAgentWorkspaceError(err), err)
			return
		}
		c.JSON(http.StatusOK, contract.AgentResponse{Agent: AgentPayloadFromEntryWithConfig(entry, &cfg)})
		return
	}

	if h.AgentCatalog != nil {
		entry, err := h.AgentCatalog.GetAgent(c.Request.Context(), c.Param("name"))
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, os.ErrNotExist) {
				status = http.StatusNotFound
			}
			h.respondError(c, status, err)
			return
		}
		c.JSON(http.StatusOK, contract.AgentResponse{Agent: AgentPayloadFromEntryWithConfig(entry, &h.Config)})
		return
	}

	agent, err := h.AgentLoader(c.Param("name"), h.HomePaths)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, os.ErrNotExist) {
			status = http.StatusNotFound
		}
		h.respondError(c, status, err)
		return
	}

	c.JSON(http.StatusOK, contract.AgentResponse{
		Agent: AgentPayloadFromEntryWithConfig(h.agentCatalogEntryFromDef(agent, ""), &h.Config),
	})
}
