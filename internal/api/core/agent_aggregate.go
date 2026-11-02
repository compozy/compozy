package core

import (
	"cmp"
	"errors"
	"net/http"
	"slices"

	"github.com/compozy/compozy/internal/workspaceaccess"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/listcursor"
	"github.com/compozy/compozy/internal/session"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/gin-gonic/gin"
)

type aggregateAgentPosition struct {
	DefinitionDigest string `json:"DefinitionDigest,omitempty"`
	Name             string
	WorkspaceID      string
}

func (h *BaseHandlers) ListAgentAggregate(c *gin.Context) {
	breadth, ok := h.aggregateBreadth(c)
	if !ok {
		return
	}
	h.listAgentAggregate(c, breadth)
}

func (h *BaseHandlers) listAgentAggregate(c *gin.Context, breadth WorkspaceBreadth) {
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
	paging, err := newAggregateAgentPaging(&query, breadth, scope.ProfileID)
	if err != nil {
		h.respondError(c, paging.errorStatus, err)
		return
	}
	response := contract.AgentCatalogResponse{
		Agents:            []contract.AgentCatalogItemPayload{},
		SessionsAvailable: true,
		Page:              contract.CountedCursorPagePayload{Limit: paging.limit},
		Facets:            contract.AgentCatalogFacetsPayload{Categories: []string{}},
	}
	userAgents, err := h.globalAgentEntries(c.Request.Context(), scope.ProfileID, profileName)
	if err != nil {
		h.respondError(c, statusForAgentWorkspaceError(err), err)
		return
	}
	userAgentNames := aggregateUserAgentNames(userAgents.Entries)
	for workspace := range allowed {
		resolved := userAgents
		if workspace != "" {
			resolved, err = h.workspaceAgentEntriesWithDiagnostics(c.Request.Context(), workspace, profileName)
			if errors.Is(err, workspacepkg.ErrWorkspaceRootMissing) {
				response.Diagnostics = append(response.Diagnostics, MissingWorkspaceDiagnostics(workspace)...)
				response.SessionsAvailable = false
				continue
			}
			if err != nil {
				h.respondError(c, statusForAgentWorkspaceError(err), err)
				return
			}
		}
		metrics, available := h.agentCatalogSessionMetrics(c, scope, agentMetricsWorkspace(workspace))
		agents := aggregateWorkspaceAgents(
			workspace,
			&resolved,
			userAgentNames,
			metrics,
			available && query.Group != agentCatalogGroupKey,
		)
		response.SessionsAvailable = response.SessionsAvailable && available
		query.Limit = len(agents) + 1
		page, buildErr := buildAgentCatalogResponse(
			agentCatalogWorkspaceQuery(query),
			workspace,
			agents,
			metrics,
			available,
		)
		if buildErr != nil {
			h.respondError(c, http.StatusInternalServerError, buildErr)
			return
		}
		mergeAgentCatalogPage(&response, page)
	}
	h.finishAggregateAgentResponse(c, &response, query, paging)
}

func (h *BaseHandlers) finishAggregateAgentResponse(
	c *gin.Context,
	response *contract.AgentCatalogResponse,
	query agentCatalogQuery,
	paging aggregateAgentPaging,
) {
	groupAgentCatalogResponse(response, query)
	if err := paging.finish(response); err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	projectGroupedAgentCatalog(response, query)
	c.JSON(http.StatusOK, response)
}

// aggregateAgentPaging carries the aggregate agent cursor, which pages the merged sorted rows.
type aggregateAgentPaging struct {
	fingerprint string
	cursor      string
	after       aggregateAgentPosition
	limit       int
	errorStatus int
}

// newAggregateAgentPaging moves the cursor and limit off the per-workspace query and decodes the
// cursor against the query, breadth, and profile it was issued for.
func newAggregateAgentPaging(
	query *agentCatalogQuery,
	breadth WorkspaceBreadth,
	profileID string,
) (aggregateAgentPaging, error) {
	paging := aggregateAgentPaging{
		cursor:      query.Cursor,
		limit:       query.Limit,
		errorStatus: http.StatusInternalServerError,
	}
	query.Cursor, query.Limit = "", 0
	fingerprint, err := listcursor.Fingerprint(struct {
		Query   agentCatalogQuery
		Breadth WorkspaceBreadth
		Profile string
	}{*query, breadth, profileID})
	if err != nil {
		return paging, err
	}
	paging.fingerprint = fingerprint
	if paging.cursor == "" {
		return paging, nil
	}
	paging.after, err = listcursor.Decode[aggregateAgentPosition](
		paging.cursor, 1, "agents_aggregate", fingerprint, listcursor.DefaultMaxEncodedSize,
	)
	if err != nil {
		paging.errorStatus = http.StatusBadRequest
	}
	return paging, err
}

// finish sorts the merged rows and facets, then cuts the requested page and its next cursor.
func (p aggregateAgentPaging) finish(response *contract.AgentCatalogResponse) error {
	slices.SortFunc(response.Diagnostics, func(a, b contract.DiagnosticItem) int {
		return cmp.Compare(a.ID, b.ID)
	})
	slices.Sort(response.Facets.Categories)
	response.Facets.Categories = slices.Compact(response.Facets.Categories)
	slices.SortFunc(response.Agents, func(a, b contract.AgentCatalogItemPayload) int {
		return compareAggregateAgentPositions(aggregateAgentRowPosition(&a), aggregateAgentRowPosition(&b))
	})
	response.Page.Total = len(response.Agents)
	start := aggregateAgentPageStart(response.Agents, p.cursor, p.after)
	end := min(start+p.limit, len(response.Agents))
	response.Page.HasMore = end < len(response.Agents)
	response.Agents = response.Agents[start:end]
	if !response.Page.HasMore {
		return nil
	}
	last := aggregateAgentRowPosition(&response.Agents[len(response.Agents)-1])
	cursor, err := listcursor.Encode(
		1,
		"agents_aggregate",
		p.fingerprint,
		last,
	)
	response.Page.NextCursor = cursor
	return err
}

// agentMetricsWorkspace is the session-metrics selector of one aggregate row set.
func agentMetricsWorkspace(workspace string) string {
	if workspace == "" {
		return workspaceaccess.UserSelector
	}
	return workspace
}

// aggregateWorkspaceAgents lists one workspace's agents and diagnostics, bound to that workspace.
func aggregateWorkspaceAgents(
	workspace string,
	resolved *workspaceAgentEntries,
	userAgentNames map[string]struct{},
	metrics map[string]session.AgentSessionMetrics,
	available bool,
) []contract.AgentPayload {
	agents := make([]contract.AgentPayload, 0, len(resolved.Entries)+len(resolved.Diagnostics))
	for i := range resolved.Entries {
		entry := &resolved.Entries[i]
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
	return agents
}

// aggregateAgentPageStart is the first row after the cursor position in the sorted rows.
func aggregateAgentPageStart(
	agents []contract.AgentCatalogItemPayload,
	cursor string,
	after aggregateAgentPosition,
) int {
	if cursor == "" {
		return 0
	}
	for i := range agents {
		if compareAggregateAgentPositions(aggregateAgentRowPosition(&agents[i]), after) > 0 {
			return i
		}
	}
	return len(agents)
}

// inheritedUserAgentWithoutProjectWork reports a project catalog entry that only inherits a User
// definition and has no sessions in that project. Such an agent is listed once, as its User row;
// a project row exists only for a definition authored in the project or one with project work
// (AP Data Models "Agent rows", same rule as loop catalog rows).
func inheritedUserAgentWithoutProjectWork(
	entry *AgentCatalogEntry,
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

func aggregateUserAgentNames(entries []AgentCatalogEntry) map[string]struct{} {
	names := make(map[string]struct{}, len(entries))
	for index := range entries {
		names[entries[index].Def.Name] = struct{}{}
	}
	return names
}

func aggregateAgentRowPosition(row *contract.AgentCatalogItemPayload) aggregateAgentPosition {
	if len(row.Scopes) > 0 {
		return aggregateAgentPosition{Name: row.Agent.Name, DefinitionDigest: row.Agent.DefinitionDigest}
	}
	return aggregateAgentPosition{Name: row.Agent.Name, WorkspaceID: row.Agent.WorkspaceID}
}

func compareAggregateAgentPositions(a, b aggregateAgentPosition) int {
	return cmp.Or(
		compareAgentCatalogNames(a.Name, b.Name),
		cmp.Compare(a.DefinitionDigest, b.DefinitionDigest),
		cmp.Compare(a.WorkspaceID, b.WorkspaceID),
	)
}

func mergeAgentCatalogPage(response *contract.AgentCatalogResponse, page contract.AgentCatalogResponse) {
	response.Agents = append(response.Agents, page.Agents...)
	response.Facets.Total += page.Facets.Total
	response.Facets.Active += page.Facets.Active
	response.Facets.Idle += page.Facets.Idle
	response.Facets.Categories = append(response.Facets.Categories, page.Facets.Categories...)
}
