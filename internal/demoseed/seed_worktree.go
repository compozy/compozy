package demoseed

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/worktree"
)

const (
	seedWorktreeID     = "wt_northstar_settlement_retry"
	seedWorktreeName   = "settlement-retry"
	seedWorktreeBranch = retryBudgetBranch
	// legacyWorktreeBranch is the branch earlier seeds created; replace still owns it.
	legacyWorktreeBranch   = "demo/settlement-retry"
	exportRenameWorktreeID = "wt_northstar_export_rename"
)

// seedWorktreeFixture is one worktree the scenario's sessions work in.
type seedWorktreeFixture struct{ id, name, branch string }

func seedWorktreeFixtures() []seedWorktreeFixture {
	return []seedWorktreeFixture{
		{seedWorktreeID, seedWorktreeName, seedWorktreeBranch},
		{exportRenameWorktreeID, exportRenameBranch, exportRenameBranch},
	}
}

type demoWorktreeResolver struct {
	workspace worktree.Workspace
}

func (r demoWorktreeResolver) ResolveWorktreeWorkspace(
	_ context.Context,
	workspaceID string,
	_ string,
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
	fixtures := seedWorktreeFixtures()
	settings := config.DefaultWorktreesConfig()
	for _, fixture := range fixtures {
		service := worktree.NewService(
			db.WorktreeStore(),
			runner,
			worktree.WithWorkspaceResolver(demoWorktreeResolver{workspace: worktree.Workspace{
				ID: record.ID, Name: record.Name, Root: record.RootDir,
				Worktrees: settings, WorktreesRoot: settings.ResolveRoot(state.paths),
			}}),
			worktree.WithConfig(settings, settings.ResolveRoot(state.paths)),
			worktree.WithClock(state.clock.Now),
			worktree.WithIDGenerator(func(string) (string, error) { return fixture.id, nil }),
		)
		created, err := service.Create(ctx, record.ID, worktree.CreateOptions{
			ProfileID: store.DefaultProfileID, Name: fixture.name, Branch: fixture.branch,
			BaseRef: "HEAD", Origin: worktree.OriginManual,
		})
		if err != nil {
			return 0, fmt.Errorf("demo seed: create worktree %q: %w", fixture.name, err)
		}
		if created == nil || created.ID != fixture.id || created.State != worktree.StateReady {
			return 0, errors.New("demo seed: worktree service returned an incomplete fixture")
		}
	}
	// The executor already finished in this worktree, so its session is bound to it.
	ended := state.clock.daysHoursAgo(2, 5)
	if err := bindSessionWorktree(
		ctx, db, state, record, sessionExportRenameID, exportRenameWorktreeID, ended,
	); err != nil {
		return 0, err
	}
	return len(fixtures), nil
}

func ensureSeedGitRepository(ctx context.Context, runner worktree.GitRunner, root string) error {
	// A rev-parse failure means the fixture root is not a Git repository yet.
	if _, _, err := runner.Run(ctx, root, "rev-parse", "--is-inside-work-tree"); err != nil {
		if _, initStderr, initErr := runner.Run(ctx, root, "init", "--initial-branch=main"); initErr != nil {
			return fmt.Errorf(
				"demo seed: initialize Git repository: %s: %w",
				strings.TrimSpace(string(initStderr)),
				initErr,
			)
		}
	}
	// The origin gives the workspace the hosted repository identity linked pull requests resolve to.
	if _, _, err := runner.Run(ctx, root, "remote", "get-url", "origin"); err != nil {
		if _, stderr, addErr := runner.Run(
			ctx,
			root,
			"remote",
			"add",
			"origin",
			northstarRepoURL+".git",
		); addErr != nil {
			return fmt.Errorf("demo seed: add Git origin: %s: %w", strings.TrimSpace(string(stderr)), addErr)
		}
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
	for _, fixture := range seedWorktreeFixtures() {
		if err := cleanupWorktreeFixture(ctx, db, state, record, fixture); err != nil {
			return err
		}
	}
	return nil
}

func cleanupWorktreeFixture(
	ctx context.Context,
	db *globaldb.GlobalDB,
	state *scenario,
	record workspaceRecord,
	fixture seedWorktreeFixture,
) error {
	item, err := db.WorktreeStore().Get(ctx, record.ID, fixture.id)
	if errors.Is(err, worktree.ErrNotFound) || item == nil {
		return nil
	}
	if err != nil {
		return fmt.Errorf("demo seed: inspect prior worktree: %w", err)
	}
	if item.Name != fixture.name || (item.Branch != fixture.branch && item.Branch != legacyWorktreeBranch) {
		return errors.New("demo seed: refusing to remove a worktree whose identity is not seed-owned")
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
	if _, err := service.Remove(ctx, record.ID, fixture.id, true); err != nil {
		return fmt.Errorf("demo seed: remove prior worktree: %w", err)
	}
	if _, stderr, err := runner.Run(ctx, record.RootDir, "branch", "--delete", "--force", item.Branch); err != nil {
		return fmt.Errorf("demo seed: remove prior worktree branch: %s: %w", strings.TrimSpace(string(stderr)), err)
	}
	return nil
}
