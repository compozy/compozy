package core

import (
	"context"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
)

func sessionReplyWatchPayloads(
	ctx context.Context,
	reader any,
	workspace, sender string,
) ([]contract.ReplyWatchPayload, error) {
	source, ok := reader.(interface {
		ReplyWatches() session.ReplyWatchService
	})
	if !ok || source.ReplyWatches() == nil {
		return nil, nil
	}
	rows, err := source.ReplyWatches().ListForSender(ctx, workspace, sender)
	if err != nil {
		return nil, err
	}
	out := make([]contract.ReplyWatchPayload, 0, len(rows))
	for _, row := range rows {
		out = append(
			out,
			contract.ReplyWatchPayload{
				ID:              row.ID,
				TargetSessionID: row.TargetSessionID,
				MessageID:       row.MessageID,
				State:           row.State,
				CreatedAt:       row.CreatedAt,
			},
		)
	}
	return out, nil
}
