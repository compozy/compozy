package cli

import (
	"fmt"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/remoteproto"
)

func newRemoteDevicesCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{Use: "devices", Short: "Manage devices allowed on this computer"}
	cmd.AddCommand(
		remoteVerb(deps, "list", "List devices and local approvals", cobra.NoArgs, runRemoteDevices),
		newRemoteDeviceScopesCommand(deps),
		remoteVerb(
			deps,
			"revoke <device>",
			"Revoke a device on your account",
			cobra.ExactArgs(1),
			runRemoteDeviceRevoke,
		),
		remoteDeviceDecision(deps, "approve", true),
		remoteDeviceDecision(deps, "deny", false),
	)
	return cmd
}

func runRemoteDevices(cmd *cobra.Command, client remoteCommandClient, _ []string) error {
	result, err := client.RemoteDevices(cmd.Context())
	if err != nil {
		return err
	}
	return writeCommandOutput(cmd, outputBundle{jsonValue: result, human: func() (string, error) {
		var output strings.Builder
		table := tabwriter.NewWriter(&output, 0, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(table, "DEVICE\tID\tSCOPES\tLAST SEEN")
		for _, device := range result.Devices {
			_, _ = fmt.Fprintf(table, "%s\t%s\t%s\t%s\n",
				device.Name, device.ID, scopeSummary(device), lastSeenSummary(device.LastSeenAt))
		}
		if err := table.Flush(); err != nil {
			return "", err
		}
		return strings.TrimSuffix(output.String(), "\n"), nil
	}})
}

func scopeSummary(device contract.RemoteDeviceResponse) string {
	if len(device.Scopes) == len(remoteproto.DefaultScopes(device.Admin)) {
		return "all"
	}
	names := make([]string, 0, len(device.Scopes))
	for _, scope := range device.Scopes {
		if scope == remoteproto.ScopeSessions {
			names = append(names, "sessions")
		} else {
			names = append(names, string(scope))
		}
	}
	return strings.Join(names, ", ")
}

func lastSeenSummary(lastSeen *time.Time) string {
	if lastSeen == nil {
		return "—"
	}
	elapsed := time.Since(*lastSeen)
	switch {
	case elapsed < time.Minute:
		return "now"
	case elapsed < time.Hour:
		return fmt.Sprintf("%dm ago", elapsed/time.Minute)
	case elapsed < 24*time.Hour:
		return fmt.Sprintf("%dh ago", elapsed/time.Hour)
	default:
		return fmt.Sprintf("%dd ago", elapsed/(24*time.Hour))
	}
}

func newRemoteDeviceScopesCommand(deps commandDeps) *cobra.Command {
	var set string
	cmd := remoteVerb(deps, "scopes <device>", "Set this device's scopes on this computer", cobra.ExactArgs(1),
		func(cmd *cobra.Command, client remoteCommandClient, args []string) error {
			if !cmd.Flags().Changed("set") {
				return newCLIUsageError("--set is required")
			}
			scopes := make([]remoteproto.Scope, 0)
			for part := range strings.SplitSeq(set, ",") {
				scope := remoteproto.Scope(strings.TrimSpace(part))
				if scope == "" {
					continue
				}
				if !scope.Valid() {
					return newCLIUsageError(fmt.Sprintf("unknown scope %q", scope))
				}
				if !slices.Contains(scopes, scope) {
					scopes = append(scopes, scope)
				}
			}
			return patchRemoteDevice(cmd, client, args[0], contract.RemoteDevicePatchRequest{Scopes: &scopes})
		})
	cmd.Flags().StringVar(&set, "set", "", "Comma-separated scopes")
	return cmd
}

func remoteDeviceDecision(deps commandDeps, verb string, approve bool) *cobra.Command {
	var approvalID string
	cmd := remoteVerb(deps, verb+" <device>", "Decide a pending device approval", cobra.ExactArgs(1),
		func(cmd *cobra.Command, client remoteCommandClient, args []string) error {
			return patchRemoteDevice(cmd, client, args[0], contract.RemoteDevicePatchRequest{
				Approved: new(approve), ApprovalID: approvalID,
			})
		})
	cmd.Flags().StringVar(&approvalID, "approval-id", "", "Select the pending key when multiple approvals exist")
	return cmd
}

func patchRemoteDevice(
	cmd *cobra.Command,
	client remoteCommandClient,
	id string,
	patch contract.RemoteDevicePatchRequest,
) error {
	result, err := client.PatchRemoteDevice(cmd.Context(), id, patch)
	if err != nil {
		return err
	}
	return writeCommandOutput(cmd, outputBundle{jsonValue: result, human: func() (string, error) {
		return fmt.Sprintf("Updated %s (%s).", result.Name, result.ID), nil
	}})
}

func runRemoteDeviceRevoke(cmd *cobra.Command, client remoteCommandClient, args []string) error {
	if err := client.RevokeRemoteDevice(cmd.Context(), args[0]); err != nil {
		return err
	}
	return writeCommandOutput(
		cmd,
		outputBundle{jsonValue: map[string]bool{cliRevokedValue: true}, human: func() (string, error) {
			return "Device revoked.", nil
		}},
	)
}
