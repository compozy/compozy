package cli

import (
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

const gatewayOperationMatrix = `Remote operation matrix:
  Remote-capable: sessions, task reads, loops, memory, settings, bridges, extensions, and gateway management.
  Local-only: task mutations, task-run queue and scheduler authority, agent kernel, ` +
	`hosted MCP and MCP Host API, and resource mutation.
  Use a local daemon or an SSH forward for local-only operations.`

const (
	gatewaySchemeHTTPS           = "https"
	gatewaySchemeSSH             = "ssh"
	gatewayCommandDisable        = "disable"
	gatewayCommandEnable         = "enable"
	gatewayDeviceStatusUnchanged = "unchanged"
	gatewayTierKey               = "tier"
)

func newGatewayCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   clientTargetGateway,
		Short: "Manage remote gateway exposure",
		Long:  "Manage remote gateway exposure.\n\n" + gatewayOperationMatrix,
	}
	cmd.AddCommand(newGatewayStatusCommand(deps))
	cmd.AddCommand(newGatewayAuditCommand(deps))
	cmd.AddCommand(newGatewaySurfaceCommand(deps))
	cmd.AddCommand(newGatewayProviderCommand(deps))
	return cmd
}

func newGatewayAuditCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "audit",
		Short: "Audit gateway security posture",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := gatewayClientFromDeps(deps)
			if err != nil {
				return err
			}
			report, err := client.GetGatewayAudit(cmd.Context())
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, gatewayAuditOutput(report))
		},
	}
}

func newGatewayStatusCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   automationStatusKey,
		Short: "Show gateway exposure and device state",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := gatewayClientFromDeps(deps)
			if err != nil {
				return err
			}
			status, err := client.GetGatewayStatus(cmd.Context())
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, gatewayStatusOutput(status))
		},
	}
}

func newGatewaySurfaceCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{Use: "surface", Short: "Manage gateway surface exposure"}
	cmd.AddCommand(newGatewaySurfaceMutationCommand(deps, true))
	cmd.AddCommand(newGatewaySurfaceMutationCommand(deps, false))
	return cmd
}

func newGatewaySurfaceMutationCommand(deps commandDeps, enable bool) *cobra.Command {
	verb := gatewayCommandDisable
	desired := daemonDisabledKey
	if enable {
		verb = gatewayCommandEnable
		desired = extensionEnabledKey
	}
	var tier string
	var generation uint64
	var consent bool
	cmd := &cobra.Command{
		Use:   verb + " <surface>",
		Short: strings.ToUpper(verb[:1]) + verb[1:] + " one gateway surface",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := gatewayClientFromDeps(deps)
			if err != nil {
				return err
			}
			status, err := client.SetGatewaySurface(cmd.Context(), contract.GatewaySurfaceRequest{
				Surface: strings.TrimSpace(args[0]), Tier: strings.TrimSpace(tier), Desired: desired,
				ExpectedGeneration: generation, Consent: consent,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, gatewayStatusOutput(status))
		},
	}
	bindGatewayTierFlag(cmd, &tier)
	cmd.Flags().Uint64Var(&generation, "generation", 0, "Expected state generation")
	if enable {
		cmd.Flags().BoolVar(&consent, "consent", false, "Confirm public operator UI exposure")
	}
	return cmd
}

func newGatewayProviderCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{Use: providerProviderKey, Short: "Manage connectivity providers"}
	cmd.AddCommand(newGatewayProviderEnableCommand(deps))
	cmd.AddCommand(newGatewayProviderDisableCommand(deps))
	return cmd
}

func newGatewayProviderEnableCommand(deps commandDeps) *cobra.Command {
	var tier string
	var source string
	var digest string
	var generation uint64
	cmd := &cobra.Command{
		Use:   "enable <name>",
		Short: "Enable a connectivity provider for one tier",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := gatewayClientFromDeps(deps)
			if err != nil {
				return err
			}
			status, err := client.EnableGatewayProvider(
				cmd.Context(),
				strings.TrimSpace(args[0]),
				contract.GatewayProviderEnableRequest{
					Tier: strings.TrimSpace(tier), InstallSource: strings.TrimSpace(source),
					DigestConfirmed: strings.TrimSpace(digest), ExpectedGeneration: generation,
				},
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, gatewayStatusOutput(status))
		},
	}
	bindGatewayTierFlag(cmd, &tier)
	cmd.Flags().StringVar(&source, "source", "", "Installed extension source")
	cmd.Flags().StringVar(&digest, "digest", "", "Confirmed extension control digest")
	cmd.Flags().Uint64Var(&generation, "generation", 0, "Expected state generation")
	mustMarkFlagRequired(cmd, "source")
	return cmd
}

func newGatewayProviderDisableCommand(deps commandDeps) *cobra.Command {
	var tier string
	cmd := &cobra.Command{
		Use:   "disable <name>",
		Short: "Disable a connectivity provider for one tier",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := gatewayClientFromDeps(deps)
			if err != nil {
				return err
			}
			status, err := client.DisableGatewayProvider(
				cmd.Context(),
				strings.TrimSpace(args[0]),
				strings.TrimSpace(tier),
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, gatewayStatusOutput(status))
		},
	}
	bindGatewayTierFlag(cmd, &tier)
	return cmd
}

func bindGatewayTierFlag(cmd *cobra.Command, tier *string) {
	cmd.Flags().StringVar(tier, gatewayTierKey, "private", "Gateway tier: private or public")
}

func pairingArtifactOutput(record gatewayPairingArtifactReference) outputBundle {
	return simpleGatewayOutput(record, func() string {
		return renderHumanSection("Pairing", []keyValue{
			{Label: "Artifact File", Value: record.ArtifactRef},
			{Label: "Expires", Value: record.ExpiresAt.Format(time.RFC3339)},
		})
	}, func() string {
		return renderToonObject("pairing", []string{"artifact_ref", "expires_at"}, []string{
			record.ArtifactRef,
			record.ExpiresAt.Format(time.RFC3339),
		})
	})
}

func gatewayDeviceMutationOutput(value any, id string, name string, status string) outputBundle {
	canceled := ""
	if revoke, ok := value.(contract.GatewayRevokePayload); ok {
		canceled = strconv.Itoa(revoke.Canceled)
	}
	return simpleGatewayOutput(value, func() string {
		rows := []keyValue{
			{Label: "ID", Value: id},
			{Label: cliNameValue, Value: name},
			{Label: automationStatusValue, Value: status},
			{Label: "Changed", Value: strconv.FormatBool(status != gatewayDeviceStatusUnchanged)},
		}
		if canceled != "" {
			rows = append(rows, keyValue{Label: "Canceled Streams", Value: canceled})
		}
		return renderHumanSection("Device", rows)
	}, func() string {
		return renderToonObject(
			"device",
			[]string{"id", automationNameKey, automationStatusKey, windowManagerChangedField, cliCanceledKey},
			[]string{
				id,
				name,
				status,
				strconv.FormatBool(status != gatewayDeviceStatusUnchanged),
				canceled,
			},
		)
	})
}
