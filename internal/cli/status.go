package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/spf13/cobra"
)

const (
	statusCommandKey          = "status"
	doctorCommandKey          = "doctor"
	statusDiagnosticsKey      = "diagnostics"
	statusLogTailKey          = "log_tail"
	statusMCPServersKey       = "mcp_servers"
	statusSubprocessHealthKey = "subprocess_health"
	statusSectionKey          = "section"
)

func newStatusCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   statusCommandKey,
		Short: "Show consolidated runtime status",
		Example: `  # Show runtime status
  agh status

  # Return machine-readable status for agents
  agh status -o json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			status, err := client.Status(cmd.Context())
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, statusBundle(&status, deps.now))
		},
	}
}

func newDoctorCommand(deps commandDeps) *cobra.Command {
	var only []string
	var exclude []string
	var quiet bool

	cmd := &cobra.Command{
		Use:   doctorCommandKey,
		Short: "Run runtime diagnostics",
		Example: `  # Run diagnostics
  agh doctor

  # Return diagnostic items for agents
  agh doctor -o json

  # Run only provider and MCP diagnostics
  agh doctor --only provider --only mcp`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			result, err := client.Doctor(cmd.Context(), DoctorQuery{
				Only:    only,
				Exclude: exclude,
				Quiet:   quiet,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, doctorBundle(result))
		},
	}
	cmd.Flags().StringSliceVar(&only, "only", nil, "Run only the named probe ids or categories")
	cmd.Flags().StringSliceVar(&exclude, "exclude", nil, "Exclude the named probe ids or categories")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "Omit OK diagnostics")
	return cmd
}

func statusBundle(status *StatusRecord, now func() time.Time) outputBundle {
	return outputBundle{
		jsonValue: status,
		jsonl: func(cmd *cobra.Command) error {
			if err := writeStatusSection(cmd, configDaemonKey, status.Daemon); err != nil {
				return err
			}
			if err := writeStatusSection(cmd, extensionHealthKey, status.Health); err != nil {
				return err
			}
			if err := writeStatusSection(cmd, statusSubprocessHealthKey, status.SubprocessHealth); err != nil {
				return err
			}
			if err := writeStatusSection(cmd, configProvidersKey, status.Providers); err != nil {
				return err
			}
			if err := writeStatusSection(cmd, statusMCPServersKey, status.MCPServers); err != nil {
				return err
			}
			return writeJSONLine(cmd, map[string]any{
				statusSectionKey: statusDiagnosticsKey,
				configConfigKey:  status.Config,
				statusLogTailKey: status.LogTail,
			})
		},
		human: func() (string, error) {
			return renderStatusHuman(status, now), nil
		},
		toon: func() (string, error) {
			return renderStatusToon(status, now), nil
		},
	}
}

func writeStatusSection(cmd *cobra.Command, section string, value any) error {
	return writeJSONLine(cmd, map[string]any{statusSectionKey: section, section: value})
}

func doctorBundle(result DoctorRecord) outputBundle {
	return outputBundle{
		jsonValue: result,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLines(cmd, result.Items)
		},
		human: func() (string, error) {
			rows := make([][]string, 0, len(result.Items))
			for _, item := range result.Items {
				rows = append(rows, []string{
					item.Severity,
					item.Category,
					item.Code,
					item.Title,
					firstNonEmptyString(item.SuggestedCommand, "-"),
				})
			}
			header := renderHumanSection("Doctor", []keyValue{
				{Label: daemonStatusValue, Value: result.Status},
				{Label: "Generated", Value: formatTime(result.GeneratedAt)},
				{Label: cliDurationValue, Value: fmt.Sprintf("%dms", result.DurationMS)},
				{Label: "Items", Value: strconv.Itoa(result.Summary.Total)},
			})
			table := renderHumanTable(
				"Diagnostics",
				[]string{cliSeverityValue, "Category", cliCodeValue, "Title", cliCommandValue},
				rows,
			)
			return header + "\n\n" + table, nil
		},
		toon: func() (string, error) {
			return renderToonObject("doctor", []string{daemonStatusKey, "items", cliDurationMSKey}, []string{
				result.Status,
				strconv.Itoa(result.Summary.Total),
				strconv.FormatInt(result.DurationMS, 10),
			}), nil
		},
	}
}

func renderStatusHuman(status *StatusRecord, now func() time.Time) string {
	daemonRows := []keyValue{
		{Label: daemonStatusValue, Value: stringOrDash(status.Daemon.Status)},
		{Label: cliPIDValue, Value: intOrDash(status.Daemon.PID)},
		{Label: daemonStartedValue, Value: stringOrDash(formatTime(status.Daemon.StartedAt))},
		{Label: cliUptimeValue, Value: stringOrDash(formatAge(now, status.Daemon.StartedAt))},
		{Label: "Socket", Value: stringOrDash(status.Daemon.Socket)},
		{Label: "HTTP", Value: statusHTTPAddress(status)},
		{Label: "Sessions", Value: fmt.Sprintf("%d active / %d total", status.Sessions.Active, status.Sessions.Total)},
		{Label: "Health", Value: stringOrDash(status.Health.Status)},
		{Label: "Subprocess Health", Value: stringOrDash(status.SubprocessHealth.Status)},
		{Label: "Needs Attention", Value: fmt.Sprintf("%d task runs", statusNeedsAttentionRuns(status.Tasks))},
		{Label: "Config", Value: stringOrDash(status.Config.Status)},
		{Label: "Log Tail", Value: stringOrDash(status.LogTail.Status)},
	}
	sections := []string{renderHumanSection("Runtime", daemonRows)}
	if len(status.Providers) > 0 {
		rows := make([][]string, 0, len(status.Providers))
		for _, provider := range status.Providers {
			defaultValue := ""
			if provider.Default {
				defaultValue = yesFlagName
			}
			rows = append(rows, []string{
				provider.Name,
				provider.State,
				provider.Mode,
				defaultValue,
				firstNonEmptyString(provider.Message, "-"),
			})
		}
		sections = append(sections, renderHumanTable(
			"Providers",
			[]string{providerNameValue, providerStateValue, taskModeValue, "Default", providerMessageValue},
			rows,
		))
	}
	if len(status.MCPServers) > 0 {
		rows := make([][]string, 0, len(status.MCPServers))
		for _, server := range status.MCPServers {
			rows = append(rows, []string{
				server.Name,
				server.RuntimeStatus,
				server.State,
				strconv.Itoa(server.ToolCount),
				firstNonEmptyString(server.Reason, "-"),
			})
		}
		sections = append(sections, renderHumanTable(
			"MCP Servers",
			[]string{
				providerNameValue,
				"Runtime",
				providerStateValue,
				toolOperatorToolsValue,
				authoredContextReasonValue,
			},
			rows,
		))
	}
	return strings.Join(sections, "\n\n")
}

func renderStatusToon(status *StatusRecord, now func() time.Time) string {
	return renderToonObject("status", []string{
		daemonStatusKey,
		cliPIDKey,
		"uptime",
		extensionHealthKey,
		"sessions_active",
		"sessions_total",
		configProvidersKey,
		statusMCPServersKey,
		statusSubprocessHealthKey,
		"needs_attention_runs",
		configConfigKey,
	}, []string{
		status.Daemon.Status,
		strconv.Itoa(status.Daemon.PID),
		formatAge(now, status.Daemon.StartedAt),
		status.Health.Status,
		strconv.Itoa(status.Sessions.Active),
		strconv.Itoa(status.Sessions.Total),
		strconv.Itoa(len(status.Providers)),
		strconv.Itoa(len(status.MCPServers)),
		status.SubprocessHealth.Status,
		strconv.Itoa(statusNeedsAttentionRuns(status.Tasks)),
		status.Config.Status,
	})
}

func statusNeedsAttentionRuns(tasks contract.TaskHealthPayload) int {
	total := 0
	for _, runTotal := range tasks.RunTotals {
		if strings.TrimSpace(runTotal.Status) == "needs_attention" {
			total += runTotal.Count
		}
	}
	return total
}

func statusHTTPAddress(status *StatusRecord) string {
	if status == nil {
		return "-"
	}
	return stringOrDash(strings.TrimSpace(status.Daemon.HTTPHost) + ":" + intOrDash(status.Daemon.HTTPPort))
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
