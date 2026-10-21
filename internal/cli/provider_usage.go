package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

const providerUsageExperimental = "Experimental: provider usage output may change."

func newProviderUsageCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{Use: "usage", Short: "Inspect provider usage (experimental)", Long: "Read and refresh profile-scoped provider usage.\n\n" + providerUsageExperimental}
	cmd.AddCommand(newProviderUsageAction(deps, false), newProviderUsageAction(deps, true))
	return cmd
}

func newProviderUsageAction(deps commandDeps, refresh bool) *cobra.Command {
	action := "list"
	if refresh {
		action = "refresh"
	}
	var maxAge time.Duration
	wait := 15 * time.Second
	cmd := &cobra.Command{
		Use: action + " [provider]", Short: strings.ToUpper(action[:1]) + action[1:] + " provider usage (experimental)", Args: optionalProviderArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if wait < 0 || wait > 30*time.Second {
				return fmt.Errorf("--wait must be between 0s and 30s (invalid_request)")
			}
			if maxAge < 0 {
				return fmt.Errorf("--max-age must be ≥ 0s (invalid_request)")
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			usage, ok := client.(ProviderUsageClient)
			if !ok {
				return errors.New("provider usage client is unavailable")
			}
			var envelope contract.ProviderUsageEnvelope
			if refresh {
				waitMS := wait.Milliseconds()
				input := contract.ProviderUsageRefreshRequest{Intent: "explicit", WaitMS: &waitMS, MaxAgeSeconds: int64(maxAge / time.Second)}
				if provider := providerArgValue(args); provider != "" {
					input.ProviderIDs = []string{provider}
				}
				envelope, err = usage.RefreshProviderUsage(cmd.Context(), input)
			} else {
				envelope, err = usage.ListProviderUsage(cmd.Context(), providerArgValue(args))
			}
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, outputBundle{jsonValue: envelope, human: func() (string, error) { return renderProviderUsage(envelope, time.Now()), nil }})
		},
	}
	if refresh {
		cmd.Flags().DurationVar(&maxAge, "max-age", 0, "Minimum age since a successful collection")
		cmd.Flags().DurationVar(&wait, "wait", 15*time.Second, "Wait for results (0s–30s)")
	}
	return cmd
}

func renderProviderUsage(envelope contract.ProviderUsageEnvelope, now time.Time) string {
	if !envelope.Enabled {
		return "Provider usage is turned off. Enable it with: compozy config set provider_usage.enabled true"
	}
	rows := [][]string{{"PROVIDER", "STATUS", "TIGHTEST WINDOW", "USED", "RESETS IN", "UPDATED"}}
	consent := false
	for _, target := range envelope.Targets {
		status := target.Status
		if target.Refreshing {
			status = "refreshing"
		}
		row := []string{target.ProviderName, status, "—", "—", "—", "—"}
		var tightest *contract.ProviderUsageWindow
		for i := range target.Windows {
			window := &target.Windows[i]
			if window.UsedPercent == nil {
				continue
			}
			if tightest == nil || *window.UsedPercent > *tightest.UsedPercent ||
				(*window.UsedPercent == *tightest.UsedPercent && window.ResetsAt != nil && (tightest.ResetsAt == nil || window.ResetsAt.Before(*tightest.ResetsAt))) {
				tightest = window
			}
		}
		if tightest != nil {
			row[2], row[3] = tightest.Label, fmt.Sprintf("%g%%", *tightest.UsedPercent)
			if tightest.ResetsAt != nil {
				row[4] = providerUsageDuration(tightest.ResetsAt.Sub(now))
			} else if tightest.ResetText != "" {
				row[4] = tightest.ResetText
			}
		}
		if target.MeasuredAt != nil {
			row[5] = providerUsageDuration(now.Sub(*target.MeasuredAt)) + " ago"
		}
		rows = append(rows, row)
		consent = consent || (target.ProviderID == "claude" && target.Error != nil && target.Error.Action == "allow_keychain")
	}
	var b strings.Builder
	widths := humanTableColumnWidths(rows)
	for _, row := range rows {
		writeHumanTableRow(&b, row, widths)
	}
	b.WriteByte('\n')
	if consent {
		b.WriteString("Claude: Needs Keychain access. Allow it with: compozy config set provider_usage.providers.claude.keychain_access true\n")
	}
	b.WriteString(providerUsageExperimental)
	return b.String()
}

func providerUsageDuration(d time.Duration) string {
	if d < time.Minute {
		return "<1m"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d/time.Minute))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh %dm", int(d/time.Hour), int(d/time.Minute)%60)
	}
	return fmt.Sprintf("%dd %dh", int(d/(24*time.Hour)), int(d/time.Hour)%24)
}
