package demoseed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/store/sessiondb"
	"github.com/compozy/compozy/internal/transcript"
	compozyworkspace "github.com/compozy/compozy/internal/workspace"
)

func seedSessions(
	ctx context.Context,
	db *globaldb.GlobalDB,
	paths config.HomePaths,
	workspace compozyworkspace.Workspace,
	stories []sessionStory,
) error {
	for _, story := range stories {
		if err := seedSession(ctx, db, paths, workspace, story); err != nil {
			return err
		}
	}
	return nil
}

func seedSession(
	ctx context.Context,
	db *globaldb.GlobalDB,
	paths config.HomePaths,
	workspace compozyworkspace.Workspace,
	story sessionStory,
) error {
	story.ToolInput = strings.ReplaceAll(story.ToolInput, "$WORKSPACE_ID", workspace.ID)
	sessionDir := filepath.Join(paths.SessionsDir, story.ID)
	if err := os.MkdirAll(sessionDir, 0o700); err != nil {
		return fmt.Errorf("demo seed: create session directory %q: %w", sessionDir, err)
	}
	if err := writeSessionTranscript(ctx, sessionDir, story); err != nil {
		return err
	}

	stopReason := store.StopCompleted
	networkSpec := participation.LocalSpec()
	meta := store.SessionMeta{
		ID: story.ID, Name: story.Name, AgentName: story.AgentName, Provider: story.Provider,
		EffectivePermissions: approveReads, WorkspaceID: workspace.ID, CWD: workspace.RootDir,
		SessionType: string(session.SessionTypeUser), State: string(session.StateStopped),
		StopReason: &stopReason, StopDetail: "Work completed and handed off.",
		NetworkParticipation: &networkSpec, CreatedAt: story.StartedAt, UpdatedAt: story.EndedAt,
	}
	if err := store.WriteSessionMeta(store.SessionMetaFile(sessionDir), meta); err != nil {
		return fmt.Errorf("demo seed: write metadata for session %q: %w", story.ID, err)
	}
	if err := db.RegisterSession(ctx, store.SessionInfo{
		ID: story.ID, Name: story.Name, AgentName: story.AgentName, Provider: story.Provider,
		WorkspaceID: workspace.ID, SessionType: string(session.SessionTypeUser),
		SessionNetworkState: &store.SessionNetworkState{NetworkSpec: networkSpec},
		State:               string(session.StateStopped), StopReason: store.StopCompleted,
		StopDetail: "Work completed and handed off.", CreatedAt: story.StartedAt, UpdatedAt: story.EndedAt,
	}); err != nil {
		return fmt.Errorf("demo seed: register session %q: %w", story.ID, err)
	}
	return nil
}

func writeSessionTranscript(ctx context.Context, sessionDir string, story sessionStory) (err error) {
	eventsDB, err := sessiondb.OpenSessionDB(ctx, story.ID, store.SessionDBFile(sessionDir))
	if err != nil {
		return fmt.Errorf("demo seed: open transcript for session %q: %w", story.ID, err)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
		defer cancel()
		if closeErr := eventsDB.Close(closeCtx); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("demo seed: close transcript for session %q: %w", story.ID, closeErr))
		}
	}()

	persisted, usage, err := sessionEvents(story)
	if err != nil {
		return err
	}
	if _, err := eventsDB.RecordPersistedBatch(ctx, persisted); err != nil {
		return fmt.Errorf("demo seed: write transcript for session %q: %w", story.ID, err)
	}
	if err := eventsDB.RecordTokenUsage(ctx, usage); err != nil {
		return fmt.Errorf("demo seed: write transcript usage for session %q: %w", story.ID, err)
	}
	return nil
}

func sessionEvents(story sessionStory) ([]store.SessionEvent, store.TokenUsage, error) {
	turnID := "turn_" + story.ID
	toolCallID := "call_" + story.ID
	total := story.Input + story.Output
	currency := "USD"
	timestamps := evenlySpaced(story.StartedAt, story.EndedAt, 6)
	toolInput := json.RawMessage(story.ToolInput)
	if !json.Valid(toolInput) {
		return nil, store.TokenUsage{}, fmt.Errorf("demo seed: invalid tool input for session %q", story.ID)
	}
	resultRaw, err := json.Marshal(map[string]any{
		"status":  taskStatusCompleted,
		"content": []map[string]string{{"type": jsonTextKey, jsonTextKey: story.ToolResult}},
	})
	if err != nil {
		return nil, store.TokenUsage{}, fmt.Errorf("demo seed: encode tool result for session %q: %w", story.ID, err)
	}
	usage := store.TokenUsage{
		TurnID: turnID, InputTokens: &story.Input, OutputTokens: &story.Output,
		TotalTokens: &total, CostAmount: &story.CostUSD, CostCurrency: &currency, Timestamp: story.EndedAt,
	}
	agentUsage := acp.TokenUsage{
		TurnID: turnID, InputTokens: &story.Input, OutputTokens: &story.Output,
		TotalTokens: &total, CostAmount: &story.CostUSD, CostCurrency: &currency, Timestamp: story.EndedAt,
	}
	toolCallEvent := acp.AgentEvent{
		Type: acp.EventTypeToolCall, SessionID: story.ID, TurnID: turnID, Title: story.ToolName,
		ToolCallID: toolCallID, Timestamp: timestamps[2],
	}
	toolCallEvent = toolCallEvent.WithTool(story.ToolName, toolInput, false).WithToolKind(story.ToolKind)
	toolResultEvent := acp.AgentEvent{
		Type: acp.EventTypeToolResult, SessionID: story.ID, TurnID: turnID, Title: story.ToolName,
		ToolCallID: toolCallID, Raw: resultRaw, Timestamp: timestamps[3],
	}
	toolResultEvent = toolResultEvent.WithTool(story.ToolName, toolInput, false).WithToolKind(story.ToolKind)
	agentEvents := []acp.AgentEvent{
		{
			Type: acp.EventTypeUserMessage, SessionID: story.ID, TurnID: turnID,
			Text: story.Prompt, Timestamp: timestamps[0],
		},
		{
			Type: acp.EventTypeAgentMessage, SessionID: story.ID, TurnID: turnID,
			Text: story.Opening, Timestamp: timestamps[1],
		},
		toolCallEvent,
		toolResultEvent,
		{
			Type: acp.EventTypeAgentMessage, SessionID: story.ID, TurnID: turnID,
			Text: story.Conclusion, Timestamp: timestamps[4],
		},
		{Type: acp.EventTypeDone, SessionID: story.ID, TurnID: turnID, StopReason: string(store.StopCompleted),
			Usage: &agentUsage, Timestamp: timestamps[5]},
	}
	persisted := make([]store.SessionEvent, 0, len(agentEvents))
	for index, event := range agentEvents {
		content, err := transcript.MarshalAgentEvent(event)
		if err != nil {
			return nil, store.TokenUsage{}, fmt.Errorf(
				"demo seed: encode event %d for session %q: %w",
				index,
				story.ID,
				err,
			)
		}
		persisted = append(persisted, store.SessionEvent{
			ID: fmt.Sprintf("evt_%s_%02d", story.ID, index+1), SessionID: story.ID,
			TurnID: turnID, Type: event.Type, AgentName: story.AgentName,
			Content: content, Timestamp: event.Timestamp,
		})
	}
	return persisted, usage, nil
}

func evenlySpaced(start time.Time, end time.Time, count int) []time.Time {
	result := make([]time.Time, count)
	step := end.Sub(start) / time.Duration(count-1)
	for index := range result {
		result[index] = start.Add(time.Duration(index) * step)
	}
	return result
}
