package globaldb

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/store"
)

// touchUnchangedModelCatalogSourceRows refreshes row timestamps in place when a
// replacement carries exactly the stored row content, so periodic refreshes of an
// unchanged source skip the delete-and-reinsert of rows, options, efforts, and bindings.
func touchUnchangedModelCatalogSourceRows(
	ctx context.Context,
	exec modelCatalogSQLExecutor,
	executionContext modelcatalog.CatalogExecutionContext,
	contextID string,
	rows []modelcatalog.ModelRow,
	status modelcatalog.SourceStatus,
) (bool, error) {
	refreshedAt, uniform := uniformModelCatalogRefreshedAt(rows)
	if !uniform {
		return false, nil
	}
	stored, err := listModelCatalogRows(ctx, exec, modelcatalog.ListOptions{
		ExecutionContext: executionContext,
		SourceID:         status.SourceID,
		ProviderID:       status.ProviderID,
		IncludeAll:       true,
		IncludeStale:     true,
	})
	if err != nil {
		return false, err
	}
	if !modelCatalogRowContentEqual(stored, rows, status) {
		return false, nil
	}
	if len(rows) == 0 {
		return true, nil
	}
	if _, err := exec.ExecContext(
		ctx,
		`UPDATE model_catalog_rows
		    SET refreshed_at = ?
		  WHERE context_id = ?
		    AND source_id = ?
		    AND provider_id = ?`,
		store.FormatNullableTimestamp(refreshedAt),
		contextID,
		status.SourceID,
		status.ProviderID,
	); err != nil {
		return false, fmt.Errorf("store: touch unchanged model catalog rows: %w", err)
	}
	return true, nil
}

func uniformModelCatalogRefreshedAt(rows []modelcatalog.ModelRow) (time.Time, bool) {
	if len(rows) == 0 {
		return time.Time{}, true
	}
	refreshedAt := rows[0].RefreshedAt
	for _, row := range rows[1:] {
		if !row.RefreshedAt.Equal(refreshedAt) {
			return time.Time{}, false
		}
	}
	return refreshedAt, true
}

func modelCatalogRowContentEqual(
	stored []modelcatalog.ModelRow,
	incoming []modelcatalog.ModelRow,
	status modelcatalog.SourceStatus,
) bool {
	if len(stored) != len(incoming) {
		return false
	}
	storedPrints, ok := modelCatalogRowFingerprints(stored, status)
	if !ok {
		return false
	}
	incomingPrints, ok := modelCatalogRowFingerprints(incoming, status)
	if !ok {
		return false
	}
	for index := range storedPrints {
		if !bytes.Equal(storedPrints[index], incomingPrints[index]) {
			return false
		}
	}
	return true
}

// modelCatalogRowFingerprints encodes normalized row content ordered by model id,
// excluding refresh timestamps and representation-only nil-versus-empty differences.
func modelCatalogRowFingerprints(rows []modelcatalog.ModelRow, status modelcatalog.SourceStatus) ([][]byte, bool) {
	normalized := make([]modelcatalog.ModelRow, 0, len(rows))
	for _, row := range rows {
		candidate, err := normalizeModelCatalogRow(status.SourceID, status.ProviderID, status, row)
		if err != nil {
			return nil, false
		}
		normalized = append(normalized, candidate)
	}
	slices.SortFunc(normalized, func(left, right modelcatalog.ModelRow) int {
		return cmp.Compare(left.ModelID, right.ModelID)
	})
	prints := make([][]byte, 0, len(normalized))
	for _, row := range normalized {
		encoded, err := json.Marshal(canonicalModelCatalogRowContent(row))
		if err != nil {
			return nil, false
		}
		prints = append(prints, encoded)
	}
	return prints, true
}

type modelCatalogRowContent struct {
	Row       modelcatalog.ModelRow
	ExpiresAt string
}

func canonicalModelCatalogRowContent(row modelcatalog.ModelRow) modelCatalogRowContent {
	content := modelCatalogRowContent{Row: row, ExpiresAt: store.FormatNullableTimestamp(row.ExpiresAt)}
	content.Row.RefreshedAt = time.Time{}
	content.Row.ExpiresAt = time.Time{}
	if len(content.Row.ReasoningEfforts) == 0 {
		content.Row.ReasoningEfforts = nil
	}
	content.Row.ConfigOptions = canonicalModelCatalogOptions(content.Row.ConfigOptions)
	if len(content.Row.TransportBindings) == 0 {
		content.Row.TransportBindings = nil
		return content
	}
	bindings := make([]modelcatalog.ModelTransportBinding, len(content.Row.TransportBindings))
	for index, binding := range content.Row.TransportBindings {
		bindings[index] = binding
		if len(binding.OptionSelections) == 0 {
			bindings[index].OptionSelections = nil
		}
	}
	content.Row.TransportBindings = bindings
	return content
}

func canonicalModelCatalogOptions(
	options []modelcatalog.ModelOptionDescriptor,
) []modelcatalog.ModelOptionDescriptor {
	if len(options) == 0 {
		return nil
	}
	canonical := make([]modelcatalog.ModelOptionDescriptor, len(options))
	for index, option := range options {
		canonical[index] = option
		if len(option.Values) == 0 {
			canonical[index].Values = nil
		}
	}
	return canonical
}
