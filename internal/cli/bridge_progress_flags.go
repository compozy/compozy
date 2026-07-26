package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/spf13/cobra"
)

const (
	bridgeDeliveryProgressFlag          = "delivery-progress"
	bridgeDeliveryProgressGroupingFlag  = "delivery-progress-grouping"
	bridgeDeliveryProgressTypingFlag    = "delivery-progress-typing"
	bridgeDeliveryProgressReactionsFlag = "delivery-progress-reactions"
)

type bridgeProgressFlags struct {
	toolProgress string
	grouping     string
	typing       bool
	reactions    bool
}

func registerBridgeProgressFlags(cmd *cobra.Command, flags *bridgeProgressFlags) {
	cmd.Flags().StringVar(
		&flags.toolProgress,
		bridgeDeliveryProgressFlag,
		"",
		"Tool progress mode: off, new, all, or verbose",
	)
	cmd.Flags().StringVar(
		&flags.grouping,
		bridgeDeliveryProgressGroupingFlag,
		"",
		"Progress delivery grouping: accumulate or separate",
	)
	cmd.Flags().BoolVar(
		&flags.typing,
		bridgeDeliveryProgressTypingFlag,
		false,
		"Emit typing affordances for tool progress",
	)
	cmd.Flags().BoolVar(
		&flags.reactions,
		bridgeDeliveryProgressReactionsFlag,
		false,
		"Emit reaction affordances for tool progress",
	)
}

func (f bridgeProgressFlags) changed(cmd *cobra.Command) bool {
	return cmd.Flags().Changed(bridgeDeliveryProgressFlag) ||
		cmd.Flags().Changed(bridgeDeliveryProgressGroupingFlag) ||
		cmd.Flags().Changed(bridgeDeliveryProgressTypingFlag) ||
		cmd.Flags().Changed(bridgeDeliveryProgressReactionsFlag)
}

func mergeBridgeProgressDefaults(
	raw json.RawMessage,
	platform string,
	cmd *cobra.Command,
	flags bridgeProgressFlags,
) (contract.BridgeDeliveryDefaultsPayload, error) {
	var validated contract.BridgeDeliveryDefaultsPayload
	if len(raw) > 0 {
		if err := validated.UnmarshalJSON(raw); err != nil {
			return nil, fmt.Errorf("cli: invalid delivery defaults: %w", err)
		}
	}

	fields := make(map[string]json.RawMessage)
	if len(validated) > 0 && string(validated) != bridgeJSONNull {
		if err := json.Unmarshal(validated, &fields); err != nil {
			return nil, fmt.Errorf("cli: decode delivery defaults: %w", err)
		}
	}
	instance := &bridgepkg.BridgeInstance{
		Platform:         strings.TrimSpace(platform),
		DeliveryDefaults: json.RawMessage(validated),
	}
	config := bridgepkg.ResolveProgressConfig(instance, platform)
	if cmd.Flags().Changed(bridgeDeliveryProgressFlag) {
		config.ToolProgress = bridgepkg.ProgressMode(flags.toolProgress).Normalize()
	}
	if cmd.Flags().Changed(bridgeDeliveryProgressGroupingFlag) {
		config.Grouping = bridgepkg.ProgressGrouping(flags.grouping).Normalize()
	}
	if cmd.Flags().Changed(bridgeDeliveryProgressTypingFlag) {
		config.Typing = flags.typing
	}
	if cmd.Flags().Changed(bridgeDeliveryProgressReactionsFlag) {
		config.Reactions = flags.reactions
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("cli: invalid delivery progress: %w", err)
	}
	progressJSON, err := json.Marshal(contract.BridgeProgressConfigPayloadFromConfig(config))
	if err != nil {
		return nil, fmt.Errorf("cli: encode delivery progress: %w", err)
	}
	fields["progress"] = progressJSON
	merged, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("cli: encode delivery defaults: %w", err)
	}
	var payload contract.BridgeDeliveryDefaultsPayload
	if err := payload.UnmarshalJSON(merged); err != nil {
		return nil, fmt.Errorf("cli: validate delivery defaults: %w", err)
	}
	return payload, nil
}
