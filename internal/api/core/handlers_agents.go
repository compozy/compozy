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
	aghconfig "github.com/compozy/agh/internal/config"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) createAgentDraftAndPath(
	ctx context.Context,
	req contract.CreateAgentRequest,
) (aghconfig.AgentDefinitionDraft, string, string, aghconfig.Config, error) {
	draft, err := createAgentDraftFromRequest(req)
	if err != nil {
		return aghconfig.AgentDefinitionDraft{}, "", "", aghconfig.Config{}, err
	}

	target, err := createAgentDefinitionTargetFor(
		ctx, req, h.HomePaths, &h.Config, h.Workspaces, h.transportName(),
	)
	if err != nil {
		return aghconfig.AgentDefinitionDraft{}, "", "", aghconfig.Config{}, err
	}
	if err := validateAgentDraftRuntime(draft, &target.Config); err != nil {
		return aghconfig.AgentDefinitionDraft{}, "", "", aghconfig.Config{}, err
	}
	return draft, target.Path, target.WorkspaceID, target.Config, nil
}

func (h *BaseHandlers) createAgentDefinitionPath(
	ctx context.Context,
	req contract.CreateAgentRequest,
) (string, error) {
	return createAgentDefinitionPathFor(ctx, req, h.HomePaths, &h.Config, h.Workspaces, h.transportName())
}

func (h *BaseHandlers) workspaceAgentEntriesWithDiagnostics(
	ctx context.Context,
	workspaceRef string,
) ([]AgentCatalogEntry, string, aghconfig.Config, []workspacepkg.AgentDiagnostic, error) {
	if h.Workspaces == nil {
		return nil, "", aghconfig.Config{}, nil,
			fmt.Errorf("api: %w", workspacepkg.ErrWorkspaceResolverUnavailable)
	}
	resolved, err := h.Workspaces.Resolve(ctx, workspaceRef)
	if err != nil {
		return nil, "", aghconfig.Config{}, nil, err
	}
	entries, err := h.workspaceDetailAgentEntries(ctx, &resolved)
	if err != nil {
		return nil, "", aghconfig.Config{}, nil, err
	}
	return entries,
		strings.TrimSpace(resolved.ID),
		resolved.Config,
		publicAgentDiagnostics(resolved.AgentDiagnostics),
		nil
}

func publicAgentDiagnostics(diagnostics []workspacepkg.AgentDiagnostic) []workspacepkg.AgentDiagnostic {
	visible := make([]workspacepkg.AgentDiagnostic, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		if aghconfig.IsReservedAgentName(diagnostic.Name) {
			continue
		}
		visible = append(visible, diagnostic)
	}
	return visible
}

func (h *BaseHandlers) workspaceAgentDef(
	ctx context.Context,
	workspaceRef string,
	name string,
) (AgentCatalogEntry, aghconfig.Config, error) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return AgentCatalogEntry{}, aghconfig.Config{},
			fmt.Errorf("api: agent name is required: %w", os.ErrNotExist)
	}

	entries, workspaceID, cfg, _, err := h.workspaceAgentEntriesWithDiagnostics(ctx, workspaceRef)
	if err != nil {
		return AgentCatalogEntry{}, aghconfig.Config{}, err
	}
	for _, entry := range entries {
		if strings.TrimSpace(entry.Def.Name) == trimmedName {
			if entry.Origin == contract.AgentOriginWorkspace {
				entry.WorkspaceID = workspaceID
			}
			return entry, cfg, nil
		}
	}
	return AgentCatalogEntry{}, aghconfig.Config{}, fmt.Errorf(
		"api: agent %q is not available in workspace %q: %w",
		trimmedName,
		strings.TrimSpace(workspaceRef),
		workspacepkg.ErrAgentNotAvailable,
	)
}

func (h *BaseHandlers) respondAgentDefs(
	c *gin.Context,
	agentDefs []aghconfig.AgentDef,
	cfg *aghconfig.Config,
	workspaceID string,
	diagnostics ...[]workspacepkg.AgentDiagnostic,
) {
	entries := make([]AgentCatalogEntry, 0, len(agentDefs))
	for _, agent := range agentDefs {
		entries = append(entries, h.agentCatalogEntryFromDef(agent, workspaceID))
	}
	h.respondAgentEntries(c, entries, cfg, workspaceID, diagnostics...)
}

func (h *BaseHandlers) respondAgentEntries(
	c *gin.Context,
	entries []AgentCatalogEntry,
	cfg *aghconfig.Config,
	diagnosticWorkspaceID string,
	diagnostics ...[]workspacepkg.AgentDiagnostic,
) {
	diagnosticCount := 0
	for _, group := range diagnostics {
		diagnosticCount += len(group)
	}
	agents := make([]contract.AgentPayload, 0, len(entries)+diagnosticCount)
	for _, entry := range entries {
		agents = append(agents, AgentPayloadFromEntryWithConfig(entry, cfg))
	}
	for _, group := range diagnostics {
		for _, diagnostic := range group {
			agents = append(agents, AgentPayloadFromDiagnostic(diagnostic, diagnosticWorkspaceID))
		}
	}
	sort.Slice(agents, func(i, j int) bool {
		return agents[i].Name < agents[j].Name
	})
	c.JSON(http.StatusOK, contract.AgentsResponse{Agents: agents})
}

func (h *BaseHandlers) agentCatalogEntryFromDef(
	agent aghconfig.AgentDef,
	workspaceID string,
) AgentCatalogEntry {
	return AgentCatalogEntryFromDef(h.HomePaths, agent, strings.TrimSpace(workspaceID))
}

func statusForAgentWorkspaceError(err error) int {
	switch {
	case errors.Is(err, workspacepkg.ErrAgentNotAvailable), errors.Is(err, os.ErrNotExist):
		return http.StatusNotFound
	default:
		return StatusForWorkspaceError(err)
	}
}

func statusForCreateAgentError(err error) int {
	switch {
	case errors.Is(err, aghconfig.ErrAgentNameReserved):
		return http.StatusUnprocessableEntity
	case errors.Is(err, errCreateAgentRequestInvalid),
		errors.Is(err, aghconfig.ErrInvalidAgentDefinition):
		return http.StatusBadRequest
	case errors.Is(err, aghconfig.ErrAgentDefinitionExists):
		return http.StatusConflict
	case errors.Is(err, workspacepkg.ErrWorkspaceNotFound),
		errors.Is(err, workspacepkg.ErrWorkspaceRootMissing),
		errors.Is(err, workspacepkg.ErrWorkspaceNameTaken),
		errors.Is(err, workspacepkg.ErrWorkspacePathTaken),
		errors.Is(err, workspacepkg.ErrWorkspaceHasSessions),
		errors.Is(err, workspacepkg.ErrWorkspaceResolverUnavailable):
		return StatusForWorkspaceError(err)
	default:
		return http.StatusInternalServerError
	}
}
