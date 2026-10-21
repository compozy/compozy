package daemon

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/resources"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

const maxLoopCatalogBatchEntries = 2000

func (s *daemonLoopAPIService) listLoopCatalogBatch(ctx context.Context, query looppkg.CatalogQuery) (contract.LoopsResponse, error) {
	if query.Cursor != "" {
		return contract.LoopsResponse{}, fmt.Errorf("%w: aggregate definitions do not support cursors", looppkg.ErrCatalogQueryInvalid)
	}
	if s.catalog == nil || s.catalogRuns == nil {
		return contract.LoopsResponse{}, looppkg.ErrCatalogUnavailable
	}
	lister, ok := s.workspaceResolver.(interface {
		List(context.Context) ([]workspacepkg.Workspace, error)
	})
	if !ok {
		return contract.LoopsResponse{}, workspacepkg.ErrWorkspaceResolverUnavailable
	}
	workspaces, err := lister.List(ctx)
	if err != nil {
		return contract.LoopsResponse{}, err
	}
	slices.SortFunc(workspaces, func(a, b workspacepkg.Workspace) int { return strings.Compare(a.ID, b.ID) })
	lens, err := resolveLoopResourceLens(ctx, s.profiles, "", query.ProfileID)
	if err != nil {
		return contract.LoopsResponse{}, err
	}
	query.ProfileID, query.ProfileName = lens.ProfileID, lens.ProfileName
	if query.Limit == 0 {
		query.Limit = looppkg.MaxCatalogLimit
	}
	query, err = looppkg.NormalizeCatalogQuery(query)
	if err != nil {
		return contract.LoopsResponse{}, err
	}
	batch := contract.LoopCatalogBatchResponse{Entries: []contract.LoopCatalogAggregateEntryPayload{}, Errors: []contract.LoopCatalogWorkspaceError{}}
	snapshot := s.catalog.Snapshot()
	ids := []string{""}
	for _, workspace := range workspaces {
		ids = append(ids, workspace.ID)
	}
	for _, ws := range ids {
		selector := ws
		if selector == "" {
			selector = "@user"
		}
		if len(query.WorkspaceIDs) > 0 && !slices.Contains(query.WorkspaceIDs, selector) {
			continue
		}
		workspaceQuery := query
		if len(batch.Entries) == maxLoopCatalogBatchEntries {
			workspaceQuery.Limit = 1
		}
		rows, truncated, err := s.loopCatalogBatchWorkspace(ctx, ws, snapshot, workspaceQuery)
		if err != nil {
			batch.Errors = append(batch.Errors, contract.LoopCatalogWorkspaceError{WorkspaceID: ws, Code: "loop_config_invalid", Message: err.Error()})
			continue
		}
		available := maxLoopCatalogBatchEntries - len(batch.Entries)
		batch.Truncated = batch.Truncated || truncated || len(rows) > available
		batch.Entries = append(batch.Entries, rows[:min(len(rows), available)]...)
		if len(rows) > available {
			break
		}
	}
	return contract.LoopsResponse{Aggregate: &batch}, nil
}

func (s *daemonLoopAPIService) loopCatalogBatchWorkspace(ctx context.Context, ws string, snapshot []resources.Record[looppkg.ResourceSpec], query looppkg.CatalogQuery) ([]contract.LoopCatalogAggregateEntryPayload, bool, error) {
	if err := s.validateLoopCatalogWorkspace(ctx, looppkg.WorkspaceID(ws)); err != nil {
		return nil, false, err
	}
	records := looppkg.ResolveEffectiveResources(snapshot, looppkg.ResourceLens{WorkspaceID: ws, ProfileID: query.ProfileID, ProfileName: query.ProfileName})
	names := make([]string, 0, len(records))
	for _, record := range records {
		names = append(names, record.Spec.Name)
	}
	summaries, err := s.catalogRuns.ListLoopCatalogRunSummaries(ctx, looppkg.CatalogRunQuery{WorkspaceID: looppkg.WorkspaceID(ws), ProfileID: query.ProfileID, LoopNames: names, AggregateAfter: s.now().Add(-loopCatalogAggregateWindow)})
	if err != nil {
		return nil, false, err
	}
	eligible := make([]resources.Record[looppkg.ResourceSpec], 0, len(records))
	for _, record := range records {
		project := record.Scope.Kind == resources.ResourceScopeKindWorkspace || record.Scope.Kind == resources.ResourceScopeKindWorkspaceProfile
		if ws == "" || project || summaries[record.Spec.Name].Aggregate30d.Runs > 0 {
			eligible = append(eligible, record)
		}
	}
	query.WorkspaceID = ws
	page, err := looppkg.BuildCatalogPage(eligible, summaries, query)
	if err != nil {
		return nil, false, err
	}
	entries := make([]contract.LoopCatalogAggregateEntryPayload, 0, len(page.Items))
	for _, item := range page.Items {
		definition, err := daemonLoopDefinitionFromSpec(item.Record.Spec)
		if err != nil {
			return nil, false, err
		}
		payload, err := loopCatalogEntryPayload(item.Record.Spec, definition, item.Summary)
		if err != nil {
			return nil, false, err
		}
		payload.WorkspaceID = ws
		entries = append(entries, contract.LoopCatalogAggregateEntryPayload{LoopCatalogEntryPayload: payload, Source: contract.LoopCatalogSourcePayload{Kind: payload.Source, Layer: string(item.Record.Scope.Kind)}})
	}
	return entries, page.HasMore, nil
}
