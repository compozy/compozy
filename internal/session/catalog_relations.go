package session

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

func (m *Manager) populateCatalogRelations(
	ctx context.Context,
	filter ListQuery,
	page []*Info,
	active map[string]*Info,
	repositoryKeys map[string]string, defaultBranches map[string][]string,
) error {
	if len(page) == 0 {
		return nil
	}
	query := store.SessionCatalogRelationsQuery{
		ReadScope:      filter.ReadScope,
		WorkspaceID:    filter.WorkspaceID,
		WorkspaceIDs:   filter.WorkspaceIDs,
		RepositoryKeys: repositoryKeys, DefaultBranches: defaultBranches,
	}
	for _, info := range page {
		query.SessionIDs = append(query.SessionIDs, info.ID)
		if info.Lineage != nil && info.Lineage.ParentSessionID != "" {
			query.ParentIDs = append(query.ParentIDs, info.Lineage.ParentSessionID)
		}
	}
	for id := range active {
		query.ActiveIDs = append(query.ActiveIDs, id)
	}
	relations := store.SessionCatalogRelations{
		Children:     map[string]store.SessionBoardCounts{},
		ParentTitles: map[string]string{},
	}
	if reader, ok := m.sessionCatalog.(store.SessionCatalogRelationReader); ok {
		var err error
		relations, err = reader.SessionCatalogRelations(ctx, query)
		if err != nil {
			return fmt.Errorf("session: catalog relationships: %w", err)
		}
	}
	for _, info := range active {
		if !catalogRelationVisible(info, filter) {
			continue
		}
		relations.ParentTitles[info.ID] = sessionCatalogDisplayTitle(info)
		parentID := store.HierarchyParentSessionID(info.Lineage)
		if info.ArchivedAt != nil || parentID == "" {
			continue
		}
		counts := relations.Children[parentID]
		counts.Add(string(info.BoardColumn), 1)
		relations.Children[parentID] = counts
	}
	for _, info := range page {
		counts := relations.Children[info.ID]
		if counts.Total() > 0 {
			info.Children = &counts
		}
		if info.Lineage != nil {
			info.ParentTitle = relations.ParentTitles[info.Lineage.ParentSessionID]
		}
	}
	return nil
}

func catalogRelationVisible(info *Info, filter ListQuery) bool {
	return isPublicSessionCatalogInfo(info) && filter.ReadScope.Matches(info.ProfileID) &&
		(filter.WorkspaceID == "" || filter.WorkspaceID == info.WorkspaceID ||
			filter.WorkspaceID == "@user" && info.WorkspaceID == "") &&
		sessionMatchesWorkspaceIDs(info.WorkspaceID, filter.WorkspaceIDs)
}
