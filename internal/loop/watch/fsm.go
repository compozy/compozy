package watch

import (
	"context"
	"fmt"

	"github.com/looplab/fsm"
)

const (
	watchStateIdle      = "idle"
	watchStatePolling   = "polling"
	watchStateReady     = "ready"
	watchStateSettling  = "settling"
	watchStateConfirmed = "confirmed"
	watchStateWaiting   = "waiting"
	watchStateStalled   = "stalled"

	watchEventPoll    = "poll"
	watchEventReady   = "ready"
	watchEventSettle  = "settle"
	watchEventConfirm = "confirm"
	watchEventWait    = "wait"
	watchEventStall   = "stall"
)

type watchFSM struct {
	machine *fsm.FSM
}

func newWatchFSM() *watchFSM {
	return &watchFSM{
		machine: fsm.NewFSM(
			watchStateIdle,
			fsm.Events{
				{Name: watchEventPoll, Src: []string{watchStateIdle}, Dst: watchStatePolling},
				{Name: watchEventReady, Src: []string{watchStatePolling}, Dst: watchStateReady},
				{Name: watchEventSettle, Src: []string{watchStateReady}, Dst: watchStateSettling},
				{Name: watchEventConfirm, Src: []string{watchStateSettling}, Dst: watchStateConfirmed},
				{Name: watchEventWait, Src: []string{watchStatePolling, watchStateReady}, Dst: watchStateWaiting},
				{Name: watchEventStall, Src: []string{watchStatePolling, watchStateReady}, Dst: watchStateStalled},
			},
			nil,
		),
	}
}

func (f *watchFSM) transition(ctx context.Context, event string) (Transition, error) {
	if f == nil || f.machine == nil {
		return Transition{}, fmt.Errorf("loop watch fsm is not initialized")
	}
	from := f.machine.Current()
	if err := f.machine.Event(ctx, event); err != nil {
		return Transition{}, fmt.Errorf("loop watch fsm transition %q from %q: %w", event, from, err)
	}
	return Transition{Event: event, From: from, To: f.machine.Current()}, nil
}
