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

func instancePromptDelivery(
	ctx context.Context,
	exec globalSQLExecutor,
	req store.SessionPromptAdmissionRequest,
) (string, error) {
	if len(req.Origin) == 0 || req.HumanIngress || req.HumanSpawnOrigin {
		return "", nil
	}
	q := sqlcgen.New(exec)
	caller, err := q.GetInstanceCaller(ctx, req.SessionID)
	if err != nil {
		return "", err
	}
	id, ok := caller.OriginatingInstanceID.(string)
	if !ok || id == "" {
		return "", nil
	}
	instance, err := q.GetAgentInstance(ctx, id)
	if err != nil {
		return "", err
	}
	if instance.State == goalStatusPaused {
		return "held", nil
	}
	return "queued", nil
}
