package loop

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/looplab/fsm"
)

const (
	coordinatorPhaseHydrated  = "hydrated"
	coordinatorPhaseDerived   = "derived"
	coordinatorPhaseEvaluated = "evaluated"
	coordinatorPhaseAssembled = "assembled"
	coordinatorPhaseYielded   = "yielded"
)

const (
	coordinatorEventDerive   = "derive"
	coordinatorEventEvaluate = "evaluate"
	coordinatorEventAssemble = "assemble"
	coordinatorEventYield    = "yield"
)

type coordinatorFSM struct {
	machine     *fsm.FSM
	logger      *slog.Logger
	loopRunID   RunID
	workspaceID WorkspaceID
	generation  int
}

func newCoordinatorFSM(logger *slog.Logger, run Run) *coordinatorFSM {
	if logger == nil {
		logger = slog.Default()
	}
	return &coordinatorFSM{
		machine: fsm.NewFSM(
			coordinatorPhaseHydrated,
			[]fsm.EventDesc{
				{
					Name: coordinatorEventDerive,
					Src:  []string{coordinatorPhaseHydrated},
					Dst:  coordinatorPhaseDerived,
				},
				{
					Name: coordinatorEventEvaluate,
					Src:  []string{coordinatorPhaseDerived},
					Dst:  coordinatorPhaseEvaluated,
				},
				{
					Name: coordinatorEventAssemble,
					Src:  []string{coordinatorPhaseEvaluated},
					Dst:  coordinatorPhaseAssembled,
				},
				{
					Name: coordinatorEventYield,
					Src:  []string{coordinatorPhaseAssembled},
					Dst:  coordinatorPhaseYielded,
				},
			},
			nil,
		),
		logger:      logger,
		loopRunID:   run.ID,
		workspaceID: run.WorkspaceID,
		generation:  run.Generation,
	}
}

func (m *coordinatorFSM) transition(ctx context.Context, event string) error {
	from := m.machine.Current()
	if err := m.machine.Event(ctx, event); err != nil {
		return fmt.Errorf(
			"%w: coordinator phase transition %q from %q: %w",
			ErrInvalidTransition,
			event,
			from,
			err,
		)
	}
	to := m.machine.Current()
	m.logger.DebugContext(
		ctx,
		"loop coordinator phase transition",
		"loop_run_id", string(m.loopRunID),
		"workspace_id", string(m.workspaceID),
		"generation", m.generation,
		"event", event,
		"from", from,
		"to", to,
	)
	return nil
}
