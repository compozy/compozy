package globaldb

import (
	"github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

func goalSessionBindingFromGenerated(row sqlcgen.LoopSessionBinding) goal.SessionBinding {
	binding := goal.SessionBinding{
		Key: goal.BindingKey{WorkspaceID: loop.WorkspaceID(row.WorkspaceID), LoopRunID: loop.RunID(row.LoopRunID),
			Handle: row.Handle},
		BindingEpoch: row.BindingEpoch, BindingAttemptID: row.BindingAttemptID, SessionID: row.SessionID,
		CreationProfileRef: row.CreationProfileRef, PolicySpecDigest: row.PolicySpecDigest,
		CreationDigest: row.CreationDigest, Ownership: goal.BindingOwnership(row.Ownership),
		State: goal.BindingState(row.State), FailureCode: row.FailureCode.String,
		CreatedAt: row.CreatedAt.UTC(), AdoptedGeneration: int(row.AdoptedGeneration),
		AdoptionAttemptID: row.AdoptionAttemptID.String,
	}
	if row.ActivatedAt.Valid {
		value := row.ActivatedAt.Time.UTC()
		binding.ActivatedAt = &value
	}
	if row.FailedAt.Valid {
		value := row.FailedAt.Time.UTC()
		binding.FailedAt = &value
	}
	if row.ClosedAt.Valid {
		value := row.ClosedAt.Time.UTC()
		binding.ClosedAt = &value
	}
	return binding
}
