package core

import (
	"cmp"
	"errors"
	"slices"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/gin-gonic/gin"
)

func instanceMainIDs(instances []contract.AgentInstancePayload) []string {
	ids := make([]string, 0, len(instances))
	for _, instance := range instances {
		if instance.MainSessionID != "" {
			ids = append(ids, instance.MainSessionID)
		}
	}
	return ids
}

func (h *BaseHandlers) instanceFacetRows(c *gin.Context, query session.ListQuery) ([]contract.AgentInstancePayload, error) {
	if h.AgentInstances == nil {
		return nil, nil
	}
	profiles := []string{query.ReadScope.ProfileID}
	if query.ReadScope.AllProfiles {
		owners, err := h.profileOwnerIdentities(c.Request.Context())
		if err != nil {
			return nil, err
		}
		profiles = nil
		for id := range owners {
			profiles = append(profiles, id)
		}
	}
	slices.Sort(profiles)
	actor, ok := h.instanceActor(c)
	if !ok {
		return nil, agentinstance.ErrForbidden
	}
	var result []contract.AgentInstancePayload
	for _, profile := range profiles {
		list := agentinstance.ListQuery{ProfileID: profile, AllWorkspaces: true, Limit: 100}
		for {
			rows, err := h.AgentInstances.Store.List(c.Request.Context(), list)
			if err != nil {
				return nil, err
			}
			for _, row := range rows {
				workspace := query.WorkspaceID
				if workspace == "@user" {
					workspace = ""
				}
				if !query.AllWorkspaces && row.WorkspaceID != workspace {
					continue
				}
				if len(query.WorkspaceIDs) > 0 && !slices.Contains(query.WorkspaceIDs, row.WorkspaceID) {
					continue
				}
				if query.AgentName != "" && row.AgentName != query.AgentName {
					continue
				}
				if err := h.AgentInstances.Authority.Authorize(c.Request.Context(), actor, row, "list"); err != nil {
					if errors.Is(err, agentinstance.ErrForbidden) {
						continue
					}
					return nil, err
				}
				payload, err := h.AgentInstances.Payload(c.Request.Context(), row)
				if err != nil {
					return nil, err
				}
				result = append(result, payload)
			}
			if len(rows) < list.Limit {
				break
			}
			list.AfterID = rows[len(rows)-1].ID
		}
	}
	return result, nil
}

func addInstanceFacets(response *contract.SessionCatalogFacetsResponse, rows []contract.AgentInstancePayload) {
	for _, row := range rows {
		if !row.NeedsAttention {
			continue
		}
		response.Facets.InstancesNeedingAttention++
		workspace := slices.IndexFunc(response.ByWorkspace, func(item contract.SessionWorkspaceFacetsPayload) bool { return item.WorkspaceID == row.WorkspaceID })
		if workspace < 0 {
			response.ByWorkspace = append(response.ByWorkspace, contract.SessionWorkspaceFacetsPayload{WorkspaceID: row.WorkspaceID})
			workspace = len(response.ByWorkspace) - 1
		}
		response.ByWorkspace[workspace].Facets.InstancesNeedingAttention++
		profile := slices.IndexFunc(response.ByProfile, func(item contract.SessionProfileFacetsPayload) bool { return item.ProfileID == row.ProfileID })
		if profile < 0 {
			response.ByProfile = append(response.ByProfile, contract.SessionProfileFacetsPayload{ProfileID: row.ProfileID})
			profile = len(response.ByProfile) - 1
		}
		response.ByProfile[profile].Facets.InstancesNeedingAttention++
	}
	slices.SortFunc(response.ByWorkspace, func(a, b contract.SessionWorkspaceFacetsPayload) int {
		return cmp.Compare(a.WorkspaceID, b.WorkspaceID)
	})
	slices.SortFunc(response.ByProfile, func(a, b contract.SessionProfileFacetsPayload) int { return cmp.Compare(a.ProfileID, b.ProfileID) })

}
