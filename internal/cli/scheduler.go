package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	schedulerKey              = "scheduler"
	schedulerStatusKey        = "status"
	schedulerPauseKey         = "pause"
	schedulerResumeKey        = "resume"
	schedulerDrainKey         = "drain"
	schedulerBacklogKey       = "backlog"
	schedulerPausedValue      = "Paused"
	schedulerPausedByValue    = "Paused By"
	schedulerPausedAtValue    = "Paused At"
	schedulerActiveClaimValue = "Active Claims"
	schedulerQueuedRunValue   = "Queued Runs"
	schedulerPausedTaskValue  = "Paused Tasks"
	schedulerStarvedRunValue  = "Starved Runs"
	schedulerNeedsAttnValue   = "Needs Attention Runs"
	schedulerCompletedValue   = "Completed"
	schedulerTimedOutValue    = "Timed Out"
	schedulerRemainingValue   = "Remaining Claims"
	defaultSchedulerDrainFlag = "60s"
)

func newSchedulerCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   schedulerKey,
		Short: "Inspect and control task scheduler dispatch",
		Example: `  # Show scheduler pause and backlog pressure
  agh scheduler status

  # Pause all new scheduler dispatch while active claims continue
  agh scheduler pause --reason "incident response"

  # Pause and wait up to 30 seconds for active claims to finish
  agh scheduler drain --timeout 30s`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newSchedulerStatusCommand(deps))
	cmd.AddCommand(newSchedulerPauseCommand(deps))
	cmd.AddCommand(newSchedulerResumeCommand(deps))
	cmd.AddCommand(newSchedulerDrainCommand(deps))
	cmd.AddCommand(newSchedulerBacklogCommand(deps))
	return cmd
}

func newSchedulerStatusCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   schedulerStatusKey,
		Short: "Show scheduler pause and queue pressure",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			record, err := client.SchedulerStatus(cmd.Context())
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, schedulerStatusBundle(record))
		},
	}
	return cmd
}

func newSchedulerPauseCommand(deps commandDeps) *cobra.Command {
	var reason string
	cmd := &cobra.Command{
		Use:   schedulerPauseKey,
		Short: "Pause new scheduler dispatch",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			record, err := client.PauseScheduler(
				cmd.Context(),
				SchedulerPauseRequest{Reason: strings.TrimSpace(reason)},
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, schedulerStatusBundle(record))
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "Optional scheduler pause reason")
	return cmd
}

func newSchedulerResumeCommand(deps commandDeps) *cobra.Command {
	var reason string
	cmd := &cobra.Command{
		Use:   schedulerResumeKey,
		Short: "Resume scheduler dispatch",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			record, err := client.ResumeScheduler(
				cmd.Context(),
				SchedulerResumeRequest{Reason: strings.TrimSpace(reason)},
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, schedulerStatusBundle(record))
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "Optional scheduler resume reason")
	return cmd
}

func newSchedulerDrainCommand(deps commandDeps) *cobra.Command {
	var reason string
	var timeoutRaw string
	cmd := &cobra.Command{
		Use:   schedulerDrainKey,
		Short: "Pause dispatch and wait for active task claims to finish",
		Args:  cobra.NoArgs,
		Example: `  # Drain active claims with the default timeout
  agh scheduler drain

  # Return immediately after pausing dispatch
  agh scheduler drain --timeout 0s`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			request := SchedulerDrainRequest{Reason: strings.TrimSpace(reason)}
			if cmd.Flags().Changed("timeout") {
				seconds, err := parseSchedulerDrainTimeout(timeoutRaw)
				if err != nil {
					return err
				}
				request.TimeoutSeconds = &seconds
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			record, err := client.DrainScheduler(cmd.Context(), request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, schedulerDrainBundle(record))
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "Optional scheduler drain reason")
	cmd.Flags().StringVar(
		&timeoutRaw,
		"timeout",
		defaultSchedulerDrainFlag,
		"Drain wait timeout as a duration; 0s returns immediately",
	)
	return cmd
}

func newSchedulerBacklogCommand(deps commandDeps) *cobra.Command {
	var last int
	var workspace string
	var includePaused bool
	cmd := &cobra.Command{
		Use:   schedulerBacklogKey,
		Short: "List queued runs visible to scheduler dispatch",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if last < 0 {
				return fmt.Errorf("cli: --last must be zero or positive: %d", last)
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			record, err := client.SchedulerBacklog(cmd.Context(), SchedulerBacklogQuery{
				Limit:         last,
				WorkspaceID:   strings.TrimSpace(workspace),
				IncludePaused: includePaused,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, schedulerBacklogBundle(record))
		},
	}
	cmd.Flags().IntVar(&last, "last", 50, "Maximum queued runs to return")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Filter queued runs by workspace ID")
	cmd.Flags().BoolVar(&includePaused, "include-paused", false, "Include queued runs blocked by task pause")
	return cmd
}

func schedulerStatusBundle(record SchedulerStatusRecord) outputBundle {
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			return renderHumanSection("Scheduler", []keyValue{
				{Label: schedulerPausedValue, Value: strconv.FormatBool(record.Paused)},
				{Label: schedulerPausedByValue, Value: stringOrDash(record.PausedBy)},
				{Label: schedulerPausedAtValue, Value: stringOrDash(formatTimePtr(record.PausedAt))},
				{Label: taskReasonValue, Value: stringOrDash(record.PausedReason)},
				{Label: schedulerActiveClaimValue, Value: strconv.Itoa(record.ActiveClaimCount)},
				{Label: schedulerQueuedRunValue, Value: strconv.Itoa(record.QueuedRunCount)},
				{Label: schedulerPausedTaskValue, Value: strconv.Itoa(record.PausedTaskCount)},
				{Label: schedulerStarvedRunValue, Value: strconv.Itoa(record.StarvedRunCount)},
				{Label: schedulerNeedsAttnValue, Value: strconv.Itoa(record.NeedsAttentionRunCount)},
				{Label: "As Of", Value: stringOrDash(formatTime(record.AsOf))},
			}), nil
		},
		toon: func() (string, error) {
			return renderJSONPreview(record)
		},
	}
}

func schedulerDrainBundle(record SchedulerDrainRecord) outputBundle {
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			return renderHumanSection("Scheduler Drain", []keyValue{
				{Label: schedulerCompletedValue, Value: strconv.FormatBool(record.Completed)},
				{Label: schedulerTimedOutValue, Value: strconv.FormatBool(record.TimedOut)},
				{Label: schedulerRemainingValue, Value: strconv.Itoa(record.RemainingClaims)},
				{Label: "Started", Value: stringOrDash(formatTime(record.StartedAt))},
				{Label: "Completed At", Value: stringOrDash(formatTime(record.CompletedAt))},
				{Label: schedulerPausedValue, Value: strconv.FormatBool(record.Scheduler.Paused)},
				{Label: schedulerActiveClaimValue, Value: strconv.Itoa(record.Scheduler.ActiveClaimCount)},
			}), nil
		},
		toon: func() (string, error) {
			return renderJSONPreview(record)
		},
	}
}

func schedulerBacklogBundle(record SchedulerBacklogRecord) outputBundle {
	return outputBundle{
		jsonValue: record,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLines(cmd, record.Runs)
		},
		human: func() (string, error) {
			rows := make([][]string, 0, len(record.Runs))
			for idx := range record.Runs {
				item := &record.Runs[idx]
				rows = append(rows, []string{
					item.Run.ID,
					item.Task.ID,
					item.Run.Status.String(),
					strconv.Itoa(item.Run.Attempt),
					formatTime(item.Run.QueuedAt),
					strconv.FormatBool(item.Task.EffectivePaused),
				})
			}
			return renderHumanTable(
				"Scheduler Backlog",
				[]string{"Run", taskTaskValue, taskStatusValue, "Attempt", "Queued", "Paused"},
				rows,
			), nil
		},
		toon: func() (string, error) {
			return renderJSONPreview(record)
		},
	}
}

func parseSchedulerDrainTimeout(raw string) (int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, errors.New("cli: --timeout cannot be empty")
	}
	timeout, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, fmt.Errorf("cli: parse --timeout: %w", err)
	}
	if timeout < 0 {
		return 0, fmt.Errorf("cli: --timeout must be non-negative: %s", trimmed)
	}
	if timeout%time.Second != 0 {
		return 0, fmt.Errorf("cli: --timeout must use whole seconds: %s", trimmed)
	}
	return int(timeout / time.Second), nil
}
