package cli

import (
	"errors"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/spf13/cobra"
)

func newBridgeRoutesCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "routes <id>",
		Short: "Inspect routes for one bridge instance",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			routes, err := client.BridgeRoutes(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, bridgeRoutesBundle(routes, deps.now))
		},
	}
}

func newBridgeTargetsCommand(deps commandDeps) *cobra.Command {
	var query string
	var limit int
	cmd := &cobra.Command{
		Use:   "targets <id>",
		Short: "List discovered targets for one bridge instance",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if limit < 0 {
				return errors.New("cli: --limit cannot be negative")
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			result, err := client.BridgeTargets(cmd.Context(), args[0], query, limit)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, bridgeTargetsBundle(result, deps.now))
		},
	}
	cmd.Flags().
		StringVarP(&query, "query", "q", "", "Filter targets by display name, qualifier, or route")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum targets to return")
	return cmd
}

func newBridgeResolveCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "resolve <id> <name>",
		Short: "Resolve a bridge target name without sending",
		Args:  exactTwoNonBlankArgs(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			result, err := client.ResolveBridgeTarget(cmd.Context(), args[0], args[1])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, bridgeResolveTargetBundle(result))
		},
	}
}

func newBridgeTestDeliveryCommand(deps commandDeps) *cobra.Command {
	var (
		message  string
		peerID   string
		threadID string
		groupID  string
		modeRaw  string
	)

	cmd := &cobra.Command{
		Use:   "test-delivery <id>",
		Short: "Resolve a typed outbound delivery target for one bridge instance",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			mode := bridgepkg.DeliveryMode(strings.TrimSpace(modeRaw)).Normalize()
			if mode != "" {
				if err := mode.Validate(); err != nil {
					return err
				}
			}

			item, err := client.TestBridgeDelivery(
				cmd.Context(),
				args[0],
				BridgeTestDeliveryRequest{
					Message: strings.TrimSpace(message),
					Target: BridgeDeliveryTargetInput{
						PeerID:   strings.TrimSpace(peerID),
						ThreadID: strings.TrimSpace(threadID),
						GroupID:  strings.TrimSpace(groupID),
						Mode:     mode,
					},
				},
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, bridgeTestDeliveryBundle(item))
		},
	}
	cmd.Flags().StringVar(&message, bridgeMessageKey, "", "Optional dry-run message label")
	cmd.Flags().StringVar(&peerID, "peer-id", "", "Override target peer ID")
	cmd.Flags().StringVar(&threadID, "thread-id", "", "Override target thread ID")
	cmd.Flags().StringVar(&groupID, "group-id", "", "Override target group ID")
	cmd.Flags().StringVar(&modeRaw, bridgeModeKey, "", "Delivery mode: direct-send or reply")
	return cmd
}
