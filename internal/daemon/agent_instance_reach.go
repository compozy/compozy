package daemon

import (
	"context"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

type instanceWorkspaceReach struct {
	policy workspaceaccess.Policy
}

var _ instanceReach = instanceWorkspaceReach{}

func (r instanceWorkspaceReach) AuthorizeInstanceWorkspace(
	ctx context.Context,
	actor workspaceaccess.ActorRef,
	workspaceID, profileID string,
) error {
	if r.policy == nil {
		return agentinstance.ErrForbidden
	}
	decision, err := r.policy.Authorize(ctx, workspaceaccess.Request{
		Actor:  actor,
		Target: workspaceaccess.ScopeTarget(workspaceID, profileID),
		Seam:   workspaceaccess.SeamSpawn,
	})
	if err != nil {
		return err
	}
	if !decision.Allowed {
		return agentinstance.ErrForbidden
	}
	return nil
}
