package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/spf13/cobra"
)

func parseBridgeScope(raw string) (bridgepkg.Scope, error) {
	scope := bridgepkg.Scope(strings.TrimSpace(raw)).Normalize()
	if err := scope.Validate(); err != nil {
		return "", err
	}
	return scope, nil
}

func validateBridgeCreatePayload(payload CreateBridgeRequest) error {
	if _, err := payload.ToCreateInstanceRequest(); err != nil {
		return fmt.Errorf("cli: invalid bridge create payload: %w", err)
	}
	return nil
}

func bridgeRoutingFlagsChanged(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	return cmd.Flags().Changed("include-peer") ||
		cmd.Flags().Changed("include-thread") ||
		cmd.Flags().Changed("include-group")
}

func parseOptionalBridgeJSON(raw string) (*json.RawMessage, error) {
	return parseOptionalBridgeJSONWithLabel(raw, "delivery defaults")
}

func parseOptionalBridgeJSONWithLabel(raw string, label string) (*json.RawMessage, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	return parseRequiredBridgeJSONWithLabel(trimmed, label)
}

func parseRequiredBridgeJSON(raw string) (*json.RawMessage, error) {
	return parseRequiredBridgeJSONWithLabel(raw, "delivery defaults")
}

func parseRequiredBridgeJSONWithLabel(raw string, label string) (*json.RawMessage, error) {
	trimmed := strings.TrimSpace(raw)
	trimmedLabel := strings.TrimSpace(label)
	if trimmedLabel == "" {
		trimmedLabel = "JSON payload"
	}
	if trimmed == "" {
		return nil, fmt.Errorf("cli: %s must be valid JSON; use null to clear", trimmedLabel)
	}
	var decoded any
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return nil, fmt.Errorf("cli: %s must be valid JSON", trimmedLabel)
	}
	switch decoded.(type) {
	case nil, map[string]any:
	default:
		return nil, fmt.Errorf("cli: %s must be a JSON object or null", trimmedLabel)
	}
	value := json.RawMessage(trimmed)
	return &value, nil
}

func bridgeRoutingPolicyLabel(policy bridgepkg.RoutingPolicy) string {
	dimensions := make([]string, 0, 3)
	if policy.IncludePeer {
		dimensions = append(dimensions, "peer")
	}
	if policy.IncludeThread {
		dimensions = append(dimensions, "thread")
	}
	if policy.IncludeGroup {
		dimensions = append(dimensions, "group")
	}
	return strings.Join(dimensions, ", ")
}
