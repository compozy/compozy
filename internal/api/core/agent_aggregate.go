package core

import (
	"cmp"
	"net/http"
	"slices"

	"github.com/compozy/compozy/internal/workspaceaccess"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/listcursor"
	"github.com/compozy/compozy/internal/session"
	"github.com/gin-gonic/gin"
)

type aggregateAgentPosition struct {
	Name        string
	WorkspaceID string
}

func (h *BaseHandlers) ListAgentAggregate(c *gin.Context) {
	breadth, ok := h.aggregateBreadth(c)
	if !ok {
		return
	}
	query, err := parseAgentCatalogQuery(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	if query.Workspace != "" {
		h.respondAggregateBreadthError(c, invalidWorkspaceBreadth("workspace conflicts with aggregate agents"))
		return
	}
	scope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	profileName, err := h.agentResourceProfileNameForScope(c.Request.Context(), scope)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	allowed, err := h.aggregateWorkspaceSet(c, breadth)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	cursor := query.Cursor
	query.Cursor = ""
	limit := query.Limit
	query.Limit = 0
	fingerprint, err := listcursor.Fingerprint(struct {
		Query   agentCatalogQuery
		Breadth WorkspaceBreadth
		Profile string
	}{query, breadth, scope.ProfileID})
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	var after aggregateAgentPosition
	if cursor != "" {
		after, err = listcursor.Decode[aggregateAgentPosition](
			cursor,
			1,
			"agents_aggregate",
			fingerprint,
			listcursor.DefaultMaxEncodedSize,
		)
		if err != nil {
			h.respondError(c, http.StatusBadRequest, err)
			return
		}
	}
	response := contract.AgentCatalogResponse{
		Agents:            []contract.AgentCatalogItemPayload{},
		SessionsAvailable: true,
		Page:              contract.CountedCursorPagePayload{Limit: limit},
		Facets:            contract.AgentCatalogFacetsPayload{Categories: []string{}},
	}
	userAgents, err := h.globalAgentEntries(c.Request.Context(), scope.ProfileID, profileName)
	if err != nil {
		h.respondError(c, statusForAgentWorkspaceError(err), err)
		return
	}
	userAgentNames := make(map[string]struct{}, len(userAgents.Entries))
	for _, entry := range userAgents.Entries {
		userAgentNames[entry.Def.Name] = struct{}{}
	}
	for workspace := range allowed {
		resolved := userAgents
		if workspace != "" {
			resolved, err = h.workspaceAgentEntriesWithDiagnostics(c.Request.Context(), workspace, profileName)
			if err != nil {
				h.respondError(c, statusForAgentWorkspaceError(err), err)
				return
			}
		}
		metricsWorkspace := workspace
		if workspace == "" {
			metricsWorkspace = workspaceaccess.UserSelector
		}
		metrics, available := h.agentCatalogSessionMetrics(c, scope, metricsWorkspace)
		agents := make([]contract.AgentPayload, 0, len(resolved.Entries)+len(resolved.Diagnostics))
		for _, entry := range resolved.Entries {
			// Unavailable counts never prove zero sessions, so inherited rows stay listed then.
			if workspace != "" && available && inheritedUserAgentWithoutProjectWork(entry, userAgentNames, metrics) {
				continue
			}
			payload := AgentPayloadFromEntryWithConfig(entry, &resolved.Config)
			payload.WorkspaceID = workspace
			agents = append(agents, payload)
		}
		for _, diagnostic := range resolved.Diagnostics {
			agents = append(agents, AgentPayloadFromDiagnostic(diagnostic, workspace))
		}
		response.SessionsAvailable = response.SessionsAvailable && available
		local := query
		local.Limit = len(agents) + 1
		page, buildErr := buildAgentCatalogResponse(local, workspace, agents, metrics, available)
		if buildErr != nil {
			h.respondError(c, http.StatusInternalServerError, buildErr)
			return
		}
		response.Agents = append(response.Agents, page.Agents...)
		response.Facets.Total += page.Facets.Total
		response.Facets.Active += page.Facets.Active
		response.Facets.Idle += page.Facets.Idle
		response.Facets.Categories = append(response.Facets.Categories, page.Facets.Categories...)
	}
	slices.Sort(response.Facets.Categories)
	response.Facets.Categories = slices.Compact(response.Facets.Categories)
	slices.SortFunc(response.Agents, func(a, b contract.AgentCatalogItemPayload) int {
		return cmp.Or(
			compareAgentCatalogNames(a.Agent.Name, b.Agent.Name),
			cmp.Compare(a.Agent.WorkspaceID, b.Agent.WorkspaceID),
		)
	})
	response.Page.Total = len(response.Agents)
	start := 0
	if cursor != "" {
		start = len(response.Agents)
		for i, item := range response.Agents {
			if cmp.Or(
				compareAgentCatalogNames(item.Agent.Name, after.Name),
				cmp.Compare(item.Agent.WorkspaceID, after.WorkspaceID),
			) > 0 {
				start = i
				break
			}
		}
	}
	end := min(start+limit, len(response.Agents))
	response.Page.HasMore = end < len(response.Agents)
	response.Agents = response.Agents[start:end]
	if response.Page.HasMore {
		last := response.Agents[len(response.Agents)-1].Agent
		response.Page.NextCursor, err = listcursor.Encode(
			1,
			"agents_aggregate",
			fingerprint,
			aggregateAgentPosition{last.Name, last.WorkspaceID},
		)
		if err != nil {
			h.respondError(c, http.StatusInternalServerError, err)
			return
		}
	}
	c.JSON(http.StatusOK, response)
}

// inheritedUserAgentWithoutProjectWork reports a project catalog entry that only inherits a User
// definition and has no sessions in that project. Such an agent is listed once, as its User row;
// a project row exists only for a definition authored in the project or one with project work
// (AP Data Models "Agent rows", same rule as loop catalog rows).
func inheritedUserAgentWithoutProjectWork(
	entry AgentCatalogEntry,
	userAgentNames map[string]struct{},
	metrics map[string]session.AgentSessionMetrics,
) bool {
	if _, inherited := userAgentNames[entry.Def.Name]; !inherited {
		return false
	}
	switch entry.Def.SourceLayer {
	case "project", "project_profile", "additional":
		return false
	}
	return metrics[entry.Def.Name].Total == 0
}
