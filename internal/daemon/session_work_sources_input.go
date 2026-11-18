package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/toolruntime"
)

type sessionInputHoldReader interface {
	SessionInputHold(context.Context, string) (*store.SessionInputHold, error)
}

// heldInputs projects a durable queue hold as attention without treating it as runnable work.
func (s sessionWorkSources) heldInputs(ctx context.Context, id string) ([]session.WorkSignal, error) {
	reader, ok := s.state.registry.(sessionInputHoldReader)
	if !ok {
		return nil, errors.New("session input hold reader is unavailable")
	}
	hold, err := reader.SessionInputHold(ctx, id)
	if err != nil || hold == nil {
		return nil, err
	}
	return []session.WorkSignal{{
		Kind: session.WorkSignalHeldInput, Since: hold.Since, Ref: hold.InstanceID,
		HeldReason: hold.Reason, HeldInstanceID: hold.InstanceID,
		ValidUntil:      s.now().Add(2 * toolruntime.SweepInterval),
		AttentionReason: fmt.Sprintf("Input held by instance %s: %s", hold.InstanceID, hold.Reason),
	}}, nil
}
