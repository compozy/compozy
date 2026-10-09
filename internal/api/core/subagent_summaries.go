package core

import (
	"context"
	"github.com/compozy/compozy/internal/api/contract"
)

func (h *BaseHandlers) decorateSubagentSummaries(ctx context.Context, payloads []contract.SessionPayload) error {
	if h.Subagents == nil || len(payloads) == 0 {
		return nil
	}
	ids := make([]string, 0, len(payloads))
	for _, payload := range payloads {
		ids = append(ids, payload.ID)
	}
	summaries, err := h.Subagents.Summaries(ctx, ids)
	if err != nil {
		return err
	}
	for i := range payloads {
		payloads[i].SubagentSummary = contract.SubagentSummaryFromStore(summaries[payloads[i].ID])
	}
	return nil
}
