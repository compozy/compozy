package session

import (
	"context"
	"errors"

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
	facets, err := m.CatalogFacets(ctx, ListQuery{
		ReadScope:     readScope,
		AllWorkspaces: true,
		Archive:       ArchiveExclude,
	})
	if err != nil {
		return store.SessionAttentionSummary{}, err
	}
	summary := store.SessionAttentionSummary{
		NeedsYou:    facets.Facets.NeedsYou,
		Finished:    facets.Facets.Finished,
		ByWorkspace: make([]store.WorkspaceAttentionSummary, 0, len(facets.ByWorkspace)),
	}
	for _, workspace := range facets.ByWorkspace {
		if workspace.Facets.NeedsYou == 0 && workspace.Facets.Finished == 0 {
			continue
		}
		summary.ByWorkspace = append(summary.ByWorkspace, store.WorkspaceAttentionSummary{
			WorkspaceID: workspace.WorkspaceID,
			NeedsYou:    workspace.Facets.NeedsYou,
			Finished:    workspace.Facets.Finished,
		})
	}
	return summary, nil
}
