package core

import (
	"errors"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/store"
)

func enrichSessionInputError(payload *contract.ErrorPayload, err error) {
	if full, ok := errors.AsType[*store.SessionInputQueueFullError](err); ok {
		item := diagnostics.NewItem(diagnostics.ItemSpec{
			ID: "session.queue.full", Code: contract.CodeSessionQueueFull, Category: contract.CategorySession,
			Title: "Session queue is full", Message: payload.Error,
			Severity: contract.SeverityError, DataFreshness: contract.FreshnessLive,
		}, diagnostics.WithEvidence(map[string]any{"queue_cap": full.Cap, "queue_count": full.Count}))
		payload.Diagnostic = &item
	}
	if entry, ok := errors.AsType[*store.SessionInputNotQueuedError](
		err,
	); ok &&
		(entry.Status == store.SessionInputQueueStatusDispatching || entry.Status == store.SessionInputQueueStatusSent) {
		payload.Code = "entry_dispatching"
		payload.Details = map[string]string{"entry_id": entry.EntryID, "text": diagnostics.Redact(entry.Text)}
	}
}
