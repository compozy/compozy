package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/sessiondb"
	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/transcript"
)

func TestManagerRepairSession(t *testing.T) {
	t.Parallel()

	t.Run("ShouldReportPlannedActionsWithoutPersistingWhenDryRun", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		meta := repairSessionMeta("sess-repair-dry", store.StopAgentCrashed, h.workspaceID)
		seedRepairSession(t, h, meta, interruptedTurnEvents(t, meta.ID, meta.AgentName)...)

		result, err := h.manager.RepairSession(testutil.Context(t), RepairOpts{
			SessionID: meta.ID,
			DryRun:    true,
		})
		if err != nil {
			t.Fatalf("RepairSession(dry-run) error = %v", err)
		}
		if result.Persisted {
			t.Fatal("RepairSession(dry-run).Persisted = true, want false")
		}
		if got, want := len(result.Actions), 2; got != want {
			t.Fatalf("RepairSession(dry-run) actions = %d, want %d", got, want)
		}
		for _, action := range result.Actions {
			if action.Persisted {
				t.Fatalf("dry-run action %#v persisted, want planned only", action)
			}
		}

		events := readRepairEvents(t, h, meta.ID)
		if got, want := len(events), 3; got != want {
			t.Fatalf("stored events after dry-run = %d, want %d", got, want)
		}
		if containsEventType(events, acp.EventTypeError) {
			t.Fatalf("stored events contain %q after dry-run", acp.EventTypeError)
		}
	})

	t.Run("ShouldAppendInterruptedToolResultAndTerminalError", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		meta := repairSessionMeta("sess-repair-append", store.StopAgentCrashed, h.workspaceID)
		seedRepairSession(t, h, meta, interruptedTurnEvents(t, meta.ID, meta.AgentName)...)

		result, err := h.manager.RepairSession(testutil.Context(t), RepairOpts{SessionID: meta.ID})
		if err != nil {
			t.Fatalf("RepairSession() error = %v", err)
		}
		if !result.Persisted {
			t.Fatal("RepairSession().Persisted = false, want true")
		}
		if got, want := len(result.Actions), 2; got != want {
			t.Fatalf("RepairSession() actions = %d, want %d", got, want)
		}
		for _, action := range result.Actions {
			if !action.Persisted || action.EventID == "" {
				t.Fatalf("persisted action = %#v, want persisted event id", action)
			}
		}

		events := readRepairEvents(t, h, meta.ID)
		if got, want := len(events), 5; got != want {
			t.Fatalf("stored events after repair = %d, want %d", got, want)
		}
		if got, want := events[3].Type, acp.EventTypeToolResult; got != want {
			t.Fatalf("events[3].Type = %q, want %q", got, want)
		}
		if got, want := events[4].Type, acp.EventTypeError; got != want {
			t.Fatalf("events[4].Type = %q, want %q", got, want)
		}

		toolResult, err := transcript.UnmarshalAgentEvent(events[3].Content)
		if err != nil {
			t.Fatalf("UnmarshalAgentEvent(tool result) error = %v", err)
		}
		if got, want := toolResult.ToolCallID, "tool-1"; got != want {
			t.Fatalf("repair tool result ToolCallID = %q, want %q", got, want)
		}
		if toolResult.Error == "" {
			t.Fatal("repair tool result Error = empty, want interruption detail")
		}

		page, err := h.manager.TranscriptPage(testutil.Context(t), meta.ID, transcript.PageQuery{})
		if err != nil {
			t.Fatalf("TranscriptPage(repaired) error = %v", err)
		}
		messages := transcript.MessagesFromEntries(page.Entries)
		assertTranscriptHasDonePart(t, messages)
	})

	t.Run("ShouldPreservePartialAssistantTranscriptWhenRepairingInterruptedTextTurn", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		meta := repairSessionMeta("sess-repair-partial-text", store.StopAgentCrashed, h.workspaceID)
		seedRepairSession(t, h, meta, interruptedTextTurnEvents(t, meta.ID, meta.AgentName)...)

		result, err := h.manager.RepairSession(testutil.Context(t), RepairOpts{SessionID: meta.ID})
		if err != nil {
			t.Fatalf("RepairSession() error = %v", err)
		}
		if !result.Persisted {
			t.Fatal("RepairSession().Persisted = false, want true")
		}

		page, err := h.manager.TranscriptPage(testutil.Context(t), meta.ID, transcript.PageQuery{})
		if err != nil {
			t.Fatalf("TranscriptPage(repaired partial text) error = %v", err)
		}
		messages := transcript.MessagesFromEntries(page.Entries)
		if got := transcript.JoinUIMessageText(messages); !strings.Contains(got, "partial before crash") {
			t.Fatalf("Transcript(repaired partial text) text = %q, want partial assistant evidence", got)
		}
		assertTranscriptHasDonePart(t, messages)
	})

	t.Run("ShouldAppendInterruptedToolResultWithoutDuplicatingTerminalEvent", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		meta := repairSessionMeta("sess-repair-terminal-present", store.StopAgentCrashed, h.workspaceID)
		events := append(
			interruptedTurnEvents(t, meta.ID, meta.AgentName),
			repairStoredEvent(
				t,
				meta.ID,
				meta.AgentName,
				acp.EventTypeError,
				"turn-1",
				time.Date(2026, 4, 28, 13, 0, 3, 0, time.UTC),
				acp.AgentEvent{
					Type:  acp.EventTypeError,
					Error: repairTerminalErrorMessage,
				},
			),
		)
		seedRepairSession(t, h, meta, events...)

		result, err := h.manager.RepairSession(testutil.Context(t), RepairOpts{SessionID: meta.ID})
		if err != nil {
			t.Fatalf("RepairSession(terminal present) error = %v", err)
		}
		if !result.Persisted {
			t.Fatal("RepairSession(terminal present).Persisted = false, want true")
		}
		if !repairIssuesContain(result.Issues, RepairIssueTerminalEventAlreadyExists) {
			t.Fatalf(
				"RepairSession(terminal present) issues = %#v, want %q",
				result.Issues,
				RepairIssueTerminalEventAlreadyExists,
			)
		}
		if got, want := len(result.Actions), 1; got != want {
			t.Fatalf("RepairSession(terminal present) actions = %d, want %d", got, want)
		}
		if got, want := result.Actions[0].Code, RepairActionAppendInterruptedToolResult; got != want {
			t.Fatalf("RepairSession(terminal present) action code = %q, want %q", got, want)
		}

		storedEvents := readRepairEvents(t, h, meta.ID)
		if got, want := len(storedEvents), 5; got != want {
			t.Fatalf("stored events after repair with terminal = %d, want %d", got, want)
		}
		if got, want := storedEvents[4].Type, acp.EventTypeToolResult; got != want {
			t.Fatalf("storedEvents[4].Type = %q, want %q", got, want)
		}
		errorCount := 0
		for _, event := range storedEvents {
			if event.Type == acp.EventTypeError {
				errorCount++
			}
		}
		if got, want := errorCount, 1; got != want {
			t.Fatalf("stored error events = %d, want %d", got, want)
		}
	})

	t.Run("ShouldReportInvalidEventJSONWithoutMutating", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		meta := repairSessionMeta("sess-repair-invalid-json", store.StopAgentCrashed, h.workspaceID)
		seedRepairSession(t, h, meta, store.SessionEvent{
			TurnID:    "turn-1",
			Type:      acp.EventTypeAgentMessage,
			AgentName: meta.AgentName,
			Content:   "{",
			Timestamp: time.Date(2026, 4, 28, 13, 0, 0, 0, time.UTC),
		})

		result, err := h.manager.RepairSession(testutil.Context(t), RepairOpts{SessionID: meta.ID})
		if err != nil {
			t.Fatalf("RepairSession(invalid JSON) error = %v", err)
		}
		if result.Persisted {
			t.Fatal("RepairSession(invalid JSON).Persisted = true, want false")
		}
		if !repairIssuesContain(result.Issues, RepairIssueInvalidEventJSON) {
			t.Fatalf("RepairSession(invalid JSON) issues = %#v, want %q", result.Issues, RepairIssueInvalidEventJSON)
		}

		events := readRepairEvents(t, h, meta.ID)
		if got, want := len(events), 1; got != want {
			t.Fatalf("stored events after invalid JSON repair = %d, want %d", got, want)
		}
	})

	t.Run("ShouldRequireForceForStoppedNonCrashSession", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		meta := repairSessionMeta("sess-repair-force", store.StopCompleted, h.workspaceID)
		seedRepairSession(t, h, meta, interruptedTurnEvents(t, meta.ID, meta.AgentName)...)

		blocked, err := h.manager.RepairSession(testutil.Context(t), RepairOpts{SessionID: meta.ID})
		if err != nil {
			t.Fatalf("RepairSession(non-force) error = %v", err)
		}
		if blocked.Persisted {
			t.Fatal("RepairSession(non-force).Persisted = true, want false")
		}
		if !repairIssuesContain(blocked.Issues, RepairIssueStopReasonRequiresForce) {
			t.Fatalf("RepairSession(non-force) issues = %#v, want force issue", blocked.Issues)
		}

		forced, err := h.manager.RepairSession(testutil.Context(t), RepairOpts{
			SessionID: meta.ID,
			Force:     true,
		})
		if err != nil {
			t.Fatalf("RepairSession(force) error = %v", err)
		}
		if !forced.Persisted {
			t.Fatal("RepairSession(force).Persisted = false, want true")
		}
		if got, want := len(forced.Actions), 2; got != want {
			t.Fatalf("RepairSession(force) actions = %d, want %d", got, want)
		}
	})
}

func repairSessionMeta(id string, reason store.StopReason, workspaceID string) store.SessionMeta {
	now := time.Date(2026, 4, 28, 13, 0, 0, 0, time.UTC)
	return store.SessionMeta{
		ID:                   id,
		Name:                 id,
		AgentName:            "coder",
		WorkspaceID:          workspaceID,
		NetworkParticipation: testLocalParticipationPtr(),
		State:                string(StateStopped),
		StopReason:           &reason,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
}

func interruptedTurnEvents(t *testing.T, sessionID string, agentName string) []store.SessionEvent {
	t.Helper()

	base := time.Date(2026, 4, 28, 13, 0, 0, 0, time.UTC)
	return []store.SessionEvent{
		repairStoredEvent(t, sessionID, agentName, acp.EventTypeUserMessage, "turn-1", base, acp.AgentEvent{
			Type: acp.EventTypeUserMessage,
			Text: "run pwd",
		}),
		repairStoredEvent(
			t,
			sessionID,
			agentName,
			acp.EventTypeAgentMessage,
			"turn-1",
			base.Add(time.Second),
			acp.AgentEvent{
				Type: acp.EventTypeAgentMessage,
				Text: "Running command",
			},
		),
		repairStoredEvent(
			t,
			sessionID,
			agentName,
			acp.EventTypeToolCall,
			"turn-1",
			base.Add(2*time.Second),
			acp.AgentEvent{
				Type:       acp.EventTypeToolCall,
				Title:      "Bash",
				ToolCallID: "tool-1",
				Raw:        []byte(`{"rawInput":{"command":"pwd"},"_meta":{"claudeCode":{"toolName":"Bash"}}}`),
			},
		),
	}
}

func interruptedTextTurnEvents(t *testing.T, sessionID string, agentName string) []store.SessionEvent {
	t.Helper()

	base := time.Date(2026, 4, 28, 14, 0, 0, 0, time.UTC)
	return []store.SessionEvent{
		repairStoredEvent(t, sessionID, agentName, acp.EventTypeUserMessage, "turn-text", base, acp.AgentEvent{
			Type: acp.EventTypeUserMessage,
			Text: "trigger crash mid-stream",
		}),
		repairStoredEvent(
			t,
			sessionID,
			agentName,
			acp.EventTypeAgentMessage,
			"turn-text",
			base.Add(time.Second),
			acp.AgentEvent{
				Type: acp.EventTypeAgentMessage,
				Text: "partial before crash",
			},
		),
	}
}

func repairStoredEvent(
	t *testing.T,
	sessionID string,
	agentName string,
	eventType string,
	turnID string,
	timestamp time.Time,
	event acp.AgentEvent,
) store.SessionEvent {
	t.Helper()

	event.SessionID = sessionID
	event.TurnID = turnID
	event.Timestamp = timestamp
	payload, err := transcript.MarshalAgentEvent(event)
	if err != nil {
		t.Fatalf("MarshalAgentEvent(%s) error = %v", eventType, err)
	}
	return store.SessionEvent{
		TurnID:    turnID,
		Type:      eventType,
		AgentName: agentName,
		Content:   payload,
		Timestamp: timestamp,
	}
}

func seedRepairSession(t *testing.T, h *harness, meta store.SessionMeta, events ...store.SessionEvent) {
	t.Helper()

	sessionDir := filepath.Join(h.homePaths.SessionsDir, meta.ID)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(sessionDir) error = %v", err)
	}
	if err := store.WriteSessionMeta(store.SessionMetaFile(sessionDir), meta); err != nil {
		t.Fatalf("WriteSessionMeta() error = %v", err)
	}

	recorder, err := sessiondb.OpenSessionDB(testutil.Context(t), meta.ID, store.SessionDBFile(sessionDir))
	if err != nil {
		t.Fatalf("OpenSessionDB() error = %v", err)
	}
	for _, event := range events {
		if err := recorder.Record(testutil.Context(t), event); err != nil {
			t.Fatalf("Record(%s) error = %v", event.Type, err)
		}
	}
	if err := recorder.Close(testutil.Context(t)); err != nil {
		t.Fatalf("Close(seed recorder) error = %v", err)
	}
}

func readRepairEvents(t *testing.T, h *harness, sessionID string) []store.SessionEvent {
	t.Helper()

	dbPath := store.SessionDBFile(filepath.Join(h.homePaths.SessionsDir, sessionID))
	recorder, err := sessiondb.OpenSessionDB(testutil.Context(t), sessionID, dbPath)
	if err != nil {
		t.Fatalf("OpenSessionDB(read) error = %v", err)
	}
	defer func() {
		if err := recorder.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(read recorder) error = %v", err)
		}
	}()

	events, err := recorder.Query(testutil.Context(t), store.EventQuery{})
	if err != nil {
		t.Fatalf("Query(read) error = %v", err)
	}
	return events
}

func repairIssuesContain(issues []RepairIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func assertTranscriptHasDonePart(t *testing.T, messages []transcript.UIMessage) {
	t.Helper()

	for _, message := range messages {
		for _, part := range message.Parts {
			if part.State == "done" {
				return
			}
		}
	}
	t.Fatalf("Transcript() = %#v, want at least one done part after repair", messages)
}
