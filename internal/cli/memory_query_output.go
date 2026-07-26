package cli

import (
	"fmt"

	"strings"
	"time"

	"github.com/spf13/cobra"
)

func boolStatus(value bool) string {
	if value {
		return toolBoolTrue
	}
	return toolBoolFalse
}

func memoryListBundle(response MemoryListRecord, now func() time.Time) outputBundle {
	items := make([]memoryListItem, 0, len(response.Memories))
	for _, memory := range response.Memories {
		items = append(items, memoryListItem{
			Filename:        memory.Filename,
			Name:            memory.Name,
			Type:            memory.Type,
			Scope:           memory.Scope,
			WorkspaceID:     memory.WorkspaceID,
			AgentName:       memory.AgentName,
			AgentTier:       memory.AgentTier,
			Age:             formatAge(now, memory.ModTime),
			Description:     memory.Description,
			StalenessBanner: memory.StalenessBanner,
			ModTime:         memory.ModTime,
		})
	}
	bundle := listBundle(
		response,
		items,
		"Memories",
		[]string{
			memoryFilenameValue,
			automationNameValue,
			sessionTypeValue,
			automationScopeValue,
			"Age",
			memoryDescriptionValue,
		},
		"memories",
		[]string{memoryFilenameKey, automationNameKey, memoryTypeKey, automationScopeKey, "age", memoryDescriptionKey},
		func(item memoryListItem) []string {
			return []string{
				stringOrDash(item.Filename),
				stringOrDash(item.Name),
				stringOrDash(string(item.Type)),
				stringOrDash(memoryScopeLabel(item.Scope, item.AgentTier)),
				stringOrDash(item.Age),
				stringOrDash(item.Description),
			}
		},
		func(item memoryListItem) []string {
			return []string{
				item.Filename,
				item.Name,
				string(item.Type),
				memoryScopeLabel(item.Scope, item.AgentTier),
				item.Age,
				item.Description,
			}
		},
	)
	bundle.jsonl = func(cmd *cobra.Command) error {
		return writeJSONLines(cmd, response.Memories)
	}
	return bundle
}

func memoryEntryBundle(response MemoryEntryRecord) outputBundle {
	return outputBundle{
		jsonValue: response,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, response)
		},
		human: func() (string, error) {
			return strings.TrimRight(response.Memory.Content, "\n"), nil
		},
		toon: func() (string, error) {
			summary := response.Memory.Summary
			return renderToonObject(
				memoryMemoryKey,
				[]string{memoryFilenameKey, automationScopeKey, memoryContentKey},
				[]string{
					summary.Filename,
					memoryScopeLabel(summary.Scope, summary.AgentTier),
					response.Memory.Content,
				},
			), nil
		},
	}
}

func memorySearchBundle(response MemorySearchRecord) outputBundle {
	items := make([]memorySearchItem, 0, len(response.Results))
	for _, result := range response.Results {
		items = append(items, memorySearchItem{
			Filename:      result.Memory.Filename,
			Name:          result.Memory.Name,
			Type:          result.Memory.Type,
			Scope:         result.Memory.Scope,
			AgentTier:     result.Memory.AgentTier,
			Score:         result.Score,
			Snippet:       result.Snippet,
			WhyRecalled:   result.WhyRecalled,
			ShadowedBy:    result.ShadowedBy,
			AlreadyShown:  result.AlreadyShown,
			StalenessNote: result.Memory.StalenessBanner,
		})
	}
	bundle := listBundle(
		response,
		items,
		"Memory Search",
		[]string{memoryFilenameValue, automationNameValue, automationScopeValue, "Score", "Snippet"},
		"results",
		[]string{memoryFilenameKey, automationNameKey, automationScopeKey, "score", "snippet"},
		func(item memorySearchItem) []string {
			return []string{
				stringOrDash(item.Filename),
				stringOrDash(item.Name),
				stringOrDash(memoryScopeLabel(item.Scope, item.AgentTier)),
				fmt.Sprintf("%.2f", item.Score),
				stringOrDash(item.Snippet),
			}
		},
		func(item memorySearchItem) []string {
			return []string{
				item.Filename,
				item.Name,
				memoryScopeLabel(item.Scope, item.AgentTier),
				fmt.Sprintf("%.2f", item.Score),
				item.Snippet,
			}
		},
	)
	bundle.jsonl = func(cmd *cobra.Command) error {
		return writeJSONLines(cmd, response.Results)
	}
	return bundle
}

func memoryHealthBundle(view MemoryHealthRecord) outputBundle {
	return outputBundle{
		jsonValue: view,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, view)
		},
		human: func() (string, error) {
			return renderHumanSection("Memory Health", []keyValue{
				{Label: memoryStatusValue, Value: stringOrDash(view.Status)},
				{Label: memoryReasonValue, Value: stringOrDash(view.Reason)},
				{Label: memoryEnabledValue, Value: fmt.Sprintf("%t", view.Enabled)},
				{Label: "Configured", Value: fmt.Sprintf("%t", view.Configured)},
				{Label: "Global Dir", Value: stringOrDash(view.GlobalDir)},
				{Label: "Global Files", Value: fmt.Sprintf("%d", view.GlobalFiles)},
				{Label: "Workspace Files", Value: fmt.Sprintf("%d", view.WorkspaceFiles)},
				{Label: "Workspace Count", Value: fmt.Sprintf("%d", view.WorkspaceCount)},
				{Label: "Indexed Files", Value: fmt.Sprintf("%d", view.IndexedFiles)},
				{Label: "Orphaned Files", Value: fmt.Sprintf("%d", view.OrphanedFiles)},
				{Label: "Operation Count", Value: fmt.Sprintf("%d", view.OperationCount)},
				{Label: "Last Operation", Value: stringOrDash(formatMemoryOptionalTime(view.LastOperationAt))},
				{Label: "Dream Enabled", Value: fmt.Sprintf("%t", view.DreamEnabled)},
				{Label: "Dream Agent", Value: stringOrDash(view.DreamAgent)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"memory_health",
				[]string{
					memoryStatusKey,
					memoryEnabledKey,
					"configured",
					"global_files",
					"workspace_files",
					"operation_count",
				},
				[]string{
					view.Status,
					fmt.Sprintf("%t", view.Enabled),
					fmt.Sprintf("%t", view.Configured),
					fmt.Sprintf("%d", view.GlobalFiles),
					fmt.Sprintf("%d", view.WorkspaceFiles),
					fmt.Sprintf("%d", view.OperationCount),
				},
			), nil
		},
	}
}
