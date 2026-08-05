package task

import (
	"context"
	"fmt"
	"strings"
)

// PendingTerminalRunCommand is the safe daemon-facing projection of one
// internal recovery obligation. It deliberately omits receipt, actor, payload,
// workspace, claim hash, and lease data.
type PendingTerminalRunCommand struct {
	RunID        string
	SessionID    string
	Phase        TerminalRunCommandPhase
	StopRequired bool
}

// TerminalRunRecoveryDisposition classifies whether boot must resume the
// external stop, may settle it, or lacks conclusive evidence for either.
type TerminalRunRecoveryDisposition string

const (
	TerminalRunRecoveryResumeStop   TerminalRunRecoveryDisposition = "resume_stop"
	TerminalRunRecoveryStopObserved TerminalRunRecoveryDisposition = "stop_observed"
	TerminalRunRecoveryAmbiguous    TerminalRunRecoveryDisposition = "ambiguous"
)

// TerminalRunRecoveryEvidence describes daemon-observed session state for one
// pending terminal command.
type TerminalRunRecoveryEvidence struct {
	RunID       string
	SessionID   string
	State       string
	Disposition TerminalRunRecoveryDisposition
}

func (e TerminalRunRecoveryEvidence) validate() error {
	if strings.TrimSpace(e.RunID) == "" {
		return fmt.Errorf("%w: terminal recovery run id is required", ErrValidation)
	}
	if err := ValidateReferenceSize(e.RunID, "terminal_recovery.run_id"); err != nil {
		return err
	}
	if err := ValidateReferenceSize(e.SessionID, "terminal_recovery.session_id"); err != nil {
		return err
	}
	if err := ValidateReferenceSize(e.State, "terminal_recovery.session_state"); err != nil {
		return err
	}
	switch e.Disposition {
	case TerminalRunRecoveryResumeStop,
		TerminalRunRecoveryStopObserved,
		TerminalRunRecoveryAmbiguous:
		return nil
	default:
		return fmt.Errorf(
			"%w: invalid terminal recovery disposition %q",
			ErrValidation,
			e.Disposition,
		)
	}
}

// ListPendingTerminalRunCommands returns only the non-sensitive session facts
// needed by daemon boot recovery.
func (m *Service) ListPendingTerminalRunCommands(
	ctx context.Context,
	actor ActorContext,
) ([]PendingTerminalRunCommand, error) {
	if err := requireDaemonTerminalRecoveryActor(actor); err != nil {
		return nil, err
	}
	commands, err := m.store.ListTerminalRunCommands(ctx)
	if err != nil {
		return nil, err
	}
	pending := make([]PendingTerminalRunCommand, 0, len(commands))
	for _, command := range commands {
		pending = append(pending, PendingTerminalRunCommand{
			RunID:        command.RunID(),
			SessionID:    command.Fence().SessionID(),
			Phase:        command.Phase(),
			StopRequired: command.StopRequired(),
		})
	}
	return pending, nil
}

// RecoverTerminalRunCommandOnBoot resumes one exact durable intent before the
// daemon performs generic active-run recovery.
func (m *Service) RecoverTerminalRunCommandOnBoot(
	ctx context.Context,
	evidence TerminalRunRecoveryEvidence,
	actor ActorContext,
) (*Run, error) {
	if err := requireDaemonTerminalRecoveryActor(actor); err != nil {
		return nil, err
	}
	if err := evidence.validate(); err != nil {
		return nil, err
	}
	command, err := m.loadTerminalRunCommandForBoot(ctx, strings.TrimSpace(evidence.RunID))
	if err != nil {
		return nil, err
	}
	if err := requireTerminalCommandSession(command, strings.TrimSpace(evidence.SessionID)); err != nil {
		return nil, err
	}
	taskRecord, err := m.store.GetTask(ctx, command.TaskID())
	if err != nil {
		return nil, err
	}
	if err := m.authorizeTaskResource(ctx, actor, taskRecord); err != nil {
		return nil, err
	}

	if command.StopRequired() && command.Phase() != TerminalRunCommandPhaseStopConfirmed {
		switch evidence.Disposition {
		case TerminalRunRecoveryResumeStop:
			command, err = m.resumeTerminalRunCommandStop(ctx, command, true)
		case TerminalRunRecoveryStopObserved:
			command, err = m.resumeTerminalRunCommandStop(ctx, command, false)
		case TerminalRunRecoveryAmbiguous:
			return nil, fmt.Errorf(
				"%w: session state %q is ambiguous for terminal task run %q",
				ErrTerminalRunCommandInProgress,
				evidence.State,
				command.RunID(),
			)
		}
		if err != nil {
			return nil, err
		}
	} else if !command.StopRequired() && evidence.Disposition != TerminalRunRecoveryStopObserved {
		return nil, fmt.Errorf(
			"%w: terminal task run %q without a stop requires observed-stop recovery",
			ErrValidation,
			command.RunID(),
		)
	}
	settlementCtx, cancel := terminalRunCommandDurabilityContext(ctx)
	defer cancel()
	settlement, err := m.settleTerminalRunCommand(settlementCtx, command)
	if err != nil {
		return nil, err
	}
	if err := m.publishTerminalRunCommandSettlement(settlementCtx, &settlement); err != nil {
		return &settlement.run, err
	}
	return &settlement.run, nil
}

func (m *Service) loadTerminalRunCommandForBoot(
	ctx context.Context,
	runID string,
) (TerminalRunCommand, error) {
	commands, err := m.store.ListTerminalRunCommands(ctx)
	if err != nil {
		return TerminalRunCommand{}, err
	}
	for _, command := range commands {
		if command.RunID() == runID {
			return command, nil
		}
	}
	return TerminalRunCommand{}, fmt.Errorf(
		"%w: no pending terminal command for task run %q",
		ErrTaskRunNotFound,
		runID,
	)
}

func (m *Service) resumeTerminalRunCommandStop(
	ctx context.Context,
	command TerminalRunCommand,
	sessionLive bool,
) (TerminalRunCommand, error) {
	if command.Phase() == TerminalRunCommandPhaseStopConfirmed {
		return command, nil
	}
	if sessionLive {
		return m.stopAdmittedTerminalRunCommand(ctx, command, true)
	}

	var err error
	if command.Phase() == TerminalRunCommandPhaseAdmitted {
		command, err = m.store.AdvanceTerminalRunCommandPhase(
			ctx,
			command,
			TerminalRunCommandPhaseStopRequested,
			m.now().UTC(),
		)
		if err != nil {
			return TerminalRunCommand{}, err
		}
	}
	if command.Phase() != TerminalRunCommandPhaseStopRequested {
		return TerminalRunCommand{}, fmt.Errorf(
			"%w: cannot recover task run %q from terminal phase %q",
			ErrValidation,
			command.RunID(),
			command.Phase(),
		)
	}
	durabilityCtx, cancel := terminalRunCommandDurabilityContext(ctx)
	defer cancel()
	return m.store.AdvanceTerminalRunCommandPhase(
		durabilityCtx,
		command,
		TerminalRunCommandPhaseStopConfirmed,
		m.now().UTC(),
	)
}

func requireDaemonTerminalRecoveryActor(actor ActorContext) error {
	if err := requireWriteAuthority(actor); err != nil {
		return err
	}
	if actor.Actor.Kind.Normalize() != ActorKindDaemon {
		return ErrPermissionDenied
	}
	return nil
}
