package temporal

import "go.temporal.io/sdk/workflow"

// -----------------------------------------------------------------------------
// Pause Gate
// -----------------------------------------------------------------------------

// PauseGate blocks workflow progress whenever a PAUSE signal is received.
type PauseGate struct {
	paused bool
	pause  workflow.ReceiveChannel
	resume workflow.ReceiveChannel
	await  func() error
}

// NewPauseGate installs signal listeners + query handler and returns a gate.
func NewPauseGate(ctx workflow.Context) (*PauseGate, error) {
	g := &PauseGate{
		pause:  workflow.GetSignalChannel(ctx, "PAUSE"),
		resume: workflow.GetSignalChannel(ctx, "RESUME"),
	}
	g.await = func() error {
		return workflow.Await(ctx, func() bool {
			return !g.paused
		})
	}

	// flip flag in a background goroutine
	workflow.Go(ctx, func(ctx workflow.Context) {
		for {
			sel := workflow.NewSelector(ctx)
			sel.AddReceive(g.pause, func(workflow.ReceiveChannel, bool) { g.paused = true })
			sel.AddReceive(g.resume, func(workflow.ReceiveChannel, bool) { g.paused = false })
			sel.Select(ctx)
			// NEW: break out as soon as the workflow is canceled
			if ctx.Err() != workflow.ErrCanceled {
				return
			}
		}
	})

	// expose live state for operators
	if err := workflow.SetQueryHandler(ctx, "state", func() (string, error) {
		if g.paused {
			return "paused", nil
		}
		return "running", nil
	}); err != nil {
		return nil, err
	}
	return g, nil
}

func (g *PauseGate) Await() error {
	return g.await()
}
