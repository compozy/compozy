package demoseed

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/worktree"
)

const (
	seedWorktreeID     = "wt_northstar_settlement_retry"
	seedWorktreeName   = "settlement-retry"
	seedWorktreeBranch = "demo/settlement-retry"
)

type demoWorktreeResolver struct {
	workspace worktree.Workspace
}

func (r demoWorktreeResolver) ResolveWorktreeWorkspace(
	_ context.Context,
	workspaceID string,
) (worktree.Workspace, error) {
	if workspaceID != r.workspace.ID {
		return worktree.Workspace{}, worktree.ErrNotFound
	}
	return r.workspace, nil
}

func (r demoWorktreeResolver) ListWorktreeWorkspaces(
	_ context.Context,
) ([]worktree.Workspace, error) {
	return []worktree.Workspace{r.workspace}, nil
}

func seedWorktree(ctx context.Context, db *globaldb.GlobalDB, state *scenario) (int, error) {
	record, err := state.recordFor(workspaceKeyPlatform)
	if err != nil {
		return 0, err
	}
	runner, err := worktree.NewRealGitRunner(30 * time.Second)
	if err != nil {
		return 0, fmt.Errorf("demo seed: prepare Git worktree runner: %w", err)
	}
	if err := ensureSeedGitRepository(ctx, runner, record.RootDir); err != nil {
		return 0, err
	}
	settings := config.DefaultWorktreesConfig()
	service := worktree.NewService(
		db.WorktreeStore(),
		runner,
		worktree.WithWorkspaceResolver(demoWorktreeResolver{workspace: worktree.Workspace{
			ID: record.ID, Name: record.Name, Root: record.RootDir,
			Worktrees: settings, WorktreesRoot: settings.ResolveRoot(state.paths),
		}}),
		worktree.WithConfig(settings, settings.ResolveRoot(state.paths)),
		worktree.WithClock(state.clock.Now),
		worktree.WithIDGenerator(func(string) (string, error) { return seedWorktreeID, nil }),
	)
	created, err := service.Create(ctx, record.ID, worktree.CreateOptions{
		Name: seedWorktreeName, Branch: seedWorktreeBranch, BaseRef: "HEAD", Origin: worktree.OriginManual,
	})
	if err != nil {
		return 0, fmt.Errorf("demo seed: create settlement retry worktree: %w", err)
	}
	if created == nil || created.ID != seedWorktreeID || created.State != worktree.StateReady {
		return 0, errors.New("demo seed: worktree service returned an incomplete fixture")
	}
	return 1, nil
}

func ensureSeedGitRepository(ctx context.Context, runner worktree.GitRunner, root string) error {
	if _, stderr, err := runner.Run(ctx, root, "rev-parse", "--is-inside-work-tree"); err != nil {
		if _, initStderr, initErr := runner.Run(ctx, root, "init", "--initial-branch=main"); initErr != nil {
			return fmt.Errorf(
				"demo seed: initialize Git repository: %s: %w",
				strings.TrimSpace(string(initStderr)),
				initErr,
			)
		}
		_ = stderr
	}
	for _, setting := range [][]string{
		{"config", "user.name", "Compozy Demo"},
		{"config", "user.email", "demo@compozy.local"},
		{
			"add", "--all", "--force", "--", ".",
			":(exclude).compozy/demo-seed.json",
			":(exclude).compozy/workspace.toml",
		},
	} {
		if _, stderr, err := runner.Run(ctx, root, setting...); err != nil {
			return fmt.Errorf(
				"demo seed: prepare Git fixture: %s: %w",
				strings.TrimSpace(string(stderr)),
				err,
			)
		}
	}
	status, stderr, err := runner.Run(ctx, root, "diff", "--cached", "--name-only")
	if err != nil {
		return fmt.Errorf(
			"demo seed: inspect Git fixture: %s: %w",
			strings.TrimSpace(string(stderr)),
			err,
		)
	}
	if strings.TrimSpace(string(status)) == "" {
		return nil
	}
	if _, stderr, err := runner.Run(ctx, root, "commit", "--message", "Seed Northstar Pay demo workspace"); err != nil {
		return fmt.Errorf(
			"demo seed: commit Git fixture: %s: %w",
			strings.TrimSpace(string(stderr)),
			err,
		)
	}
	return nil
}

func cleanupSeedWorktree(
	ctx context.Context,
	db *globaldb.GlobalDB,
	state *scenario,
	record workspaceRecord,
) error {
	item, err := db.WorktreeStore().Get(ctx, record.ID, seedWorktreeID)
	if errors.Is(err, worktree.ErrNotFound) || item == nil {
		return nil
	}
	if err != nil {
		return fmt.Errorf("demo seed: inspect prior worktree: %w", err)
	}
	if item.Name != seedWorktreeName || item.Branch != seedWorktreeBranch {
		return errors.New(
			"demo seed: refusing to remove a worktree whose identity is not seed-owned",
		)
	}
	runner, err := worktree.NewRealGitRunner(30 * time.Second)
	if err != nil {
		return fmt.Errorf("demo seed: prepare worktree cleanup: %w", err)
	}
	settings := config.DefaultWorktreesConfig()
	service := worktree.NewService(
		db.WorktreeStore(), runner,
		worktree.WithWorkspaceResolver(demoWorktreeResolver{workspace: worktree.Workspace{
			ID: record.ID, Name: record.Name, Root: record.RootDir,
			Worktrees: settings, WorktreesRoot: settings.ResolveRoot(state.paths),
		}}),
		worktree.WithConfig(settings, settings.ResolveRoot(state.paths)),
	)
	if _, err := service.Remove(ctx, record.ID, seedWorktreeID, true); err != nil {
		return fmt.Errorf("demo seed: remove prior worktree: %w", err)
	}
	if _, stderr, err := runner.Run(
		ctx,
		record.RootDir,
		"branch",
		"--delete",
		"--force",
		seedWorktreeBranch,
	); err != nil {
		return fmt.Errorf(
			"demo seed: remove prior worktree branch: %s: %w",
			strings.TrimSpace(string(stderr)),
			err,
		)
	}
	return nil
}
