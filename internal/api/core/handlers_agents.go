package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/gin-gonic/gin"
)

type workspaceAgentEntries struct {
	Entries       []AgentCatalogEntry
	WorkspaceID   string
	WorkspaceRoot string
	Config        compozyconfig.Config
	Diagnostics   []workspacepkg.AgentDiagnostic
}

func (h *BaseHandlers) createAgentDraftAndPath(
	ctx context.Context,
	req contract.CreateAgentRequest,
) (compozyconfig.AgentDefinitionDraft, string, string, compozyconfig.Config, error) {
	draft, err := createAgentDraftFromRequest(req)
	if err != nil {
		return compozyconfig.AgentDefinitionDraft{}, "", "", compozyconfig.Config{}, err
	}

	target, err := createAgentDefinitionTargetFor(
		ctx, req, h.HomePaths, &h.Config, h.Workspaces, h.transportName(),
	)
	if err != nil {
		return compozyconfig.AgentDefinitionDraft{}, "", "", compozyconfig.Config{}, err
	}
	if err := validateAgentDraftRuntime(draft, &target.Config); err != nil {
		return compozyconfig.AgentDefinitionDraft{}, "", "", compozyconfig.Config{}, err
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
) (workspaceAgentEntries, error) {
	if h.Workspaces == nil {
		return workspaceAgentEntries{},
			fmt.Errorf("api: %w", workspacepkg.ErrWorkspaceResolverUnavailable)
	}
	resolved, err := h.Workspaces.Resolve(ctx, workspaceRef)
	if err != nil {
		return workspaceAgentEntries{}, err
	}
	entries, err := h.workspaceDetailAgentEntries(ctx, &resolved)
	if err != nil {
		return workspaceAgentEntries{}, err
	}
	workspaceID := strings.TrimSpace(resolved.ID)
	for index := range entries {
		if entries[index].Origin == contract.AgentOriginWorkspace {
			entries[index].WorkspaceID = workspaceID
		}
	}
	return workspaceAgentEntries{
		Entries:       entries,
		WorkspaceID:   workspaceID,
		WorkspaceRoot: strings.TrimSpace(resolved.RootDir),
		Config:        resolved.Config,
		Diagnostics:   publicAgentDiagnostics(resolved.AgentDiagnostics),
	}, nil
}

func publicAgentDiagnostics(diagnostics []workspacepkg.AgentDiagnostic) []workspacepkg.AgentDiagnostic {
	visible := make([]workspacepkg.AgentDiagnostic, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		if compozyconfig.IsReservedAgentName(diagnostic.Name) {
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
) (AgentCatalogEntry, compozyconfig.Config, error) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return AgentCatalogEntry{}, compozyconfig.Config{},
			fmt.Errorf("api: agent name is required: %w", os.ErrNotExist)
	}

	resolved, err := h.workspaceAgentEntriesWithDiagnostics(ctx, workspaceRef)
	if err != nil {
		return AgentCatalogEntry{}, compozyconfig.Config{}, err
	}
	for _, entry := range resolved.Entries {
		if strings.TrimSpace(entry.Def.Name) == trimmedName {
			return entry, resolved.Config, nil
		}
	}
	workspaceLabel := workspaceDisplay(resolved.WorkspaceRoot, workspaceRef)
	return AgentCatalogEntry{}, compozyconfig.Config{}, fmt.Errorf(
		"api: agent %q is not available in workspace %q: %w",
		trimmedName,
		workspaceLabel,
		workspacepkg.ErrAgentNotAvailable,
	)
}

func (h *BaseHandlers) respondAgentDefs(
	c *gin.Context,
	agentDefs []compozyconfig.AgentDef,
	cfg *compozyconfig.Config,
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
	cfg *compozyconfig.Config,
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
	agent compozyconfig.AgentDef,
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
	case errors.Is(err, compozyconfig.ErrAgentNameReserved):
		return http.StatusUnprocessableEntity
	case errors.Is(err, errCreateAgentRequestInvalid),
		errors.Is(err, compozyconfig.ErrInvalidAgentDefinition):
		return http.StatusBadRequest
	case errors.Is(err, compozyconfig.ErrAgentDefinitionExists):
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
