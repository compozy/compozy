package worktree

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/diagnostics"
)

// gitWorktreeAddCommand is the `git worktree` subcommand that writes a checkout.
const gitWorktreeAddCommand = "add"

// errCreationInterrupted names a pending creation the daemon restart cut short.
var errCreationInterrupted = errors.New("creation interrupted by daemon restart")

// addCheckout runs `git worktree add` under worktrees.checkout_timeout rather
// than the runner's short per-command default: checkout time scales with the
// tree, and a killed add leaves a locked, half-written worktree to roll back.
func (s *Service) addCheckout(ctx context.Context, workspace Workspace, item Worktree) error {
	timeout := s.checkoutTimeout(workspace)
	args := []string{worktreeDescriptorName, gitWorktreeAddCommand, item.Path, item.Branch}
	var stderr []byte
	var err error
	if runner, ok := s.runner.(GitTimeoutRunner); ok {
		_, stderr, err = runner.RunWithTimeout(ctx, workspace.Root, timeout, args...)
	} else {
		_, stderr, err = s.runner.Run(ctx, workspace.Root, args...)
	}
	if err == nil {
		return nil
	}
	detail := diagnostics.RedactAndBound(string(stderr), 2048)
	// Only the runner's own budget names the key; a parent deadline keeps the generic error.
	if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
		return fmt.Errorf(
			"worktree: checkout exceeded worktrees.checkout_timeout (%s); raise it for large repositories: %s: %w",
			timeout, detail, err,
		)
	}
	return fmt.Errorf("worktree: add checkout: %s: %w", detail, err)
}

func (s *Service) checkoutTimeout(workspace Workspace) time.Duration {
	settings, _ := s.worktreeSettings(workspace)
	if settings.CheckoutTimeout > 0 {
		return settings.CheckoutTimeout
	}
	return config.DefaultWorktreesConfig().CheckoutTimeout
}
