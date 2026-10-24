package session

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

func (m *Manager) catalogRepositoryKeys(ctx context.Context, query ListQuery) map[string]string {
	m.mu.RLock()
	reader, ok := m.pullRequestReader.(PullRequestRepositoryReader)
	m.mu.RUnlock()
	if !ok {
		return nil
	}
	return reader.RepositoryKeys(ctx, query.ReadScope, catalogWorkspaceIDs(query))
}

func (m *Manager) activeBoardSessionCatalogRows(
	ctx context.Context,
	query ListQuery,
) (map[string]*Info, []string, []store.SessionInfo, error) {
	infos := m.List()
	visible := make([]*Info, 0, len(infos))
	for _, info := range infos {
		if catalogRelationVisible(info, query) {
			visible = append(visible, info)
		}
	}
	views, err := m.SessionPullRequests(ctx, visible)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("session: active catalog pull requests: %w", err)
	}
	for _, info := range visible {
		var state PullRequestState
		if pr := views[info.ID].PullRequest; pr != nil {
			state = pr.State
		}
		info.BoardColumn = BoardColumnFor(BadgeForInfo(info), state)
	}
	active, ids, matches := m.activeSessionCatalogRowsFromInfos(query, infos)
	return active, ids, matches, nil
}

// PopulateSessionRelations enriches one already-authorized detail with scoped family metadata.
func (m *Manager) PopulateSessionRelations(ctx context.Context, info *Info) error {
	if info == nil {
		return nil
	}
	workspace := info.WorkspaceID
	if workspace == "" {
		workspace = workspaceaccess.UserSelector
	}
	query := ListQuery{ReadScope: store.ReadScope{ProfileID: info.ProfileID}, WorkspaceID: workspace}
	active, _, _, err := m.activeBoardSessionCatalogRows(ctx, query)
	if err != nil {
		return err
	}
	return m.populateCatalogRelations(
		ctx,
		query,
		[]*Info{info},
		active,
		m.catalogRepositoryKeys(ctx, query),
		m.catalogDefaultBranches(ctx, query),
	)
}

func catalogWorkspaceIDs(query ListQuery) []string {
	if query.WorkspaceID != "" {
		return []string{query.WorkspaceID}
	}
	return query.WorkspaceIDs
}

func (m *Manager) catalogDefaultBranches(ctx context.Context, query ListQuery) map[string][]string {
	m.mu.RLock()
	reader, ok := m.pullRequestReader.(PullRequestDefaultBranchReader)
	m.mu.RUnlock()
	if !ok {
		return nil
	}
	return reader.DefaultBranchNames(ctx, query.ReadScope, catalogWorkspaceIDs(query))
}
