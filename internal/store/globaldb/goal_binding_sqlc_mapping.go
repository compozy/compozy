package globaldb

import (
	"github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/goal"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
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
		binding.ActivatedAt = new(row.ActivatedAt.Time.UTC())
	}
	if row.FailedAt.Valid {
		binding.FailedAt = new(row.FailedAt.Time.UTC())
	}
	if row.ClosedAt.Valid {
		binding.ClosedAt = new(row.ClosedAt.Time.UTC())
	}
	return binding
}
