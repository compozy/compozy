package parallelrun

import "github.com/looplab/fsm"

type parallelState string

const (
	parallelStatePlanning       parallelState = "planning"
	parallelStateWaveRunning    parallelState = "wave_running"
	parallelStateWaveRecovering parallelState = "wave_recovering"
	parallelStateWaveMerging    parallelState = "wave_merging"
	parallelStateResolving      parallelState = "resolving_conflict"
	parallelStateWaveDone       parallelState = "wave_done"
	parallelStateFinalizing     parallelState = "finalizing"
	parallelStateCompleted      parallelState = "completed"
	parallelStateFailed         parallelState = "failed"
	parallelStateRolledBack     parallelState = "rolled_back"
	parallelStateCanceled       parallelState = "canceled"
)

const (
	parallelEventStartWave   = "start_wave"
	parallelEventRecoverWave = "recover_wave"
	parallelEventMergeWave   = "merge_wave"
	parallelEventResolve     = "resolve_conflict"
	parallelEventResolved    = "conflict_resolved"
	parallelEventFinishWave  = "finish_wave"
	parallelEventFinalize    = "finalize"
	parallelEventComplete    = "complete"
	parallelEventFail        = "fail"
	parallelEventRollback    = "rollback"
	parallelEventCancel      = "cancel"
)

func newParallelFSM() *fsm.FSM {
	return fsm.NewFSM(
		string(parallelStatePlanning),
		parallelFSMEvents(),
		nil,
	)
}

func parallelFSMEvents() []fsm.EventDesc {
	return []fsm.EventDesc{
		{
			Name: parallelEventStartWave,
			Src:  []string{string(parallelStatePlanning), string(parallelStateWaveDone)},
			Dst:  string(parallelStateWaveRunning),
		},
		{
			Name: parallelEventRecoverWave,
			Src:  []string{string(parallelStateWaveRunning)},
			Dst:  string(parallelStateWaveRecovering),
		},
		{
			Name: parallelEventMergeWave,
			Src:  []string{string(parallelStateWaveRunning), string(parallelStateWaveRecovering)},
			Dst:  string(parallelStateWaveMerging),
		},
		{
			Name: parallelEventFinishWave,
			Src:  []string{string(parallelStateWaveMerging)},
			Dst:  string(parallelStateWaveDone),
		},
		{
			Name: parallelEventResolve,
			Src:  []string{string(parallelStateWaveMerging)},
			Dst:  string(parallelStateResolving),
		},
		{
			Name: parallelEventResolved,
			Src:  []string{string(parallelStateResolving)},
			Dst:  string(parallelStateWaveMerging),
		},
		{
			Name: parallelEventFinalize,
			Src:  []string{string(parallelStatePlanning), string(parallelStateWaveDone)},
			Dst:  string(parallelStateFinalizing),
		},
		{
			Name: parallelEventComplete,
			Src:  []string{string(parallelStateFinalizing)},
			Dst:  string(parallelStateCompleted),
		},
		{Name: parallelEventCancel, Src: parallelTerminalSourceStates(), Dst: string(parallelStateCanceled)},
		{Name: parallelEventFail, Src: parallelTerminalSourceStates(), Dst: string(parallelStateFailed)},
		{Name: parallelEventRollback, Src: parallelTerminalSourceStates(), Dst: string(parallelStateRolledBack)},
	}
}

func parallelTerminalSourceStates() []string {
	return []string{
		string(parallelStatePlanning),
		string(parallelStateWaveRunning),
		string(parallelStateWaveRecovering),
		string(parallelStateWaveMerging),
		string(parallelStateResolving),
		string(parallelStateWaveDone),
		string(parallelStateFinalizing),
	}
}
