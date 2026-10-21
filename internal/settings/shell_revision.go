package settings

import (
	"context"
	"fmt"
	"strconv"
)

const shellScopeRevisionKey = "shell.sessions.scope_revision"

func (s *service) shellScopeRevision(ctx context.Context) (uint64, error) {
	if s.shellScopeRevisions == nil {
		return 0, nil
	}
	value, exists, err := s.shellScopeRevisions.GetAppMetadata(ctx, shellScopeRevisionKey)
	if err != nil {
		return 0, err
	}
	if !exists {
		return 0, nil
	}
	revision, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("settings: invalid legacy scope revision: %w", err)
	}
	return revision, nil
}

func (s *service) bumpShellScopeRevision(ctx context.Context) error {
	if s.shellScopeRevisions == nil {
		return nil
	}
	revision, err := s.shellScopeRevision(ctx)
	if err != nil {
		return err
	}
	if revision == ^uint64(0) {
		return fmt.Errorf("settings: legacy scope revision exhausted")
	}
	return s.shellScopeRevisions.SetAppMetadata(ctx, shellScopeRevisionKey, strconv.FormatUint(revision+1, 10))
}
