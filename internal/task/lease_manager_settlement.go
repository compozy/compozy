package task

import "context"

type leaseSettlementCommandResult[T any] struct {
	settlement T
	events     []Event
}

type sessionLeaseReleaseOutcome struct {
	result   SessionLeaseReleaseResult
	previous Run
	task     Task
}

type sessionLeaseReleaseSettlement struct {
	outcomes          []sessionLeaseReleaseOutcome
	events            []Event
	statusTransitions []StatusTransition
}

func (m *Service) claimNextRunSettlement(
	ctx context.Context,
	criteria ClaimCriteria,
	actor ActorContext,
) (ClaimResult, ClaimedRunSettlement, error) {
	var claim ClaimResult
	commandResult := leaseSettlementCommandResult[ClaimedRunSettlement]{}
	err := m.store.WithLeaseSettlementTransaction(
		ctx,
		"claim task run lease",
		func(store LeaseSettlementMutationStore) error {
			var commandErr error
			claim, commandErr = store.ClaimNextRun(ctx, criteria)
			if commandErr != nil {
				return commandErr
			}
			claimResultWithoutRawTokenInMetadata(&claim)
			commandResult.settlement.Run = claim.Run
			if claim.Run.IsNetworkWake() {
				return nil
			}

			taskRecord, transitions, commandErr := m.reconcileTaskSettlementCascadeWithStore(
				ctx,
				store,
				claim.Run.TaskID,
				actor,
				criteria.Now,
			)
			if commandErr != nil {
				return commandErr
			}
			event, commandErr := m.newTaskEventAt(
				claim.Run.TaskID,
				claim.Run.ID,
				taskEventRunClaimed,
				actor,
				criteria.Now,
				runClaimedPayload{
					Status:         claim.Run.Status,
					TaskStatus:     taskRecord.Status,
					ClaimedBy:      ActorIdentity{Kind: actor.Actor.Kind, Ref: actor.Actor.Ref},
					ClaimTokenHash: claim.Run.ClaimTokenHash,
					LeaseUntil:     claim.Run.LeaseUntil,
				},
			)
			if commandErr != nil {
				return commandErr
			}
			if commandErr := store.CreateTaskEvent(ctx, event); commandErr != nil {
				return commandErr
			}

			claim.Task = &taskRecord
			claimResultWithoutRawTokenInMetadata(&claim)
			commandResult.settlement.Task = *claim.Task
			commandResult.settlement.StatusTransitions = transitions
			commandResult.events = []Event{event}
			return nil
		},
	)
	if err != nil {
		return ClaimResult{}, ClaimedRunSettlement{}, err
	}
	m.publishLeaseSettlement(ctx, commandResult.events, commandResult.settlement.StatusTransitions, actor)
	return claim, commandResult.settlement, nil
}

func (m *Service) heartbeatRunLeaseSettlement(
	ctx context.Context,
	heartbeat LeaseHeartbeat,
	actor ActorContext,
) (HeartbeatRunLeaseSettlement, error) {
	commandResult := leaseSettlementCommandResult[HeartbeatRunLeaseSettlement]{}
	err := m.store.WithLeaseSettlementTransaction(
		ctx,
		"heartbeat task run lease",
		func(store LeaseSettlementMutationStore) error {
			run, commandErr := store.HeartbeatRunLease(ctx, heartbeat)
			if commandErr != nil {
				return commandErr
			}
			commandResult.settlement.Run = run
			if run.IsNetworkWake() {
				return nil
			}

			taskRecord, commandErr := store.GetTask(ctx, run.TaskID)
			if commandErr != nil {
				return commandErr
			}
			event, commandErr := m.newTaskEventAt(
				run.TaskID,
				run.ID,
				taskEventRunLeaseExtended,
				actor,
				heartbeat.Now,
				leaseExtendedPayload{
					Status:         run.Status,
					TaskStatus:     taskRecord.Status,
					LeaseUntil:     run.LeaseUntil,
					HeartbeatAt:    run.HeartbeatAt,
					ClaimTokenHash: run.ClaimTokenHash,
				},
			)
			if commandErr != nil {
				return commandErr
			}
			if commandErr := store.CreateTaskEvent(ctx, event); commandErr != nil {
				return commandErr
			}

			commandResult.settlement.Task = taskRecord
			commandResult.events = []Event{event}
			return nil
		},
	)
	if err != nil {
		return HeartbeatRunLeaseSettlement{}, err
	}
	m.publishLeaseSettlement(ctx, commandResult.events, nil, actor)
	return commandResult.settlement, nil
}

func (m *Service) bindLeasedRunSessionSettlement(
	ctx context.Context,
	binding LeaseSessionBinding,
	actor ActorContext,
) (BoundRunSessionSettlement, error) {
	commandResult := leaseSettlementCommandResult[BoundRunSessionSettlement]{}
	err := m.store.WithLeaseSettlementTransaction(
		ctx,
		"bind leased task run session",
		func(store LeaseSettlementMutationStore) error {
			run, commandErr := store.BindLeasedRunSession(ctx, binding)
			if commandErr != nil {
				return commandErr
			}
			commandResult.settlement.Run = run

			taskRecord, commandErr := store.GetTask(ctx, run.TaskID)
			if commandErr != nil {
				return commandErr
			}
			event, commandErr := m.newTaskEventAt(
				run.TaskID,
				run.ID,
				taskEventRunSessionBound,
				actor,
				binding.Now,
				runTransitionPayload{
					Status:     run.Status,
					TaskStatus: taskRecord.Status,
					SessionID:  run.SessionID,
				},
			)
			if commandErr != nil {
				return commandErr
			}
			if commandErr := store.CreateTaskEvent(ctx, event); commandErr != nil {
				return commandErr
			}

			commandResult.settlement.Task = taskRecord
			commandResult.events = []Event{event}
			return nil
		},
	)
	if err != nil {
		return BoundRunSessionSettlement{}, err
	}
	m.publishLeaseSettlement(ctx, commandResult.events, nil, actor)
	return commandResult.settlement, nil
}

func (m *Service) releaseRunLeaseSettlement(
	ctx context.Context,
	release LeaseRelease,
	actor ActorContext,
) (ReleasedRunLeaseSettlement, error) {
	commandResult := leaseSettlementCommandResult[ReleasedRunLeaseSettlement]{}
	err := m.store.WithLeaseSettlementTransaction(
		ctx,
		"release task run lease",
		func(store LeaseSettlementMutationStore) error {
			previous, commandErr := store.GetTaskRun(ctx, release.RunID)
			if commandErr != nil {
				return commandErr
			}
			run, commandErr := store.ReleaseRunLease(ctx, release)
			if commandErr != nil {
				return commandErr
			}
			commandResult.settlement.Run = run
			commandResult.settlement.PreviousRun = previous
			if run.IsNetworkWake() {
				return nil
			}

			taskRecord, transitions, commandErr := m.reconcileTaskSettlementCascadeWithStore(
				ctx,
				store,
				run.TaskID,
				actor,
				release.Now,
			)
			if commandErr != nil {
				return commandErr
			}
			event, commandErr := m.newTaskEventAt(
				run.TaskID,
				run.ID,
				taskEventRunReleased,
				actor,
				release.Now,
				releasedRunPayload{
					PreviousStatus: previous.Status,
					Status:         run.Status,
					TaskStatus:     taskRecord.Status,
					Reason:         release.Reason,
					SessionID:      previous.SessionID,
				},
			)
			if commandErr != nil {
				return commandErr
			}
			if commandErr := store.CreateTaskEvent(ctx, event); commandErr != nil {
				return commandErr
			}

			commandResult.settlement.Task = taskRecord
			commandResult.settlement.StatusTransitions = transitions
			commandResult.events = []Event{event}
			return nil
		},
	)
	if err != nil {
		return ReleasedRunLeaseSettlement{}, err
	}
	m.publishLeaseSettlement(ctx, commandResult.events, commandResult.settlement.StatusTransitions, actor)
	return commandResult.settlement, nil
}

func (m *Service) releaseSessionRunLeasesSettlement(
	ctx context.Context,
	release SessionLeaseRelease,
	actor ActorContext,
) (sessionLeaseReleaseSettlement, error) {
	commandResult := sessionLeaseReleaseSettlement{}
	err := m.store.WithLeaseSettlementTransaction(
		ctx,
		"release session task-run leases",
		func(store LeaseSettlementMutationStore) error {
			previousRuns, commandErr := store.ListActiveSessionRunLeases(ctx, release.SessionID)
			if commandErr != nil {
				return commandErr
			}

			commandResult.outcomes = make([]sessionLeaseReleaseOutcome, 0, len(previousRuns))
			for _, previous := range previousRuns {
				run, releaseErr := store.ReleaseSessionRunLease(ctx, previous, release, actor)
				if releaseErr != nil {
					return releaseErr
				}

				outcome := sessionLeaseReleaseOutcome{
					result:   newSessionLeaseReleaseResult(run, previous, release.Reason),
					previous: previous,
				}
				if run.IsNetworkWake() {
					commandResult.outcomes = append(commandResult.outcomes, outcome)
					continue
				}

				taskRecord, transitions, reconcileErr := m.reconcileTaskSettlementCascadeWithStore(
					ctx,
					store,
					run.TaskID,
					actor,
					release.Now,
				)
				if reconcileErr != nil {
					return reconcileErr
				}
				event, eventErr := m.newTaskEventAt(
					run.TaskID,
					run.ID,
					taskEventRunReleased,
					actor,
					release.Now,
					releasedRunPayload{
						PreviousStatus: previous.Status,
						Status:         run.Status,
						TaskStatus:     taskRecord.Status,
						Reason:         release.Reason,
						SessionID:      previous.SessionID,
					},
				)
				if eventErr != nil {
					return eventErr
				}
				if eventErr := store.CreateTaskEvent(ctx, event); eventErr != nil {
					return eventErr
				}

				outcome.task = taskRecord
				commandResult.outcomes = append(commandResult.outcomes, outcome)
				commandResult.events = append(commandResult.events, event)
				commandResult.statusTransitions = append(commandResult.statusTransitions, transitions...)
			}
			return nil
		},
	)
	if err != nil {
		return sessionLeaseReleaseSettlement{}, err
	}
	m.publishLeaseSettlement(ctx, commandResult.events, commandResult.statusTransitions, actor)
	return commandResult, nil
}

func newSessionLeaseReleaseResult(run Run, previous Run, reason string) SessionLeaseReleaseResult {
	return SessionLeaseReleaseResult{
		Run:                    run,
		PreviousRunStatus:      previous.Status,
		PreviousSessionID:      previous.SessionID,
		PreviousLeaseUntil:     previous.LeaseUntil,
		PreviousClaimTokenHash: previous.ClaimTokenHash,
		Reason:                 reason,
	}
}

func (m *Service) failRunLeaseSettlement(
	ctx context.Context,
	failure LeaseFailure,
	actor ActorContext,
) (FailedRunLeaseSettlement, error) {
	commandResult := leaseSettlementCommandResult[FailedRunLeaseSettlement]{}
	err := m.store.WithLeaseSettlementTransaction(
		ctx,
		"fail task run lease",
		func(store LeaseSettlementMutationStore) error {
			mutation, commandErr := store.FailRunLeaseMutation(ctx, failure)
			if commandErr != nil {
				return commandErr
			}

			taskRecord, transitions, commandErr := m.reconcileTaskSettlementCascadeWithStore(
				ctx,
				store,
				mutation.Run.TaskID,
				actor,
				failure.Now,
			)
			if commandErr != nil {
				return commandErr
			}
			event, commandErr := m.newTransactionalTaskEventAt(
				mutation.Run.TaskID,
				mutation.Run.ID,
				taskWatchEventRunFailed,
				actor,
				failure.Now,
				NewRunLeaseFailedEventPayload(&mutation),
			)
			if commandErr != nil {
				return commandErr
			}
			if commandErr := store.CreateTaskEvent(ctx, event); commandErr != nil {
				return commandErr
			}

			commandResult.settlement = FailedRunLeaseSettlement{
				Run:               mutation.Run,
				Task:              taskRecord,
				StatusTransitions: append(mutation.StatusTransitions, transitions...),
			}
			commandResult.events = []Event{event}
			return nil
		},
	)
	if err != nil {
		return FailedRunLeaseSettlement{}, err
	}
	m.publishLeaseSettlement(ctx, commandResult.events, commandResult.settlement.StatusTransitions, actor)
	return commandResult.settlement, nil
}

func (m *Service) publishLeaseSettlement(
	ctx context.Context,
	events []Event,
	transitions []StatusTransition,
	actor ActorContext,
) {
	m.publishTaskEventsAfterCommand(ctx, events)
	for _, transition := range transitions {
		m.dispatchTaskStatusChangedAfterWrite(
			ctx,
			transition.Task,
			transition.PreviousStatus,
			transition.Task.Status,
			actor,
		)
	}
}
