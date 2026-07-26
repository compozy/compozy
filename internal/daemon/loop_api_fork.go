package daemon

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	looppkg "github.com/compozy/agh/internal/loop"
)

const loopForkRollbackTimeout = 5 * time.Second

func (s *daemonLoopAPIService) forkLoop(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	sourcePath string,
	targetRoot string,
	name string,
) (_ contract.LoopResponse, err error) {
	path, err := looppkg.ForkDefinitionFile(sourcePath, targetRoot)
	if err != nil {
		return contract.LoopResponse{}, err
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		if rollbackErr := s.rollbackLoopFork(ctx, targetRoot, name); rollbackErr != nil {
			err = errors.Join(err, rollbackErr)
		}
	}()

	if err := s.syncLoopResources(ctx); err != nil {
		return contract.LoopResponse{}, err
	}
	response, err := s.loopResponseFromDefinitionFile(ctx, path, workspaceID, name)
	if err != nil {
		return contract.LoopResponse{}, err
	}
	committed = true
	return response, nil
}

func (s *daemonLoopAPIService) rollbackLoopFork(
	ctx context.Context,
	targetRoot string,
	name string,
) error {
	rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), loopForkRollbackTimeout)
	defer cancel()

	if err := looppkg.DeleteWritableDefinition(targetRoot, name); err != nil {
		return fmt.Errorf("daemon: remove failed loop fork %q: %w", name, err)
	}
	if err := s.syncLoopResources(rollbackCtx); err != nil {
		return fmt.Errorf("daemon: reconcile rolled back loop fork %q: %w", name, err)
	}
	return nil
}
