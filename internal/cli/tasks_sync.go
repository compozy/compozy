package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	corepkg "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/taskgroups"
	"github.com/spf13/cobra"
)

type tasksSyncHydrator func(context.Context, string, string) ([]string, error)

type tasksSyncCommandState struct {
	hydrate tasksSyncHydrator
}

func newTasksSyncCommand() *cobra.Command {
	state := &tasksSyncCommandState{hydrate: corepkg.HydratePlanCompletion}
	command := &cobra.Command{
		Use:          "sync [initiative]",
		Short:        "Refresh task-group completion from global.db",
		SilenceUsage: true,
		Args:         cobra.MaximumNArgs(1),
		Long: `Project authoritative task-group completion from global.db into the
current workspace's _task_groups.md files. Existing checked boxes are never cleared.

With no initiative argument, every task-group initiative in the workspace is refreshed.`,
		Example: `  compozy tasks sync
  compozy tasks sync my-initiative`,
		RunE: state.run,
	}
	return command
}

func (s *tasksSyncCommandState) run(command *cobra.Command, args []string) error {
	ctx, stop := signalCommandContext(command)
	defer stop()

	workspaceRoot, err := discoverWorkspaceRoot(ctx)
	if err != nil {
		return withExitCode(2, fmt.Errorf("discover tasks sync workspace: %w", err))
	}
	initiatives, err := taskGroupSyncInitiatives(workspaceRoot, args)
	if err != nil {
		return withExitCode(1, err)
	}
	hydrate := s.hydrate
	if hydrate == nil {
		hydrate = corepkg.HydratePlanCompletion
	}

	totalMarked := 0
	statuses := make([]string, 0, len(initiatives))
	for _, initiative := range initiatives {
		marked, hydrateErr := hydrate(ctx, workspaceRoot, initiative)
		if hydrateErr != nil {
			return withExitCode(
				1,
				fmt.Errorf("sync task-group completion for %s: %w", initiative, hydrateErr),
			)
		}
		totalMarked += len(marked)
		if len(marked) == 0 {
			statuses = append(statuses, initiative+": up to date")
			continue
		}
		statuses = append(statuses, initiative+": marked "+strings.Join(marked, ", "))
	}

	if _, err := fmt.Fprintf(
		command.OutOrStdout(),
		"Completion sync workspace: %s\nInitiatives checked: %d\nNewly marked: %d\n",
		workspaceRoot,
		len(initiatives),
		totalMarked,
	); err != nil {
		return withExitCode(2, fmt.Errorf("write tasks sync output: %w", err))
	}
	for _, status := range statuses {
		if _, err := fmt.Fprintln(command.OutOrStdout(), status); err != nil {
			return withExitCode(2, fmt.Errorf("write tasks sync status: %w", err))
		}
	}
	return nil
}

func taskGroupSyncInitiatives(workspaceRoot string, args []string) ([]string, error) {
	if len(args) == 1 {
		initiative := strings.TrimSpace(args[0])
		if initiative == "" ||
			filepath.Base(initiative) != initiative ||
			!model.IsActiveWorkflowDirName(initiative) {
			return nil, fmt.Errorf("invalid task-group initiative %q", args[0])
		}
		return []string{initiative}, nil
	}

	tasksRoot := model.TasksBaseDirForWorkspace(workspaceRoot)
	entries, err := os.ReadDir(tasksRoot)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read task workflow root: %w", err)
	}
	initiatives := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() || !model.IsActiveWorkflowDirName(entry.Name()) {
			continue
		}
		planPath := filepath.Join(tasksRoot, entry.Name(), taskgroups.ManifestFileName)
		if _, err := os.Stat(planPath); err == nil {
			initiatives = append(initiatives, entry.Name())
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("inspect task-group plan %s: %w", planPath, err)
		}
	}
	sort.Strings(initiatives)
	return initiatives, nil
}
