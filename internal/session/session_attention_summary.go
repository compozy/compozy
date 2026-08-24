package session

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

// AttentionSummary derives exact counts with the same badge authority used by row payloads.
func (m *Manager) AttentionSummary(
	ctx context.Context,
	readScope store.ReadScope,
) (store.SessionAttentionSummary, error) {
	if m == nil {
		return store.SessionAttentionSummary{}, errors.New("session: manager is required")
	}
	if ctx == nil {
		return store.SessionAttentionSummary{}, errors.New("session: attention summary context is required")
	}
	pager, ok := m.sessionCatalog.(store.SessionCatalogPager)
	if !ok || pager == nil {
		return store.SessionAttentionSummary{}, errors.New("session: paged session catalog is required")
	}
	query, err := normalizeListQuery(ListQuery{
		ReadScope:     readScope,
		AllWorkspaces: true,
		Archive:       ArchiveExclude,
		Sort:          ListSortAttention,
		Limit:         MaxListLimit,
	})
	if err != nil {
		return store.SessionAttentionSummary{}, err
	}
	activeByID, activeIDs, activeMatches := m.activeSessionCatalogRows(query)
	durable, err := m.scanAttentionCatalog(ctx, query, pager, activeIDs)
	if err != nil {
		return store.SessionAttentionSummary{}, err
	}
	infos := projectSessionCatalogPage(append(durable, activeMatches...), activeByID)
	return summarizeAttentionInfos(infos), nil
}

func summarizeAttentionInfos(infos []*Info) store.SessionAttentionSummary {
	byWorkspace := make(map[string]*store.WorkspaceAttentionSummary)
	for _, info := range infos {
		if info == nil {
			continue
		}
		class := ClassForBadge(BadgeForInfo(info))
		if class == AttentionNone {
			continue
		}
		workspaceID := strings.TrimSpace(info.WorkspaceID)
		workspace := byWorkspace[workspaceID]
		if workspace == nil {
			workspace = &store.WorkspaceAttentionSummary{WorkspaceID: workspaceID}
			byWorkspace[workspaceID] = workspace
		}
		switch class {
		case AttentionNeedsYou:
			workspace.NeedsYou++
		case AttentionFinished:
			workspace.Finished++
		}
	}
	workspaceIDs := make([]string, 0, len(byWorkspace))
	for workspaceID := range byWorkspace {
		workspaceIDs = append(workspaceIDs, workspaceID)
	}
	sort.Strings(workspaceIDs)
	summary := store.SessionAttentionSummary{
		ByWorkspace: make([]store.WorkspaceAttentionSummary, 0, len(workspaceIDs)),
	}
	for _, workspaceID := range workspaceIDs {
		workspace := *byWorkspace[workspaceID]
		summary.NeedsYou += workspace.NeedsYou
		summary.Finished += workspace.Finished
		summary.ByWorkspace = append(summary.ByWorkspace, workspace)
	}
	return summary
}
