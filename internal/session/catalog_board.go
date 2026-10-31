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
	active := make(map[string]*Info)
	children := make([]*Info, 0)
	for _, candidate := range m.activeSessionFamily(info) {
		if !catalogRelationVisible(candidate, query) {
			continue
		}
		if info.Lineage != nil && candidate.ID == info.Lineage.ParentSessionID {
			active[candidate.ID] = candidate
		}
		if store.HierarchyParentSessionID(candidate.Lineage) == info.ID {
			active[candidate.ID] = candidate
			children = append(children, candidate)
		}
	}
	views, err := m.SessionPullRequests(ctx, children)
	if err != nil {
		return err
	}
	for _, child := range children {
		var state PullRequestState
		if pr := views[child.ID].PullRequest; pr != nil {
			state = pr.State
		}
		child.BoardColumn = BoardColumnFor(BadgeForInfo(child), state)
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

func (m *Manager) activeSessionFamily(info *Info) []*Info {
	parentID := ""
	if info.Lineage != nil {
		parentID = info.Lineage.ParentSessionID
	}
	m.mu.RLock()
	sessions := make([]*Session, 0, len(m.sessions))
	for _, candidate := range m.sessions {
		sessions = append(sessions, candidate)
	}
	m.mu.RUnlock()
	family := make([]*Info, 0)
	for _, candidate := range sessions {
		candidate.mu.RLock()
		related := parentID != "" && candidate.ID == parentID ||
			candidate.Lineage != nil && candidate.Lineage.ParentSessionID == info.ID
		candidate.mu.RUnlock()
		if related {
			family = append(family, m.sessionInfoForRead(candidate))
		}
	}
	return family
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
