package cli

import (
	"fmt"

	"time"

	"github.com/compozy/agh/internal/api/contract"

	"github.com/spf13/cobra"
)

func memoryHistoryBundle(records []MemoryHistoryRecord, now func() time.Time) outputBundle {
	items := make([]memoryHistoryItem, 0, len(records))
	for _, record := range records {
		items = append(items, memoryHistoryItem{
			ID:          record.ID,
			Operation:   record.Operation,
			Scope:       record.Scope,
			WorkspaceID: record.WorkspaceID,
			Filename:    record.Filename,
			AgentName:   record.AgentName,
			AgentTier:   record.AgentTier,
			Summary:     record.Summary,
			Age:         formatAge(now, record.Timestamp),
			Timestamp:   record.Timestamp,
		})
	}
	return listBundle(
		contract.MemoryOperationHistoryResponse{Operations: records},
		items,
		"Memory History",
		[]string{"Time", memoryOperationValue, automationScopeValue, memoryFilenameValue, authoredContextSummaryValue},
		"operations",
		[]string{networkTimestampKey, memoryOperationKey, automationScopeKey, memoryFilenameKey, memorySummaryKey},
		func(item memoryHistoryItem) []string {
			return []string{
				stringOrDash(formatTime(item.Timestamp)),
				stringOrDash(string(item.Operation)),
				stringOrDash(memoryScopeLabel(item.Scope, item.AgentTier)),
				stringOrDash(item.Filename),
				stringOrDash(item.Summary),
			}
		},
		func(item memoryHistoryItem) []string {
			return []string{
				formatTime(item.Timestamp),
				string(item.Operation),
				memoryScopeLabel(item.Scope, item.AgentTier),
				item.Filename,
				item.Summary,
			}
		},
	)
}

func memoryMutationBundle(title string, response MemoryMutationRecord) outputBundle {
	return memoryDecisionBundle(title, response.Decision, response)
}

func memoryDeleteBundle(response MemoryDeleteRecord) outputBundle {
	return memoryDecisionBundle("Memory Delete", response.Decision, response)
}

func memoryPromoteBundle(response MemoryPromoteRecord) outputBundle {
	return memoryDecisionBundle("Memory Promote", response.Decision, response)
}

func memoryDecisionRevertBundle(response MemoryDecisionRevertRecord) outputBundle {
	return memoryDecisionBundle("Memory Decision Revert", response.Decision, response)
}

func memoryDecisionBundle(title string, decision contract.MemoryDecisionPayload, jsonValue any) outputBundle {
	return outputBundle{
		jsonValue: jsonValue,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, jsonValue)
		},
		human: func() (string, error) {
			return renderHumanSection(title, []keyValue{
				{Label: "Decision ID", Value: stringOrDash(decision.ID)},
				{Label: memoryOperationValue, Value: stringOrDash(string(decision.Op))},
				{
					Label: automationScopeValue,
					Value: stringOrDash(memoryScopeLabel(decision.Scope, decision.AgentTier)),
				},
				{Label: memoryFilenameValue, Value: stringOrDash(decision.TargetFilename)},
				{Label: "Confidence", Value: fmt.Sprintf("%.2f", decision.Confidence)},
				{Label: memorySourceValue, Value: stringOrDash(string(decision.Source))},
				{Label: memoryReasonValue, Value: stringOrDash(decision.Reason)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"memory_decision",
				[]string{"id", "op", automationScopeKey, memoryFilenameKey, memoryReasonKey},
				[]string{
					decision.ID,
					string(decision.Op),
					memoryScopeLabel(decision.Scope, decision.AgentTier),
					decision.TargetFilename,
					decision.Reason,
				},
			), nil
		},
	}
}

func memoryReindexBundle(view MemoryReindexRecord) outputBundle {
	return outputBundle{
		jsonValue: view,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, view)
		},
		human: func() (string, error) {
			return renderHumanSection("Memory Reindex", []keyValue{
				{Label: automationScopeValue, Value: stringOrDash(memoryScopeLabel(view.Scope, view.AgentTier))},
				{Label: "Workspace ID", Value: stringOrDash(view.WorkspaceID)},
				{Label: memoryAgentValue, Value: stringOrDash(view.AgentName)},
				{Label: "Indexed Files", Value: fmt.Sprintf("%d", view.IndexedFiles)},
				{Label: "Completed At", Value: view.CompletedAt.Format(time.RFC3339)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"memory_reindex",
				[]string{
					automationScopeKey,
					automationWorkspaceIDKey,
					memoryAgentNameKey,
					"indexed_files",
					"completed_at",
				},
				[]string{
					memoryScopeLabel(view.Scope, view.AgentTier),
					view.WorkspaceID,
					view.AgentName,
					fmt.Sprintf("%d", view.IndexedFiles),
					view.CompletedAt.Format(time.RFC3339),
				},
			), nil
		},
	}
}

func memoryScopeShowBundle(response MemoryScopeShowRecord) outputBundle {
	return memoryObjectBundle("Memory Scope", response)
}

func memoryDecisionListBundle(response MemoryDecisionListRecord) outputBundle {
	bundle := memoryObjectBundle("Memory Decisions", response)
	bundle.jsonl = func(cmd *cobra.Command) error {
		return writeJSONLines(cmd, response.Decisions)
	}
	return bundle
}
