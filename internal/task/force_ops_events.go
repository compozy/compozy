package task

import "math"

func (m *Service) preflightForceReleaseRun(
	previous Run,
	release ForceReleaseRun,
	actor ActorContext,
) error {
	return m.preflightTaskEvent(
		previous.TaskID,
		previous.ID,
		taskEventRunReleased,
		actor,
		forceReleaseRunPayload(
			previous,
			TaskRunStatusQueued,
			TaskStatusNeedsAttention,
			release,
			math.MaxInt64,
			math.MaxInt,
			actor,
		),
	)
}

func forceReleaseRunPayload(
	previous Run,
	status RunStatus,
	taskStatus Status,
	release ForceReleaseRun,
	queueGeneration int64,
	canceledInputs int,
	actor ActorContext,
) releasedRunPayload {
	return releasedRunPayload{
		Manual:                          true,
		ActorKind:                       actor.Actor.Kind.Normalize(),
		ActorID:                         actor.Actor.Ref,
		PreviousStatus:                  previous.Status,
		Status:                          status,
		TaskStatus:                      taskStatus,
		Reason:                          release.Reason,
		SessionID:                       previous.SessionID,
		PreviousSessionID:               previous.SessionID,
		PreviousClaimTokenHashTruncated: truncateClaimTokenHash(previous.ClaimTokenHash),
		PreviousLeaseUntil:              optionalPayloadTime(previous.LeaseUntil),
		QueueGeneration:                 queueGeneration,
		CanceledQueuedInputs:            canceledInputs,
		Metadata:                        cloneRawJSON(release.Metadata),
	}
}

func (m *Service) preflightForceFailRun(previous Run, failure ForceFailRun, actor ActorContext) error {
	return m.preflightTaskEvent(
		previous.TaskID,
		previous.ID,
		taskEventRunOperatorForcedFail,
		actor,
		forceFailRunPayload(
			previous,
			TaskRunStatusFailed,
			TaskStatusNeedsAttention,
			failure,
			math.MaxInt64,
			math.MaxInt,
			actor,
		),
	)
}

func forceFailRunPayload(
	previous Run,
	status RunStatus,
	taskStatus Status,
	failure ForceFailRun,
	queueGeneration int64,
	canceledInputs int,
	actor ActorContext,
) operatorForcedFailPayload {
	return operatorForcedFailPayload{
		Manual:               true,
		ActorKind:            actor.Actor.Kind.Normalize(),
		ActorID:              actor.Actor.Ref,
		PreviousStatus:       previous.Status,
		Status:               status,
		TaskStatus:           taskStatus,
		Reason:               failure.Reason,
		SessionID:            previous.SessionID,
		QueueGeneration:      queueGeneration,
		CanceledQueuedInputs: canceledInputs,
		Metadata:             cloneRawJSON(failure.Metadata),
	}
}

func (m *Service) preflightRetryRun(source Run, newRunID string, actor ActorContext) error {
	return m.preflightTaskEvent(
		source.TaskID,
		newRunID,
		taskEventRunOperatorRetry,
		actor,
		operatorRetryPayload{
			Manual:      true,
			ActorKind:   actor.Actor.Kind.Normalize(),
			ActorID:     actor.Actor.Ref,
			SourceRunID: source.ID,
			NewRunID:    newRunID,
			TaskStatus:  TaskStatusNeedsAttention,
		},
	)
}

func (m *Service) preflightRecoverRun(
	source Run,
	newRunID string,
	recovery RecoverRunRequest,
	actor ActorContext,
) error {
	return m.preflightTaskEvent(
		source.TaskID,
		newRunID,
		taskEventRunRecoveredFromAttention,
		actor,
		recoveredFromAttentionEventPayload(
			source,
			newRunID,
			TaskRunStatusQueued,
			TaskStatusNeedsAttention,
			recovery.Reason,
			math.MaxInt64,
			math.MaxInt,
			actor,
		),
	)
}

func recoveredFromAttentionEventPayload(
	source Run,
	newRunID string,
	status RunStatus,
	taskStatus Status,
	reason string,
	queueGeneration int64,
	canceledInputs int,
	actor ActorContext,
) recoveredFromAttentionPayload {
	return recoveredFromAttentionPayload{
		Manual:               true,
		ActorKind:            actor.Actor.Kind.Normalize(),
		ActorID:              actor.Actor.Ref,
		SourceRunID:          source.ID,
		NewRunID:             newRunID,
		PreviousStatus:       TaskRunStatusNeedsAttention,
		Status:               status,
		TaskStatus:           taskStatus,
		Reason:               reason,
		SessionID:            source.SessionID,
		QueueGeneration:      queueGeneration,
		CanceledQueuedInputs: canceledInputs,
	}
}
