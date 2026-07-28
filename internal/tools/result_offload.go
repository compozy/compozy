package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// Finalize reapplies redaction after hooks, then offloads one oversized canonical result.
func (l *DefaultResultProcessor) Finalize(
	ctx context.Context,
	scope Scope,
	d Descriptor,
	result ToolResult,
) (ToolResult, error) {
	finalized, err := l.Sanitize(ctx, d, result)
	if err != nil {
		return ToolResult{}, err
	}
	maxBytes := l.maxBytes(d)
	if maxBytes < 0 || finalized.Bytes <= maxBytes {
		if err := finalized.Validate(maxBytes); err != nil {
			return ToolResult{}, resultLimiterRejection(d.ID, err)
		}
		return finalized, nil
	}

	canonical, err := json.Marshal(finalized)
	if err != nil {
		return ToolResult{}, resultLimiterRejection(d.ID, err)
	}
	prospectiveRef := toolArtifactRef(artifactID(canonical), int64(len(canonical)))
	limited, err := truncateToolResult(finalized, maxBytes, []ArtifactRef{prospectiveRef})
	if err != nil {
		return ToolResult{}, resultLimiterRejection(d.ID, err)
	}
	if limited.Bytes > maxBytes {
		partial, partialErr := truncateToolResult(finalized, maxBytes, nil)
		if partialErr != nil {
			return ToolResult{}, resultLimiterRejection(d.ID, partialErr)
		}
		return partial, resultLimiterRejection(
			d.ID,
			NewValidationError("artifacts", ReasonResultBudgetExceeded, "artifact reference exceeds result budget"),
		).WithPartialResult(partial)
	}
	if l.artifacts == nil {
		partial, partialErr := truncateToolResult(finalized, maxBytes, nil)
		if partialErr != nil {
			return ToolResult{}, resultLimiterRejection(d.ID, partialErr)
		}
		return partial, resultPersistenceError(d.ID, errors.New("artifact store is unavailable"), partial)
	}
	ref, err := l.artifacts.Put(ctx, scope.WorkspaceID, canonical)
	if err != nil {
		partial, partialErr := truncateToolResult(finalized, maxBytes, nil)
		if partialErr != nil {
			return ToolResult{}, resultLimiterRejection(d.ID, partialErr)
		}
		return partial, resultPersistenceError(d.ID, err, partial)
	}
	if ref != prospectiveRef {
		partial, partialErr := truncateToolResult(finalized, maxBytes, nil)
		if partialErr != nil {
			return ToolResult{}, resultLimiterRejection(d.ID, partialErr)
		}
		return partial, resultPersistenceError(d.ID, ErrToolArtifactCorrupt, partial)
	}
	if err := limited.Validate(maxBytes); err != nil {
		return ToolResult{}, resultLimiterRejection(d.ID, err)
	}
	return limited, nil
}

func resultPersistenceError(id ToolID, err error, partial ToolResult) *ToolError {
	return NewToolError(
		ErrorCodeResultPersistenceFailed,
		id,
		fmt.Sprintf("tool %q result could not be retained", id),
		fmt.Errorf("%w: %w", ErrToolResultPersistence, err),
		ReasonResultPersistenceFailed,
	).WithPartialResult(partial)
}
