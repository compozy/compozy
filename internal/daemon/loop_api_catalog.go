package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	looppkg "github.com/compozy/agh/internal/loop"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

const loopCatalogAggregateWindow = 30 * 24 * time.Hour

func (s *daemonLoopAPIService) ListLoops(
	ctx context.Context,
	workspaceID string,
	query looppkg.CatalogQuery,
) (contract.LoopsResponse, error) {
	ws, err := normalizeLoopWorkspaceID(workspaceID)
	if err != nil {
		return contract.LoopsResponse{}, err
	}
	query.WorkspaceID = string(ws)
	normalized, err := looppkg.NormalizeCatalogQuery(query)
	if err != nil {
		return contract.LoopsResponse{}, err
	}
	if err := s.validateLoopCatalogWorkspace(ctx, ws); err != nil {
		return contract.LoopsResponse{}, err
	}
	if s == nil || s.catalog == nil || s.catalogRuns == nil {
		return contract.LoopsResponse{}, looppkg.ErrCatalogUnavailable
	}
	records := looppkg.ResolveEffectiveResources(s.catalog.Snapshot(), string(ws))
	names := make([]string, 0, len(records))
	for _, record := range records {
		names = append(names, record.Spec.Name)
	}
	summaries, err := s.catalogRuns.ListLoopCatalogRunSummaries(ctx, looppkg.CatalogRunQuery{
		WorkspaceID:    ws,
		LoopNames:      names,
		AggregateAfter: s.now().Add(-loopCatalogAggregateWindow),
	})
	if err != nil {
		return contract.LoopsResponse{}, err
	}
	page, err := looppkg.BuildCatalogPage(records, summaries, normalized)
	if err != nil {
		return contract.LoopsResponse{}, err
	}
	loops := make([]contract.LoopCatalogEntryPayload, 0, len(page.Items))
	for _, item := range page.Items {
		definition, err := daemonLoopDefinitionFromSpec(item.Record.Spec)
		if err != nil {
			return contract.LoopsResponse{}, err
		}
		payload, err := loopCatalogEntryPayload(item.Record.Spec, definition, item.Summary)
		if err != nil {
			return contract.LoopsResponse{}, err
		}
		loops = append(loops, payload)
	}
	return loopCatalogResponse(loops, page), nil
}

func (s *daemonLoopAPIService) validateLoopCatalogWorkspace(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
) error {
	if s == nil || s.workspaceResolver == nil {
		return workspacepkg.ErrWorkspaceResolverUnavailable
	}
	workspace, err := s.workspaceResolver.Get(ctx, string(workspaceID))
	if err != nil {
		return err
	}
	root := strings.TrimSpace(workspace.RootDir)
	if root == "" {
		return workspacepkg.ErrWorkspaceRootMissing
	}
	info, err := os.Stat(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return workspacepkg.ErrWorkspaceRootMissing
		}
		return fmt.Errorf("daemon: stat loop catalog workspace root %q: %w", root, err)
	}
	if !info.IsDir() {
		return workspacepkg.ErrWorkspaceRootMissing
	}
	return nil
}

func loopCatalogResponse(
	loops []contract.LoopCatalogEntryPayload,
	page looppkg.CatalogPage,
) contract.LoopsResponse {
	kinds := make(map[string]int, len(page.Facets.Kinds))
	for kind, count := range page.Facets.Kinds {
		kinds[string(kind)] = count
	}
	statuses := make(map[string]int, len(page.Facets.Statuses))
	for status, count := range page.Facets.Statuses {
		statuses[string(status)] = count
	}
	return contract.LoopsResponse{
		Loops: loops,
		Facets: contract.LoopCatalogFacetsPayload{
			Kinds:      kinds,
			Categories: page.Facets.Categories,
			Statuses:   statuses,
		},
		Page: contract.CountedCursorPagePayload{
			NextCursor: page.NextCursor,
			HasMore:    page.HasMore,
			Total:      page.Total,
			Limit:      page.Limit,
		},
	}
}
