package cli

import (
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func toolSourceSummary(source contract.ToolSourceRefPayload) string {
	owner := strings.TrimSpace(source.Owner)
	if owner == "" {
		return string(source.Kind)
	}
	return string(source.Kind) + ":" + owner
}

func toolAvailabilitySummary(availability contract.ToolAvailabilityPayload) string {
	switch {
	case availability.Conflicted:
		return "conflicted"
	case !availability.Registered:
		return "unregistered"
	case !availability.Enabled:
		return toolOperatorDisabledKey
	case !availability.Available:
		return "unavailable"
	case !availability.Authorized:
		return "auth-required"
	case !availability.Executable:
		return "not-executable"
	default:
		return toolOperatorAvailableKey
	}
}

func joinReasons(groups ...[]toolspkg.ReasonCode) string {
	seen := make(map[toolspkg.ReasonCode]struct{})
	values := make([]string, 0)
	for _, group := range groups {
		for _, reason := range group {
			if reason == "" {
				continue
			}
			if _, ok := seen[reason]; ok {
				continue
			}
			seen[reason] = struct{}{}
			values = append(values, string(reason))
		}
	}
	return strings.Join(values, ",")
}

func toolIDsToStrings(ids []toolspkg.ToolID) []string {
	values := make([]string, 0, len(ids))
	for _, id := range ids {
		values = append(values, id.String())
	}
	return values
}

func toolsetIDsToStrings(ids []toolspkg.ToolsetID) []string {
	values := make([]string, 0, len(ids))
	for _, id := range ids {
		values = append(values, id.String())
	}
	return values
}

func formatBool(value bool) string {
	if value {
		return toolBoolTrue
	}
	return toolBoolFalse
}

const (
	toolBoolFalse = "false"
)
