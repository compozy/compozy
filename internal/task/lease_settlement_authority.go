package task

import "fmt"

// RequireLeaseSettlementActor keeps daemon-owned Loop workers under the
// daemon lease owner's sole completion and failure authority.
func RequireLeaseSettlementActor(run Run, actor ActorContext) error {
	if !run.IsLoopWorker() || run.ClaimedBy == nil || run.ClaimedBy.Kind.Normalize() != ActorKindDaemon {
		return nil
	}
	if sameActorIdentity(*run.ClaimedBy, actor.Actor) {
		return nil
	}
	return fmt.Errorf(
		"%w: daemon-owned Loop worker run %q may only be settled by daemon owner %q",
		ErrPermissionDenied,
		run.ID,
		run.ClaimedBy.Ref,
	)
}
