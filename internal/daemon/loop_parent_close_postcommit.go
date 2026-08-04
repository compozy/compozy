package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	taskpkg "github.com/compozy/compozy/internal/task"
)

type loopParentClosePostCommit struct {
	state *bootState
}

var _ taskpkg.CoordinatorPostCommitHandler = loopParentClosePostCommit{}

func (h loopParentClosePostCommit) ApplyCoordinatorPostCommit(
	ctx context.Context,
	specs []taskpkg.CoordinatorParentCloseSpec,
	actor taskpkg.ActorContext,
) error {
	if len(specs) == 0 {
		return nil
	}
	if h.state == nil {
		return errors.New("daemon: Loop parent-close state is unavailable")
	}
	api, ok := h.state.deps.Loops.(*daemonLoopAPIService)
	if !ok || api == nil || api.aggregate == nil {
		return errors.New("daemon: Loop parent-close service is unavailable")
	}
	var errs []error
	for _, raw := range specs {
		spec := raw.Normalize()
		reason := "parent Loop run " + spec.ParentLoopRunID + " closed"
		workspaceID, err := parentCloseWorkspace(ctx, api, spec)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		switch dsl.ParentClosePolicy(spec.Policy) {
		case dsl.ParentCloseTerminate:
			err = api.aggregate.KillRun(
				ctx,
				workspaceID,
				looppkg.RunID(spec.ChildLoopRunID),
				reason,
				actor,
			)
		case dsl.ParentCloseCancel:
			err = api.aggregate.CancelRun(
				ctx,
				workspaceID,
				looppkg.RunID(spec.ChildLoopRunID),
				reason,
				actor,
			)
		default:
			err = fmt.Errorf("%w: invalid parent-close policy %q", looppkg.ErrValidation, spec.Policy)
		}
		if err != nil {
			errs = append(
				errs,
				fmt.Errorf("apply parent-close %s to child %q: %w", spec.Policy, spec.ChildLoopRunID, err),
			)
		}
	}
	return errors.Join(errs...)
}

func parentCloseWorkspace(
	ctx context.Context,
	api *daemonLoopAPIService,
	spec taskpkg.CoordinatorParentCloseSpec,
) (looppkg.WorkspaceID, error) {
	run, err := api.persistence.GetLoopRunByID(ctx, looppkg.RunID(spec.ChildLoopRunID))
	if err != nil {
		return "", fmt.Errorf("load parent-close child %q: %w", spec.ChildLoopRunID, err)
	}
	if strings.TrimSpace(string(run.WorkspaceID)) == "" {
		return "", fmt.Errorf("%w: parent-close child %q has no workspace", looppkg.ErrValidation, spec.ChildLoopRunID)
	}
	return run.WorkspaceID, nil
}
