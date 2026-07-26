package task

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
)

func (m *Service) timelineItemsFromRecords(
	ctx context.Context,
	records []EventRecord,
) ([]TimelineItem, error) {
	cache := make(map[string]*taskLiveContext)
	items := make([]TimelineItem, 0, len(records))
	for _, record := range records {
		contextForTask, err := m.loadTaskLiveContext(ctx, record.Event.TaskID, cache)
		if err != nil {
			return nil, err
		}

		var runSummary *RunSummary
		if strings.TrimSpace(record.Event.RunID) != "" {
			if run, ok := contextForTask.runsByID[record.Event.RunID]; ok {
				runSummary = runSummaryFromRun(run, contextForTask.record.MaxAttempts)
			}
		}

		items = append(items, TimelineItem{
			Sequence:  record.Sequence,
			EventID:   record.Event.ID,
			Task:      contextForTask.reference,
			Run:       runSummary,
			EventType: record.Event.EventType,
			Actor:     record.Event.Actor,
			Origin:    record.Event.Origin,
			Payload:   cloneRawJSON(record.Event.Payload),
			Timestamp: record.Event.Timestamp,
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		if !items[i].Timestamp.Equal(items[j].Timestamp) {
			return items[i].Timestamp.Before(items[j].Timestamp)
		}
		if items[i].Sequence != items[j].Sequence {
			return items[i].Sequence < items[j].Sequence
		}
		return items[i].EventID < items[j].EventID
	})
	return items, nil
}

func runSummaryFromRun(run Run, maxAttempts int) *RunSummary {
	summary := &RunSummary{
		ID:                           run.ID,
		TaskID:                       run.TaskID,
		Status:                       run.Status,
		Attempt:                      int(run.Attempt),
		RecoveryCount:                int(run.RecoveryCount),
		PreviousRunID:                run.PreviousRunID,
		FailureKind:                  run.FailureKind,
		MaxAttempts:                  maxAttempts,
		SessionID:                    run.SessionID,
		ClaimedBy:                    cloneActorIdentity(run.ClaimedBy),
		ClaimTokenHash:               run.ClaimTokenHash,
		LeaseUntil:                   run.LeaseUntil,
		HeartbeatAt:                  run.HeartbeatAt,
		ResolvedNetworkParticipation: participation.CloneSpec(run.NetworkSpecSnapshot()),
		DesignationGroupID:           run.DesignationGroupID,
		QueuedAt:                     run.QueuedAt,
		ClaimedAt:                    run.ClaimedAt,
		StartedAt:                    run.StartedAt,
		EndedAt:                      run.EndedAt,
		Error:                        run.Error,
	}
	ApplyRunDesignationSummary(summary, run)
	return summary
}

func baseRunSessionRef(sessionID string) *RunSessionRef {
	trimmed := strings.TrimSpace(sessionID)
	if trimmed == "" {
		return nil
	}
	return &RunSessionRef{SessionID: trimmed}
}

func (m *Service) bestEffortRunOperationalSummary(ctx context.Context, sessionID string) RunOperationalSummary {
	if m.runtimeViews == nil || strings.TrimSpace(sessionID) == "" {
		return RunOperationalSummary{}
	}

	events, eventsErr := m.runtimeViews.ListSessionEvents(ctx, sessionID, store.EventQuery{})
	stats, statsErr := m.runtimeViews.ListSessionTokenStats(ctx, sessionID)
	if eventsErr != nil && statsErr != nil {
		return RunOperationalSummary{}
	}

	summary := RunOperationalSummary{}
	if eventsErr == nil {
		summary = summarizeSessionEvents(events)
	}
	if statsErr == nil {
		mergeTokenStatsSummary(&summary, stats)
	}
	return summary
}

func summarizeSessionEvents(events []store.SessionEvent) RunOperationalSummary {
	sorted := append([]store.SessionEvent(nil), events...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Sequence != sorted[j].Sequence {
			return sorted[i].Sequence < sorted[j].Sequence
		}
		if !sorted[i].Timestamp.Equal(sorted[j].Timestamp) {
			return sorted[i].Timestamp.Before(sorted[j].Timestamp)
		}
		return sorted[i].ID < sorted[j].ID
	})

	summary := RunOperationalSummary{}
	toolCalls := make(map[string]struct{})
	for _, event := range sorted {
		if event.Timestamp.After(summary.LastActivityAt) {
			summary.LastActivityAt = event.Timestamp
			summary.LastEventType = event.Type
		}
		if event.Type != sessionEventTypeToolCall && event.Type != sessionEventTypeToolResult {
			continue
		}

		decoded := struct {
			ToolCallIDSnake string `json:"tool_call_id"`
			ToolCallIDCamel string `json:"toolCallId"`
		}{}
		if err := json.Unmarshal([]byte(event.Content), &decoded); err == nil {
			toolCallID := strings.TrimSpace(decoded.ToolCallIDSnake)
			if toolCallID == "" {
				toolCallID = strings.TrimSpace(decoded.ToolCallIDCamel)
			}
			if toolCallID != "" {
				toolCalls[toolCallID] = struct{}{}
				continue
			}
		}

		fallbackID := strings.TrimSpace(event.TurnID) + ":" + event.ID
		if fallbackID != ":" {
			toolCalls[fallbackID] = struct{}{}
			continue
		}
		toolCalls[event.ID] = struct{}{}
	}

	if len(toolCalls) > 0 {
		count := int64(len(toolCalls))
		summary.ToolCallCount = &count
	}
	return summary
}
