package globaldb

import (
	"context"
	"database/sql"
	"errors"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// Resolve and fence the address under the same transaction as the receipt and
// queue row. The committed row then participates in the existing rotation gate.
func checkInstancePromptTarget(
	ctx context.Context,
	exec globalSQLExecutor,
	req store.SessionPromptAdmissionRequest,
) error {
	if req.InstanceTarget == nil {
		return nil
	}
	target := req.InstanceTarget
	row, err := sqlcgen.New(exec).FindLiveAgentInstance(ctx, sqlcgen.FindLiveAgentInstanceParams{
		ProfileID: target.ProfileID, WorkspaceID: target.WorkspaceID, AgentName: target.AgentName,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return agentinstance.ErrNotFound
	}
	if err != nil {
		return err
	}
	if row.MainSessionID.String == "" {
		return agentinstance.ErrStarting
	}
	if row.MainSessionID.String != req.SessionID || row.WorkspaceID != req.WorkspaceID {
		return store.ErrInstancePromptTargetChanged
	}
	return nil
}

// applyInstancePromptDelivery records the admission-time hold, preserving human ingress exemptions.
func applyInstancePromptDelivery(
	ctx context.Context,
	exec globalSQLExecutor,
	req store.SessionPromptAdmissionRequest,
	receipt *store.SessionPromptAdmissionResult,
) error {
	if len(req.Origin) == 0 || req.HumanIngress || req.HumanSpawnOrigin {
		return nil
	}
	hold, err := sqlcgen.New(exec).GetSessionInputHold(ctx, req.SessionID)
	if errors.Is(err, sql.ErrNoRows) {
		caller, callerErr := sqlcgen.New(exec).GetInstanceCaller(ctx, req.SessionID)
		if callerErr != nil {
			return callerErr
		}
		if id, ok := caller.OriginatingInstanceID.(string); ok && id != "" {
			receipt.Delivery = inputStatusQueued
		}
		return nil
	}
	if err != nil {
		return err
	}
	receipt.Delivery = "held"
	receipt.HeldReason = hold.PauseReason
	receipt.HeldInstanceID = hold.ID
	return nil
}
