package core

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/listcursor"
	"github.com/compozy/agh/internal/session"
	"github.com/gin-gonic/gin"
)

const (
	agentCatalogDefaultLimit  = 50
	agentCatalogMaxLimit      = 100
	agentCatalogCursorKind    = "agent_catalog"
	agentCatalogCursorVersion = 1
)

var errAgentCatalogQueryInvalid = errors.New("api: agent catalog query is invalid")

type agentCatalogQuery struct {
	Workspace string
	Name      string
	Search    string
	Category  string
	Status    string
	Cursor    string
	Limit     int
}

type agentCatalogFingerprint struct {
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
	Search      string `json:"q"`
	Category    string `json:"category"`
	Status      string `json:"status"`
}

type agentCatalogCursor struct {
	Name string `json:"name"`
}

// ListAgentCatalog returns one filtered, counted workspace fleet page with
// exact per-agent session metrics when the session authority is available.
func (h *BaseHandlers) ListAgentCatalog(c *gin.Context) {
	query, err := parseAgentCatalogQuery(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	entries, workspaceID, cfg, diagnostics, err := h.workspaceAgentEntriesWithDiagnostics(
		c.Request.Context(),
		query.Workspace,
	)
	if err != nil {
		h.respondError(c, statusForAgentWorkspaceError(err), err)
		return
	}

	agents := make([]contract.AgentPayload, 0, len(entries)+len(diagnostics))
	for _, entry := range entries {
		agents = append(agents, AgentPayloadFromEntryWithConfig(entry, &cfg))
	}
	for _, diagnostic := range diagnostics {
		agents = append(agents, AgentPayloadFromDiagnostic(diagnostic, workspaceID))
	}
	slices.SortFunc(agents, func(left, right contract.AgentPayload) int {
		return compareAgentCatalogNames(left.Name, right.Name)
	})

	metrics, sessionsAvailable := h.agentCatalogSessionMetrics(c, workspaceID)
	response, err := buildAgentCatalogResponse(query, workspaceID, agents, metrics, sessionsAvailable)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (h *BaseHandlers) agentCatalogSessionMetrics(
	c *gin.Context,
	workspaceID string,
) (map[string]session.AgentSessionMetrics, bool) {
	reader, ok := h.Sessions.(AgentSessionMetricsReader)
	if !ok || reader == nil {
		return nil, false
	}
	metrics, err := reader.AggregateSessionsByAgent(c.Request.Context(), workspaceID)
	if err != nil {
		h.Logger.Warn(
			"api: agent catalog session metrics unavailable",
			"workspace_id", workspaceID,
			handlersErrorKey, err,
		)
		return nil, false
	}
	return metrics, true
}

func parseAgentCatalogQuery(c *gin.Context) (agentCatalogQuery, error) {
	limit, err := ParseOptionalInt(c.Query("limit"))
	if err != nil {
		return agentCatalogQuery{}, fmt.Errorf("%w: %w", errAgentCatalogQueryInvalid, err)
	}
	if limit == 0 {
		limit = agentCatalogDefaultLimit
	}
	if limit < 1 || limit > agentCatalogMaxLimit {
		return agentCatalogQuery{}, fmt.Errorf(
			"%w: limit must be between 1 and %d",
			errAgentCatalogQueryInvalid,
			agentCatalogMaxLimit,
		)
	}
	workspace := strings.TrimSpace(c.Query("workspace"))
	if workspace == "" {
		return agentCatalogQuery{}, fmt.Errorf("%w: workspace is required", errAgentCatalogQueryInvalid)
	}
	status := strings.ToLower(strings.TrimSpace(c.Query("status")))
	if status != "" && status != "active" && status != "idle" {
		return agentCatalogQuery{}, fmt.Errorf(
			"%w: status must be active or idle",
			errAgentCatalogQueryInvalid,
		)
	}
	return agentCatalogQuery{
		Workspace: workspace,
		Name:      strings.TrimSpace(c.Query("name")),
		Search:    strings.ToLower(strings.TrimSpace(c.Query("q"))),
		Category:  strings.TrimSpace(c.Query("category")),
		Status:    status,
		Cursor:    strings.TrimSpace(c.Query("cursor")),
		Limit:     limit,
	}, nil
}

func buildAgentCatalogResponse(
	query agentCatalogQuery,
	workspaceID string,
	agents []contract.AgentPayload,
	metrics map[string]session.AgentSessionMetrics,
	sessionsAvailable bool,
) (contract.AgentCatalogResponse, error) {
	fingerprint, err := listcursor.Fingerprint(agentCatalogFingerprint{
		WorkspaceID: workspaceID,
		Name:        query.Name,
		Search:      query.Search,
		Category:    query.Category,
		Status:      query.Status,
	})
	if err != nil {
		return contract.AgentCatalogResponse{}, fmt.Errorf("api: fingerprint agent catalog: %w", err)
	}
	after, err := decodeAgentCatalogCursor(query.Cursor, fingerprint)
	if err != nil {
		return contract.AgentCatalogResponse{}, err
	}

	facets, filtered := filterAgentCatalog(agents, metrics, sessionsAvailable, query)
	response := contract.AgentCatalogResponse{
		Agents:            []contract.AgentCatalogItemPayload{},
		SessionsAvailable: sessionsAvailable,
		Facets:            facets,
		Page:              contract.CountedCursorPagePayload{Limit: query.Limit},
	}
	response.Page.Total = len(filtered)

	start := 0
	if after != "" {
		start = len(filtered)
		for index, agent := range filtered {
			if compareAgentCatalogNames(agent.Name, after) > 0 {
				start = index
				break
			}
		}
	}
	end := min(start+query.Limit, len(filtered))
	for _, agent := range filtered[start:end] {
		item := contract.AgentCatalogItemPayload{Agent: agent}
		if sessionsAvailable {
			metric := metrics[agent.Name]
			item.Sessions = &contract.AgentSessionMetricsPayload{
				Total:          metric.Total,
				Active:         metric.Active,
				Failed:         metric.Failed,
				RuntimeSeconds: metric.RuntimeSeconds,
			}
			if !metric.LastActivityAt.IsZero() {
				lastActivityAt := metric.LastActivityAt.UTC()
				item.Sessions.LastActivityAt = &lastActivityAt
			}
		}
		response.Agents = append(response.Agents, item)
	}
	response.Page.HasMore = end < len(filtered)
	if response.Page.HasMore && len(response.Agents) > 0 {
		last := response.Agents[len(response.Agents)-1].Agent.Name
		response.Page.NextCursor, err = listcursor.Encode(
			agentCatalogCursorVersion,
			agentCatalogCursorKind,
			fingerprint,
			agentCatalogCursor{Name: last},
		)
		if err != nil {
			return contract.AgentCatalogResponse{}, fmt.Errorf("api: encode agent catalog cursor: %w", err)
		}
	}
	return response, nil
}

func filterAgentCatalog(
	agents []contract.AgentPayload,
	metrics map[string]session.AgentSessionMetrics,
	sessionsAvailable bool,
	query agentCatalogQuery,
) (contract.AgentCatalogFacetsPayload, []contract.AgentPayload) {
	facets := contract.AgentCatalogFacetsPayload{
		Categories: []string{},
		Total:      len(agents),
	}
	categorySet := make(map[string]struct{})
	filtered := make([]contract.AgentPayload, 0, len(agents))
	for _, agent := range agents {
		category := strings.Join(agent.CategoryPath, " / ")
		if category != "" {
			categorySet[category] = struct{}{}
		}
		metric := metrics[agent.Name]
		if sessionsAvailable {
			if metric.Active > 0 {
				facets.Active++
			} else {
				facets.Idle++
			}
		}
		if !agentMatchesCatalogQuery(agent, metric, sessionsAvailable, query) {
			continue
		}
		filtered = append(filtered, agent)
	}
	for category := range categorySet {
		facets.Categories = append(facets.Categories, category)
	}
	slices.SortFunc(facets.Categories, compareAgentCatalogNames)
	return facets, filtered
}

func agentMatchesCatalogQuery(
	agent contract.AgentPayload,
	metric session.AgentSessionMetrics,
	sessionsAvailable bool,
	query agentCatalogQuery,
) bool {
	if query.Name != "" && agent.Name != query.Name {
		return false
	}
	category := strings.Join(agent.CategoryPath, " / ")
	if query.Category != "" && category != query.Category {
		return false
	}
	if query.Search != "" {
		matched := strings.Contains(strings.ToLower(agent.Name), query.Search) ||
			strings.Contains(strings.ToLower(category), query.Search)
		if !matched {
			return false
		}
	}
	if sessionsAvailable && query.Status == "active" && metric.Active == 0 {
		return false
	}
	if sessionsAvailable && query.Status == "idle" && metric.Active > 0 {
		return false
	}
	return true
}

func decodeAgentCatalogCursor(raw string, fingerprint string) (string, error) {
	if raw == "" {
		return "", nil
	}
	position, err := listcursor.Decode[agentCatalogCursor](
		raw,
		agentCatalogCursorVersion,
		agentCatalogCursorKind,
		fingerprint,
		listcursor.DefaultMaxEncodedSize,
	)
	if err != nil || strings.TrimSpace(position.Name) == "" {
		if err == nil {
			err = errors.New("cursor name is required")
		}
		return "", fmt.Errorf("%w: %w", errAgentCatalogQueryInvalid, err)
	}
	return strings.TrimSpace(position.Name), nil
}

func compareAgentCatalogNames(left string, right string) int {
	leftLower := strings.ToLower(strings.TrimSpace(left))
	rightLower := strings.ToLower(strings.TrimSpace(right))
	if leftLower == rightLower {
		return strings.Compare(strings.TrimSpace(left), strings.TrimSpace(right))
	}
	return strings.Compare(leftLower, rightLower)
}
