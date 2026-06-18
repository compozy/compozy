package recovery

import "github.com/looplab/fsm"

type recoveryState string

const (
	recoveryStateExecuting   recoveryState = "executing"
	recoveryStateFailed      recoveryState = "failed"
	recoveryStateTriaging    recoveryState = "triaging"
	recoveryStateRemediating recoveryState = "remediating"
	recoveryStateRestarting  recoveryState = "restarting"
	recoveryStateRecovered   recoveryState = "recovered"
	recoveryStateExhausted   recoveryState = "exhausted"
	recoveryStateCanceled    recoveryState = "canceled"
)

const (
	recoveryEventFail      = "fail"
	recoveryEventTriage    = "triage"
	recoveryEventRemediate = "remediate"
	recoveryEventRestart   = "restart"
	recoveryEventRecover   = "recover"
	recoveryEventExhaust   = "exhaust"
	recoveryEventCancel    = "cancel"
)

func newRecoveryFSM() *fsm.FSM {
	return fsm.NewFSM(
		string(recoveryStateExecuting),
		[]fsm.EventDesc{
			{
				Name: recoveryEventFail,
				Src:  []string{string(recoveryStateExecuting), string(recoveryStateRestarting)},
				Dst:  string(recoveryStateFailed),
			},
			{
				Name: recoveryEventTriage,
				Src:  []string{string(recoveryStateFailed)},
				Dst:  string(recoveryStateTriaging),
			},
			{
				Name: recoveryEventRemediate,
				Src:  []string{string(recoveryStateTriaging)},
				Dst:  string(recoveryStateRemediating),
			},
			{
				Name: recoveryEventRestart,
				Src:  []string{string(recoveryStateRemediating)},
				Dst:  string(recoveryStateRestarting),
			},
			{
				Name: recoveryEventRecover,
				Src:  []string{string(recoveryStateExecuting), string(recoveryStateRestarting)},
				Dst:  string(recoveryStateRecovered),
			},
			{
				Name: recoveryEventExhaust,
				Src: []string{
					string(recoveryStateFailed),
					string(recoveryStateTriaging),
					string(recoveryStateRemediating),
					string(recoveryStateRestarting),
				},
				Dst: string(recoveryStateExhausted),
			},
			{
				Name: recoveryEventCancel,
				Src: []string{
					string(recoveryStateExecuting),
					string(recoveryStateFailed),
					string(recoveryStateTriaging),
					string(recoveryStateRemediating),
					string(recoveryStateRestarting),
				},
				Dst: string(recoveryStateCanceled),
			},
		},
		nil,
	)
}
