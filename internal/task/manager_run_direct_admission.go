package task

import (
	"context"
	"fmt"
	"strings"
)

func (m *Service) admitQueuedRunForDirectExecution(
	ctx context.Context,
	run Run,
	actor ActorContext,
) (Run, Task, error) {
	if err := requireWriteAuthority(actor); err != nil {
		return Run{}, Task{}, err
	}
	if run.Status.Normalize() != TaskRunStatusQueued ||
		run.RunKind.Normalize() != RunKindWorker ||
		strings.TrimSpace(run.LoopRunID) != "" ||
		!run.IsTaskAnchored() {
		return Run{}, Task{}, fmt.Errorf(
			"%w: task run %q cannot enter direct execution from %q",
			ErrInvalidStatusTransition,
			run.ID,
			run.Status.Normalize(),
		)
	}

	commandAt := m.now().UTC()
	settlement, err := m.commitNominalRunSettlement(
		ctx,
		"admit task run direct execution",
		taskEventRunClaimed,
		actor,
		commandAt,
		func(store runMutationStore) (NominalRunMutationResult, error) {
			return store.AdmitRunDirectExecution(
				ctx,
				NewRunDirectExecutionAdmissionMutation(run, actor, commandAt),
			)
		},
		directRunClaimedPayload,
	)
	if err != nil {
		return Run{}, Task{}, err
	}
	m.dispatchTaskRunPostClaim(ctx, settlement.mutation.Run, settlement.task, actor)
	return settlement.mutation.Run, settlement.task, nil
}

func directRunClaimedPayload(mutation *NominalRunMutationResult, taskRecord Task) any {
	claimedBy := ActorIdentity{}
	if mutation.Run.ClaimedBy != nil {
		claimedBy = *mutation.Run.ClaimedBy
	}
	return runClaimedPayload{
		Status:         mutation.Run.Status,
		TaskStatus:     taskRecord.Status,
		ClaimedBy:      claimedBy,
		ClaimTokenHash: mutation.Run.ClaimTokenHash,
		LeaseUntil:     mutation.Run.LeaseUntil,
	}
}
