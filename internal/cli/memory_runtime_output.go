package cli

import (
	"encoding/json"

	"fmt"

	"time"

	"github.com/compozy/agh/internal/api/contract"
	memcontract "github.com/compozy/agh/internal/memory/contract"

	"github.com/spf13/cobra"
)

func memoryDreamTriggerBundle(response MemoryDreamTriggerRecord) outputBundle {
	return outputBundle{
		jsonValue: response,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, response)
		},
		human: func() (string, error) {
			return renderHumanSection("Memory Dream Trigger", []keyValue{
				{Label: "Triggered", Value: boolStatus(response.Triggered)},
				{Label: memoryStatusValue, Value: stringOrDash(string(response.Dream.Status))},
				{
					Label: automationScopeValue,
					Value: stringOrDash(memoryScopeLabel(response.Dream.Scope, response.Dream.AgentTier)),
				},
				{Label: memoryReasonValue, Value: stringOrDash(response.Reason)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"memory_dream_trigger",
				[]string{"triggered", memoryStatusKey, automationScopeKey, memoryReasonKey},
				[]string{
					boolStatus(response.Triggered),
					string(response.Dream.Status),
					memoryScopeLabel(response.Dream.Scope, response.Dream.AgentTier),
					response.Reason,
				},
			), nil
		},
	}
}

func memoryDreamListBundle(response MemoryDreamListRecord) outputBundle {
	bundle := memoryObjectBundle("Memory Dreams", response)
	bundle.jsonl = func(cmd *cobra.Command) error {
		return writeJSONLines(cmd, response.Dreams)
	}
	return bundle
}

func memoryProviderListBundle(response MemoryProviderListRecord) outputBundle {
	bundle := listBundle(
		response,
		response.Providers,
		"Memory Providers",
		[]string{automationNameValue, memoryStatusValue, memoryActiveValue, "Builtin", "Failure Count"},
		"providers",
		[]string{automationNameKey, memoryStatusKey, memoryActiveKey, "builtin", "failure_count"},
		func(item contract.MemoryProviderPayload) []string {
			return []string{
				stringOrDash(item.Name),
				stringOrDash(string(item.Status)),
				boolStatus(item.Active),
				boolStatus(item.Builtin),
				fmt.Sprintf("%d", item.FailureCount),
			}
		},
		func(item contract.MemoryProviderPayload) []string {
			return []string{
				item.Name,
				string(item.Status),
				boolStatus(item.Active),
				boolStatus(item.Builtin),
				fmt.Sprintf("%d", item.FailureCount),
			}
		},
	)
	bundle.jsonl = func(cmd *cobra.Command) error {
		return writeJSONLines(cmd, response.Providers)
	}
	return bundle
}

func memoryObjectBundle(title string, value any) outputBundle {
	return outputBundle{
		jsonValue: value,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, value)
		},
		human: func() (string, error) {
			data, err := json.MarshalIndent(value, "", "  ")
			if err != nil {
				return "", fmt.Errorf("cli: render %s: %w", title, err)
			}
			return renderHumanBlocks(title, string(data)), nil
		},
		toon: func() (string, error) {
			data, err := json.Marshal(value)
			if err != nil {
				return "", fmt.Errorf("cli: render %s toon: %w", title, err)
			}
			return renderToonObject(memoryMemoryKey, []string{memoryPayloadKey}, []string{string(data)}), nil
		},
	}
}

func memoryScopeLabel(scope memcontract.Scope, tier memcontract.AgentTier) string {
	normalized := scope.Normalize()
	if normalized == memcontract.ScopeAgent && tier.Normalize() != "" {
		return string(normalized) + ":" + string(tier.Normalize())
	}
	return string(normalized)
}

func formatMemoryOptionalTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return formatTime(*value)
}
