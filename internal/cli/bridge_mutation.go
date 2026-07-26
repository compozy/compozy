package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/spf13/cobra"
)

const (
	bridgeDeliveryDefaultsFlag = "delivery-defaults"
	bridgeJSONNull             = "null"
	bridgeProviderConfigFlag   = "provider-config"
)

type bridgeCreateFlags struct {
	scopeRaw             string
	workspaceID          string
	platform             string
	extensionName        string
	displayName          string
	enabled              bool
	includePeer          bool
	includeThread        bool
	includeGroup         bool
	notificationSuppress bool
	providerConfig       string
	deliveryDefaults     string
	progress             bridgeProgressFlags
}

func newBridgeCreateCommand(deps commandDeps) *cobra.Command {
	flags := bridgeCreateFlags{}
	cmd := &cobra.Command{
		Use:   bridgeCreateKey,
		Short: "Create a bridge instance",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			payload, err := buildBridgeCreatePayload(cmd, flags)
			if err != nil {
				return err
			}
			item, err := client.CreateBridge(cmd.Context(), payload)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, bridgeBundle(item))
		},
	}
	cmd.Flags().StringVar(
		&flags.scopeRaw,
		bridgeScopeKey,
		string(bridgepkg.ScopeGlobal),
		"Bridge scope: global or workspace",
	)
	cmd.Flags().StringVar(&flags.workspaceID, "workspace-id", "", "Owning workspace ID for workspace-scoped bridges")
	cmd.Flags().StringVar(&flags.platform, "platform", "", "Messaging platform name")
	cmd.Flags().StringVar(&flags.extensionName, "extension", "", "Owning extension name")
	cmd.Flags().StringVar(&flags.displayName, "display-name", "", "Operator-facing bridge display name")
	cmd.Flags().BoolVar(&flags.enabled, bridgeEnabledKey, true, "Whether the instance starts enabled")
	cmd.Flags().BoolVar(&flags.includePeer, "include-peer", false, "Include peer identity in routing")
	cmd.Flags().BoolVar(&flags.includeThread, "include-thread", false, "Include thread identity in routing")
	cmd.Flags().BoolVar(&flags.includeGroup, "include-group", false, "Include group identity in routing")
	cmd.Flags().BoolVar(
		&flags.notificationSuppress,
		"notification-suppress",
		false,
		"Suppress notification deliveries to this bridge",
	)
	cmd.Flags().StringVar(
		&flags.providerConfig,
		bridgeProviderConfigFlag,
		"",
		"JSON object or null for provider runtime config",
	)
	cmd.Flags().StringVar(
		&flags.deliveryDefaults,
		bridgeDeliveryDefaultsFlag,
		"",
		"JSON object or null for delivery target defaults",
	)
	registerBridgeProgressFlags(cmd, &flags.progress)
	mustMarkFlagRequired(cmd, "platform")
	mustMarkFlagRequired(cmd, "extension")
	mustMarkFlagRequired(cmd, "display-name")
	return cmd
}

func buildBridgeCreatePayload(cmd *cobra.Command, flags bridgeCreateFlags) (CreateBridgeRequest, error) {
	scope, err := parseBridgeScope(flags.scopeRaw)
	if err != nil {
		return CreateBridgeRequest{}, err
	}
	if scope == bridgepkg.ScopeWorkspace && strings.TrimSpace(flags.workspaceID) == "" {
		return CreateBridgeRequest{}, errors.New("cli: --workspace-id is required when --scope=workspace")
	}

	payload := CreateBridgeRequest{
		Scope:         scope,
		WorkspaceID:   strings.TrimSpace(flags.workspaceID),
		Platform:      strings.TrimSpace(flags.platform),
		ExtensionName: strings.TrimSpace(flags.extensionName),
		DisplayName:   strings.TrimSpace(flags.displayName),
		Enabled:       flags.enabled,
		RoutingPolicy: bridgepkg.RoutingPolicy{
			IncludePeer:   flags.includePeer,
			IncludeThread: flags.includeThread,
			IncludeGroup:  flags.includeGroup,
		},
		NotificationSuppress: flags.notificationSuppress,
	}

	providerRaw, err := parseOptionalBridgeJSONWithLabel(flags.providerConfig, "provider config")
	if err != nil {
		return CreateBridgeRequest{}, err
	}
	if providerRaw != nil {
		payload.ProviderConfig = contract.BridgeProviderConfigPayload(*providerRaw)
	}
	deliveryRaw, err := parseOptionalBridgeJSON(flags.deliveryDefaults)
	if err != nil {
		return CreateBridgeRequest{}, err
	}
	if deliveryRaw != nil {
		if err := payload.DeliveryDefaults.UnmarshalJSON(*deliveryRaw); err != nil {
			return CreateBridgeRequest{}, fmt.Errorf("cli: invalid delivery defaults: %w", err)
		}
	}
	if flags.progress.changed(cmd) {
		payload.DeliveryDefaults, err = mergeBridgeProgressDefaults(
			json.RawMessage(payload.DeliveryDefaults),
			payload.Platform,
			cmd,
			flags.progress,
		)
		if err != nil {
			return CreateBridgeRequest{}, err
		}
	}
	if err := validateBridgeCreatePayload(payload); err != nil {
		return CreateBridgeRequest{}, err
	}
	return payload, nil
}

type bridgeUpdateFlags struct {
	displayName          string
	includePeer          bool
	includeThread        bool
	includeGroup         bool
	notificationSuppress bool
	providerConfig       string
	deliveryDefaults     string
	progress             bridgeProgressFlags
}

func newBridgeUpdateCommand(deps commandDeps) *cobra.Command {
	flags := bridgeUpdateFlags{}
	cmd := &cobra.Command{
		Use:   bridgeUpdateIDValue,
		Short: "Update mutable bridge fields",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBridgeUpdateCommand(cmd, deps, args[0], flags)
		},
	}
	cmd.Flags().StringVar(
		&flags.displayName,
		"display-name",
		"",
		"New operator-facing bridge display name",
	)
	cmd.Flags().BoolVar(
		&flags.includePeer,
		"include-peer",
		false,
		"Override whether routing includes peer identity",
	)
	cmd.Flags().BoolVar(
		&flags.includeThread,
		"include-thread",
		false,
		"Override whether routing includes thread identity",
	)
	cmd.Flags().BoolVar(
		&flags.includeGroup,
		"include-group",
		false,
		"Override whether routing includes group identity",
	)
	cmd.Flags().BoolVar(
		&flags.notificationSuppress,
		"notification-suppress",
		false,
		"Override whether notification deliveries are suppressed",
	)
	cmd.Flags().StringVar(
		&flags.providerConfig,
		bridgeProviderConfigFlag,
		"",
		"JSON object or null for provider runtime config",
	)
	cmd.Flags().StringVar(
		&flags.deliveryDefaults,
		bridgeDeliveryDefaultsFlag,
		"",
		"JSON object or null for delivery target defaults",
	)
	registerBridgeProgressFlags(cmd, &flags.progress)
	return cmd
}

func runBridgeUpdateCommand(cmd *cobra.Command, deps commandDeps, id string, flags bridgeUpdateFlags) error {
	client, err := clientFromDeps(deps)
	if err != nil {
		return err
	}
	req, err := buildBridgeUpdateRequest(cmd, client, id, flags)
	if err != nil {
		return err
	}
	item, err := client.UpdateBridge(cmd.Context(), id, req)
	if err != nil {
		return err
	}
	return writeCommandOutput(cmd, bridgeBundle(item))
}

func buildBridgeUpdateRequest(
	cmd *cobra.Command,
	client DaemonClient,
	id string,
	flags bridgeUpdateFlags,
) (UpdateBridgeRequest, error) {
	changes := bridgeUpdateChangesFromCommand(cmd, flags)
	if !changes.any() {
		return UpdateBridgeRequest{}, errors.New("cli: at least one update flag is required")
	}

	var current *BridgeRecord
	if changes.routing || changes.progress {
		item, err := client.GetBridge(cmd.Context(), id)
		if err != nil {
			return UpdateBridgeRequest{}, err
		}
		current = &item
	}

	req := UpdateBridgeRequest{}
	if changes.display {
		trimmed := strings.TrimSpace(flags.displayName)
		if trimmed == "" {
			return UpdateBridgeRequest{}, errors.New("cli: --display-name cannot be empty")
		}
		req.DisplayName = &trimmed
	}
	if changes.routing {
		policy := bridgeRoutingPolicyForUpdate(cmd, *current, flags)
		req.RoutingPolicy = &policy
	}
	deliveryDefaults, err := bridgeDeliveryDefaultsPatch(cmd, current, flags, changes)
	if err != nil {
		return UpdateBridgeRequest{}, err
	}
	req.DeliveryDefaults = deliveryDefaults
	if changes.provider {
		value, err := bridgeProviderConfigForUpdate(flags.providerConfig)
		if err != nil {
			return UpdateBridgeRequest{}, err
		}
		req.ProviderConfig = &value
	}
	if changes.notification {
		req.NotificationSuppress = &flags.notificationSuppress
	}
	return req, nil
}

type bridgeUpdateChanges struct {
	display      bool
	routing      bool
	delivery     bool
	progress     bool
	provider     bool
	notification bool
}

func bridgeUpdateChangesFromCommand(cmd *cobra.Command, flags bridgeUpdateFlags) bridgeUpdateChanges {
	return bridgeUpdateChanges{
		display:      cmd.Flags().Changed("display-name"),
		routing:      bridgeRoutingFlagsChanged(cmd),
		delivery:     cmd.Flags().Changed(bridgeDeliveryDefaultsFlag),
		progress:     flags.progress.changed(cmd),
		provider:     cmd.Flags().Changed(bridgeProviderConfigFlag),
		notification: cmd.Flags().Changed("notification-suppress"),
	}
}

func (c bridgeUpdateChanges) any() bool {
	return c.display || c.routing || c.delivery || c.progress || c.provider || c.notification
}

func bridgeRoutingPolicyForUpdate(
	cmd *cobra.Command,
	current BridgeRecord,
	flags bridgeUpdateFlags,
) bridgepkg.RoutingPolicy {
	policy := current.RoutingPolicy
	if cmd.Flags().Changed("include-peer") {
		policy.IncludePeer = flags.includePeer
	}
	if cmd.Flags().Changed("include-thread") {
		policy.IncludeThread = flags.includeThread
	}
	if cmd.Flags().Changed("include-group") {
		policy.IncludeGroup = flags.includeGroup
	}
	return policy
}

func bridgeDeliveryDefaultsPatch(
	cmd *cobra.Command,
	current *BridgeRecord,
	flags bridgeUpdateFlags,
	changes bridgeUpdateChanges,
) (*contract.BridgeDeliveryDefaultsPayload, error) {
	var value *contract.BridgeDeliveryDefaultsPayload
	if changes.delivery {
		deliveryDefaults, err := bridgeDeliveryDefaultsForUpdate(flags.deliveryDefaults)
		if err != nil {
			return nil, err
		}
		value = &deliveryDefaults
	}
	if !changes.progress {
		return value, nil
	}

	raw := current.DeliveryDefaults
	if value != nil {
		raw = json.RawMessage(*value)
	}
	merged, err := mergeBridgeProgressDefaults(raw, current.Platform, cmd, flags.progress)
	if err != nil {
		return nil, err
	}
	return &merged, nil
}

func bridgeDeliveryDefaultsForUpdate(rawValue string) (contract.BridgeDeliveryDefaultsPayload, error) {
	raw, err := parseRequiredBridgeJSON(strings.TrimSpace(rawValue))
	if err != nil {
		return nil, err
	}
	var payload contract.BridgeDeliveryDefaultsPayload
	if err := payload.UnmarshalJSON(*raw); err != nil {
		return nil, fmt.Errorf("cli: invalid delivery defaults: %w", err)
	}
	return payload, nil
}

func bridgeProviderConfigForUpdate(rawValue string) (contract.BridgeProviderConfigPayload, error) {
	raw, err := parseRequiredBridgeJSONWithLabel(strings.TrimSpace(rawValue), "provider config")
	if err != nil {
		return nil, err
	}
	return contract.BridgeProviderConfigPayload(*raw), nil
}
