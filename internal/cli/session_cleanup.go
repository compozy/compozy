package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

const defaultCleanupOlderThanDays = 7

type sessionCleanupFlags struct {
	everything    bool
	children      bool
	subagents     bool
	olderThan     int
	before        string
	anyTime       bool
	allWorkspaces bool
	sessionIDs    []string
	yes           bool
}

func newSessionCleanupCommand(deps commandDeps) *cobra.Command {
	flags := sessionCleanupFlags{}
	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Delete archived sessions in bulk, in the background",
		Long: "Plan a cleanup of the profile's archived sessions and, with --yes, delete them in the daemon's " +
			"background. Without --yes it only previews what would go. Only archived sessions are deleted: " +
			"child sessions still in the session list or in Later stay, with everything under them. " +
			"Worktrees stay, as with a single delete. By default it picks threads archived more than 7 days " +
			"ago in the current workspace and deletes them whole.",
		Example: `  # Preview what a cleanup of the current workspace would delete
  compozy session cleanup

  # Delete every thread archived more than 30 days ago, across all workspaces
  compozy session cleanup --older-than 30 --all-workspaces --yes

  # Keep main sessions; delete only their archived subagents
  compozy session cleanup --subagents --any-time --yes

  # Follow or stop the running cleanup
  compozy session cleanup status
  compozy session cleanup stop`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runSessionCleanup(cmd, deps, flags)
		},
	}
	bindSessionCleanupFlags(cmd, &flags)
	cmd.AddCommand(newSessionCleanupStatusCommand(deps))
	cmd.AddCommand(newSessionCleanupStopCommand(deps))
	return cmd
}

func bindSessionCleanupFlags(cmd *cobra.Command, flags *sessionCleanupFlags) {
	cmd.Flags().BoolVar(&flags.everything, "everything", false,
		"Delete picked threads whole: main sessions, child sessions, and subagents (the default)")
	cmd.Flags().BoolVar(&flags.children, "children", false, "Delete archived child sessions, keeping main sessions")
	cmd.Flags().BoolVar(&flags.subagents, "subagents", false, "Delete archived subagents, keeping main sessions")
	cmd.Flags().IntVar(&flags.olderThan, "older-than", defaultCleanupOlderThanDays,
		"Pick threads archived more than this many days ago")
	cmd.Flags().StringVar(&flags.before, "before", "",
		"Pick threads archived before this date: YYYY-MM-DD (local midnight) or RFC 3339")
	cmd.Flags().BoolVar(&flags.anyTime, "any-time", false, "Pick every archived thread, whenever it was archived")
	cmd.Flags().String(workspaceFlagName, "", "Workspace ID, name, or path (defaults to the current workspace)")
	cmd.Flags().BoolVar(&flags.allWorkspaces, allWorkspacesFlagName, false, "Clean up every project and User scope")
	cmd.Flags().StringSliceVar(&flags.sessionIDs, "session", nil,
		"Limit the cleanup to these archived main sessions, as a retry of failed threads does")
	cmd.Flags().BoolVar(&flags.yes, "yes", false, "Delete; without it the command only previews")
	cmd.MarkFlagsMutuallyExclusive("older-than", "before", "any-time")
	cmd.MarkFlagsMutuallyExclusive(workspaceFlagName, allWorkspacesFlagName)
}

func runSessionCleanup(cmd *cobra.Command, deps commandDeps, flags sessionCleanupFlags) error {
	base, err := clientFromDeps(deps)
	if err != nil {
		return err
	}
	client, ok := base.(sessionHistoryCleanupClient)
	if !ok {
		return errors.New("cli: session history cleanup is unavailable")
	}
	request, err := sessionCleanupRequest(cmd, flags)
	if err != nil {
		return withCommandExitCode(2, err)
	}
	if !flags.allWorkspaces {
		workspaceID, resolveErr := resolveWorkspaceFlagOverride(cmd, deps, base, true)
		if resolveErr != nil {
			return resolveErr
		}
		request.WorkspaceID = workspaceID
	}
	if !flags.yes {
		preview, previewErr := client.PreviewSessionHistoryCleanup(cmd.Context(), request)
		if previewErr != nil {
			return previewErr
		}
		return writeCommandOutput(cmd, sessionCleanupPlanBundle(preview.Plan, false))
	}
	started, err := client.StartSessionHistoryCleanup(cmd.Context(), request)
	if err != nil {
		return err
	}
	return writeCommandOutput(cmd, sessionCleanupStartBundle(started))
}

func sessionCleanupRequest(
	cmd *cobra.Command,
	flags sessionCleanupFlags,
) (contract.SessionHistoryCleanupRequest, error) {
	if cmd.Flags().Changed("before") && strings.TrimSpace(flags.before) == "" {
		return contract.SessionHistoryCleanupRequest{}, errors.New("cli: --before cannot be empty")
	}
	request := contract.SessionHistoryCleanupRequest{
		AllWorkspaces: flags.allWorkspaces,
		Delete: contract.SessionHistoryCleanupSelection{
			Everything: flags.everything,
			Children:   flags.children,
			Subagents:  flags.subagents,
		},
		SessionIDs: flags.sessionIDs,
	}
	if !flags.everything && !flags.children && !flags.subagents {
		request.Delete.Everything = true
	}
	if request.Delete.Everything {
		request.Delete.Children, request.Delete.Subagents = true, true
	}
	switch {
	case flags.anyTime:
	case strings.TrimSpace(flags.before) != "":
		before, err := parseCleanupBefore(flags.before)
		if err != nil {
			return request, err
		}
		request.ArchivedBefore = &before
	default:
		if flags.olderThan < 1 {
			return request, errors.New("cli: --older-than must be at least 1 day")
		}
		days := flags.olderThan
		request.OlderThanDays = &days
	}
	return request, nil
}

func parseCleanupBefore(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if at, err := time.Parse(time.RFC3339, value); err == nil {
		return at, nil
	}
	if day, err := time.ParseInLocation(time.DateOnly, value, time.Local); err == nil {
		return day, nil
	}
	return time.Time{}, fmt.Errorf("cli: --before must be YYYY-MM-DD or RFC 3339, got %q", value)
}

func newSessionCleanupStatusCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the running or most recent archived-history cleanup",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := sessionCleanupClientFromDeps(deps)
			if err != nil {
				return err
			}
			response, err := client.GetSessionHistoryCleanup(cmd.Context())
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionCleanupRunBundle(response))
		},
	}
}

func newSessionCleanupStopCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the running archived-history cleanup; what it deleted stays deleted",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := sessionCleanupClientFromDeps(deps)
			if err != nil {
				return err
			}
			response, err := client.StopSessionHistoryCleanup(cmd.Context())
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionCleanupRunBundle(response))
		},
	}
}

func sessionCleanupClientFromDeps(deps commandDeps) (sessionHistoryCleanupClient, error) {
	base, err := clientFromDeps(deps)
	if err != nil {
		return nil, err
	}
	client, ok := base.(sessionHistoryCleanupClient)
	if !ok {
		return nil, errors.New("cli: session history cleanup is unavailable")
	}
	return client, nil
}

func sessionCleanupPlanBundle(plan contract.SessionHistoryCleanupPlan, started bool) outputBundle {
	return outputBundle{
		jsonValue: contract.SessionHistoryCleanupPreviewResponse{Plan: plan},
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, plan)
		},
		human: func() (string, error) {
			blocks := []string{renderHumanSection("Cleanup preview", sessionCleanupPlanRows(plan))}
			if !started && plan.Counts.Total > 0 {
				blocks = append(blocks, fmt.Sprintf(
					"Run again with --yes to delete %d %s.", plan.Counts.Total, sessionNoun(plan.Counts.Total),
				))
			}
			return renderHumanBlocks(blocks...), nil
		},
		toon: func() (string, error) {
			return renderToonObject("plan", sessionCleanupPlanFields(), sessionCleanupPlanValues(plan)), nil
		},
	}
}

func sessionCleanupStartBundle(response contract.SessionHistoryCleanupStartResponse) outputBundle {
	return outputBundle{
		jsonValue: response,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, response)
		},
		human: func() (string, error) {
			return renderHumanBlocks(
				renderHumanSection("Cleanup started", sessionCleanupRunRows(response.Run)),
				"Follow it with `compozy session cleanup status`.",
			), nil
		},
		toon: func() (string, error) {
			return renderToonObject("run", sessionCleanupRunFields(), sessionCleanupRunValues(response.Run)), nil
		},
	}
}

func sessionCleanupRunBundle(response contract.SessionHistoryCleanupRunResponse) outputBundle {
	return outputBundle{
		jsonValue: response,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, response)
		},
		human: func() (string, error) {
			if response.Run == nil {
				return "No cleanup has run since the daemon started.", nil
			}
			blocks := []string{renderHumanSection("Cleanup", sessionCleanupRunRows(*response.Run))}
			if len(response.Run.Failures) > 0 {
				blocks = append(blocks, sessionCleanupFailureTable(response.Run.Failures))
			}
			return renderHumanBlocks(blocks...), nil
		},
		toon: func() (string, error) {
			if response.Run == nil {
				return renderToonObject("run", []string{"state"}, []string{"none"}), nil
			}
			return renderToonObject("run", sessionCleanupRunFields(), sessionCleanupRunValues(*response.Run)), nil
		},
	}
}

func sessionCleanupPlanRows(plan contract.SessionHistoryCleanupPlan) []keyValue {
	return []keyValue{
		{Label: "Archived before", Value: formatTimePtr(plan.ArchivedBefore)},
		{Label: "Sessions to delete", Value: strconv.Itoa(plan.Counts.Total)},
		{Label: "Main sessions", Value: strconv.Itoa(plan.Counts.Sessions)},
		{Label: "Child sessions", Value: strconv.Itoa(plan.Counts.Children)},
		{Label: "Subagents", Value: strconv.Itoa(plan.Counts.Subagents)},
		{Label: "Threads", Value: strconv.Itoa(plan.Threads)},
		{Label: "Kept (still in the list or Later)", Value: strconv.Itoa(plan.KeptLive)},
	}
}

func sessionCleanupPlanFields() []string {
	return []string{"archived_before", "total", "sessions", "children", "subagents", "threads", "kept_live"}
}

func sessionCleanupPlanValues(plan contract.SessionHistoryCleanupPlan) []string {
	return []string{
		formatTimePtr(plan.ArchivedBefore),
		strconv.Itoa(plan.Counts.Total),
		strconv.Itoa(plan.Counts.Sessions),
		strconv.Itoa(plan.Counts.Children),
		strconv.Itoa(plan.Counts.Subagents),
		strconv.Itoa(plan.Threads),
		strconv.Itoa(plan.KeptLive),
	}
}

func sessionCleanupRunRows(run contract.SessionHistoryCleanupRun) []keyValue {
	return []keyValue{
		{Label: "ID", Value: run.ID},
		{Label: "State", Value: run.State},
		{Label: "Progress", Value: fmt.Sprintf("%d of %d", run.Done, run.Total)},
		{Label: "Deleted", Value: strconv.Itoa(run.Deleted)},
		{Label: "Skipped (restored meanwhile)", Value: strconv.Itoa(run.Skipped)},
		{Label: "Failed threads", Value: strconv.Itoa(run.FailedThreads)},
		{Label: "Started", Value: formatTime(run.StartedAt)},
		{Label: "Finished", Value: formatTimePtr(run.FinishedAt)},
	}
}

func sessionCleanupRunFields() []string {
	return []string{"id", "state", "done", "total", "deleted", "skipped", "failed_threads"}
}

func sessionCleanupRunValues(run contract.SessionHistoryCleanupRun) []string {
	return []string{
		run.ID,
		run.State,
		strconv.Itoa(run.Done),
		strconv.Itoa(run.Total),
		strconv.Itoa(run.Deleted),
		strconv.Itoa(run.Skipped),
		strconv.Itoa(run.FailedThreads),
	}
}

func sessionCleanupFailureTable(failures []contract.SessionHistoryCleanupFailure) string {
	rows := make([][]string, 0, len(failures))
	for _, failure := range failures {
		rows = append(rows, []string{failure.SessionID, stringOrDash(failure.Title), failure.Message})
	}
	return renderHumanTable(
		"Failed threads (retry with --session <id>)",
		[]string{"Session", "Title", "Reason"},
		rows,
	)
}

func sessionNoun(count int) string {
	if count == 1 {
		return "session"
	}
	return "sessions"
}
