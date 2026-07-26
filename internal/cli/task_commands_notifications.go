package cli

import (
	"errors"
	"fmt"

	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges"

	"github.com/spf13/cobra"
)

func newTaskNotificationCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "notification",
		Short: "Manage task terminal notifications",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newTaskNotificationSubscribeCommand(deps))
	cmd.AddCommand(newTaskNotificationListCommand(deps))
	cmd.AddCommand(newTaskNotificationShowCommand(deps))
	cmd.AddCommand(newTaskNotificationDeleteCommand(deps))
	return cmd
}

func newTaskNotificationSubscribeCommand(deps commandDeps) *cobra.Command {
	input := taskNotificationSubscribeInput{}
	cmd := &cobra.Command{
		Use:   "subscribe <task-id>",
		Short: "Subscribe a bridge target to task terminal notifications",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			request, err := buildTaskBridgeNotificationSubscriptionRequest(input)
			if err != nil {
				return err
			}
			subscription, err := client.CreateTaskBridgeNotificationSubscription(
				cmd.Context(),
				args[0],
				request,
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskBridgeNotificationSubscriptionBundle(&subscription))
		},
	}
	cmd.Flags().
		StringVar(&input.SubscriptionID, "subscription-id", "", "Idempotent subscription ID")
	cmd.Flags().StringVar(&input.BridgeInstanceID, "bridge", "", "Bridge instance ID")
	cmd.Flags().
		StringVar(&input.ScopeRaw, taskScopeKey, string(bridgepkg.ScopeGlobal), "Bridge scope: global or workspace")
	cmd.Flags().
		StringVar(&input.WorkspaceID, "workspace", "", "Workspace ID for workspace bridge scope")
	cmd.Flags().StringVar(&input.PeerID, "peer", "", "Bridge peer ID")
	cmd.Flags().StringVar(&input.ThreadID, "thread", "", "Bridge thread ID")
	cmd.Flags().StringVar(&input.GroupID, "group", "", "Bridge group ID")
	cmd.Flags().StringVar(
		&input.DeliveryModeRaw,
		"mode",
		string(bridgepkg.DeliveryModeReply),
		"Delivery mode: reply or direct-send",
	)
	mustMarkFlagRequired(cmd, "bridge")
	return cmd
}

func newTaskNotificationListCommand(deps commandDeps) *cobra.Command {
	var (
		bridgeInstanceID string
		scopeRaw         string
		workspaceID      string
		last             int
	)
	cmd := &cobra.Command{
		Use:   "list <task-id>",
		Short: "List bridge terminal notification subscriptions for one task",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			query, err := buildTaskBridgeNotificationSubscriptionListQuery(
				bridgeInstanceID,
				scopeRaw,
				workspaceID,
				last,
			)
			if err != nil {
				return err
			}
			subscriptions, err := client.ListTaskBridgeNotificationSubscriptions(
				cmd.Context(),
				args[0],
				query,
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(
				cmd,
				taskBridgeNotificationSubscriptionListBundle(subscriptions),
			)
		},
	}
	cmd.Flags().StringVar(&bridgeInstanceID, "bridge", "", "Filter by bridge instance ID")
	cmd.Flags().
		StringVar(&scopeRaw, taskScopeKey, "", "Filter by bridge scope: global or workspace")
	cmd.Flags().StringVar(&workspaceID, "workspace", "", "Filter by workspace ID")
	cmd.Flags().IntVar(&last, "last", 0, "Show only the most recent N subscriptions")
	return cmd
}

func newTaskNotificationShowCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "show <task-id> <subscription-id>",
		Short: "Show one bridge terminal notification subscription",
		Args:  exactTwoNonBlankArgs(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			subscription, err := client.GetTaskBridgeNotificationSubscription(
				cmd.Context(),
				args[0],
				args[1],
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskBridgeNotificationSubscriptionBundle(&subscription))
		},
	}
}

func newTaskNotificationDeleteCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <task-id> <subscription-id>",
		Short: "Delete one bridge terminal notification subscription",
		Args:  exactTwoNonBlankArgs(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			if err := client.DeleteTaskBridgeNotificationSubscription(cmd.Context(), args[0], args[1]); err != nil {
				return err
			}
			return writeCommandOutput(
				cmd,
				taskBridgeNotificationSubscriptionDeleteBundle(args[0], args[1]),
			)
		},
	}
}

func buildTaskBridgeNotificationSubscriptionRequest(
	input taskNotificationSubscribeInput,
) (*TaskBridgeNotificationSubscriptionRequest, error) {
	scope := bridgepkg.Scope(strings.TrimSpace(input.ScopeRaw)).Normalize()
	if err := scope.Validate(); err != nil {
		return nil, fmt.Errorf("cli: invalid notification scope: %w", err)
	}
	mode := bridgepkg.DeliveryMode(input.DeliveryModeRaw).Normalize()
	if err := mode.Validate(); err != nil {
		return nil, fmt.Errorf("cli: invalid delivery mode: %w", err)
	}
	request := TaskBridgeNotificationSubscriptionRequest{
		SubscriptionID:   strings.TrimSpace(input.SubscriptionID),
		BridgeInstanceID: strings.TrimSpace(input.BridgeInstanceID),
		Scope:            scope,
		WorkspaceID:      strings.TrimSpace(input.WorkspaceID),
		PeerID:           strings.TrimSpace(input.PeerID),
		ThreadID:         strings.TrimSpace(input.ThreadID),
		GroupID:          strings.TrimSpace(input.GroupID),
		DeliveryMode:     mode,
	}
	if err := bridgepkg.ValidateScopeWorkspaceID(request.Scope, request.WorkspaceID); err != nil {
		return nil, fmt.Errorf("cli: invalid notification scope: %w", err)
	}
	if strings.TrimSpace(request.PeerID) == "" && strings.TrimSpace(request.GroupID) == "" {
		return nil, errors.New("cli: notification subscription requires --peer or --group")
	}
	return &request, nil
}

func buildTaskBridgeNotificationSubscriptionListQuery(
	bridgeInstanceID string,
	scopeRaw string,
	workspaceID string,
	last int,
) (TaskBridgeNotificationSubscriptionQuery, error) {
	if last < 0 {
		return TaskBridgeNotificationSubscriptionQuery{}, errors.New(
			"cli: --last must be zero or positive",
		)
	}
	query := TaskBridgeNotificationSubscriptionQuery{
		BridgeInstanceID: strings.TrimSpace(bridgeInstanceID),
		WorkspaceID:      strings.TrimSpace(workspaceID),
		Limit:            last,
	}
	if trimmed := strings.TrimSpace(scopeRaw); trimmed != "" {
		scope := bridgepkg.Scope(trimmed).Normalize()
		if err := scope.Validate(); err != nil {
			return TaskBridgeNotificationSubscriptionQuery{}, fmt.Errorf(
				"cli: invalid notification scope: %w",
				err,
			)
		}
		query.Scope = scope
	}
	return query, nil
}
