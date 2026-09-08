package inputqueue

import (
	"context"

	"github.com/compozy/compozy/internal/store"
)

// EnqueueSynthetic captures daemon work under the same capacity and claim policy as human input.
func (s *Service) EnqueueSynthetic(
	ctx context.Context,
	req InputRequest,
	turnID string,
	prompt *store.SessionInputSyntheticPrompt,
	taskRunID string,
) (store.SessionInputQueueEntry, error) {
	insert, err := s.PrepareQueue(req)
	if err != nil {
		return store.SessionInputQueueEntry{}, err
	}
	insert.OwnerKind = store.SessionInputOwnerSynthetic
	insert.SyntheticPrompt = prompt.Clone()
	insert.TurnID = turnID
	insert.TaskRunID = taskRunID
	entry, _, err := s.store.EnqueueSessionInput(ctx, insert)
	return entry, err
}
