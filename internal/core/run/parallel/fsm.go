package parallelrun

import "github.com/looplab/fsm"

type parallelState string

const (
	parallelStatePlanning       parallelState = "planning"
	parallelStateWaveRunning    parallelState = "wave_running"
	parallelStateWaveRecovering parallelState = "wave_recovering"
	parallelStateWaveMerging    parallelState = "wave_merging"
	parallelStateWaveDone       parallelState = "wave_done"
	parallelStateFinalizing     parallelState = "finalizing"
	parallelStateCompleted      parallelState = "completed"
	parallelStateCanceled       parallelState = "canceled"
)

const (
	parallelEventStartWave   = "start_wave"
	parallelEventRecoverWave = "recover_wave"
	parallelEventMergeWave   = "merge_wave"
	parallelEventFinishWave  = "finish_wave"
	parallelEventFinalize    = "finalize"
	parallelEventComplete    = "complete"
	parallelEventCancel      = "cancel"
)

func newParallelFSM() *fsm.FSM {
	return fsm.NewFSM(
		string(parallelStatePlanning),
		[]fsm.EventDesc{
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
				Name: parallelEventFinalize,
				Src:  []string{string(parallelStatePlanning), string(parallelStateWaveDone)},
				Dst:  string(parallelStateFinalizing),
			},
			{
				Name: parallelEventComplete,
				Src:  []string{string(parallelStateFinalizing)},
				Dst:  string(parallelStateCompleted),
			},
			{
				Name: parallelEventCancel,
				Src: []string{
					string(parallelStatePlanning),
					string(parallelStateWaveRunning),
					string(parallelStateWaveRecovering),
					string(parallelStateWaveMerging),
					string(parallelStateWaveDone),
					string(parallelStateFinalizing),
				},
				Dst: string(parallelStateCanceled),
			},
		},
		nil,
	)
}
