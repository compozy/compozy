package cli

import (
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/observe"
	"github.com/spf13/cobra"
)

const (
	observeCommandKey         = "observe"
	observeOverviewCommandKey = "overview"
	observeSectionKey         = "section"
	observeSectionMeta        = "meta"
	observeSectionNetwork     = "network"
	observeUsageSectionKey    = "usage"
	observeTokensLabel        = "tokens"
	observeApprovalsLabel     = "approvals"
	observeWindowLabel        = "window"
	observeLabelCompleted     = "completed"
	observeLabelFailed        = "failed"
	observeCostUnknown        = "unknown"
)

func newObserveCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   observeCommandKey,
		Short: "Inspect runtime observability read models",
	}
	cmd.AddCommand(newObserveOverviewCommand(deps))
	return cmd
}

func newObserveOverviewCommand(deps commandDeps) *cobra.Command {
	var workspace string
	var usageWindow int

	cmd := &cobra.Command{
		Use:   observeOverviewCommandKey,
		Short: "Show the home overview: what agents did, what needs you, what it costs",
		Example: `  # Show the home overview
  agh observe overview

  # Return the machine-readable overview for agents
  agh observe overview -o json

  # Scope aggregates to one workspace with a 7-day usage window
  agh observe overview --workspace launch-hq --usage-window 7`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if usageWindow != 0 {
				if err := observe.ValidateUsageWindowDays(usageWindow); err != nil {
					return fmt.Errorf("--usage-window must be 7, 30, or 90")
				}
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			response, err := client.ObserveOverview(cmd.Context(), ObserveOverviewQuery{
				Workspace:       workspace,
				UsageWindowDays: usageWindow,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, observeOverviewBundle(&response.Overview))
		},
	}

	cmd.Flags().StringVar(&workspace, "workspace", "", "Scope aggregates to one workspace (path, name, or ID)")
	cmd.Flags().IntVar(&usageWindow, "usage-window", 0, "Usage window in days: 7, 30, or 90 (default 30)")
	return cmd
}

func observeOverviewBundle(overview *contract.ObserveOverviewPayload) outputBundle {
	return outputBundle{
		jsonValue: overview,
		jsonl: func(cmd *cobra.Command) error {
			if err := writeJSONLine(cmd, map[string]any{
				observeSectionKey: observeSectionMeta,
				"schema_version":  overview.SchemaVersion,
				"generated_at":    overview.GeneratedAt,
			}); err != nil {
				return err
			}
			sections := []struct {
				name  string
				value any
			}{
				{"attention", overview.Attention},
				{"today", overview.Today},
				{"outcomes", overview.Outcomes},
				{observeUsageSectionKey, overview.Usage},
				{"pulse", overview.Pulse},
				{observeSectionNetwork, overview.Network},
				{"system", overview.System},
				{"freshness", overview.Freshness},
			}
			for _, section := range sections {
				if err := writeJSONLine(cmd, map[string]any{
					observeSectionKey: section.name,
					section.name:      section.value,
				}); err != nil {
					return err
				}
			}
			return nil
		},
		human: func() (string, error) {
			return renderObserveOverviewHuman(overview), nil
		},
	}
}

func renderObserveOverviewHuman(overview *contract.ObserveOverviewPayload) string {
	sections := []string{
		renderHumanSection("Needs you", []keyValue{
			{Label: "total", Value: fmt.Sprintf("%d", overview.Attention.Total)},
			{Label: observeApprovalsLabel, Value: fmt.Sprintf("%d", overview.Attention.ByKind[attentionKindApproval])},
			{Label: "needs input", Value: fmt.Sprintf("%d", overview.Attention.ByKind[attentionKindNeedsInput])},
			{Label: "failures", Value: fmt.Sprintf("%d", overview.Attention.ByKind[attentionKindFailure])},
		}),
		renderHumanSection("Today", []keyValue{
			{Label: "runs completed", Value: fmt.Sprintf("%d", overview.Today.RunsCompleted)},
			{Label: "runs failed", Value: fmt.Sprintf("%d", overview.Today.RunsFailed)},
			{Label: "tasks closed", Value: fmt.Sprintf("%d", overview.Today.TasksClosed)},
		}),
		renderHumanSection(
			fmt.Sprintf("Outcomes · last %d days", overview.Outcomes.WindowDays),
			[]keyValue{
				{Label: observeLabelCompleted, Value: fmt.Sprintf("%d", overview.Outcomes.Completed)},
				{Label: observeLabelFailed, Value: fmt.Sprintf("%d", overview.Outcomes.Failed)},
				{Label: "canceled", Value: fmt.Sprintf("%d", overview.Outcomes.Canceled)},
				{Label: "success", Value: fmt.Sprintf("%.0f%%", overview.Outcomes.SuccessPct)},
			},
		),
		renderHumanSection("Usage", renderOverviewUsageRows(&overview.Usage)),
		renderHumanSection(
			fmt.Sprintf("Pulse · last %d days", overview.Pulse.WindowDays),
			renderOverviewPulseRows(&overview.Pulse),
		),
		renderHumanSection("System", []keyValue{
			{Label: "network messages today", Value: fmt.Sprintf("%d", overview.Network.MessagesToday)},
			{
				Label: "hooks today",
				Value: fmt.Sprintf(
					"%d runs · %d failed",
					overview.System.HookRunsToday, overview.System.HookFailuresToday,
				),
			},
			{Label: "retention", Value: fmt.Sprintf("%d days", overview.System.RetentionDays)},
		}),
	}
	return strings.Join(sections, "\n")
}

func renderOverviewUsageRows(usage *contract.OverviewUsagePayload) []keyValue {
	rows := []keyValue{
		{Label: observeWindowLabel, Value: fmt.Sprintf("%dd", usage.WindowDays)},
		{Label: observeTokensLabel, Value: fmt.Sprintf("%d", usage.TotalTokens)},
		{Label: "cost", Value: renderOverviewCost(usage)},
	}
	if usage.Truncated {
		rows = append(rows, keyValue{
			Label: "retention",
			Value: fmt.Sprintf("bounded to %d days", usage.RetentionDays),
		})
	}
	return rows
}

func renderOverviewPulseRows(pulse *contract.OverviewPulsePayload) []keyValue {
	rows := []keyValue{}
	if pulse.Busiest != nil {
		rows = append(rows, keyValue{
			Label: "busiest hour",
			Value: fmt.Sprintf(
				"%s %02d:00 · %d events",
				overviewWeekdayLabel(pulse.Busiest.Weekday),
				pulse.Busiest.Hour,
				pulse.Busiest.Events,
			),
		})
	}
	if pulse.LongestSession != nil {
		rows = append(rows, keyValue{
			Label: "longest session",
			Value: fmt.Sprintf(
				"%s · %s · %s",
				pulse.LongestSession.AgentName,
				formatOverviewDuration(pulse.LongestSession.DurationSeconds),
				pulse.LongestSession.Date,
			),
		})
	}
	if len(rows) == 0 {
		rows = append(rows, keyValue{Label: "activity", Value: "no events in window"})
	}
	return rows
}

const (
	attentionKindApproval   = "approval"
	attentionKindNeedsInput = "needs_input"
	attentionKindFailure    = "failure"
)

func renderOverviewCost(usage *contract.OverviewUsagePayload) string {
	if usage.EstimatedCost == nil {
		if usage.CostStatus != "" {
			return usage.CostStatus
		}
		return observeCostUnknown
	}
	return fmt.Sprintf("%.2f %s (%s)", *usage.EstimatedCost, usage.CostCurrency, usage.CostStatus)
}

func formatOverviewDuration(seconds int64) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	minutes := seconds / 60
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%dh %02dm", minutes/60, minutes%60)
}

func overviewWeekdayLabel(weekday int) string {
	labels := [...]string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	if weekday < 0 || weekday >= len(labels) {
		return "?"
	}
	return labels[weekday]
}
