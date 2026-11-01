package core

import (
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
)

func groupedAgentRows(rows []contract.AgentCatalogItemPayload) []contract.AgentCatalogItemPayload {
	indexes := make(map[string]int)
	result := make([]contract.AgentCatalogItemPayload, 0, len(rows))
	for _, row := range rows {
		key := row.Agent.Name + "\x00" + row.Agent.DefinitionDigest
		scope := row.Agent.WorkspaceID
		if scope == "" {
			scope = userScopeWorkspaceRef
		}
		if i, ok := indexes[key]; ok {
			result[i].Scopes = append(result[i].Scopes, scope)
			if row.Sessions != nil && result[i].Sessions != nil {
				result[i].Sessions.Active += row.Sessions.Active
				result[i].Sessions.Total += row.Sessions.Total
				result[i].Sessions.Failed += row.Sessions.Failed
				result[i].Sessions.RuntimeSeconds += row.Sessions.RuntimeSeconds
			}
			continue
		}
		indexes[key] = len(result)
		row.Scopes = []string{scope}
		result = append(result, row)
	}
	for i := range result {
		slices.Sort(result[i].Scopes)
		result[i].Scopes = slices.Compact(result[i].Scopes)
	}
	return result
}

func groupAgentCatalogItems(rows []contract.AgentCatalogItemPayload) []contract.GroupedAgentCatalogItemPayload {
	result := make([]contract.GroupedAgentCatalogItemPayload, 0, len(rows))
	for _, row := range rows {
		scopes := row.Scopes
		if len(scopes) == 0 {
			scope := row.Agent.WorkspaceID
			if scope == "" {
				scope = userScopeWorkspaceRef
			}
			scopes = []string{scope}
		}
		item := contract.GroupedAgentCatalogItemPayload{
			Name:             row.Agent.Name,
			Label:            row.Agent.Label,
			Kind:             row.Agent.Kind,
			DefinitionDigest: row.Agent.DefinitionDigest,
			Scopes:           scopes,
			Agent:            row.Agent,
		}
		if row.Sessions != nil {
			item.Active = row.Sessions.Active
		}
		result = append(result, item)
	}
	return result
}

func filterGroupedAgentRows(
	rows []contract.AgentCatalogItemPayload,
	query agentCatalogQuery,
	available bool,
) (contract.AgentCatalogFacetsPayload, []contract.AgentCatalogItemPayload) {
	facets := contract.AgentCatalogFacetsPayload{Total: len(rows), Categories: []string{}}
	result := make([]contract.AgentCatalogItemPayload, 0, len(rows))
	for _, row := range rows {
		metric := session.AgentSessionMetrics{}
		if row.Sessions != nil {
			metric.Active = row.Sessions.Active
		}
		if available {
			if metric.Active > 0 {
				facets.Active++
			} else {
				facets.Idle++
			}
		}
		if category := strings.Join(row.Agent.CategoryPath, " / "); category != "" {
			facets.Categories = append(facets.Categories, category)
		}
		if agentMatchesCatalogQuery(row.Agent, metric, available, query) {
			result = append(result, row)
		}
	}
	slices.Sort(facets.Categories)
	facets.Categories = slices.Compact(facets.Categories)
	return facets, result
}

func agentCatalogWorkspaceQuery(query agentCatalogQuery) agentCatalogQuery {
	if query.Group == agentCatalogGroupKey {
		query.Name = ""
		query.Search = ""
		query.Category = ""
		query.Status = ""
	}
	return query
}

func groupAgentCatalogResponse(response *contract.AgentCatalogResponse, query agentCatalogQuery) {
	if query.Group != agentCatalogGroupKey {
		return
	}
	response.Agents = groupedAgentRows(response.Agents)
	response.Facets, response.Agents = filterGroupedAgentRows(response.Agents, query, response.SessionsAvailable)
}

func projectGroupedAgentCatalog(response *contract.AgentCatalogResponse, query agentCatalogQuery) {
	if query.Group == agentCatalogGroupKey {
		response.Items = groupAgentCatalogItems(response.Agents)
		response.NextCursor = response.Page.NextCursor
	}
}

const agentCatalogGroupKey = "agent"
