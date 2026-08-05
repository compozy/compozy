package task

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const terminalRunCommandDurabilityTimeout = 30 * time.Second

type terminalRunCommandSettlement struct {
	command           TerminalRunCommand
	run               Run
	task              Task
	statusTransitions []StatusTransition
	rolledUpRuns      []Run
	events            []Event
}

func (m *Service) executeTerminalRunCommand(
	ctx context.Context,
	candidate TerminalRunCommand,
) (terminalRunCommandSettlement, error) {
	command, inserted, err := m.store.ReserveTerminalRunCommand(ctx, candidate)
	if err != nil {
		return terminalRunCommandSettlement{}, err
	}
	if !command.sameIntent(candidate) {
		return terminalRunCommandSettlement{}, fmt.Errorf(
			"%w: task run %q already has a different terminal intent",
			ErrTerminalRunCommandInProgress,
			candidate.RunID(),
		)
	}
	if command.StopRequired() {
		if err := m.requireSessionExecutor("execute terminal run command"); err != nil {
			if inserted {
				return terminalRunCommandSettlement{}, errors.Join(
					err,
					m.releaseTerminalRunCommandAfterFailure(ctx, command),
				)
			}
			return terminalRunCommandSettlement{}, err
		}
		command, err = m.stopAdmittedTerminalRunCommand(ctx, command, false)
		if err != nil {
			return terminalRunCommandSettlement{}, err
		}
	}

	settlementCtx := ctx
	settlementCancel := func() {}
	if command.Phase() == TerminalRunCommandPhaseStopConfirmed {
		settlementCtx, settlementCancel = terminalRunCommandDurabilityContext(ctx)
	}
	defer settlementCancel()
	return m.settleTerminalRunCommand(settlementCtx, command)
}

func (m *Service) stopAdmittedTerminalRunCommand(
	ctx context.Context,
	command TerminalRunCommand,
	resumeStopRequested bool,
) (TerminalRunCommand, error) {
	switch command.Phase() {
	case TerminalRunCommandPhaseAdmitted:
		var err error
		command, err = m.store.AdvanceTerminalRunCommandPhase(
			ctx,
			command,
			TerminalRunCommandPhaseStopRequested,
			m.now().UTC(),
		)
		if err != nil {
			return TerminalRunCommand{}, err
		}
	case TerminalRunCommandPhaseStopRequested:
		if !resumeStopRequested {
			return TerminalRunCommand{}, fmt.Errorf(
				"%w: stop is already in progress for task run %q",
				ErrTerminalRunCommandInProgress,
				command.RunID(),
			)
		}
	case TerminalRunCommandPhaseStopConfirmed:
		return command, nil
	default:
		return TerminalRunCommand{}, fmt.Errorf(
			"%w: task run %q has terminal command phase %q",
			ErrValidation,
			command.RunID(),
			command.Phase(),
		)
	}

	if err := m.stopTerminalRunCommandSession(ctx, command); err != nil {
		return TerminalRunCommand{}, err
	}

	durabilityCtx, cancel := terminalRunCommandDurabilityContext(ctx)
	defer cancel()
	confirmed, err := m.store.AdvanceTerminalRunCommandPhase(
		durabilityCtx,
		command,
		TerminalRunCommandPhaseStopConfirmed,
		m.now().UTC(),
	)
	if err != nil {
		return TerminalRunCommand{}, fmt.Errorf(
			"task: persist confirmed stop for run %q: %w",
			command.RunID(),
			err,
		)
	}
	return confirmed, nil
}

func (m *Service) stopTerminalRunCommandSession(
	ctx context.Context,
	command TerminalRunCommand,
) error {
	run := Run{SessionID: command.Fence().SessionID()}
	if audit := command.recoveryAudit(); audit != nil {
		return m.stopRecoveredRunSession(ctx, run, audit.recovery)
	}
	return m.stopTerminalRunSession(ctx, run, command.StopReason())
}

func (m *Service) releaseTerminalRunCommandAfterFailure(
	ctx context.Context,
	command TerminalRunCommand,
) error {
	durabilityCtx, cancel := terminalRunCommandDurabilityContext(ctx)
	defer cancel()
	if err := m.store.ReleaseTerminalRunCommand(durabilityCtx, command); err != nil {
		return fmt.Errorf(
			"task: release failed terminal command for run %q: %w",
			command.RunID(),
			err,
		)
	}
	return nil
}

func terminalRunCommandDurabilityContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), terminalRunCommandDurabilityTimeout)
}

func (m *Service) publishTerminalRunCommandSettlement(
	ctx context.Context,
	settlement *terminalRunCommandSettlement,
) error {
	publicationCtx, cancel := completedSettlementPublicationContext(ctx)
	defer cancel()

	actor := settlement.command.Actor()
	switch settlement.command.Kind() {
	case TerminalRunCommandCompleted:
		m.publishTaskEventsAfterCommand(publicationCtx, settlement.events)
		taskRecord, err := m.publishCompletedRunSettlement(publicationCtx, &CompletedRunSettlement{
			Run:               settlement.run,
			Task:              settlement.task,
			StatusTransitions: settlement.statusTransitions,
			RolledUpRuns:      settlement.rolledUpRuns,
		}, actor)
		m.dispatchTerminalWake(publicationCtx, taskRecord, settlement.run, actor)
		return err
	case TerminalRunCommandFailed, TerminalRunCommandCanceled:
		m.publishTaskRunMutationSettlement(publicationCtx, &taskRunMutationSettlement{
			Run:               settlement.run,
			Task:              settlement.task,
			StatusTransitions: settlement.statusTransitions,
			Events:            settlement.events,
		}, actor)
		return nil
	case TerminalRunCommandNeedsAttention:
		m.publishTaskEventsAfterCommand(publicationCtx, settlement.events)
		return nil
	default:
		return fmt.Errorf(
			"%w: unsupported terminal run command kind %q",
			ErrValidation,
			settlement.command.Kind(),
		)
	}
}

func terminalRunCommandAction(kind TerminalRunCommandKind) string {
	switch kind {
	case TerminalRunCommandCompleted:
		return "complete task run"
	case TerminalRunCommandFailed:
		return "fail task run"
	case TerminalRunCommandCanceled:
		return "cancel task run"
	case TerminalRunCommandNeedsAttention:
		return "mark task run needs attention"
	default:
		return "settle terminal task run command"
	}
}

func requireTerminalCommandSession(command TerminalRunCommand, sessionID string) error {
	if strings.TrimSpace(sessionID) != command.Fence().SessionID() {
		return fmt.Errorf(
			"%w: session evidence for task run %q does not match its terminal command",
			ErrInvalidStatusTransition,
			command.RunID(),
		)
	}
	return nil
}
