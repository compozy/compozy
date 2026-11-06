package daemon

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/toolruntime"
)

func (s sessionWorkSources) heldInputs(ctx context.Context, id string) ([]session.WorkSignal, error) {
	inputs, err := s.manager.ListPendingInputs(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, input := range inputs {
		if input.Held {
			return []session.WorkSignal{
				{
					Kind:           session.WorkSignalHeldInput,
					Since:          input.EnqueuedAt,
					Ref:            input.HeldInstanceID,
					HeldReason:     input.HeldReason,
					HeldInstanceID: input.HeldInstanceID,
					ValidUntil:     s.now().Add(2 * toolruntime.SweepInterval),
					AttentionReason: fmt.Sprintf(
						"Input held by paused instance %s: %s",
						input.HeldInstanceID,
						input.HeldReason,
					),
				},
			}, nil
		}
	}
	return nil, nil
}
