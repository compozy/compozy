package globaldb

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

type modelCatalogContextQuery struct {
	contextID string
	sourceID  string
}

func upsertModelCatalogExecutionContext(
	ctx context.Context,
	exec modelCatalogSQLExecutor,
	executionContext modelcatalog.CatalogExecutionContext,
) (string, error) {
	normalized, err := modelcatalog.NormalizePersistedExecutionContext(executionContext)
	if err != nil {
		return "", err
	}
	contextID, err := normalized.ID()
	if err != nil {
		return "", err
	}
	if err := sqlcgen.New(exec).UpsertModelCatalogExecutionContext(
		ctx,
		sqlcgen.UpsertModelCatalogExecutionContextParams{
			ContextID:          contextID,
			Scope:              string(normalized.Scope),
			ProfileID:          normalized.ProfileID,
			WorkspaceID:        normalized.WorkspaceID,
			CommandFingerprint: normalized.CommandFingerprint,
		},
	); err != nil {
		return "", fmt.Errorf("store: upsert model catalog execution context: %w", err)
	}
	return contextID, nil
}

func pruneSupersededModelCatalogSourceContexts(
	ctx context.Context,
	exec modelCatalogSQLExecutor,
	executionContext modelcatalog.CatalogExecutionContext,
	contextID string,
	sourceID string,
	providerID string,
) error {
	if executionContext.Scope == modelcatalog.ExecutionScopeGlobal {
		return nil
	}
	if _, err := exec.ExecContext(
		ctx,
		`DELETE FROM model_catalog_sources
		 WHERE source_id = ?
		   AND provider_id = ?
		   AND context_id <> ?
		   AND context_id IN (
			SELECT context_id
			  FROM model_catalog_execution_contexts
			 WHERE scope = ?
			   AND profile_id = ?
			   AND workspace_id = ?
		   )`,
		strings.TrimSpace(sourceID),
		strings.TrimSpace(providerID),
		contextID,
		string(executionContext.Scope),
		executionContext.ProfileID,
		executionContext.WorkspaceID,
	); err != nil {
		return fmt.Errorf("store: prune superseded model catalog source contexts: %w", err)
	}
	return nil
}

func deleteUnusedModelCatalogExecutionContexts(
	ctx context.Context,
	exec modelCatalogSQLExecutor,
) error {
	globalID, err := modelcatalog.GlobalCatalogExecutionContext().ID()
	if err != nil {
		return err
	}
	if _, err := exec.ExecContext(
		ctx,
		`DELETE FROM model_catalog_execution_contexts
		 WHERE context_id <> ?
		   AND NOT EXISTS (
			SELECT 1
			  FROM model_catalog_sources
			 WHERE model_catalog_sources.context_id = model_catalog_execution_contexts.context_id
		   )`,
		globalID,
	); err != nil {
		return fmt.Errorf("store: delete unused model catalog execution contexts: %w", err)
	}
	return nil
}

func modelCatalogContextFilterClause(
	executionContext modelcatalog.CatalogExecutionContext,
	sourceContexts map[string]modelcatalog.CatalogExecutionContext,
	sourceColumn string,
	contextColumn string,
) (string, []any, error) {
	if len(sourceContexts) == 0 {
		if executionContext.Scope == "" {
			executionContext = modelcatalog.GlobalCatalogExecutionContext()
		}
		contextID, err := executionContext.ID()
		if err != nil {
			return "", nil, err
		}
		return contextColumn + " = ?", []any{contextID}, nil
	}
	sourceIDs := make([]string, 0, len(sourceContexts))
	for sourceID := range sourceContexts {
		sourceIDs = append(sourceIDs, sourceID)
	}
	sort.Strings(sourceIDs)
	parts := make([]string, 0, len(sourceIDs))
	args := make([]any, 0, len(sourceIDs)*2)
	for _, sourceID := range sourceIDs {
		contextID, err := sourceContexts[sourceID].ID()
		if err != nil {
			return "", nil, fmt.Errorf("store: model catalog context for source %q: %w", sourceID, err)
		}
		parts = append(parts, "("+sourceColumn+" = ? AND "+contextColumn+" = ?)")
		args = append(args, strings.TrimSpace(sourceID), contextID)
	}
	return "(" + strings.Join(parts, " OR ") + ")", args, nil
}

func modelCatalogStatusFilterClauses(
	opts modelcatalog.StatusOptions,
) ([]string, []any, error) {
	contextClause, contextArgs, err := modelCatalogContextFilterClause(
		opts.ExecutionContext,
		opts.SourceContexts,
		"source_id",
		"context_id",
	)
	if err != nil {
		return nil, nil, err
	}
	where := []string{contextClause}
	args := append([]any(nil), contextArgs...)
	if providerID := strings.TrimSpace(opts.ProviderID); providerID != "" {
		where = append(where, "provider_id = ?")
		args = append(args, providerID)
	}
	return where, args, nil
}

func modelCatalogContextQueries(opts modelcatalog.ListOptions) ([]modelCatalogContextQuery, error) {
	requestedSource := strings.TrimSpace(opts.SourceID)
	if len(opts.SourceContexts) == 0 {
		executionContext := opts.ExecutionContext
		if executionContext.Scope == "" {
			executionContext = modelcatalog.GlobalCatalogExecutionContext()
		}
		contextID, err := executionContext.ID()
		if err != nil {
			return nil, err
		}
		return []modelCatalogContextQuery{{contextID: contextID, sourceID: requestedSource}}, nil
	}
	sourceIDs := make([]string, 0, len(opts.SourceContexts))
	for sourceID := range opts.SourceContexts {
		if requestedSource == "" || requestedSource == sourceID {
			sourceIDs = append(sourceIDs, sourceID)
		}
	}
	sort.Strings(sourceIDs)
	queries := make([]modelCatalogContextQuery, 0, len(sourceIDs))
	for _, sourceID := range sourceIDs {
		contextID, err := opts.SourceContexts[sourceID].ID()
		if err != nil {
			return nil, fmt.Errorf("store: model catalog context for source %q: %w", sourceID, err)
		}
		queries = append(queries, modelCatalogContextQuery{contextID: contextID, sourceID: sourceID})
	}
	return queries, nil
}
