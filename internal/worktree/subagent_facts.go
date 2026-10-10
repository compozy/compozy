package worktree

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// CommitsAheadOf fails closed: an unreadable base must never authorize cleanup.
func (s *Service) CommitsAheadOf(ctx context.Context, workspaceID, worktreeID, base string) (int, error) {
	item, err := s.Get(ctx, workspaceID, worktreeID)
	if err != nil {
		return 0, err
	}
	if strings.TrimSpace(base) == "" {
		return 0, ErrBaseRefNotFound
	}
	stdout, _, err := s.runner.Run(ctx, item.Path, "rev-list", "--count", base+"..HEAD")
	if err != nil {
		return 0, fmt.Errorf("worktree: count commits ahead: %w", err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(stdout)))
	if err != nil || n < 0 {
		return 0, fmt.Errorf("worktree: invalid ahead count %q", stdout)
	}
	return n, nil
}
