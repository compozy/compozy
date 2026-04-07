package plan

import (
	"context"
	"log/slog"
	"time"

	"github.com/compozy/compozy/internal/core/model"
)

// ClosePreparationJournal closes the preparation journal with a bounded timeout
// while preserving the caller context when it exists.
func ClosePreparationJournal(ctx context.Context, prep *model.SolvePreparation) {
	if prep == nil || prep.Journal() == nil {
		return
	}

	closeCtx := ctx
	if closeCtx == nil {
		closeCtx = context.Background()
	}
	if _, hasDeadline := closeCtx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		closeCtx, cancel = context.WithTimeout(closeCtx, time.Second)
		defer cancel()
	}

	if err := prep.CloseJournal(closeCtx); err != nil {
		slog.Warn(
			"close preparation journal",
			"run_id",
			prep.RunArtifacts.RunID,
			"events_path",
			prep.RunArtifacts.EventsPath,
			"error",
			err,
		)
	}
}
