package loop

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/compozy/compozy/internal/task"
)

func (r *CoordinatorRunner) provenanceParentSessionID(ctx context.Context, run Run) (string, error) {
	if strings.TrimSpace(string(run.ID)) == "" || strings.TrimSpace(string(run.WorkspaceID)) == "" {
		return "", fmt.Errorf("%w: loop provenance run identity is incomplete", ErrValidation)
	}
	if sessionID := provenanceSessionID(run); sessionID != "" {
		return sessionID, nil
	}

	seen := map[RunID]struct{}{run.ID: {}}
	parentID := run.ParentLoopRunID
	for depth := 1; parentID != ""; depth++ {
		if depth >= LoopMaxAncestryDepth {
			return "", fmt.Errorf("%w: loop provenance ancestry exceeds the maximum depth", ErrValidation)
		}
		if _, ok := seen[parentID]; ok {
			return "", fmt.Errorf("%w: loop provenance ancestry contains a cycle", ErrValidation)
		}
		parent, err := r.store.GetLoopRunByID(ctx, parentID)
		if err != nil {
			if errors.Is(err, ErrRunNotFound) {
				logger := r.logger
				if logger == nil {
					logger = slog.Default()
				}
				logger.WarnContext(ctx, "loop provenance ancestor is unavailable",
					"workspace_id", run.WorkspaceID,
					"loop_run_id", run.ID,
					"parent_loop_run_id", parentID,
				)
				return "", nil
			}
			return "", fmt.Errorf("loop: resolve provenance ancestor: %w", err)
		}
		if parent.ID != parentID {
			return "", fmt.Errorf("%w: loop provenance ancestor identity mismatch", ErrValidation)
		}
		if parent.WorkspaceID != run.WorkspaceID {
			return "", fmt.Errorf("%w: loop provenance ancestor workspace mismatch", ErrValidation)
		}
		seen[parentID] = struct{}{}
		if sessionID := provenanceSessionID(parent); sessionID != "" {
			return sessionID, nil
		}
		parentID = parent.ParentLoopRunID
	}
	return "", nil
}

func provenanceSessionID(run Run) string {
	if run.Origin != nil && run.Origin.Kind == RunOriginSession {
		if sessionID := strings.TrimSpace(run.Origin.SessionID); sessionID != "" {
			return sessionID
		}
	}
	if run.StartedBy.Kind.Normalize() == task.ActorKindAgentSession {
		return strings.TrimSpace(run.StartedBy.Ref)
	}
	return ""
}
