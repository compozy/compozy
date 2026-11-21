package cli

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

const (
	archiveStoppedFlagName = "stopped"
	archiveDryRunFlagName  = "dry-run"
)

// archiveStoppedConflicts are the single-session archive flags --stopped excludes.
var archiveStoppedConflicts = []string{"stop", "with-children", "terminate-processes", "keep-processes"}

// sessionArchiveStoppedFlags are the bulk options of `session archive --stopped`.
type sessionArchiveStoppedFlags struct {
	stopped, allWorkspaces, dryRun bool
}

func bindSessionArchiveStoppedFlags(cmd *cobra.Command, flags *sessionArchiveStoppedFlags) {
	cmd.Flags().BoolVar(&flags.stopped, archiveStoppedFlagName, false,
		"Archive every stopped thread of the project instead of one session; running, waiting, queued, "+
			"pinned, Later, and main-instance threads are skipped")
	cmd.Flags().String(workspaceFlagName, "",
		"With --stopped: workspace ID, name, or path (defaults to the current workspace)")
	cmd.Flags().BoolVar(&flags.allWorkspaces, allWorkspacesFlagName, false,
		"With --stopped: archive stopped threads in every project")
	cmd.Flags().BoolVar(&flags.dryRun, archiveDryRunFlagName, false, "With --stopped: preview what would be archived")
	cmd.MarkFlagsMutuallyExclusive(workspaceFlagName, allWorkspacesFlagName)
}

// sessionArchiveArgs requires the session ID for a single archive and refuses it with --stopped.
func sessionArchiveArgs(flags *sessionArchiveStoppedFlags) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if !flags.stopped {
			return exactOneNonBlankArg()(cmd, args)
		}
		return sessionArchiveStoppedArgs(cmd, args)
	}
}

func sessionArchiveStoppedArgs(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return newCLIUsageError("cli: --stopped archives a whole project; drop the session ID")
	}
	for _, name := range archiveStoppedConflicts {
		if cmd.Flags().Changed(name) {
			return newCLIUsageError(fmt.Sprintf("cli: --stopped cannot be combined with --%s", name))
		}
	}
	return nil
}

func sessionArchiveStoppedOnlyFlagError(cmd *cobra.Command) error {
	for _, name := range []string{workspaceFlagName, allWorkspacesFlagName, archiveDryRunFlagName} {
		if cmd.Flags().Changed(name) {
			return newCLIUsageError(fmt.Sprintf("cli: --%s requires --stopped", name))
		}
	}
	return nil
}

func runSessionArchiveStopped(cmd *cobra.Command, deps commandDeps, flags sessionArchiveStoppedFlags) error {
	base, err := clientFromDeps(deps)
	if err != nil {
		return err
	}
	client, ok := base.(sessionArchiveStoppedClient)
	if !ok {
		return errors.New("cli: archive stopped is unavailable")
	}
	request := contract.SessionArchiveStoppedRequest{AllWorkspaces: flags.allWorkspaces, DryRun: flags.dryRun}
	scope := "all projects"
	if !flags.allWorkspaces {
		workspaceRef, flagErr := commandWorkspaceFlag(cmd)
		if flagErr != nil {
			return withCommandExitCode(2, flagErr)
		}
		resolution, resolveErr := resolveCommandWorkspace(
			cmd.Context(), cmd, deps, base, workspaceResolutionRequest{FlagRef: workspaceRef},
		)
		if resolveErr != nil {
			return resolveErr
		}
		request.WorkspaceID = resolution.ID
		scope = cmp.Or(strings.TrimSpace(resolution.Detail.Workspace.Name), resolution.ID)
	}
	response, err := client.ArchiveStoppedSessions(cmd.Context(), request)
	if err != nil {
		return err
	}
	if err := writeCommandOutput(cmd, sessionArchiveStoppedBundle(response, scope)); err != nil {
		return err
	}
	if len(response.Failures) > 0 {
		return withCommandExitCode(1, errors.New("cli: some threads failed to archive"))
	}
	return nil
}

func sessionArchiveStoppedBundle(response contract.SessionArchiveStoppedResponse, scope string) outputBundle {
	return outputBundle{
		// -o json is the HTTP response DTO unchanged, without a resolution envelope.
		json: func(cmd *cobra.Command) error {
			return writeJSONWithoutWorkspaceResolution(cmd, response)
		},
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLineWithoutWorkspaceResolution(cmd, response)
		},
		human: func() (string, error) {
			return renderSessionArchiveStoppedHuman(response, scope), nil
		},
		toon: func() (string, error) {
			skipped := 0
			for _, count := range response.SkipCounts {
				skipped += count
			}
			return renderToonObject(
				"archive_stopped",
				[]string{"dry_run", "archived_threads", "archived_sessions", "skipped_threads", "failed_threads"},
				[]string{
					strconv.FormatBool(response.DryRun),
					strconv.Itoa(response.ArchivedThreadCount),
					strconv.Itoa(response.ArchivedSessionCount),
					strconv.Itoa(skipped),
					strconv.Itoa(len(response.Failures)),
				},
			), nil
		},
	}
}

func renderSessionArchiveStoppedHuman(response contract.SessionArchiveStoppedResponse, scope string) string {
	verb := "Archived"
	if response.DryRun {
		verb = "Would archive"
	}
	lines := []string{fmt.Sprintf("%s %d %s (%d %s) in %s.",
		verb,
		response.ArchivedThreadCount, threadNoun(response.ArchivedThreadCount),
		response.ArchivedSessionCount, sessionNoun(response.ArchivedSessionCount),
		scope,
	)}
	if skips := sessionArchiveStoppedSkipSummary(response.SkipCounts); skips != "" {
		lines = append(lines, skips)
	}
	if len(response.Failures) > 0 {
		lines = append(lines, fmt.Sprintf("Failed %d %s:", len(response.Failures), threadNoun(len(response.Failures))))
		for _, failure := range response.Failures {
			lines = append(
				lines,
				fmt.Sprintf("  %s %s: %s", failure.RootID, stringOrDash(failure.Title), failure.Message),
			)
		}
		if response.FailuresTruncated {
			lines = append(lines, "  (more failures not shown)")
		}
	}
	return strings.Join(lines, "\n")
}

// sessionArchiveStoppedSkipSummary renders "Skipped 3 threads: 2 waiting_on_you, 1 running.",
// largest reason first.
func sessionArchiveStoppedSkipSummary(counts map[string]int) string {
	type reasonCount struct {
		reason string
		count  int
	}
	reasons := make([]reasonCount, 0, len(counts))
	total := 0
	for reason, count := range counts {
		if count > 0 {
			reasons = append(reasons, reasonCount{reason: reason, count: count})
			total += count
		}
	}
	if total == 0 {
		return ""
	}
	slices.SortFunc(reasons, func(a, b reasonCount) int {
		return cmp.Or(cmp.Compare(b.count, a.count), cmp.Compare(a.reason, b.reason))
	})
	parts := make([]string, 0, len(reasons))
	for _, item := range reasons {
		parts = append(parts, fmt.Sprintf("%d %s", item.count, item.reason))
	}
	return fmt.Sprintf("Skipped %d %s: %s.", total, threadNoun(total), strings.Join(parts, ", "))
}

func threadNoun(count int) string {
	if count == 1 {
		return "thread"
	}
	return "threads"
}
